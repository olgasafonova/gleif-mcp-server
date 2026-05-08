package gleif

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

const (
	// BaseURL is the GLEIF API base URL.
	BaseURL = "https://api.gleif.org/api/v1"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// LEI format: 20 alphanumeric characters.
	LEIPattern = `^[A-Z0-9]{4}[A-Z0-9]{2}[A-Z0-9]{12}[0-9]{2}$`

	// ISIN format: 2-letter country code + 9 alphanumeric + 1 check digit (12 total).
	// Matches the Pattern declared in tools/definitions.go for search_by_isin.
	ISINPattern = `^[A-Z]{2}[A-Z0-9]{10}$`

	// BIC format: 8 or 11 characters (matches search_by_bic tool spec Pattern).
	BICPattern = `^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?$`

	// CountryPattern: ISO 3166-1 alpha-2.
	CountryPattern = `^[A-Z]{2}$`

	// IssuerIDPattern: LEI issuer (LOU) ID. GLEIF issues these as 20-character
	// alphanumeric (often the LEI of the issuing organization), but historic
	// IDs can be shorter alphanumeric tokens. Allow 4-32 alphanumeric chars to
	// cover both shapes without admitting URL-pivot characters.
	IssuerIDPattern = `^[A-Z0-9]{4,32}$`

	// Rate limit: GLEIF allows 60/min, we use 50 to be safe.
	DefaultRateLimit = 50.0 / 60.0 // ~0.83 requests per second
	DefaultBurstSize = 10
)

var (
	leiRegex      = regexp.MustCompile(LEIPattern)
	isinRegex     = regexp.MustCompile(ISINPattern)
	bicRegex      = regexp.MustCompile(BICPattern)
	countryRegex  = regexp.MustCompile(CountryPattern)
	issuerIDRegex = regexp.MustCompile(IssuerIDPattern)
)

// ValidateIssuerID checks that an LEI issuer (LOU) ID is well-formed and
// safe for URL-path interpolation. Without this check, an adversarial MCP
// caller could send issuer_id="../lei-records/SOMELEI" and pivot the
// request away from /lei-issuers/ to a different GLEIF endpoint.
func ValidateIssuerID(id string) error {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return NewInvalidFormatError("issuer_id", "is required")
	}
	if !issuerIDRegex.MatchString(id) {
		return NewInvalidFormatError("issuer_id", "must be 4-32 alphanumeric characters")
	}
	return nil
}

// Config holds client configuration.
type Config struct {
	BaseURL     string
	Timeout     time.Duration
	RateLimit   float64       // Requests per second
	BurstSize   int           // Burst size for rate limiter
	MaxRetries  int           // Max retry attempts
	RetryDelay  time.Duration // Initial retry delay
	EnableCache bool
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
	}
}

// Client is a GLEIF API client with rate limiting, retries, and caching.
type Client struct {
	httpClient *http.Client
	baseURL    string
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
			// GLEIF's API does not redirect under normal operation, so any
			// 3xx is either misconfiguration or an SSRF attempt. Returning
			// http.ErrUseLastResponse short-circuits the redirect and
			// surfaces the 3xx to the caller as an error.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL: config.BaseURL,
		logger:  logger,
		limiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstSize),
		cache:   cache,
		config:  config,
	}
}

// doRequestWithRetry executes a request with rate limiting and retry logic.
func (c *Client) doRequestWithRetry(ctx context.Context, url string, result any) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Wait for rate limiter
		if err := c.limiter.Wait(ctx); err != nil {
			return NewRateLimitError(60)
		}

		err := c.doRequest(ctx, url, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if retryable
		if !IsRetryable(err) {
			return err
		}

		// Exponential backoff
		if attempt < c.config.MaxRetries {
			delay := c.config.RetryDelay * time.Duration(1<<attempt)
			c.logger.Debug("Retrying request", "attempt", attempt+1, "delay", delay, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return lastErr
}

// doRequest executes an HTTP GET request and decodes the JSON response.
func (c *Client) doRequest(ctx context.Context, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return NewNetworkError(err)
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "gleif-mcp-server/0.2.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return NewTimeoutError()
		}
		return NewNetworkError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewNetworkError(err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// Success
	case http.StatusNotFound:
		return NewNotFoundError("LEI record")
	case http.StatusTooManyRequests:
		return NewRateLimitError(60)
	default:
		// Don't leak raw response bodies into the MCP caller's error string.
		// HTML/CDN error pages, MITM proxy responses, and prompt-injection
		// payloads in echoed input are all blocked at this gate. Operators
		// can still recover the body via debug-level request logging; only
		// the MCP caller's error string is sanitized. See HG-2 in
		// rules/code-review-prompts.md.
		if resp.StatusCode >= 500 {
			return NewServerError(resp.StatusCode, "")
		}
		return &APIError{
			Code:       ErrCodeServerError,
			Message:    http.StatusText(resp.StatusCode),
			StatusCode: resp.StatusCode,
			Retryable:  false,
		}
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// CacheStats returns cache performance statistics.
func (c *Client) CacheStats() CacheStats {
	return c.cache.Stats()
}

// ClearCache clears all cached data.
func (c *Client) ClearCache() {
	c.cache.Clear()
}
