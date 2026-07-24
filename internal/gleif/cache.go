package gleif

import (
	"encoding/json"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// cloneLEIRecord returns a deep copy of an LEIRecord. LEIRecord contains
// nested slices (OtherNames, AddressLines) and pointer fields
// (AssociatedEntity, SuccessorEntity, ValidationAuthority,
// EntityExpirationDate/Reason), so a value copy of the top-level struct
// would still share inner state. JSON round-trip is the simplest deep
// copy that stays correct as the type evolves; cost is a few µs per
// call, negligible against any GLEIF round-trip.
//
// Returns (nil, error) on marshal/unmarshal failure. Callers treat that
// as a cache miss / silent skip rather than failing the request.
func cloneLEIRecord(r *LEIRecord) (*LEIRecord, error) {
	if r == nil {
		return nil, nil
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var out LEIRecord
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CacheConfig configures the cache behavior.
type CacheConfig struct {
	LEIRecordTTL    time.Duration // TTL for LEI record lookups
	ValidationTTL   time.Duration // TTL for validation results
	AutocompleteTTL time.Duration // TTL for autocomplete results
	SearchTTL       time.Duration // TTL for search results
	MaxEntries      int           // Max entries in cache
	Enabled         bool          // Enable/disable caching
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
	config      CacheConfig
	leiCache    *lru.Cache[string, cacheEntry[*LEIRecord]]
	validCache  *lru.Cache[string, cacheEntry[*ValidationResult]]
	searchCache *lru.Cache[string, cacheEntry[[]LEIRecord]]
	autoCache   *lru.Cache[string, cacheEntry[[]AutocompleteResult]]
	mu          sync.RWMutex
	stats       CacheStats
}

// CacheStats tracks cache performance.
type CacheStats struct {
	Hits      int64
	Misses    int64
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

// recordHit and recordMiss centralize the locked counter updates used by
// every Get* method on the cache.
func (c *Cache) recordHit() {
	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()
}

func (c *Cache) recordMiss() {
	c.mu.Lock()
	c.stats.Misses++
	c.mu.Unlock()
}

// lookupEntry performs the shared "check enabled, fetch, expire-check, count"
// flow for every typed cache. It returns the stored value and ok=true on a
// hit. On a miss (disabled cache, absent key, or expired entry) it bumps the
// miss counter, evicts any stale entry, and returns the zero value with
// ok=false.
//
// Because Go generics don't allow methods to be generic, this is a package-
// level function taking the cache, the typed LRU, and the key.
func lookupEntry[K comparable, T any](
	c *Cache,
	store *lru.Cache[K, cacheEntry[T]],
	key K,
) (T, bool) {
	var zero T
	if !c.config.Enabled || store == nil {
		return zero, false
	}

	entry, ok := store.Get(key)
	if !ok || entry.isExpired() {
		c.recordMiss()
		if ok {
			store.Remove(key)
		}
		return zero, false
	}

	return entry.value, true
}

// storeEntry is the write-side counterpart to lookupEntry: it performs the
// shared "check enabled, wrap with TTL, add" flow for every typed cache.
// Disabled caches and nil stores are silently skipped, matching the read side.
func storeEntry[K comparable, T any](
	c *Cache,
	store *lru.Cache[K, cacheEntry[T]],
	key K,
	value T,
	ttl time.Duration,
) {
	if !c.config.Enabled || store == nil {
		return
	}
	store.Add(key, cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
}

// GetLEI retrieves a cached LEI record. Returns a deep copy so callers can
// freely mutate without affecting the cache or other concurrent callers.
func (c *Cache) GetLEI(lei string) (*LEIRecord, bool) {
	value, ok := lookupEntry(c, c.leiCache, lei)
	if !ok {
		return nil, false
	}

	out, err := cloneLEIRecord(value)
	if err != nil || out == nil {
		// Treat clone failure as a cache miss rather than risk handing
		// the shared pointer back to the caller.
		c.recordMiss()
		return nil, false
	}

	c.recordHit()
	return out, true
}

// SetLEI caches an LEI record. Stores a deep copy so subsequent caller
// mutations of the passed-in record don't leak into the cache.
func (c *Cache) SetLEI(lei string, record *LEIRecord) {
	if !c.config.Enabled || c.leiCache == nil {
		return
	}

	stored, err := cloneLEIRecord(record)
	if err != nil || stored == nil {
		// Skip caching rather than store an aliased pointer.
		return
	}

	storeEntry(c, c.leiCache, lei, stored, c.config.LEIRecordTTL)
}

// GetValidation retrieves a cached validation result.
func (c *Cache) GetValidation(lei string) (*ValidationResult, bool) {
	value, ok := lookupEntry(c, c.validCache, lei)
	if !ok {
		return nil, false
	}
	c.recordHit()
	return value, true
}

// SetValidation caches a validation result.
func (c *Cache) SetValidation(lei string, result *ValidationResult) {
	storeEntry(c, c.validCache, lei, result, c.config.ValidationTTL)
}

// GetSearch retrieves cached search results.
func (c *Cache) GetSearch(key string) ([]LEIRecord, bool) {
	value, ok := lookupEntry(c, c.searchCache, key)
	if !ok {
		return nil, false
	}
	c.recordHit()
	return value, true
}

// SetSearch caches search results.
func (c *Cache) SetSearch(key string, records []LEIRecord) {
	storeEntry(c, c.searchCache, key, records, c.config.SearchTTL)
}

// GetAutocomplete retrieves cached autocomplete results.
func (c *Cache) GetAutocomplete(prefix string) ([]AutocompleteResult, bool) {
	value, ok := lookupEntry(c, c.autoCache, prefix)
	if !ok {
		return nil, false
	}
	c.recordHit()
	return value, true
}

// SetAutocomplete caches autocomplete results.
func (c *Cache) SetAutocomplete(prefix string, results []AutocompleteResult) {
	storeEntry(c, c.autoCache, prefix, results, c.config.AutocompleteTTL)
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
