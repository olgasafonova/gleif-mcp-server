package gleif

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

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
		},
		baseURL: config.BaseURL,
		logger:  logger,
		limiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstSize),
		cache:   cache,
		config:  config,
	}
}

// GetLEI retrieves a single LEI record by LEI code.
func (c *Client) GetLEI(ctx context.Context, lei string) (*LEIRecord, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	// Check cache first
	if record, ok := c.cache.GetLEI(lei); ok {
		c.logger.Debug("Cache hit for LEI", "lei", lei)
		return record, nil
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s", c.baseURL, lei)
	c.logger.Debug("Fetching LEI", "lei", lei, "url", reqURL)

	var resp SingleResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	record := resp.Data.Attributes
	record.LEI = resp.Data.ID

	// Cache the result
	c.cache.SetLEI(lei, &record)

	return &record, nil
}

// GetBatchLEI retrieves multiple LEI records in one request.
func (c *Client) GetBatchLEI(ctx context.Context, leis []string) ([]LEIRecord, error) {
	if len(leis) == 0 {
		return nil, NewInvalidFormatError("leis", "at least one LEI required")
	}
	if len(leis) > 100 {
		return nil, NewInvalidFormatError("leis", "maximum 100 LEIs per request")
	}

	// Validate and normalize all LEIs
	normalized := make([]string, len(leis))
	var cached []LEIRecord
	var toFetch []string

	for i, lei := range leis {
		lei = strings.ToUpper(strings.TrimSpace(lei))
		if !leiRegex.MatchString(lei) {
			return nil, NewInvalidFormatError("LEI", fmt.Sprintf("invalid LEI at position %d: %s", i, lei))
		}
		normalized[i] = lei

		// Check cache
		if record, ok := c.cache.GetLEI(lei); ok {
			cached = append(cached, *record)
		} else {
			toFetch = append(toFetch, lei)
		}
	}

	// If all cached, return
	if len(toFetch) == 0 {
		c.logger.Debug("All LEIs found in cache", "count", len(cached))
		return cached, nil
	}

	// Fetch remaining
	params := url.Values{}
	params.Set("filter[lei]", strings.Join(toFetch, ","))
	params.Set("page[size]", "100")

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Batch fetching LEIs", "count", len(toFetch), "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	results := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		results[i] = item.Attributes
		results[i].LEI = item.ID
		c.cache.SetLEI(item.ID, &results[i])
	}

	// Combine cached and fetched
	return append(cached, results...), nil
}

// SearchEntities searches for entities by name with pagination.
func (c *Client) SearchEntities(ctx context.Context, query string, limit, page int) ([]LEIRecord, *Pagination, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	params := url.Values{}
	params.Set("filter[entity.legalName]", query)
	params.Set("page[size]", fmt.Sprintf("%d", limit))
	params.Set("page[number]", fmt.Sprintf("%d", page))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching entities", "query", query, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, &resp.Meta.Pagination, nil
}

// FuzzySearch performs fuzzy matching on entity names with pagination.
func (c *Client) FuzzySearch(ctx context.Context, query string, limit, page int) ([]LEIRecord, *Pagination, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	// Check cache (only for first page)
	cacheKey := fmt.Sprintf("fuzzy:%s:%d:%d", query, limit, page)
	if page == 1 {
		if records, ok := c.cache.GetSearch(cacheKey); ok {
			c.logger.Debug("Cache hit for fuzzy search", "query", query)
			return records, nil, nil
		}
	}

	params := url.Values{}
	params.Set("filter[fulltext]", query)
	params.Set("page[size]", fmt.Sprintf("%d", limit))
	params.Set("page[number]", fmt.Sprintf("%d", page))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Fuzzy searching", "query", query, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}

	// Cache first page results
	if page == 1 {
		c.cache.SetSearch(cacheKey, records)
	}
	return records, &resp.Meta.Pagination, nil
}

// SearchByBIC finds LEI records by BIC/SWIFT code.
func (c *Client) SearchByBIC(ctx context.Context, bic string) ([]LEIRecord, error) {
	bic = strings.ToUpper(strings.TrimSpace(bic))
	if !bicRegex.MatchString(bic) {
		return nil, NewInvalidFormatError("BIC", "must be 8 or 11 characters: 4 letters + 2 letters + 2 alphanumeric, optional 3 alphanumeric")
	}

	// Check cache
	cacheKey := fmt.Sprintf("bic:%s", bic)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	params := url.Values{}
	params.Set("filter[bic]", bic)

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by BIC", "bic", bic, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}

	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// SearchByISIN finds LEI records associated with an ISIN.
func (c *Client) SearchByISIN(ctx context.Context, isin string) ([]LEIRecord, error) {
	isin = strings.ToUpper(strings.TrimSpace(isin))
	if !isinRegex.MatchString(isin) {
		return nil, NewInvalidFormatError("ISIN", "must be 2 letters followed by 10 alphanumeric characters (12 total)")
	}

	// Check cache
	cacheKey := fmt.Sprintf("isin:%s", isin)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	// GLEIF uses the ISIN-LEI mapping endpoint. Build the URL via url.Values
	// rather than raw fmt.Sprintf — an unvalidated value containing `&` could
	// otherwise smuggle an additional query parameter past the intended one.
	primaryParams := url.Values{}
	primaryParams.Set("filter[isin]", isin)
	reqURL := fmt.Sprintf("%s/lei-issuer?%s", c.baseURL, primaryParams.Encode())
	c.logger.Debug("Searching by ISIN", "isin", isin, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		// Try alternative approach - search in lei-records with ISIN filter
		params := url.Values{}
		params.Set("filter[isin]", isin)
		reqURL = fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())

		if err2 := c.doRequestWithRetry(ctx, reqURL, &resp); err2 != nil {
			return nil, fmt.Errorf("ISIN lookup failed: %v", err)
		}
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}

	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// GetRelationships retrieves parent/child relationships for an LEI.
func (c *Client) GetRelationships(ctx context.Context, lei string, relType string) ([]Relationship, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	// Build relationship URL
	var endpoint string
	switch strings.ToLower(relType) {
	case "direct-parent", "parent":
		endpoint = "direct-parent-relationship"
	case "ultimate-parent", "ultimate":
		endpoint = "ultimate-parent-relationship"
	case "direct-children", "children":
		endpoint = "direct-child-relationships"
	case "fund-manager":
		endpoint = "fund-manager-relationship"
	case "umbrella-fund":
		endpoint = "umbrella-fund-relationship"
	case "sub-funds":
		endpoint = "sub-fund-relationships"
	default:
		// Get all relationships
		endpoint = "direct-parent-relationship"
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s/%s", c.baseURL, lei, endpoint)
	c.logger.Debug("Fetching relationships", "lei", lei, "type", relType, "url", reqURL)

	// Relationship responses have a different structure
	type RelData struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Relationship Relationship `json:"relationship"`
		} `json:"attributes"`
	}
	type RelResponse struct {
		Data []RelData `json:"data"`
	}

	var resp RelResponse
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		// Check if it's a single response (for parent relationships)
		type SingleRelResponse struct {
			Data RelData `json:"data"`
		}
		var singleResp SingleRelResponse
		if err2 := c.doRequestWithRetry(ctx, reqURL, &singleResp); err2 != nil {
			return nil, err
		}
		if singleResp.Data.ID != "" {
			return []Relationship{singleResp.Data.Attributes.Relationship}, nil
		}
		return nil, err
	}

	rels := make([]Relationship, len(resp.Data))
	for i, item := range resp.Data {
		rels[i] = item.Attributes.Relationship
	}
	return rels, nil
}

// ValidateLEI checks if an LEI is valid and returns its status.
func (c *Client) ValidateLEI(ctx context.Context, lei string) (*ValidationResult, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))

	// Check cache first
	if result, ok := c.cache.GetValidation(lei); ok {
		return result, nil
	}

	result := &ValidationResult{LEI: lei}

	// First check format
	if !leiRegex.MatchString(lei) {
		result.Valid = false
		result.Message = "Invalid LEI format: must be 20 alphanumeric characters"
		return result, nil
	}

	// Validate check digits (ISO 17442)
	if !validateLEICheckDigits(lei) {
		result.Valid = false
		result.Message = "Invalid check digits: LEI fails ISO 17442 validation"
		return result, nil
	}

	// Try to fetch the record
	record, err := c.GetLEI(ctx, lei)
	if err != nil {
		result.Valid = false
		result.Message = "LEI not found in GLEIF database"
		c.cache.SetValidation(lei, result)
		return result, nil
	}

	result.Valid = true
	result.Status = record.Registration.Status
	result.EntityStatus = record.Entity.Status
	result.NextRenewal = record.Registration.NextRenewalDate.Format("2006-01-02")
	result.Message = fmt.Sprintf("Valid LEI, registration status: %s", record.Registration.Status)

	c.cache.SetValidation(lei, result)
	return result, nil
}

// Autocomplete returns entity name suggestions.
func (c *Client) Autocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	// Check cache
	if results, ok := c.cache.GetAutocomplete(prefix); ok {
		if len(results) > limit {
			return results[:limit], nil
		}
		return results, nil
	}

	params := url.Values{}
	params.Set("q", prefix)

	reqURL := fmt.Sprintf("%s/autocompletions?%s", c.baseURL, params.Encode())
	c.logger.Debug("Autocomplete", "prefix", prefix, "url", reqURL)

	var resp AutocompleteResponse
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		// Fallback to fuzzy search if autocomplete endpoint doesn't work
		records, _, searchErr := c.FuzzySearch(ctx, prefix, limit, 1)
		if searchErr != nil {
			return nil, searchErr
		}
		results := make([]AutocompleteResult, len(records))
		for i, r := range records {
			results[i] = AutocompleteResult{
				LEI:       r.LEI,
				LegalName: r.Entity.LegalName.Name,
				Country:   r.Entity.LegalAddress.Country,
			}
		}
		return results, nil
	}

	c.cache.SetAutocomplete(prefix, resp.Data)

	if len(resp.Data) > limit {
		return resp.Data[:limit], nil
	}
	return resp.Data, nil
}

// SearchByCountry finds entities in a specific country.
func (c *Client) SearchByCountry(ctx context.Context, country string, limit int) ([]LEIRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	country = strings.ToUpper(strings.TrimSpace(country))
	if !countryRegex.MatchString(country) {
		return nil, NewInvalidFormatError("country", "must be a 2-letter ISO 3166-1 alpha-2 code")
	}

	// Check cache
	cacheKey := fmt.Sprintf("country:%s:%d", country, limit)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	params := url.Values{}
	params.Set("filter[entity.legalAddress.country]", country)
	params.Set("page[size]", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by country", "country", country, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}

	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// GetLEIIssuer retrieves information about an LEI issuer (LOU).
func (c *Client) GetLEIIssuer(ctx context.Context, issuerID string) (*LEIIssuer, error) {
	if err := ValidateIssuerID(issuerID); err != nil {
		return nil, err
	}
	issuerID = strings.ToUpper(strings.TrimSpace(issuerID))
	// url.PathEscape is a no-op for validator-approved IDs (the regex already
	// restricts to URL-safe chars), but stays as belt-and-braces against
	// future regex loosening.
	reqURL := fmt.Sprintf("%s/lei-issuers/%s", c.baseURL, url.PathEscape(issuerID))
	c.logger.Debug("Fetching LEI issuer", "id", issuerID, "url", reqURL)

	var resp SingleResponse[LEIIssuer]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	issuer := resp.Data.Attributes
	issuer.ID = resp.Data.ID
	return &issuer, nil
}

// ListLEIIssuers lists all LEI issuers (LOUs).
func (c *Client) ListLEIIssuers(ctx context.Context) ([]LEIIssuer, error) {
	reqURL := fmt.Sprintf("%s/lei-issuers", c.baseURL)
	c.logger.Debug("Listing LEI issuers", "url", reqURL)

	var resp APIResponse[LEIIssuer]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	issuers := make([]LEIIssuer, len(resp.Data))
	for i, item := range resp.Data {
		issuers[i] = item.Attributes
		issuers[i].ID = item.ID
	}
	return issuers, nil
}

// GetReportingExceptions retrieves reporting exceptions for an LEI.
func (c *Client) GetReportingExceptions(ctx context.Context, lei string) ([]ReportingException, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s/reporting-exceptions", c.baseURL, lei)
	c.logger.Debug("Fetching reporting exceptions", "lei", lei, "url", reqURL)

	var resp APIResponse[ReportingException]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	exceptions := make([]ReportingException, len(resp.Data))
	for i, item := range resp.Data {
		exceptions[i] = item.Attributes
	}
	return exceptions, nil
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

// validateLEICheckDigits validates the ISO 17442 check digits.
func validateLEICheckDigits(lei string) bool {
	if len(lei) != 20 {
		return false
	}

	// Convert letters to numbers (A=10, B=11, ..., Z=35)
	var numStr strings.Builder
	for _, ch := range lei {
		if ch >= 'A' && ch <= 'Z' {
			fmt.Fprintf(&numStr, "%d", ch-'A'+10)
		} else if ch >= '0' && ch <= '9' {
			numStr.WriteByte(byte(ch))
		} else {
			return false
		}
	}

	// Calculate mod 97 (ISO 7064)
	num := numStr.String()
	remainder := 0
	for _, ch := range num {
		remainder = (remainder*10 + int(ch-'0')) % 97
	}

	return remainder == 1
}
