package gleif

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetLEI retrieves a single LEI record by LEI code.
func (c *Client) GetLEI(ctx context.Context, lei LEI) (*LEIRecord, error) {
	key := string(lei)

	// Cache hit fast-path (no singleflight overhead on the warm path).
	if record, ok := c.cache.GetLEI(key); ok {
		c.logger.Debug("Cache hit for LEI", "lei", key)
		return record, nil
	}

	// Cold path: collapse N concurrent callers for the same LEI into a
	// single upstream fetch. The leader does the GLEIF round-trip and
	// populates the cache; followers receive the leader's result without
	// consuming a rate-limit slot of their own.
	v, err, _ := c.sfGroup.Do("lei:"+key, func() (any, error) {
		// Re-check cache inside the singleflight: a previous leader for
		// this same key may have populated it between our outer miss and
		// our turn here.
		if record, ok := c.cache.GetLEI(key); ok {
			return record, nil
		}
		return c.fetchLEIRecord(ctx, key)
	})

	if err != nil {
		return nil, err
	}
	return v.(*LEIRecord), nil
}

// fetchLEIRecord performs the upstream GET for a single LEI and populates
// the cache. Extracted from GetLEI's singleflight closure to keep that
// closure short.
func (c *Client) fetchLEIRecord(ctx context.Context, lei string) (*LEIRecord, error) {
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
}

// GetBatchLEI retrieves multiple LEI records in one request.
func (c *Client) GetBatchLEI(ctx context.Context, leis []LEI) ([]LEIRecord, error) {
	if len(leis) == 0 {
		return nil, NewInvalidFormatError("leis", "at least one LEI required")
	}
	if len(leis) > maxBatchSize {
		return nil, NewInvalidFormatError("leis", fmt.Sprintf("maximum %d LEIs per request", maxBatchSize))
	}

	cached, toFetch := c.partitionCachedLEIs(leis)
	if len(toFetch) == 0 {
		c.logger.Debug("All LEIs found in cache", "count", len(cached))
		return cached, nil
	}

	fetched, err := c.fetchBatchLEIs(ctx, toFetch)
	if err != nil {
		return nil, err
	}
	return append(cached, fetched...), nil
}

// partitionCachedLEIs splits a batch into cached records (returned directly)
// and uncached LEIs that still need an upstream fetch.
func (c *Client) partitionCachedLEIs(leis []LEI) (cached []LEIRecord, toFetch []string) {
	for _, lei := range leis {
		key := string(lei)
		if record, ok := c.cache.GetLEI(key); ok {
			cached = append(cached, *record)
		} else {
			toFetch = append(toFetch, key)
		}
	}
	return cached, toFetch
}

// fetchBatchLEIs performs the upstream batch GET and populates the cache.
func (c *Client) fetchBatchLEIs(ctx context.Context, toFetch []string) ([]LEIRecord, error) {
	params := url.Values{}
	params.Set("filter[lei]", strings.Join(toFetch, ","))
	params.Set("page[size]", fmt.Sprintf("%d", maxBatchSize))

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
	return results, nil
}
