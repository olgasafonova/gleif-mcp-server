package gleif

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestValidateLEI_DoesNotCacheTransientError is the regression test for the
// cache poisoning finding. Before the fix, ANY error from GetLEI (including
// transient 5xx, timeouts, rate-limit retry exhaustion) was cached as
// "LEI not found" for the configured ValidationTTL (24 hours).
//
// An attacker who could briefly disrupt the upstream path (saturate the
// shared rate.Limiter, trigger a transient 5xx) could mark any LEI as
// invalid for the rest of the day, breaking downstream business logic
// that gates on validate_lei.
//
// After the fix: only definitive 404 responses (NewNotFoundError code
// ErrCodeNotFound) get cached as negative. 5xx and other transient errors
// flow through without poisoning the cache.
func TestValidateLEI_DoesNotCacheTransientError(t *testing.T) {
	const validLEI = "529900T8BM49AURSDO55"

	// First call returns 503 (transient). Second call would return 200 if
	// the cache wasn't poisoned by the first.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Subsequent calls return 200 with a minimal valid record envelope.
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"type":"lei-records","id":"` + validLEI + `","attributes":{}}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	// First call: 503 transient error. validate_lei should NOT cache the
	// negative result.
	res, err := c.ValidateLEI(context.Background(), validLEI)
	if err != nil {
		t.Fatalf("first ValidateLEI returned err: %v", err)
	}
	if res.Valid {
		t.Errorf("first call: expected Valid=false (transient error), got true")
	}

	// Second call: should hit the upstream again (NOT the poisoned cache)
	// and return Valid=true.
	res2, err := c.ValidateLEI(context.Background(), validLEI)
	if err != nil {
		t.Fatalf("second ValidateLEI returned err: %v", err)
	}
	if !res2.Valid {
		t.Errorf("cache-poisoning regression: second call returned Valid=false; expected fresh GLEIF call after transient error. message=%q", res2.Message)
	}

	// Sanity: upstream was hit twice (no cache hit on second call after
	// transient error).
	if h := hits.Load(); h < 2 {
		t.Errorf("expected at least 2 upstream hits, got %d", h)
	}
}

// TestValidateLEI_CachesDefinitiveNotFound asserts the success path: a real
// 404 from GLEIF still gets cached as Valid=false (no upstream call on the
// second invocation).
func TestValidateLEI_CachesDefinitiveNotFound(t *testing.T) {
	const validShapeLEI = "529900T8BM49AURSDO55"

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cache.Clear()

	res, err := c.ValidateLEI(context.Background(), validShapeLEI)
	if err != nil {
		t.Fatalf("first ValidateLEI: %v", err)
	}
	if res.Valid {
		t.Errorf("expected Valid=false on 404, got true")
	}

	res2, err := c.ValidateLEI(context.Background(), validShapeLEI)
	if err != nil {
		t.Fatalf("second ValidateLEI: %v", err)
	}
	if res2.Valid {
		t.Errorf("expected Valid=false from cached 404, got true")
	}

	// 404 should have been cached after the first call; second call must
	// not touch upstream.
	if h := hits.Load(); h != 1 {
		t.Errorf("expected exactly 1 upstream hit on cached 404, got %d", h)
	}
}
