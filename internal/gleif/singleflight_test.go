package gleif

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGetLEI_SingleflightCollapsesConcurrentMisses is the regression test
// for the cold-cache rate-limit-amplification finding from the 2026-05-02
// Carlini scaffold sweep.
//
// Before the fix: 50 concurrent callers asking for the same uncached LEI
// each consumed a slot in the shared rate limiter (50 rpm, burst 10),
// trivially draining it. Combined with the (now-fixed) validation cache
// that treated every error as "not found" for 24 hours, this turned a
// momentary burst into a day-long cache poison reachable from a single
// client.
//
// After the fix: GetLEI wraps the cold-path fetch with a singleflight.Group.
// One leader does the upstream round-trip; followers receive the leader's
// result. Upstream sees 1 request, not N.
func TestGetLEI_SingleflightCollapsesConcurrentMisses(t *testing.T) {
	const lei = "529900T8BM49AURSDO55"
	const concurrentCallers = 50

	// Test server: counts requests, blocks until released so all callers
	// pile up before the leader's response unblocks them.
	var requestCount int32
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		<-release
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":{"type":"lei-records","id":"` + lei + `","attributes":{"lei":"` + lei +
			`","entity":{"legalName":{"name":"Acme Corp","language":"en"},"legalAddress":{"city":"X","country":"US"},"headquartersAddress":{"city":"X","country":"US"},"jurisdiction":"US","status":"ACTIVE"},"registration":{"initialRegistrationDate":"2020-01-01T00:00:00Z","lastUpdateDate":"2024-01-01T00:00:00Z","status":"ISSUED","nextRenewalDate":"2025-01-01T00:00:00Z","managingLou":"TESTLOU"}}}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	// Spawn N callers; they all block in the singleflight Do until the
	// leader's response unblocks via release.
	results := make([]*LEIRecord, concurrentCallers)
	errs := make([]error, concurrentCallers)
	var wg sync.WaitGroup
	wg.Add(concurrentCallers)

	for i := range concurrentCallers {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = c.GetLEI(context.Background(), lei)
		}(i)
	}

	// Give callers time to all enter the singleflight Do.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	hits := atomic.LoadInt32(&requestCount)
	if hits != 1 {
		t.Errorf("singleflight regression: %d concurrent callers caused %d upstream requests, want 1", concurrentCallers, hits)
	}

	// All callers must have received a successful, valid result.
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: unexpected error: %v", i, err)
		}
	}
	for i, r := range results {
		if r == nil || r.LEI != lei {
			t.Errorf("caller %d: got %+v, want LEI %q", i, r, lei)
		}
	}
}

// TestGetLEI_SingleflightDoesNotShareErrors verifies that an upstream error
// is propagated to all in-flight callers (they all see the same error) but
// a subsequent call after the failure can succeed if the upstream recovers.
// Singleflight's behavior: the leader's error is returned to all followers
// of the same in-flight call, but the singleflight key is released after
// the leader returns, so the next call starts a fresh upstream request.
func TestGetLEI_SingleflightDoesNotShareErrors(t *testing.T) {
	const lei = "529900T8BM49AURSDO55"

	var requestCount int32
	failFirst := make(chan struct{})
	close(failFirst) // first request fails

	var firstFailed atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if firstFailed.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500"}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":{"type":"lei-records","id":"` + lei + `","attributes":{"lei":"` + lei +
			`","entity":{"legalName":{"name":"Acme","language":"en"},"legalAddress":{"city":"X","country":"US"},"headquartersAddress":{"city":"X","country":"US"},"jurisdiction":"US","status":"ACTIVE"},"registration":{"initialRegistrationDate":"2020-01-01T00:00:00Z","lastUpdateDate":"2024-01-01T00:00:00Z","status":"ISSUED","nextRenewalDate":"2025-01-01T00:00:00Z","managingLou":"TESTLOU"}}}}`))
	}))
	defer srv.Close()

	// Use a client with no retries to keep the first failure observable.
	c := newTestClient(srv.URL)
	c.config.MaxRetries = 0

	if _, err := c.GetLEI(context.Background(), lei); err == nil {
		t.Fatal("first call: expected error from 500, got nil")
	}

	// Second call must NOT reuse the failed singleflight result; it must
	// hit upstream again (which now succeeds).
	r, err := c.GetLEI(context.Background(), lei)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if r == nil || r.LEI != lei {
		t.Errorf("second call: got %+v, want LEI %q", r, lei)
	}

	// Two upstream requests total: 1 failed, 1 succeeded.
	if hits := atomic.LoadInt32(&requestCount); hits != 2 {
		t.Errorf("expected 2 upstream requests (1 fail + 1 success), got %d", hits)
	}
}
