package gleif

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestAPIClientRefusesRedirectToInternalIP is the SSRF redirect-chain
// regression test. Before the fix, the http.Client had no CheckRedirect
// policy, so Go followed up to 10 redirects with no host allowlist. A
// misconfigured deployment combined with a wiki/proxy returning
// `Location: http://169.254.169.254/...` would pivot a GLEIF lookup into
// a fetch against cloud metadata or other link-local internal services,
// with the body landing in the agent context via the regular response
// path.
//
// After the fix: CheckRedirect returns http.ErrUseLastResponse, so the
// 3xx response surfaces to the caller without ever touching the redirect
// target. This test verifies the attacker server (stand-in for an
// internal IP) is never hit.
func TestAPIClientRefusesRedirectToInternalIP(t *testing.T) {
	var attackerHits int32
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attackerHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer attacker.Close()

	// Configured "GLEIF" server returns 302 redirecting to the attacker
	// (stand-in for 169.254.169.254 / metadata service).
	gleif := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", attacker.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer gleif.Close()

	c := newTestClient(gleif.URL)

	_, err := c.GetLEI(context.Background(), "529900T8BM49AURSDO55")
	if err == nil {
		t.Fatal("expected error from 302 (redirect refused), got nil")
	}

	if hits := atomic.LoadInt32(&attackerHits); hits != 0 {
		t.Errorf("SSRF regression: attacker server was hit %d times; CheckRedirect should have refused the redirect", hits)
	}

	// Sanity: error message must not echo the attacker URL (would be HG-2 regression too).
	if strings.Contains(err.Error(), attacker.URL) {
		t.Errorf("error message leaks attacker URL: %v", err)
	}
}

// TestAPIClientRefusesRedirectChain verifies that a multi-hop redirect chain
// is also refused at the first hop, not after N hops. Before the fix Go
// followed up to 10 redirects by default; after the fix, the first 3xx
// short-circuits.
func TestAPIClientRefusesRedirectChain(t *testing.T) {
	var hop1Hits, hop2Hits int32

	hop2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hop2Hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer hop2.Close()

	hop1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hop1Hits, 1)
		w.Header().Set("Location", hop2.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer hop1.Close()

	gleif := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", hop1.URL)
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer gleif.Close()

	c := newTestClient(gleif.URL)

	_, _ = c.GetLEI(context.Background(), "529900T8BM49AURSDO55")

	if h1 := atomic.LoadInt32(&hop1Hits); h1 != 0 {
		t.Errorf("redirect-chain regression: hop1 was hit %d times; first 3xx should short-circuit", h1)
	}
	if h2 := atomic.LoadInt32(&hop2Hits); h2 != 0 {
		t.Errorf("redirect-chain regression: hop2 was hit %d times; first 3xx should short-circuit", h2)
	}
}
