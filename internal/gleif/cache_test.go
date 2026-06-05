package gleif

import (
	"testing"
	"time"
)

// TestDefaultCacheConfig pins the default cache configuration values.
func TestDefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()
	if !cfg.Enabled {
		t.Error("default cache should be enabled")
	}
	if cfg.MaxEntries != 10000 {
		t.Errorf("MaxEntries = %d, want 10000", cfg.MaxEntries)
	}
	if cfg.LEIRecordTTL != time.Hour {
		t.Errorf("LEIRecordTTL = %v, want 1h", cfg.LEIRecordTTL)
	}
	if cfg.ValidationTTL != 24*time.Hour {
		t.Errorf("ValidationTTL = %v, want 24h", cfg.ValidationTTL)
	}
	if cfg.AutocompleteTTL != 5*time.Minute {
		t.Errorf("AutocompleteTTL = %v, want 5m", cfg.AutocompleteTTL)
	}
	if cfg.SearchTTL != 10*time.Minute {
		t.Errorf("SearchTTL = %v, want 10m", cfg.SearchTTL)
	}
}

// TestNewCacheDisabled verifies a disabled config yields a cache with nil
// stores that never persists anything.
func TestNewCacheDisabled(t *testing.T) {
	c, err := NewCache(CacheConfig{Enabled: false})
	if err != nil {
		t.Fatalf("NewCache(disabled) returned error: %v", err)
	}
	if c.leiCache != nil || c.validCache != nil || c.searchCache != nil || c.autoCache != nil {
		t.Error("disabled cache should have nil LRU stores")
	}

	c.SetLEI("HWUPKR0MPOU8FGXBT394", &LEIRecord{LEI: "HWUPKR0MPOU8FGXBT394"})
	if _, ok := c.GetLEI("HWUPKR0MPOU8FGXBT394"); ok {
		t.Error("disabled cache must always miss")
	}
}

// TestNewCacheInvalidSize verifies that a non-positive MaxEntries propagates
// the LRU constructor error.
func TestNewCacheInvalidSize(t *testing.T) {
	_, err := NewCache(CacheConfig{Enabled: true, MaxEntries: 0})
	if err == nil {
		t.Error("NewCache with MaxEntries=0 should error from the LRU constructor")
	}
}

// TestCloneLEIRecordIsolation verifies cloneLEIRecord returns a deep copy so
// mutating the result never leaks into the original.
func TestCloneLEIRecordIsolation(t *testing.T) {
	original := &LEIRecord{
		LEI: "HWUPKR0MPOU8FGXBT394",
		Entity: Entity{
			LegalName:  LegalName{Name: "Apple Inc."},
			OtherNames: []OtherName{{Name: "Apple"}},
			LegalAddress: Address{
				City:         "Cupertino",
				Country:      "US",
				AddressLines: []string{"One Apple Park Way"},
			},
		},
	}

	clone, err := cloneLEIRecord(original)
	if err != nil {
		t.Fatalf("cloneLEIRecord returned error: %v", err)
	}
	if clone == original {
		t.Fatal("clone should be a distinct pointer")
	}

	// Mutate clone's nested slices; original must be untouched.
	clone.Entity.LegalName.Name = "Mutated"
	clone.Entity.OtherNames[0].Name = "MutatedAlias"
	clone.Entity.LegalAddress.AddressLines[0] = "Mutated Street"

	if original.Entity.LegalName.Name != "Apple Inc." {
		t.Error("mutating clone leaked into original legal name")
	}
	if original.Entity.OtherNames[0].Name != "Apple" {
		t.Error("mutating clone leaked into original other names slice")
	}
	if original.Entity.LegalAddress.AddressLines[0] != "One Apple Park Way" {
		t.Error("mutating clone leaked into original address lines slice")
	}
}

// TestCloneLEIRecordNil verifies the nil input is handled as (nil, nil).
func TestCloneLEIRecordNil(t *testing.T) {
	clone, err := cloneLEIRecord(nil)
	if err != nil {
		t.Errorf("cloneLEIRecord(nil) error = %v, want nil", err)
	}
	if clone != nil {
		t.Errorf("cloneLEIRecord(nil) = %v, want nil", clone)
	}
}

// TestGetLEIReturnsCopy verifies callers cannot mutate the cached record by
// mutating the value GetLEI hands back.
func TestGetLEIReturnsCopy(t *testing.T) {
	c := newEnabledCache(t)
	lei := "HWUPKR0MPOU8FGXBT394"
	c.SetLEI(lei, &LEIRecord{
		LEI:    lei,
		Entity: Entity{LegalName: LegalName{Name: "Apple Inc."}},
	})

	first, ok := c.GetLEI(lei)
	if !ok {
		t.Fatal("expected hit")
	}
	first.Entity.LegalName.Name = "Tampered"

	second, ok := c.GetLEI(lei)
	if !ok {
		t.Fatal("expected second hit")
	}
	if second.Entity.LegalName.Name != "Apple Inc." {
		t.Error("mutation through returned pointer leaked into cache")
	}
}

// TestSetLEIStoresCopy verifies mutating the record after SetLEI does not
// affect what the cache returns.
func TestSetLEIStoresCopy(t *testing.T) {
	c := newEnabledCache(t)
	lei := "HWUPKR0MPOU8FGXBT394"
	record := &LEIRecord{LEI: lei, Entity: Entity{LegalName: LegalName{Name: "Apple Inc."}}}
	c.SetLEI(lei, record)

	record.Entity.LegalName.Name = "MutatedAfterSet"

	got, ok := c.GetLEI(lei)
	if !ok {
		t.Fatal("expected hit")
	}
	if got.Entity.LegalName.Name != "Apple Inc." {
		t.Error("post-Set mutation of source record leaked into cache")
	}
}

// TestCacheExpiry verifies an entry past its TTL is treated as a miss and is
// counted as such.
func TestCacheExpiry(t *testing.T) {
	c, err := NewCache(CacheConfig{
		Enabled:       true,
		MaxEntries:    100,
		LEIRecordTTL:  -1 * time.Second, // already-expired on write
		ValidationTTL: time.Hour,
		SearchTTL:     time.Hour,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	lei := "HWUPKR0MPOU8FGXBT394"
	c.SetLEI(lei, &LEIRecord{LEI: lei})

	if _, ok := c.GetLEI(lei); ok {
		t.Error("expected miss for already-expired entry")
	}
	if c.Stats().Misses == 0 {
		t.Error("expired lookup should record a miss")
	}
}

// TestValidationAndSearchMiss exercises the lookupEntry miss paths for the
// validation, search, and autocomplete typed caches.
func TestValidationAndSearchMiss(t *testing.T) {
	c := newEnabledCache(t)

	if _, ok := c.GetValidation("NONE"); ok {
		t.Error("expected validation miss")
	}
	if _, ok := c.GetSearch("none:key"); ok {
		t.Error("expected search miss")
	}
	if _, ok := c.GetAutocomplete("none"); ok {
		t.Error("expected autocomplete miss")
	}
	if c.Stats().Misses < 3 {
		t.Errorf("expected at least 3 misses, got %d", c.Stats().Misses)
	}
}

// TestStatsCountsHitsAndMisses verifies hit/miss counters move as expected
// across a set/get cycle on every typed cache.
func TestStatsCountsHitsAndMisses(t *testing.T) {
	c := newEnabledCache(t)

	c.GetLEI("MISS")          // miss
	c.GetValidation("MISS")   // miss
	c.GetSearch("MISS")       // miss
	c.GetAutocomplete("MISS") // miss

	lei := "HWUPKR0MPOU8FGXBT394"
	c.SetLEI(lei, &LEIRecord{LEI: lei})
	c.SetValidation(lei, &ValidationResult{LEI: lei, Valid: true})
	c.SetSearch("s", []LEIRecord{{LEI: lei}})
	c.SetAutocomplete("p", []AutocompleteResult{{LEI: lei}})

	c.GetLEI(lei)          // hit
	c.GetValidation(lei)   // hit
	c.GetSearch("s")       // hit
	c.GetAutocomplete("p") // hit

	stats := c.Stats()
	if stats.Hits < 4 {
		t.Errorf("Hits = %d, want >= 4", stats.Hits)
	}
	if stats.Misses < 4 {
		t.Errorf("Misses = %d, want >= 4", stats.Misses)
	}
}

// TestClearOnDisabledCacheIsSafe verifies Clear does not panic when stores are
// nil (disabled cache).
func TestClearOnDisabledCacheIsSafe(t *testing.T) {
	c, err := NewCache(CacheConfig{Enabled: false})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	c.Clear() // must not panic
}

// newEnabledCache builds an enabled cache with hour TTLs for test reuse.
func newEnabledCache(t *testing.T) *Cache {
	t.Helper()
	c, err := NewCache(CacheConfig{
		Enabled:         true,
		MaxEntries:      100,
		LEIRecordTTL:    time.Hour,
		ValidationTTL:   time.Hour,
		SearchTTL:       time.Hour,
		AutocompleteTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	return c
}
