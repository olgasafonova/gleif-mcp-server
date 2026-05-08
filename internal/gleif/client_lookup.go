package gleif

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetLEI retrieves a single LEI record by LEI code.
func (c *Client) GetLEI(ctx context.Context, lei string) (*LEIRecord, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	// Cache hit fast-path (no singleflight overhead on the warm path).
	if record, ok := c.cache.GetLEI(lei); ok {
		c.logger.Debug("Cache hit for LEI", "lei", lei)
		return record, nil
	}

	// Cold path: collapse N concurrent callers for the same LEI into a
	// single upstream fetch. The leader does the GLEIF round-trip and
	// populates the cache; followers receive the leader's result without
	// consuming a rate-limit slot of their own.
	v, err, _ := c.sfGroup.Do("lei:"+lei, func() (any, error) {
		// Re-check cache inside the singleflight: a previous leader for
		// this same key may have populated it between our outer miss and
		// our turn here. (Cache hit paths return a cloned record per the
		// cache_isolation fix; that contract holds here too.)
		if record, ok := c.cache.GetLEI(lei); ok {
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

		c.cache.SetLEI(lei, &record)
		return &record, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*LEIRecord), nil
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
