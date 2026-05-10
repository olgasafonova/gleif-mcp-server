package gleif

import (
	"context"
	"fmt"
	"net/url"
)

// SearchEntities searches for entities by name with pagination.
func (c *Client) SearchEntities(ctx context.Context, query string, limit, page int) ([]LEIRecord, *Pagination, error) {
	limit, page = clampLimitAndPage(limit, page)

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

	return extractLEIRecords(resp), &resp.Meta.Pagination, nil
}

// FuzzySearch performs fuzzy matching on entity names with pagination.
func (c *Client) FuzzySearch(ctx context.Context, query string, limit, page int) ([]LEIRecord, *Pagination, error) {
	limit, page = clampLimitAndPage(limit, page)

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

	records := extractLEIRecords(resp)
	if page == 1 {
		c.cache.SetSearch(cacheKey, records)
	}
	return records, &resp.Meta.Pagination, nil
}

// SearchByBIC finds LEI records by BIC/SWIFT code.
func (c *Client) SearchByBIC(ctx context.Context, bic BIC) ([]LEIRecord, error) {
	cacheKey := "bic:" + string(bic)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	params := url.Values{}
	params.Set("filter[bic]", string(bic))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by BIC", "bic", string(bic), "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := extractLEIRecords(resp)
	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// SearchByISIN finds LEI records associated with an ISIN. GLEIF exposes two
// endpoints (lei-issuer and lei-records) that can answer ISIN lookups; the
// primary returns the issuer mapping, the fallback uses the generic record
// filter when the primary path is unavailable.
func (c *Client) SearchByISIN(ctx context.Context, isin ISIN) ([]LEIRecord, error) {
	cacheKey := "isin:" + string(isin)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	resp, err := c.fetchISIN(ctx, isin)
	if err != nil {
		return nil, err
	}

	records := extractLEIRecords(resp)
	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// fetchISIN tries the primary ISIN endpoint and falls back to the generic
// lei-records filter if the primary path errors.
func (c *Client) fetchISIN(ctx context.Context, isin ISIN) (APIResponse[LEIRecord], error) {
	// Build the URL via url.Values rather than raw fmt.Sprintf — an
	// unvalidated value containing `&` could otherwise smuggle an additional
	// query parameter past the intended one.
	primaryParams := url.Values{}
	primaryParams.Set("filter[isin]", string(isin))
	primaryURL := fmt.Sprintf("%s/lei-issuer?%s", c.baseURL, primaryParams.Encode())
	c.logger.Debug("Searching by ISIN", "isin", string(isin), "url", primaryURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, primaryURL, &resp); err == nil {
		return resp, nil
	}

	fallbackParams := url.Values{}
	fallbackParams.Set("filter[isin]", string(isin))
	fallbackURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, fallbackParams.Encode())
	if err := c.doRequestWithRetry(ctx, fallbackURL, &resp); err != nil {
		return resp, fmt.Errorf("ISIN lookup failed: %w", err)
	}
	return resp, nil
}

// SearchByCountry finds entities in a specific country.
func (c *Client) SearchByCountry(ctx context.Context, country Country, limit int) ([]LEIRecord, error) {
	if limit <= 0 || limit > maxBatchSize {
		limit = 20
	}

	cacheKey := fmt.Sprintf("country:%s:%d", string(country), limit)
	if records, ok := c.cache.GetSearch(cacheKey); ok {
		return records, nil
	}

	params := url.Values{}
	params.Set("filter[entity.legalAddress.country]", string(country))
	params.Set("page[size]", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/lei-records?%s", c.baseURL, params.Encode())
	c.logger.Debug("Searching by country", "country", string(country), "url", reqURL)

	var resp APIResponse[LEIRecord]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	records := extractLEIRecords(resp)
	c.cache.SetSearch(cacheKey, records)
	return records, nil
}

// Autocomplete returns entity name suggestions.
func (c *Client) Autocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	if results, ok := c.cache.GetAutocomplete(prefix); ok {
		return applyLimit(results, limit), nil
	}

	params := url.Values{}
	params.Set("q", prefix)
	reqURL := fmt.Sprintf("%s/autocompletions?%s", c.baseURL, params.Encode())
	c.logger.Debug("Autocomplete", "prefix", prefix, "url", reqURL)

	var resp AutocompleteResponse
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return c.fuzzyAutocompleteFallback(ctx, prefix, limit)
	}

	c.cache.SetAutocomplete(prefix, resp.Data)
	return applyLimit(resp.Data, limit), nil
}

// fuzzyAutocompleteFallback runs a fuzzy search and reshapes the records into
// autocomplete suggestions when the primary autocomplete endpoint errors.
func (c *Client) fuzzyAutocompleteFallback(ctx context.Context, prefix string, limit int) ([]AutocompleteResult, error) {
	records, _, err := c.FuzzySearch(ctx, prefix, limit, 1)
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

// clampLimitAndPage normalizes pagination inputs. Limits above 100 or below 1
// fall back to the default 20; non-positive page numbers fall back to 1.
func clampLimitAndPage(limit, page int) (int, int) {
	if limit <= 0 || limit > maxBatchSize {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return limit, page
}

// applyLimit returns the first n elements of s, or all of s if it is shorter.
// Centralizes the slice-or-return pattern that appeared in two cache-hit paths.
func applyLimit[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
