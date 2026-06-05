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
