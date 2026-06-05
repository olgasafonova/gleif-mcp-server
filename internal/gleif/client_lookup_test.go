package gleif

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
)

// newMockClient stands up a client wired to the given handler, with caching
// off by default so request-building behavior is exercised on every call.
func newMockClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(Config{
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		RateLimit:   1000,
		BurstSize:   100,
		MaxRetries:  0,
		EnableCache: false,
	}, logger)
	return client, server
}

// newMockClientCached is like newMockClient but with caching enabled.
func newMockClientCached(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(Config{
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		RateLimit:   1000,
		BurstSize:   100,
		MaxRetries:  0,
		EnableCache: true,
	}, logger)
	return client, server
}

// TestGetLEISuccess verifies the single-LEI fetch parses the record and stamps
// the LEI from the response ID, and that the request hits the lei-records path.
func TestGetLEISuccess(t *testing.T) {
	var gotPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := SingleResponse[LEIRecord]{
			Data: DataItem[LEIRecord]{
				ID:   "HWUPKR0MPOU8FGXBT394",
				Type: "lei-records",
				Attributes: LEIRecord{
					Entity:       Entity{LegalName: LegalName{Name: "Apple Inc."}, Status: "ACTIVE"},
					Registration: Registration{Status: "ISSUED"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	record, err := client.GetLEI(context.Background(), LEI("HWUPKR0MPOU8FGXBT394"))
	if err != nil {
		t.Fatalf("GetLEI failed: %v", err)
	}
	if record.LEI != "HWUPKR0MPOU8FGXBT394" {
		t.Errorf("LEI = %q, want HWUPKR0MPOU8FGXBT394", record.LEI)
	}
	if record.Entity.LegalName.Name != "Apple Inc." {
		t.Errorf("LegalName = %q, want Apple Inc.", record.Entity.LegalName.Name)
	}
	if !strings.Contains(gotPath, "/lei-records/HWUPKR0MPOU8FGXBT394") {
		t.Errorf("request path = %q, want it to contain /lei-records/<lei>", gotPath)
	}
}

// TestGetLEIServesFromCache verifies a warm cache short-circuits the upstream.
func TestGetLEIServesFromCache(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := SingleResponse[LEIRecord]{
			Data: DataItem[LEIRecord]{
				ID:         "HWUPKR0MPOU8FGXBT394",
				Attributes: LEIRecord{Entity: Entity{LegalName: LegalName{Name: "Apple Inc."}}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	lei := LEI("HWUPKR0MPOU8FGXBT394")
	if _, err := client.GetLEI(context.Background(), lei); err != nil {
		t.Fatalf("first GetLEI failed: %v", err)
	}
	if _, err := client.GetLEI(context.Background(), lei); err != nil {
		t.Fatalf("second GetLEI failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1 (second should hit cache)", calls)
	}
}

// TestGetLEINotFound verifies a 404 surfaces a not_found APIError.
func TestGetLEINotFound(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetLEI(context.Background(), LEI("HWUPKR0MPOU8FGXBT394"))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var apiErr *APIError
	if !isAPIError(err, &apiErr) || apiErr.Code != ErrCodeNotFound {
		t.Errorf("error = %v, want not_found APIError", err)
	}
}

// TestGetBatchLEIValidation verifies batch size guards before any request.
func TestGetBatchLEIValidation(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for invalid batch sizes")
	})

	t.Run("empty list", func(t *testing.T) {
		_, err := client.GetBatchLEI(context.Background(), nil)
		if err == nil {
			t.Error("expected error for empty batch")
		}
	})

	t.Run("over max", func(t *testing.T) {
		leis := make([]LEI, maxBatchSize+1)
		for i := range leis {
			leis[i] = LEI("HWUPKR0MPOU8FGXBT394")
		}
		_, err := client.GetBatchLEI(context.Background(), leis)
		if err == nil {
			t.Error("expected error for oversized batch")
		}
	})
}

// TestGetBatchLEIFetchesAndStamps verifies the batch fetch maps response items
// to records, stamping LEI from each item ID.
func TestGetBatchLEIFetchesAndStamps(t *testing.T) {
	var gotQuery string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{
				{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{Entity: Entity{LegalName: LegalName{Name: "Apple Inc."}}}},
				{ID: "7LTWFZYICNSX8D621K86", Attributes: LEIRecord{Entity: Entity{LegalName: LegalName{Name: "Deutsche Bank AG"}}}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	records, err := client.GetBatchLEI(context.Background(), []LEI{
		"HWUPKR0MPOU8FGXBT394", "7LTWFZYICNSX8D621K86",
	})
	if err != nil {
		t.Fatalf("GetBatchLEI failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].LEI != "HWUPKR0MPOU8FGXBT394" || records[1].LEI != "7LTWFZYICNSX8D621K86" {
		t.Errorf("LEIs not stamped from item IDs: %+v", records)
	}
	if !strings.Contains(gotQuery, "filter%5Blei%5D=") {
		t.Errorf("query = %q, want a filter[lei] parameter", gotQuery)
	}
}

// TestGetBatchLEIAllCached verifies a fully-cached batch makes no upstream
// request.
func TestGetBatchLEIAllCached(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := APIResponse[LEIRecord]{
			Data: []DataItem[LEIRecord]{
				{ID: "HWUPKR0MPOU8FGXBT394", Attributes: LEIRecord{}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	lei := LEI("HWUPKR0MPOU8FGXBT394")
	// Prime the cache with one fetch.
	if _, err := client.GetBatchLEI(context.Background(), []LEI{lei}); err != nil {
		t.Fatalf("priming GetBatchLEI failed: %v", err)
	}
	callsAfterPrime := calls

	// Second batch for the same LEI should be served entirely from cache.
	if _, err := client.GetBatchLEI(context.Background(), []LEI{lei}); err != nil {
		t.Fatalf("cached GetBatchLEI failed: %v", err)
	}
	if calls != callsAfterPrime {
		t.Errorf("upstream called again (%d -> %d); fully-cached batch should skip the fetch", callsAfterPrime, calls)
	}
}
