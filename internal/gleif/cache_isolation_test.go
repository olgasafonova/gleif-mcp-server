package gleif

import (
	"testing"
	"time"
)

// TestCacheReturnsIndependentCopies is the regression test for the
// shared-pointer finding from the 2026-05-02 Carlini scaffold sweep.
//
// Before the fix: SetLEI stored the caller's pointer as-is and GetLEI
// returned the stored pointer. Five concurrent callers asking for the
// same LEI received the identical *LEIRecord. Any future handler that
// mutated a returned field (redaction, normalization, locale fix-ups)
// would silently corrupt the cache for every other caller.
//
// After the fix: SetLEI stores a deep copy and GetLEI returns a deep
// copy. Caller mutations stay local.
func TestCacheReturnsIndependentCopies(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL: time.Hour,
		MaxEntries:   10,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	original := &LEIRecord{
		LEI: "TESTABC1234567890123",
		Entity: Entity{
			LegalName: LegalName{Name: "Acme Corp", Language: "en"},
			OtherNames: []OtherName{
				{Name: "Acme", Type: "TRADING"},
			},
			LegalAddress: Address{
				AddressLines: []string{"123 Main St", "Suite 4"},
				City:         "Springfield",
				Country:      "US",
			},
		},
	}

	cache.SetLEI(original.LEI, original)

	// Mutate the original after Set; cache must not reflect this.
	original.Entity.LegalName.Name = "MUTATED-AFTER-SET"
	original.Entity.OtherNames[0].Name = "ALSO-MUTATED"
	original.Entity.LegalAddress.AddressLines[0] = "EVIL"

	a, ok := cache.GetLEI(original.LEI)
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if a.Entity.LegalName.Name != "Acme Corp" {
		t.Errorf("Set-side aliasing: cached name = %q, want %q", a.Entity.LegalName.Name, "Acme Corp")
	}
	if a.Entity.OtherNames[0].Name != "Acme" {
		t.Errorf("Set-side aliasing on slice: cached OtherNames[0] = %q, want %q", a.Entity.OtherNames[0].Name, "Acme")
	}
	if a.Entity.LegalAddress.AddressLines[0] != "123 Main St" {
		t.Errorf("Set-side aliasing on AddressLines: got %q, want %q", a.Entity.LegalAddress.AddressLines[0], "123 Main St")
	}

	// Mutate caller A's copy; caller B (also Get) must see clean data.
	a.Entity.LegalName.Name = "POISONED-BY-A"
	a.Entity.OtherNames[0].Name = "POISONED-NESTED"
	a.Entity.LegalAddress.AddressLines[0] = "POISONED-SLICE"

	b, ok := cache.GetLEI(original.LEI)
	if !ok {
		t.Fatal("expected second cache hit, got miss")
	}
	if b.Entity.LegalName.Name != "Acme Corp" {
		t.Errorf("Get-side aliasing: caller-B name = %q, want %q (caller A poisoned it via shared pointer)", b.Entity.LegalName.Name, "Acme Corp")
	}
	if b.Entity.OtherNames[0].Name != "Acme" {
		t.Errorf("Get-side aliasing on slice: caller-B OtherNames[0] = %q, want %q", b.Entity.OtherNames[0].Name, "Acme")
	}
	if b.Entity.LegalAddress.AddressLines[0] != "123 Main St" {
		t.Errorf("Get-side aliasing on AddressLines: caller-B got %q, want %q", b.Entity.LegalAddress.AddressLines[0], "123 Main St")
	}

	// And a and b must themselves be distinct allocations (different pointers).
	if a == b {
		t.Error("GetLEI returned the same pointer to two callers; they should each get an independent copy")
	}
}

// TestCloneLEIRecord_Nil ensures the helper handles nil safely.
func TestCloneLEIRecord_Nil(t *testing.T) {
	out, err := cloneLEIRecord(nil)
	if err != nil {
		t.Errorf("cloneLEIRecord(nil) returned err = %v, want nil", err)
	}
	if out != nil {
		t.Errorf("cloneLEIRecord(nil) returned %v, want nil", out)
	}
}
