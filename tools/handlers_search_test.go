package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Tests for the search and autocomplete handlers. Shared test helpers
// (newTestServer, newTestRegistry, mockSearchResponse) live in handlers_test.go
// in the same package.

// TestHandleSearchEntity tests the search_entity handler.
func TestHandleSearchEntity(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "HWUPKR0MPOU8FGXBT394",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Apple Inc."}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleSearchEntity(context.Background(), SearchEntityArgs{Query: "Apple"})
		if err != nil {
			t.Fatalf("handleSearchEntity returned error: %v", err)
		}
		if result.Count != 1 || result.Results[0].LegalName != "Apple Inc." {
			t.Errorf("Expected one Apple Inc. result, got: %+v", result)
		}
	})

	t.Run("with pagination parameters", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse()
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		noFuzzy := false
		_, err := registry.handleSearchEntity(context.Background(), SearchEntityArgs{
			Query: "Test",
			Limit: 10,
			Page:  2,
			Fuzzy: &noFuzzy,
		})
		if err != nil {
			t.Fatalf("handleSearchEntity returned error: %v", err)
		}
	})

	t.Run("empty query parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchEntity(context.Background(), SearchEntityArgs{Query: ""})
		if err == nil {
			t.Error("Expected error for empty query parameter")
		}
	})
}

// TestHandleSearchByBIC tests the search_by_bic handler.
func TestHandleSearchByBIC(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "7LTWFZYICNSX8D621K86",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Deutsche Bank AG"}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleSearchByBIC(context.Background(), SearchByBICArgs{BIC: "DEUTDEFF"})
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if !result.Found {
			t.Error("Expected found=true")
		}
	})

	t.Run("no results", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse()
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleSearchByBIC(context.Background(), SearchByBICArgs{BIC: "TESTTEST"})
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if result.Found {
			t.Error("Expected found=false")
		}
		if result.Records == nil {
			t.Error("Expected non-nil (empty) records slice on no-results")
		}
	})

	t.Run("invalid bic length", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByBIC(context.Background(), SearchByBICArgs{BIC: "TOOLONG1234567890"})
		if err == nil {
			t.Error("Expected error for invalid bic length")
		}
	})

	t.Run("empty bic parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByBIC(context.Background(), SearchByBICArgs{BIC: ""})
		if err == nil {
			t.Error("Expected error for empty bic parameter")
		}
	})
}

// TestHandleSearchByISIN tests the search_by_isin handler.
func TestHandleSearchByISIN(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "HWUPKR0MPOU8FGXBT394",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Apple Inc."}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleSearchByISIN(context.Background(), SearchByISINArgs{ISIN: "US0378331005"})
		if err != nil {
			t.Fatalf("handleSearchByISIN returned error: %v", err)
		}
		if !result.Found {
			t.Error("Expected found=true")
		}
	})

	t.Run("invalid isin length", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByISIN(context.Background(), SearchByISINArgs{ISIN: "TOOLONG1234567890"})
		if err == nil {
			t.Error("Expected error for invalid isin length")
		}
	})

	t.Run("empty isin parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByISIN(context.Background(), SearchByISINArgs{ISIN: ""})
		if err == nil {
			t.Error("Expected error for empty isin parameter")
		}
	})
}

// TestHandleSearchByCountry tests the search_by_country handler.
func TestHandleSearchByCountry(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "HWUPKR0MPOU8FGXBT394",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Apple Inc."}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleSearchByCountry(context.Background(), SearchByCountryArgs{Country: "US"})
		if err != nil {
			t.Fatalf("handleSearchByCountry returned error: %v", err)
		}
		if result.Country != "US" {
			t.Errorf("Expected country=US, got %q", result.Country)
		}
	})

	t.Run("invalid country code", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByCountry(context.Background(), SearchByCountryArgs{Country: "USA"})
		if err == nil {
			t.Error("Expected error for invalid country code")
		}
	})

	t.Run("empty country parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleSearchByCountry(context.Background(), SearchByCountryArgs{Country: ""})
		if err == nil {
			t.Error("Expected error for empty country parameter")
		}
	})
}

// TestHandleAutocomplete tests the autocomplete handler.
func TestHandleAutocomplete(t *testing.T) {
	t.Run("successful autocomplete via fallback", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "HWUPKR0MPOU8FGXBT394",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Apple Inc."}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleAutocomplete(context.Background(), AutocompleteArgs{Prefix: "Apple"})
		if err != nil {
			t.Fatalf("handleAutocomplete returned error: %v", err)
		}
		if result.Prefix != "Apple" {
			t.Errorf("Expected prefix echoed back, got %q", result.Prefix)
		}
	})

	t.Run("prefix too short", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleAutocomplete(context.Background(), AutocompleteArgs{Prefix: "A"})
		if err == nil {
			t.Error("Expected error for prefix shorter than 2 characters")
		}
	})
}
