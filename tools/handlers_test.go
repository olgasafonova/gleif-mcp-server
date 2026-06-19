package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

// TestHandleLEILookup tests the lei_lookup handler.
func TestHandleLEILookup(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("successful lookup", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records/HWUPKR0MPOU8FGXBT394", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockLEIResponse("HWUPKR0MPOU8FGXBT394", "Apple Inc.", "US", "Cupertino")
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		record, err := registry.handleLEILookup(context.Background(), LEILookupArgs{LEI: "HWUPKR0MPOU8FGXBT394"})
		if err != nil {
			t.Fatalf("handleLEILookup returned error: %v", err)
		}
		if record == nil {
			t.Fatal("handleLEILookup returned nil record")
		}
		if record.Entity.LegalName.Name != "Apple Inc." {
			t.Errorf("Expected legal name 'Apple Inc.', got: %s", record.Entity.LegalName.Name)
		}
	})

	t.Run("empty lei parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		_, err := registry.handleLEILookup(context.Background(), LEILookupArgs{LEI: ""})
		if err == nil {
			t.Error("Expected error for empty lei parameter")
		}
	})

	t.Run("invalid lei format", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		_, err := registry.handleLEILookup(context.Background(), LEILookupArgs{LEI: "INVALID"})
		if err == nil {
			t.Error("Expected error for invalid lei format")
		}
	})
}

// TestHandleValidateLEI tests the validate_lei handler.
func TestHandleValidateLEI(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("valid lei with record", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records/HWUPKR0MPOU8FGXBT394", func(w http.ResponseWriter, _ *http.Request) {
			resp := mockLEIResponse("HWUPKR0MPOU8FGXBT394", "Apple Inc.", "US", "Cupertino")
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		})

		registry := newTestRegistry(ts.URL())
		result, err := registry.handleValidateLEI(context.Background(), ValidateLEIArgs{LEI: "HWUPKR0MPOU8FGXBT394"})
		if err != nil {
			t.Fatalf("handleValidateLEI returned error: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid=true, got valid=false (message: %s)", result.Message)
		}
	})

	t.Run("invalid format without API call", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		result, err := registry.handleValidateLEI(context.Background(), ValidateLEIArgs{LEI: "TOOSHORT"})
		if err != nil {
			t.Fatalf("handleValidateLEI returned error: %v", err)
		}
		if result.Valid {
			t.Error("Expected valid=false for malformed LEI")
		}
	})
}

// TestHandleBatchLEILookup tests the batch_lei_lookup handler.
func TestHandleBatchLEILookup(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	t.Run("successful batch lookup", func(t *testing.T) {
		ts.mux.HandleFunc("/lei-records", func(w http.ResponseWriter, _ *http.Request) {
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
		result, err := registry.handleBatchLEILookup(context.Background(), BatchLEIArgs{LEIs: "HWUPKR0MPOU8FGXBT394,7LTWFZYICNSX8D621K86"})
		if err != nil {
			t.Fatalf("handleBatchLEILookup returned error: %v", err)
		}
		if result.Requested != 2 {
			t.Errorf("Expected requested=2, got %d", result.Requested)
		}
	})

	t.Run("empty leis parameter", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		_, err := registry.handleBatchLEILookup(context.Background(), BatchLEIArgs{LEIs: ""})
		if err == nil {
			t.Error("Expected error for empty leis parameter")
		}
	})

	t.Run("invalid lei in batch", func(t *testing.T) {
		registry := newTestRegistry(ts.URL())
		_, err := registry.handleBatchLEILookup(context.Background(), BatchLEIArgs{LEIs: "HWUPKR0MPOU8FGXBT394,INVALID"})
		if err == nil {
			t.Error("Expected error for invalid lei in batch")
		}
	})
}

// TestEveryToolHasHandler asserts the dispatch map covers every AllTools entry,
// so a tool can never be advertised without a registered handler.
func TestEveryToolHasHandler(t *testing.T) {
	registry := newTestRegistry("http://example.invalid")
	for _, spec := range AllTools {
		if _, ok := registry.handlers[spec.Name]; !ok {
			t.Errorf("tool %q has no handler in the dispatch map", spec.Name)
		}
	}
	if len(registry.handlers) != len(AllTools) {
		t.Errorf("handler count %d != AllTools count %d (orphaned handler?)", len(registry.handlers), len(AllTools))
	}
}
