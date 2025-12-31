package gleif

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// CacheConfig configures the cache behavior.
type CacheConfig struct {
	LEIRecordTTL      time.Duration // TTL for LEI record lookups
	ValidationTTL     time.Duration // TTL for validation results
	AutocompleteTTL   time.Duration // TTL for autocomplete results
	SearchTTL         time.Duration // TTL for search results
	MaxEntries        int           // Max entries in cache
	Enabled           bool          // Enable/disable caching
}

// DefaultCacheConfig returns sensible cache defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		LEIRecordTTL:    1 * time.Hour,  // LEI data updates daily
		ValidationTTL:   24 * time.Hour, // Validation stable
		AutocompleteTTL: 5 * time.Minute,
		SearchTTL:       10 * time.Minute,
		MaxEntries:      10000,
		Enabled:         true,
	}
}

// cacheEntry wraps cached values with expiration.
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func (e cacheEntry[T]) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// Cache provides TTL-based caching for GLEIF API responses.
type Cache struct {
	config     CacheConfig
	leiCache   *lru.Cache[string, cacheEntry[*LEIRecord]]
	validCache *lru.Cache[string, cacheEntry[*ValidationResult]]
	searchCache *lru.Cache[string, cacheEntry[[]LEIRecord]]
	autoCache  *lru.Cache[string, cacheEntry[[]AutocompleteResult]]
	mu         sync.RWMutex
	stats      CacheStats
}

// CacheStats tracks cache performance.
type CacheStats struct {
	Hits   int64
	Misses int64
	Evictions int64
}

// NewCache creates a new cache instance.
func NewCache(config CacheConfig) (*Cache, error) {
	if !config.Enabled {
		return &Cache{config: config}, nil
	}

	leiCache, err := lru.New[string, cacheEntry[*LEIRecord]](config.MaxEntries)
	if err != nil {
		return nil, err
	}

	validCache, err := lru.New[string, cacheEntry[*ValidationResult]](config.MaxEntries / 4)
	if err != nil {
		return nil, err
	}

	searchCache, err := lru.New[string, cacheEntry[[]LEIRecord]](config.MaxEntries / 2)
	if err != nil {
		return nil, err
	}

	autoCache, err := lru.New[string, cacheEntry[[]AutocompleteResult]](config.MaxEntries / 4)
	if err != nil {
		return nil, err
	}

	return &Cache{
		config:      config,
		leiCache:    leiCache,
		validCache:  validCache,
		searchCache: searchCache,
		autoCache:   autoCache,
	}, nil
}

// GetLEI retrieves a cached LEI record.
func (c *Cache) GetLEI(lei string) (*LEIRecord, bool) {
	if !c.config.Enabled || c.leiCache == nil {
		return nil, false
	}

	entry, ok := c.leiCache.Get(lei)
	if !ok || entry.isExpired() {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		if ok {
			c.leiCache.Remove(lei)
		}
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
	return entry.value, true
}

// SetLEI caches an LEI record.
func (c *Cache) SetLEI(lei string, record *LEIRecord) {
	if !c.config.Enabled || c.leiCache == nil {
		return
	}

	c.leiCache.Add(lei, cacheEntry[*LEIRecord]{
		value:     record,
		expiresAt: time.Now().Add(c.config.LEIRecordTTL),
	})
}

// GetValidation retrieves a cached validation result.
func (c *Cache) GetValidation(lei string) (*ValidationResult, bool) {
	if !c.config.Enabled || c.validCache == nil {
		return nil, false
	}

	entry, ok := c.validCache.Get(lei)
	if !ok || entry.isExpired() {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		if ok {
			c.validCache.Remove(lei)
		}
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
	return entry.value, true
}

// SetValidation caches a validation result.
func (c *Cache) SetValidation(lei string, result *ValidationResult) {
	if !c.config.Enabled || c.validCache == nil {
		return
	}

	c.validCache.Add(lei, cacheEntry[*ValidationResult]{
		value:     result,
		expiresAt: time.Now().Add(c.config.ValidationTTL),
	})
}

// GetSearch retrieves cached search results.
func (c *Cache) GetSearch(key string) ([]LEIRecord, bool) {
	if !c.config.Enabled || c.searchCache == nil {
		return nil, false
	}

	entry, ok := c.searchCache.Get(key)
	if !ok || entry.isExpired() {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		if ok {
			c.searchCache.Remove(key)
		}
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
	return entry.value, true
}

// SetSearch caches search results.
func (c *Cache) SetSearch(key string, records []LEIRecord) {
	if !c.config.Enabled || c.searchCache == nil {
		return
	}

	c.searchCache.Add(key, cacheEntry[[]LEIRecord]{
		value:     records,
		expiresAt: time.Now().Add(c.config.SearchTTL),
	})
}

// GetAutocomplete retrieves cached autocomplete results.
func (c *Cache) GetAutocomplete(prefix string) ([]AutocompleteResult, bool) {
	if !c.config.Enabled || c.autoCache == nil {
		return nil, false
	}

	entry, ok := c.autoCache.Get(prefix)
	if !ok || entry.isExpired() {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		if ok {
			c.autoCache.Remove(prefix)
		}
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
	return entry.value, true
}

// SetAutocomplete caches autocomplete results.
func (c *Cache) SetAutocomplete(prefix string, results []AutocompleteResult) {
	if !c.config.Enabled || c.autoCache == nil {
		return
	}

	c.autoCache.Add(prefix, cacheEntry[[]AutocompleteResult]{
		value:     results,
		expiresAt: time.Now().Add(c.config.AutocompleteTTL),
	})
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Clear empties all caches.
func (c *Cache) Clear() {
	if c.leiCache != nil {
		c.leiCache.Purge()
	}
	if c.validCache != nil {
		c.validCache.Purge()
	}
	if c.searchCache != nil {
		c.searchCache.Purge()
	}
	if c.autoCache != nil {
		c.autoCache.Purge()
	}
}
