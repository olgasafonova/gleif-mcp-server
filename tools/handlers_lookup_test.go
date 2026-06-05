package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Tests for relationship, issuer, and reporting-exception handlers. Shared
// test helpers (newTestServer, newTestRegistry, makeRequest, getResultText)
// live in handlers_test.go in the same package.

// TestHandleGetRelationships tests the get_relationships handler.
func TestHandleGetRelationships(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				Data []any `json:"data"`
			}{Data: []any{}}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"})

		result, err := registry.handleGetRelationships(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetRelationships returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleGetRelationships returned error result: %s", getResultText(result))
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{"lei": "INVALID"})

		result, err := registry.handleGetRelationships(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetRelationships returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid lei format")
		}
	})

	t.Run("missing lei parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleGetRelationships(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetRelationships returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing lei parameter")
		}
	})
}

// TestHandleGetLEIIssuer tests the get_lei_issuer handler.
func TestHandleGetLEIIssuer(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-issuers/", func(w http.ResponseWriter, r *http.Request) {
			resp := gleif.SingleResponse[gleif.LEIIssuer]{
				Data: gleif.DataItem[gleif.LEIIssuer]{
					ID:   "EVK05KS7XY1DEII3R011",
					Type: "lei-issuers",
					Attributes: gleif.LEIIssuer{
						Name:    "WM Datenservice",
						Country: "DE",
						Status:  "ACTIVE",
					},
				},
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"issuer_id": "EVK05KS7XY1DEII3R011"})

		result, err := registry.handleGetLEIIssuer(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetLEIIssuer returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleGetLEIIssuer returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, "WM Datenservice") {
			t.Errorf("Expected result to contain 'WM Datenservice', got: %s", text)
		}
	})

	t.Run("missing issuer_id parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleGetLEIIssuer(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetLEIIssuer returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing issuer_id parameter")
		}
	})
}

// TestHandleListLEIIssuers tests the list_lei_issuers handler.
func TestHandleListLEIIssuers(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-issuers", func(w http.ResponseWriter, r *http.Request) {
			resp := gleif.APIResponse[gleif.LEIIssuer]{
				Data: []gleif.DataItem[gleif.LEIIssuer]{
					{
						ID:   "EVK05KS7XY1DEII3R011",
						Type: "lei-issuers",
						Attributes: gleif.LEIIssuer{
							Name:    "WM Datenservice",
							Country: "DE",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{})

		result, err := registry.handleListLEIIssuers(context.Background(), req)
		if err != nil {
			t.Fatalf("handleListLEIIssuers returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleListLEIIssuers returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"count": 1`) {
			t.Errorf("Expected count=1 in result, got: %s", text)
		}
	})
}

// TestHandleGetReportingExceptions tests the get_reporting_exceptions handler.
func TestHandleGetReportingExceptions(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			resp := gleif.APIResponse[gleif.ReportingException]{
				Data: []gleif.DataItem[gleif.ReportingException]{},
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"})

		result, err := registry.handleGetReportingExceptions(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetReportingExceptions returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleGetReportingExceptions returned error result: %s", getResultText(result))
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{"lei": "INVALID"})

		result, err := registry.handleGetReportingExceptions(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetReportingExceptions returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid lei format")
		}
	})

	t.Run("missing lei parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		req := makeRequest(map[string]any{})

		result, err := registry.handleGetReportingExceptions(context.Background(), req)
		if err != nil {
			t.Fatalf("handleGetReportingExceptions returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing lei parameter")
		}
	})
}
