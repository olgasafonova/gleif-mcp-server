package gleif

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestClampLimitAndPage verifies pagination normalization.
func TestClampLimitAndPage(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		page      int
		wantLimit int
		wantPage  int
	}{
		{"valid passthrough", 20, 2, 20, 2},
		{"zero limit defaults to 20", 0, 1, 20, 1},
		{"negative limit defaults to 20", -5, 1, 20, 1},
		{"over-max limit defaults to 20", maxBatchSize + 1, 1, 20, 1},
		{"max limit allowed", maxBatchSize, 1, maxBatchSize, 1},
		{"zero page defaults to 1", 10, 0, 10, 1},
		{"negative page defaults to 1", 10, -3, 10, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotPage := clampLimitAndPage(tt.limit, tt.page)
			if gotLimit != tt.wantLimit || gotPage != tt.wantPage {
				t.Errorf("clampLimitAndPage(%d,%d) = (%d,%d), want (%d,%d)",
					tt.limit, tt.page, gotLimit, gotPage, tt.wantLimit, tt.wantPage)
			}
		})
	}
}

// TestApplyLimit verifies the slice-truncate helper.
func TestApplyLimit(t *testing.T) {
	tests := []struct {
		name    string
		in      []int
		n       int
		wantLen int
	}{
		{"truncates longer slice", []int{1, 2, 3, 4, 5}, 3, 3},
		{"returns all when shorter", []int{1, 2}, 5, 2},
		{"returns all when equal", []int{1, 2, 3}, 3, 3},
		{"empty slice", nil, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyLimit(tt.in, tt.n)
			if len(got) != tt.wantLen {
				t.Errorf("applyLimit(%v,%d) len = %d, want %d", tt.in, tt.n, len(got), tt.wantLen)
			}
		})
	}
}

// TestSearchEntities verifies the legalName filter and pagination passthrough.
func TestSearchEntities(t *testing.T) {
	var gotQuery string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{
				{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{Entity: Entity{LegalName: LegalName{Name: "Apple Inc."}}}},
			},
			Meta: Meta{Pagination: Pagination{CurrentPage: 1, Total: 1}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	records, pagination, err := client.SearchEntities(context.Background(), "Apple", 20, 1)
	if err != nil {
		t.Fatalf("SearchEntities failed: %v", err)
	}
	if len(records) != 1 || records[0].LEI != "HWUPKR0MPOU8FGXBT394" {
		t.Errorf("unexpected records: %+v", records)
	}
	if pagination == nil || pagination.Total != 1 {
		t.Errorf("pagination = %+v, want Total=1", pagination)
	}
	if !strings.Contains(gotQuery, "entity.legalName") {
		t.Errorf("query = %q, want legalName filter", gotQuery)
	}
}

// TestFuzzySearchCachesFirstPage verifies page-1 fuzzy results come from cache
// on the second call.
func TestFuzzySearchCachesFirstPage(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	if _, _, err := client.FuzzySearch(context.Background(), "Apple", 20, 1); err != nil {
		t.Fatalf("first FuzzySearch failed: %v", err)
	}
	if _, _, err := client.FuzzySearch(context.Background(), "Apple", 20, 1); err != nil {
		t.Fatalf("second FuzzySearch failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1 (page 1 should cache)", calls)
	}
}

// TestFuzzySearchPageTwoNotCached verifies non-first pages always hit upstream.
func TestFuzzySearchPageTwoNotCached(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := APIResponse[LEIRecord]{Data: []DataItem[LEIRecord]{}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	_, _, _ = client.FuzzySearch(context.Background(), "Apple", 20, 2)
	_, _, _ = client.FuzzySearch(context.Background(), "Apple", 20, 2)
	if calls != 2 {
		t.Errorf("upstream called %d times, want 2 (page 2 must not cache)", calls)
	}
}

// TestSearchByBIC verifies the BIC filter and result mapping.
func TestSearchByBIC(t *testing.T) {
	var gotQuery string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{{ID: "7LTWFZYICNSX8D621K86", Attributes: LEIRecord{}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	records, err := client.SearchByBIC(context.Background(), BIC("DEUTDEFF"))
	if err != nil {
		t.Fatalf("SearchByBIC failed: %v", err)
	}
	if len(records) != 1 || records[0].LEI != "7LTWFZYICNSX8D621K86" {
		t.Errorf("unexpected records: %+v", records)
	}
	if !strings.Contains(gotQuery, "filter%5Bbic%5D=DEUTDEFF") {
		t.Errorf("query = %q, want filter[bic]=DEUTDEFF", gotQuery)
	}
}

// TestSearchByISINPrimary verifies the primary lei-issuer endpoint is used and
// results are returned.
func TestSearchByISINPrimary(t *testing.T) {
	var firstPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if firstPath == "" {
			firstPath = r.URL.Path
		}
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	records, err := client.SearchByISIN(context.Background(), ISIN("US0378331005"))
	if err != nil {
		t.Fatalf("SearchByISIN failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("got %d records, want 1", len(records))
	}
	if !strings.Contains(firstPath, "/lei-issuer") {
		t.Errorf("first path = %q, want the lei-issuer primary endpoint", firstPath)
	}
}

// TestSearchByISINFallback verifies a primary-endpoint failure falls through to
// the generic lei-records filter.
func TestSearchByISINFallback(t *testing.T) {
	var paths []string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "/lei-issuer") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	records, err := client.SearchByISIN(context.Background(), ISIN("US0378331005"))
	if err != nil {
		t.Fatalf("SearchByISIN fallback failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("got %d records via fallback, want 1", len(records))
	}
	if len(paths) < 2 {
		t.Errorf("expected primary + fallback requests, got paths %v", paths)
	}
}

// TestSearchByISINBothFail verifies an error surfaces when both endpoints fail.
func TestSearchByISINBothFail(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.SearchByISIN(context.Background(), ISIN("US0378331005"))
	if err == nil {
		t.Fatal("expected error when both ISIN endpoints fail")
	}
}

// TestSearchByCountry verifies the country filter, limit clamp, and mapping.
func TestSearchByCountry(t *testing.T) {
	var gotQuery string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// limit 0 should clamp to the default 20.
	records, err := client.SearchByCountry(context.Background(), Country("US"), 0)
	if err != nil {
		t.Fatalf("SearchByCountry failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("got %d records, want 1", len(records))
	}
	if !strings.Contains(gotQuery, "legalAddress.country") {
		t.Errorf("query = %q, want country filter", gotQuery)
	}
	if !strings.Contains(gotQuery, "page%5Bsize%5D=20") {
		t.Errorf("query = %q, want clamped page size 20", gotQuery)
	}
}

// TestAutocompletePrimary verifies the autocomplete endpoint shapes results and
// applies the limit.
func TestAutocompletePrimary(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := AutocompleteResponse{
			Data: []AutocompleteResult{
				{LEI: "1", LegalName: "Apple Inc."},
				{LEI: "2", LegalName: "Apple Bank"},
				{LEI: "3", LegalName: "Apple Leasing"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	results, err := client.Autocomplete(context.Background(), "App", 2)
	if err != nil {
		t.Fatalf("Autocomplete failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2 (limit applied)", len(results))
	}
}

// TestAutocompleteFallback verifies a primary-endpoint failure reshapes a fuzzy
// search into autocomplete suggestions.
func TestAutocompleteFallback(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "autocompletions") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Fuzzy search (lei-records) succeeds.
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{
				{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{
					Entity: Entity{
						LegalName:    LegalName{Name: "Apple Inc."},
						LegalAddress: Address{Country: "US"},
					},
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	results, err := client.Autocomplete(context.Background(), "Apple", 10)
	if err != nil {
		t.Fatalf("Autocomplete fallback failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 from fuzzy fallback", len(results))
	}
	if results[0].LegalName != "Apple Inc." || results[0].Country != "US" {
		t.Errorf("fallback result not reshaped from record: %+v", results[0])
	}
}

// TestAutocompleteServesFromCache verifies cached autocomplete results skip the
// upstream and still apply the requested limit.
func TestAutocompleteServesFromCache(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := AutocompleteResponse{
			Data: []AutocompleteResult{
				{LEI: "1", LegalName: "Apple Inc."},
				{LEI: "2", LegalName: "Apple Bank"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	if _, err := client.Autocomplete(context.Background(), "App", 5); err != nil {
		t.Fatalf("first Autocomplete failed: %v", err)
	}
	results, err := client.Autocomplete(context.Background(), "App", 1)
	if err != nil {
		t.Fatalf("cached Autocomplete failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1", calls)
	}
	if len(results) != 1 {
		t.Errorf("cached results len = %d, want 1 (limit applied)", len(results))
	}
}
