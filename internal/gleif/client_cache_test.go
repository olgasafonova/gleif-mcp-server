package gleif

import (
	"testing"
	"time"
)

// Cache behaviour tests for the client-level cache wiring. The lower-level
// cache unit tests (isolation, expiry, clone semantics) live in cache_test.go;
// these exercise the hit/miss/stats/clear surface used by the client.

// TestCacheHitMiss tests cache behavior.
func TestCacheHitMiss(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL:    time.Hour,
		ValidationTTL:   time.Hour,
		AutocompleteTTL: time.Hour,
		SearchTTL:       time.Hour,
		MaxEntries:      100,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	t.Run("LEI cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetLEI("NONEXISTENT0000000000")
		if ok {
			t.Error("Expected cache miss for nonexistent LEI")
		}
	})

	t.Run("LEI cache hit after set", func(t *testing.T) {
		lei := "HWUPKR0MPOU8FGXBT394"
		record := &LEIRecord{
			LEI: lei,
			Entity: Entity{
				LegalName: LegalName{Name: "Apple Inc."},
			},
		}

		cache.SetLEI(lei, record)

		got, ok := cache.GetLEI(lei)
		if !ok {
			t.Error("Expected cache hit after SetLEI")
		}
		if got.Entity.LegalName.Name != "Apple Inc." {
			t.Errorf("Got wrong record, expected Apple Inc., got %s", got.Entity.LegalName.Name)
		}
	})

	t.Run("Validation cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetValidation("NONEXISTENT0000000000")
		if ok {
			t.Error("Expected cache miss for nonexistent validation")
		}
	})

	t.Run("Validation cache hit after set", func(t *testing.T) {
		lei := "7LTWFZYICNSX8D621K86"
		result := &ValidationResult{
			LEI:   lei,
			Valid: true,
		}

		cache.SetValidation(lei, result)

		got, ok := cache.GetValidation(lei)
		if !ok {
			t.Error("Expected cache hit after SetValidation")
		}
		if !got.Valid {
			t.Error("Expected Valid=true")
		}
	})

	t.Run("Search cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetSearch("nonexistent:query")
		if ok {
			t.Error("Expected cache miss for nonexistent search")
		}
	})

	t.Run("Search cache hit after set", func(t *testing.T) {
		key := "bic:DEUTDEFF"
		records := []LEIRecord{{LEI: "7LTWFZYICNSX8D621K86"}}

		cache.SetSearch(key, records)

		got, ok := cache.GetSearch(key)
		if !ok {
			t.Error("Expected cache hit after SetSearch")
		}
		if len(got) != 1 {
			t.Errorf("Expected 1 record, got %d", len(got))
		}
	})

	t.Run("Autocomplete cache hit after set", func(t *testing.T) {
		prefix := "Apple"
		results := []AutocompleteResult{{LEI: "HWUPKR0MPOU8FGXBT394", LegalName: "Apple Inc."}}

		cache.SetAutocomplete(prefix, results)

		got, ok := cache.GetAutocomplete(prefix)
		if !ok {
			t.Error("Expected cache hit after SetAutocomplete")
		}
		if len(got) != 1 || got[0].LegalName != "Apple Inc." {
			t.Error("Got wrong autocomplete results")
		}
	})
}

// TestCacheDisabled tests that disabled cache returns misses.
func TestCacheDisabled(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	lei := "HWUPKR0MPOU8FGXBT394"
	record := &LEIRecord{LEI: lei}
	cache.SetLEI(lei, record)

	_, ok := cache.GetLEI(lei)
	if ok {
		t.Error("Expected cache miss when cache is disabled")
	}
}

// TestCacheStats tests cache statistics tracking.
func TestCacheStats(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL: time.Hour,
		MaxEntries:   100,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	// Generate some hits and misses
	cache.GetLEI("MISS1")
	cache.GetLEI("MISS2")

	lei := "HWUPKR0MPOU8FGXBT394"
	cache.SetLEI(lei, &LEIRecord{LEI: lei})
	cache.GetLEI(lei) // hit

	stats := cache.Stats()
	if stats.Misses < 2 {
		t.Errorf("Expected at least 2 misses, got %d", stats.Misses)
	}
	if stats.Hits < 1 {
		t.Errorf("Expected at least 1 hit, got %d", stats.Hits)
	}
}

// TestCacheClear tests cache clearing.
func TestCacheClear(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL: time.Hour,
		MaxEntries:   100,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	lei := "HWUPKR0MPOU8FGXBT394"
	cache.SetLEI(lei, &LEIRecord{LEI: lei})
	cache.SetSearch("test:key", []LEIRecord{{LEI: lei}})

	cache.Clear()

	if _, ok := cache.GetLEI(lei); ok {
		t.Error("Expected cache miss after Clear")
	}
	if _, ok := cache.GetSearch("test:key"); ok {
		t.Error("Expected search cache miss after Clear")
	}
}
