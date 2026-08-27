package gleif

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

const (
	// BaseURL is the GLEIF API base URL.
	BaseURL = "https://api.gleif.org/api/v1"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// Rate limit: GLEIF allows 60/min, we use 50 to be safe.
	DefaultRateLimit = 50.0 / 60.0 // ~0.83 requests per second
	DefaultBurstSize = 10

	// maxBatchSize is the upper bound on a single batch lookup.
	maxBatchSize = 100

	// defaultUserAgent is the fallback User-Agent when Config.UserAgent is
	// empty. Release builds pass the stamped version through Config.UserAgent
	// (see main.ServerVersion); "dev" marks an unstamped build.
	defaultUserAgent = "gleif-mcp-server/dev"
)

// Config holds client configuration.
type Config struct {
	BaseURL     string
	Timeout     time.Duration
	RateLimit   float64       // Requests per second
	BurstSize   int           // Burst size for rate limiter
	MaxRetries  int           // Max retry attempts
	RetryDelay  time.Duration // Initial retry delay
	EnableCache bool
	UserAgent   string // User-Agent header for GLEIF API requests; defaults to defaultUserAgent when empty
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL:     BaseURL,
		Timeout:     DefaultTimeout,
		RateLimit:   DefaultRateLimit,
		BurstSize:   DefaultBurstSize,
		MaxRetries:  3,
		RetryDelay:  time.Second,
		EnableCache: true,
		UserAgent:   defaultUserAgent,
	}
}

// Client is a GLEIF API client with rate limiting, retries, and caching.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	logger     *slog.Logger
	limiter    *rate.Limiter
	cache      *Cache
	config     Config
	// sfGroup collapses N concurrent calls for the same uncached key into
	// a single upstream fetch. Without it, a burst of identical requests
	// during a cold-cache window each consume a slot in the shared rate
	// limiter (50 rpm, burst 10), trivially draining it. Combined with
	// the validation cache logic, that drain is what made the (now-fixed)
	// 24-hour cache-poisoning trigger reachable from a single client.
	// Used today for GetLEI; other cacheable methods may adopt the same
	// pattern in follow-up changes.
	sfGroup singleflight.Group
}

// NewClient creates a new GLEIF client.
func NewClient(config Config, logger *slog.Logger) *Client {
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	cache, _ := NewCache(CacheConfig{
		LEIRecordTTL:    time.Hour,
		ValidationTTL:   24 * time.Hour,
		AutocompleteTTL: 5 * time.Minute,
		SearchTTL:       10 * time.Minute,
		MaxEntries:      10000,
		Enabled:         config.EnableCache,
	})

	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			// SECURITY: Refuse all redirects. The configured BaseURL
			// (api.gleif.org by default, operator-overridable) is the only
			// legitimate target. Without CheckRedirect, Go follows up to 10
			// 3xx responses; a misconfigured deployment combined with a wiki
			// or proxy returning Location: http://169.254.169.254/... would
			// pivot the request to internal IPs (cloud metadata, link-local).
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL:   config.BaseURL,
		userAgent: userAgent,
		logger:    logger,
		limiter:   rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstSize),
		cache:     cache,
		config:    config,
	}
}

// doRequestWithRetry executes a request with rate limiting and retry logic.
func (c *Client) doRequestWithRetry(ctx context.Context, url string, result any) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		done, err := c.tryRequest(ctx, attempt, url, result)
		if done {
			return err
		}
		lastErr = err
	}

	return lastErr
}

// tryRequest performs one attempt of doRequestWithRetry. It returns done=true
// when the caller should stop iterating (either success or a terminal error)
// and done=false plus the underlying retryable error when another iteration
// should run. The retry backoff is handled here so the caller stays linear.
func (c *Client) tryRequest(ctx context.Context, attempt int, url string, result any) (bool, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return true, NewRateLimitError(60)
	}

	err := c.doRequest(ctx, url, result)
	if err == nil {
		return true, nil
	}

	if !IsRetryable(err) {
		return true, err
	}

	if attempt >= c.config.MaxRetries {
		return false, err
	}

	if waitErr := c.waitForRetry(ctx, attempt, err); waitErr != nil {
		return true, waitErr
	}

	return false, err
}

// waitForRetry sleeps with exponential backoff and respects context cancellation.
func (c *Client) waitForRetry(ctx context.Context, attempt int, err error) error {
	delay := c.config.RetryDelay * time.Duration(1<<attempt)
	c.logger.Debug("Retrying request", "attempt", attempt+1, "delay", delay, "error", err)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// doRequest executes an HTTP GET request and decodes the JSON response.
func (c *Client) doRequest(ctx context.Context, url string, result any) error {
	body, statusCode, err := c.executeAndRead(ctx, url)
	if err != nil {
		return err
	}

	if statusErr := mapStatusToError(statusCode); statusErr != nil {
		return statusErr
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// executeAndRead builds the request, executes it, and reads the response body.
// Extracted from doRequest to flatten its conditional structure.
func (c *Client) executeAndRead(ctx context.Context, reqURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, NewNetworkError(err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, NewTimeoutError()
		}
		return nil, 0, NewNetworkError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, NewNetworkError(err)
	}
	return body, resp.StatusCode, nil
}

// mapStatusToError maps an HTTP status code to a sanitized APIError. Returns
// nil for 200 OK. Never includes raw response bodies in the returned error;
// HG-2 in code-review-prompts.md requires sanitization at this boundary so
// HTML/CDN error pages and prompt-injection echoes never reach the MCP caller.
func mapStatusToError(statusCode int) error {
	switch {
	case statusCode == http.StatusOK:
		return nil
	case statusCode == http.StatusNotFound:
		return NewNotFoundError("LEI record")
	case statusCode == http.StatusTooManyRequests:
		return NewRateLimitError(60)
	case statusCode >= 500:
		return NewServerError(statusCode, "")
	default:
		return &APIError{
			Code:       ErrCodeServerError,
			Message:    http.StatusText(statusCode),
			StatusCode: statusCode,
			Retryable:  false,
		}
	}
}

// CacheStats returns cache performance statistics.
func (c *Client) CacheStats() CacheStats {
	return c.cache.Stats()
}

// ClearCache clears all cached data.
func (c *Client) ClearCache() {
	c.cache.Clear()
}

// extractLEIRecords lifts records out of a GLEIF JSON:API envelope, copying
// the LEI from the JSON:API id field into the LEIRecord struct so callers
// see a fully-populated record. Shared between lookup and search paths.
func extractLEIRecords(resp APIResponse[LEIRecord]) []LEIRecord {
	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records
}
