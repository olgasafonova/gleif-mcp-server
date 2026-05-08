package gleif

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

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
