package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// testServer creates a mock GLEIF API server with configurable responses.
type testServer struct {
	server *httptest.Server
	mux    *http.ServeMux
}

func newTestServer() *testServer {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	return &testServer{server: server, mux: mux}
}

func (ts *testServer) Close() {
	ts.server.Close()
}

func (ts *testServer) URL() string {
	return ts.server.URL
}

// mockLEIResponse returns a successful LEI record response.
func mockLEIResponse(lei, name, country, city string) gleif.SingleResponse[gleif.LEIRecord] {
	return gleif.SingleResponse[gleif.LEIRecord]{
		Data: gleif.DataItem[gleif.LEIRecord]{
			ID:   lei,
			Type: "lei-records",
			Attributes: gleif.LEIRecord{
				Entity: gleif.Entity{
					LegalName:    gleif.LegalName{Name: name},
					LegalAddress: gleif.Address{Country: country, City: city},
					Status:       "ACTIVE",
				},
				Registration: gleif.Registration{
					Status:          "ISSUED",
					NextRenewalDate: time.Now().Add(365 * 24 * time.Hour),
				},
			},
		},
	}
}

// mockSearchResponse returns a search results response.
func mockSearchResponse(records ...gleif.LEIRecord) gleif.APIResponse[gleif.LEIRecord] {
	items := make([]gleif.DataItem[gleif.LEIRecord], len(records))
	for i, rec := range records {
		items[i] = gleif.DataItem[gleif.LEIRecord]{
			ID:         rec.LEI,
			Type:       "lei-records",
			Attributes: rec,
		}
	}
	return gleif.APIResponse[gleif.LEIRecord]{
		Data: items,
		Meta: gleif.Meta{
			Pagination: gleif.Pagination{
				CurrentPage: 1,
				PerPage:     20,
				Total:       len(records),
				LastPage:    1,
			},
		},
	}
}

// newTestRegistry creates a Registry with a mock server.
func newTestRegistry(serverURL string) *Registry {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := gleif.NewClient(gleif.Config{
		BaseURL:     serverURL,
		Timeout:     5 * time.Second,
		RateLimit:   100,
		BurstSize:   10,
		MaxRetries:  0,
		EnableCache: false,
	}, logger)
	return NewRegistry(client, logger)
}

// makeRequest creates a CallToolRequest with given arguments.
func makeRequest(args map[string]any) *mcp.CallToolRequest {
	argBytes, _ := json.Marshal(args)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: argBytes,
		},
	}
}

// getResultText extracts text from a CallToolResult.
func getResultText(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
		return textContent.Text
	}
	return ""
}

// TestHandleLEILookup tests the lei_lookup handler.
func TestHandleLEILookup(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("successful lookup", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records/HWUPKR0MPOU8FGXBT394", func(w http.ResponseWriter, r *http.Request) {
			resp := mockLEIResponse("HWUPKR0MPOU8FGXBT394", "Apple Inc.", "US", "Cupertino")
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"})

		result, err := registry.handleLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleLEILookup returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleLEILookup returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, "Apple Inc.") {
			t.Errorf("Expected result to contain 'Apple Inc.', got: %s", text)
		}
	})

	t.Run("missing lei parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{})

		result, err := registry.handleLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleLEILookup returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing lei parameter")
		}
		if !strings.Contains(getResultText(result), "lei parameter is required") {
			t.Errorf("Expected error message containing 'lei parameter is required', got: %s", getResultText(result))
		}
	})

	t.Run("empty lei parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": ""})

		result, err := registry.handleLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleLEILookup returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for empty lei parameter")
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "INVALID"})

		result, err := registry.handleLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleLEILookup returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid lei format")
		}
	})
}

// TestHandleValidateLEI tests the validate_lei handler.
func TestHandleValidateLEI(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("valid lei with record", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records/HWUPKR0MPOU8FGXBT394", func(w http.ResponseWriter, r *http.Request) {
			resp := mockLEIResponse("HWUPKR0MPOU8FGXBT394", "Apple Inc.", "US", "Cupertino")
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"})

		result, err := registry.handleValidateLEI(context.Background(), req)
		if err != nil {
			t.Fatalf("handleValidateLEI returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleValidateLEI returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"valid": true`) {
			t.Errorf("Expected valid=true in result, got: %s", text)
		}
	})

	t.Run("invalid format without API call", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"lei": "TOOSHORT"})

		result, err := registry.handleValidateLEI(context.Background(), req)
		if err != nil {
			t.Fatalf("handleValidateLEI returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleValidateLEI returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"valid": false`) {
			t.Errorf("Expected valid=false in result, got: %s", text)
		}
	})

	t.Run("missing lei parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{})

		result, err := registry.handleValidateLEI(context.Background(), req)
		if err != nil {
			t.Fatalf("handleValidateLEI returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing lei parameter")
		}
	})
}

// TestHandleBatchLEILookup tests the batch_lei_lookup handler.
func TestHandleBatchLEILookup(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("successful batch lookup", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, r *http.Request) {
			resp := mockSearchResponse(
				gleif.LEIRecord{
					LEI:    "HWUPKR0MPOU8FGXBT394",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Apple Inc."}},
				},
				gleif.LEIRecord{
					LEI:    "7LTWFZYICNSX8D621K86",
					Entity: gleif.Entity{LegalName: gleif.LegalName{Name: "Deutsche Bank AG"}},
				},
			)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"leis": "HWUPKR0MPOU8FGXBT394,7LTWFZYICNSX8D621K86"})

		result, err := registry.handleBatchLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleBatchLEILookup returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("handleBatchLEILookup returned error result: %s", getResultText(result))
		}

		text := getResultText(result)
		if !strings.Contains(text, `"requested": 2`) {
			t.Errorf("Expected requested=2 in result, got: %s", text)
		}
	})

	t.Run("missing leis parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{})

		result, err := registry.handleBatchLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleBatchLEILookup returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for missing leis parameter")
		}
	})

	t.Run("invalid lei in batch", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		req := makeRequest(map[string]any{"leis": "HWUPKR0MPOU8FGXBT394,INVALID"})

		result, err := registry.handleBatchLEILookup(context.Background(), req)
		if err != nil {
			t.Fatalf("handleBatchLEILookup returned error: %v", err)
		}
		if !result.IsError {
			t.Error("Expected error result for invalid lei in batch")
		}
	})
}

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

// TestGetHandler tests the getHandler dispatcher.
func TestGetHandler(t *testing.T) {
	registry := newTestRegistry("http://unused")

	tests := []struct {
		name     string
		toolName string
		wantNil  bool
	}{
		{"lei_lookup", "lei_lookup", false},
		{"validate_lei", "validate_lei", false},
		{"batch_lei_lookup", "batch_lei_lookup", false},
		{"search_entity", "search_entity", false},
		{"search_by_bic", "search_by_bic", false},
		{"search_by_isin", "search_by_isin", false},
		{"search_by_country", "search_by_country", false},
		{"get_relationships", "get_relationships", false},
		{"autocomplete", "autocomplete", false},
		{"get_lei_issuer", "get_lei_issuer", false},
		{"list_lei_issuers", "list_lei_issuers", false},
		{"get_reporting_exceptions", "get_reporting_exceptions", false},
		{"unknown_tool", "nonexistent_tool", false}, // Returns error handler
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := registry.lookupHandler(tt.toolName)
			if handler == nil && !tt.wantNil {
				t.Errorf("lookupHandler(%q) returned nil", tt.toolName)
			}
		})
	}
}

// TestUnknownToolHandler tests that unknown tools return an error.
func TestUnknownToolHandler(t *testing.T) {
	registry := newTestRegistry("http://unused")
	handler := registry.lookupHandler("nonexistent_tool")
	req := makeRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Unknown tool handler returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for unknown tool")
	}
	text := getResultText(result)
	if !strings.Contains(text, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' in error message, got: %s", text)
	}
}

// TestParseArgumentsEmptyRequest tests parseArguments with empty request.
func TestParseArgumentsEmptyRequest(t *testing.T) {
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: nil,
		},
	}

	args, err := parseArguments(req)
	if err != nil {
		t.Fatalf("parseArguments returned error: %v", err)
	}
	if len(args) != 0 {
		t.Errorf("Expected empty args map, got %d entries", len(args))
	}
}

// TestParseArgumentsInvalidJSON tests parseArguments with invalid JSON.
func TestParseArgumentsInvalidJSON(t *testing.T) {
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(`{invalid json}`),
		},
	}

	_, err := parseArguments(req)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestErrorResult tests the errorResult helper.
func TestErrorResult(t *testing.T) {
	result, err := errorResult("test error message")
	if err != nil {
		t.Fatalf("errorResult returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true")
	}
	text := getResultText(result)
	// Error results are now structured JSON
	var errResp map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &errResp); jsonErr != nil {
		t.Fatalf("errorResult did not return valid JSON: %v", jsonErr)
	}
	if errResp["error"] != true {
		t.Error("Expected error=true in JSON response")
	}
	if errResp["message"] != "test error message" {
		t.Errorf("Expected message 'test error message', got: %v", errResp["message"])
	}
	if errResp["retryable"] != false {
		t.Error("Expected retryable=false in JSON response")
	}
}

// TestJSONResult tests the jsonResult helper.
func TestJSONResult(t *testing.T) {
	data := map[string]any{"key": "value"}
	result, err := jsonResult(data)
	if err != nil {
		t.Fatalf("jsonResult returned error: %v", err)
	}
	if result.IsError {
		t.Error("Expected IsError=false")
	}
	text := getResultText(result)
	if !strings.Contains(text, `"key": "value"`) {
		t.Errorf("Expected JSON with key=value, got: %s", text)
	}
}

// TestGetToolByName tests tool spec lookup.
func TestGetToolByName(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantNil  bool
	}{
		{"existing tool", "lei_lookup", false},
		{"another tool", "search_entity", false},
		{"nonexistent", "nonexistent_tool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := GetToolByName(tt.toolName)
			if (spec == nil) != tt.wantNil {
				t.Errorf("GetToolByName(%q) nil=%v, wantNil=%v", tt.toolName, spec == nil, tt.wantNil)
			}
		})
	}
}

// TestToolSpecsComplete verifies all tools have required fields.
func TestToolSpecsComplete(t *testing.T) {
	for _, spec := range AllTools {
		t.Run(spec.Name, func(t *testing.T) {
			if spec.Name == "" {
				t.Error("Tool has empty name")
			}
			if spec.Description == "" {
				t.Errorf("Tool %s has empty description", spec.Name)
			}
			if spec.Category == "" {
				t.Errorf("Tool %s has empty category", spec.Name)
			}

			// Check required parameters have descriptions
			for _, param := range spec.Parameters {
				if param.Name == "" {
					t.Errorf("Tool %s has parameter with empty name", spec.Name)
				}
				if param.Type == "" {
					t.Errorf("Tool %s parameter %s has empty type", spec.Name, param.Name)
				}
			}
		})
	}
}
