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
)

const (
	// BaseURL is the GLEIF API base URL.
	BaseURL = "https://api.gleif.org/api/v1"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// LEI format: 20 alphanumeric characters.
	LEIPattern = `^[A-Z0-9]{4}[A-Z0-9]{2}[A-Z0-9]{12}[0-9]{2}$`
)

var leiRegex = regexp.MustCompile(LEIPattern)

// Config holds client configuration.
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL: BaseURL,
		Timeout: DefaultTimeout,
	}
}

// Client is a GLEIF API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// NewClient creates a new GLEIF client.
func NewClient(config Config, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: config.Timeout},
		baseURL:    config.BaseURL,
		logger:     logger,
	}
}

// GetLEI retrieves a single LEI record by LEI code.
func (c *Client) GetLEI(ctx context.Context, lei string) (*LEIRecord, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, fmt.Errorf("invalid LEI format: %s", lei)
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s", c.baseURL, lei)
	c.logger.Debug("Fetching LEI", "lei", lei, "url", reqURL)

	var resp SingleResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	record := resp.Data.Attributes
	record.LEI = resp.Data.ID
	return &record, nil
}

// SearchEntities searches for entities by name.
func (c *Client) SearchEntities(ctx context.Context, query string, limit int) ([]LEIRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	params := url.Values{}
	params.Set("filter[entity.legalName]", query)
	params.Set("page[size]", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching entities", "query", query, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, nil
}

// FuzzySearch performs fuzzy matching on entity names.
func (c *Client) FuzzySearch(ctx context.Context, query string, limit int) ([]LEIRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	params := url.Values{}
	params.Set("filter[fulltext]", query)
	params.Set("page[size]", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Fuzzy searching", "query", query, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, nil
}

// SearchByBIC finds LEI records by BIC/SWIFT code.
func (c *Client) SearchByBIC(ctx context.Context, bic string) ([]LEIRecord, error) {
	bic = strings.ToUpper(strings.TrimSpace(bic))
	if len(bic) != 8 && len(bic) != 11 {
		return nil, fmt.Errorf("invalid BIC format: must be 8 or 11 characters")
	}

	params := url.Values{}
	params.Set("filter[bic]", bic)

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by BIC", "bic", bic, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, nil
}

// SearchByISIN finds LEI records associated with an ISIN.
func (c *Client) SearchByISIN(ctx context.Context, isin string) ([]LEIRecord, error) {
	isin = strings.ToUpper(strings.TrimSpace(isin))
	if len(isin) != 12 {
		return nil, fmt.Errorf("invalid ISIN format: must be 12 characters")
	}

	// GLEIF uses the ISIN-LEI mapping endpoint
	reqURL := fmt.Sprintf("%s/lei-issuer?filter[isin]=%s", c.baseURL, isin)
	c.logger.Debug("Searching by ISIN", "isin", isin, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		// Try alternative approach - search in lei-records with ISIN filter
		params := url.Values{}
		params.Set("filter[isin]", isin)
		reqURL = fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())

		if err2 := c.doRequest(ctx, reqURL, &resp); err2 != nil {
			return nil, fmt.Errorf("ISIN lookup failed: %v", err)
		}
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, nil
}

// GetRelationships retrieves parent/child relationships for an LEI.
func (c *Client) GetRelationships(ctx context.Context, lei string, relType string) ([]Relationship, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, fmt.Errorf("invalid LEI format: %s", lei)
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
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		// Check if it's a single response (for parent relationships)
		type SingleRelResponse struct {
			Data RelData `json:"data"`
		}
		var singleResp SingleRelResponse
		if err2 := c.doRequest(ctx, reqURL, &singleResp); err2 != nil {
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

	result := &ValidationResult{LEI: lei}

	// First check format
	if !leiRegex.MatchString(lei) {
		result.Valid = false
		result.Message = "Invalid LEI format"
		return result, nil
	}

	// Validate check digits (ISO 17442)
	if !validateLEICheckDigits(lei) {
		result.Valid = false
		result.Message = "Invalid check digits"
		return result, nil
	}

	// Try to fetch the record
	record, err := c.GetLEI(ctx, lei)
	if err != nil {
		result.Valid = false
		result.Message = "LEI not found in GLEIF database"
		return result, nil
	}

	result.Valid = true
	result.Status = record.Registration.Status
	result.EntityStatus = record.Entity.Status
	result.NextRenewal = record.Registration.NextRenewalDate.Format("2006-01-02")
	result.Message = fmt.Sprintf("Valid LEI, registration status: %s", record.Registration.Status)

	return result, nil
}

// Autocomplete returns entity name suggestions.
func (c *Client) Autocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	params := url.Values{}
	params.Set("q", prefix)

	reqURL := fmt.Sprintf("%s/autocompletions?%s", c.baseURL, params.Encode())
	c.logger.Debug("Autocomplete", "prefix", prefix, "url", reqURL)

	var resp AutocompleteResponse
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		// Fallback to fuzzy search if autocomplete endpoint doesn't work
		records, err := c.FuzzySearch(ctx, prefix, limit)
		if err != nil {
			return nil, err
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

	if len(resp.Data) > limit {
		resp.Data = resp.Data[:limit]
	}
	return resp.Data, nil
}

// SearchByCountry finds entities in a specific country.
func (c *Client) SearchByCountry(ctx context.Context, country string, limit int) ([]LEIRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	country = strings.ToUpper(strings.TrimSpace(country))
	if len(country) != 2 {
		return nil, fmt.Errorf("country must be a 2-letter ISO code")
	}

	params := url.Values{}
	params.Set("filter[entity.legalAddress.country]", country)
	params.Set("page[size]", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by country", "country", country, "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequest(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := make([]LEIRecord, len(resp.Data))
	for i, item := range resp.Data {
		records[i] = item.Attributes
		records[i].LEI = item.ID
	}
	return records, nil
}

// doRequest executes an HTTP GET request and decodes the JSON response.
func (c *Client) doRequest(ctx context.Context, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("User-Agent", "gleif-mcp-server/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
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
			numStr.WriteString(fmt.Sprintf("%d", ch-'A'+10))
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
