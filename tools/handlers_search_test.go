package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Tests for the search and autocomplete handlers. Shared test helpers
// (newTestServer, newTestRegistry, makeRequest, getResultText,
// mockSearchResponse) live in handlers_test.go in the same package.

// TestHandleSearchEntity tests the search_entity handler.
func TestHandleSearchEntity(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
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
		req := makeRequest(map[string]any{"query": "Apple"})

		result, err := registry.handleSearchEntity(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchEntity returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchEntity returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, "Apple Inc.") {
			t.Errorf("Expected result to contain 'Apple Inc.', got: %s", text)
		}
	})

	t.Run("with pagination parameters", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
			resp := mockSearchResponse()
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{
			"query": "Test",
			"limit": float64(10),
			"page":  float64(2),
			"fuzzy": false,
		})

		result, err := registry.handleSearchEntity(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchEntity returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchEntity returned error result: %s", getResultText(result))
		}
	})

	t.Run("missing query parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleSearchEntity(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchEntity returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing query parameter")
		}
	})
}

// TestHandleSearchByBIC tests the search_by_bic handler.
func TestHandleSearchByBIC(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
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
		req := makeRequest(map[string]any{"bic": "DEUTDEFF"})

		result, err := registry.handleSearchByBIC(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchByBIC returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"found": true`) {
			t.Errorf("Expected found=true in result, got: %s", text)
		}
	})

	t.Run("no results", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
			resp := mockSearchResponse()
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"bic": "TESTTEST"})

		result, err := registry.handleSearchByBIC(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchByBIC returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"found": false`) {
			t.Errorf("Expected found=false in result, got: %s", text)
		}
	})

	t.Run("invalid bic length", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{"bic": "TOOLONG1234567890"})

		result, err := registry.handleSearchByBIC(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid bic length")
		}
	})

	t.Run("missing bic parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleSearchByBIC(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByBIC returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing bic parameter")
		}
	})
}

// TestHandleSearchByISIN tests the search_by_isin handler.
func TestHandleSearchByISIN(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		req := makeRequest(map[string]any{"isin": "US0378331005"})

		result, err := registry.handleSearchByISIN(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByISIN returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchByISIN returned error result: %s", getResultText(result))
		}
	})

	t.Run("invalid isin length", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{"isin": "TOOLONG1234567890"})

		result, err := registry.handleSearchByISIN(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByISIN returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid isin length")
		}
	})

	t.Run("missing isin parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleSearchByISIN(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByISIN returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing isin parameter")
		}
	})
}

// TestHandleSearchByCountry tests the search_by_country handler.
func TestHandleSearchByCountry(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
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
		req := makeRequest(map[string]any{"country": "US"})

		result, err := registry.handleSearchByCountry(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByCountry returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleSearchByCountry returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"country": "US"`) {
			t.Errorf("Expected country=US in result, got: %s", text)
		}
	})

	t.Run("invalid country code", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{"country": "USA"})

		result, err := registry.handleSearchByCountry(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByCountry returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid country code")
		}
	})

	t.Run("missing country parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleSearchByCountry(context.Background(), req)
		if err != nil {
			t.Fatalf("handleSearchByCountry returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing country parameter")
		}
	})
}

// TestHandleAutocomplete tests the autocomplete handler.
func TestHandleAutocomplete(t *testing.T) {
	t.Run("successful autocomplete via fallback", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		req := makeRequest(map[string]any{"prefix": "Apple"})

		result, err := registry.handleAutocomplete(context.Background(), req)
		if err != nil {
			t.Fatalf("handleAutocomplete returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleAutocomplete returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, "suggestions") {
			t.Errorf("Expected result to contain 'suggestions', got: %s", text)
		}
	})

	t.Run("missing prefix parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleAutocomplete(context.Background(), req)
		if err != nil {
			t.Fatalf("handleAutocomplete returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing prefix parameter")
		}
	})
}
