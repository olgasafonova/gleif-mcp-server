package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Tests for relationship, issuer, and reporting-exception handlers. Shared
// test helpers (newTestServer, newTestRegistry) live in handlers_test.go in the
// same package.

// TestHandleGetRelationships tests the get_relationships handler.
func TestHandleGetRelationships(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			resp := struct {
				Data []any `json:"data"`
			}{Data: []any{}}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleGetRelationships(context.Background(), GetRelationshipsArgs{LEI: "HWUPKR0MPOU8FGXBT394"})
		if err != nil {
			t.Fatalf("handleGetRelationships returned error: %v", err)
		}
		if result.Type != "direct-parent" {
			t.Errorf("expected default type 'direct-parent', got %q", result.Type)
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleGetRelationships(context.Background(), GetRelationshipsArgs{LEI: "INVALID"})
		if err == nil {
			t.Error("Expected error for invalid lei format")
		}
	})

	t.Run("empty lei parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleGetRelationships(context.Background(), GetRelationshipsArgs{LEI: ""})
		if err == nil {
			t.Error("Expected error for empty lei parameter")
		}
	})
}

// TestHandleGetLEIIssuer tests the get_lei_issuer handler.
func TestHandleGetLEIIssuer(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-issuers/", func(w http.ResponseWriter, _ *http.Request) {
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
		issuer, err := registry.handleGetLEIIssuer(context.Background(), GetLEIIssuerArgs{IssuerID: "EVK05KS7XY1DEII3R011"})
		if err != nil {
			t.Fatalf("handleGetLEIIssuer returned error: %v", err)
		}
		if issuer == nil || issuer.Name != "WM Datenservice" {
			t.Errorf("Expected issuer 'WM Datenservice', got: %+v", issuer)
		}
	})

	t.Run("empty issuer_id parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleGetLEIIssuer(context.Background(), GetLEIIssuerArgs{IssuerID: ""})
		if err == nil {
			t.Error("Expected error for empty issuer_id parameter")
		}
	})
}

// TestHandleListLEIIssuers tests the list_lei_issuers handler.
func TestHandleListLEIIssuers(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/lei-issuers", func(w http.ResponseWriter, _ *http.Request) {
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
		result, err := registry.handleListLEIIssuers(context.Background(), ListLEIIssuersArgs{})
		if err != nil {
			t.Fatalf("handleListLEIIssuers returned error: %v", err)
		}
		if result.Count != 1 {
			t.Errorf("Expected count=1, got %d", result.Count)
		}
	})
}

// TestHandleGetReportingExceptions tests the get_reporting_exceptions handler.
func TestHandleGetReportingExceptions(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		ts := newTestServer()
		defer ts.Close()

		ts.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			resp := gleif.APIResponse[gleif.ReportingException]{
				Data: []gleif.DataItem[gleif.ReportingException]{},
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleGetReportingExceptions(context.Background(), GetReportingExceptionsArgs{LEI: "HWUPKR0MPOU8FGXBT394"})
		if err != nil {
			t.Fatalf("handleGetReportingExceptions returned error: %v", err)
		}
		if result.LEI != "HWUPKR0MPOU8FGXBT394" {
			t.Errorf("Expected lei echoed back, got %q", result.LEI)
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleGetReportingExceptions(context.Background(), GetReportingExceptionsArgs{LEI: "INVALID"})
		if err == nil {
			t.Error("Expected error for invalid lei format")
		}
	})

	t.Run("empty lei parameter", func(t *testing.T) {
		registry := newTestRegistry("http://unused")
		_, err := registry.handleGetReportingExceptions(context.Background(), GetReportingExceptionsArgs{LEI: ""})
		if err == nil {
			t.Error("Expected error for empty lei parameter")
		}
	})
}
