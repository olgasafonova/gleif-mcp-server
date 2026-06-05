package gleif

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestValidateLEIInvalidFormat verifies a malformed LEI returns an invalid
// result without contacting the API.
func TestValidateLEIInvalidFormat(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("API must not be called for an unparseable LEI")
	})

	result, err := client.ValidateLEI(context.Background(), "NOT-AN-LEI")
	if err != nil {
		t.Fatalf("ValidateLEI returned error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for invalid format")
	}
	if !strings.Contains(result.Message, "format") {
		t.Errorf("Message = %q, want a format message", result.Message)
	}
}

// TestValidateLEIBadCheckDigits verifies a well-formed but check-digit-invalid
// LEI returns invalid without contacting the API.
func TestValidateLEIBadCheckDigits(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("API must not be called when check digits fail")
	})

	// Valid 20-char format, wrong check digits.
	result, err := client.ValidateLEI(context.Background(), "HWUPKR0MPOU8FGXBT300")
	if err != nil {
		t.Fatalf("ValidateLEI returned error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for bad check digits")
	}
	if !strings.Contains(result.Message, "check digit") {
		t.Errorf("Message = %q, want a check-digit message", result.Message)
	}
}

// TestValidateLEISuccess verifies a real LEI confirmed by the API yields a
// valid result with status fields populated.
func TestValidateLEISuccess(t *testing.T) {
	renewal := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := SingleResponse[LEIRecord]{
			Data: DataItem[LEIRecord]{
				ID: "HWUPKR0MPOU8FGXBT394",
				Attributes: LEIRecord{
					Entity: Entity{Status: "ACTIVE"},
					Registration: Registration{
						Status:          "ISSUED",
						NextRenewalDate: renewal,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	result, err := client.ValidateLEI(context.Background(), "HWUPKR0MPOU8FGXBT394")
	if err != nil {
		t.Fatalf("ValidateLEI returned error: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected Valid=true for an API-confirmed LEI")
	}
	if result.Status != "ISSUED" {
		t.Errorf("Status = %q, want ISSUED", result.Status)
	}
	if result.EntityStatus != "ACTIVE" {
		t.Errorf("EntityStatus = %q, want ACTIVE", result.EntityStatus)
	}
	if result.NextRenewal != "2025-08-15" {
		t.Errorf("NextRenewal = %q, want 2025-08-15", result.NextRenewal)
	}
}

// TestValidateLEINotFound verifies a 404 from the API produces an invalid
// result with a not-found message, and that the negative result is cached.
func TestValidateLEINotFound(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	const lei = "HWUPKR0MPOU8FGXBT394"
	result, err := client.ValidateLEI(context.Background(), lei)
	if err != nil {
		t.Fatalf("ValidateLEI returned error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for not-found LEI")
	}
	if !strings.Contains(result.Message, "not found") {
		t.Errorf("Message = %q, want a not-found message", result.Message)
	}

	// Second call should be served from the validation cache (404 is cached).
	if _, err := client.ValidateLEI(context.Background(), lei); err != nil {
		t.Fatalf("second ValidateLEI failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("API called %d times, want 1 (404 result should be cached)", calls)
	}
}

// TestValidateLEITransientErrorNotCached verifies a transient (5xx) error does
// not get cached, so a later call re-attempts the API.
func TestValidateLEITransientErrorNotCached(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})

	const lei = "HWUPKR0MPOU8FGXBT394"
	result, err := client.ValidateLEI(context.Background(), lei)
	if err != nil {
		t.Fatalf("ValidateLEI returned error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false when validation is unavailable")
	}
	if !strings.Contains(result.Message, "unavailable") {
		t.Errorf("Message = %q, want an unavailable message", result.Message)
	}

	// Second call must re-attempt: the transient result was not cached.
	if _, err := client.ValidateLEI(context.Background(), lei); err != nil {
		t.Fatalf("second ValidateLEI failed: %v", err)
	}
	if calls < 2 {
		t.Errorf("API called %d times, want >= 2 (transient error must not cache)", calls)
	}
}

// TestValidateLEIServesFromCache verifies a cached validation result short-
// circuits the whole flow on a repeat call.
func TestValidateLEIServesFromCache(t *testing.T) {
	var calls int
	client, _ := newMockClientCached(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := SingleResponse[LEIRecord]{
			Data: DataItem[LEIRecord]{
				ID:         "HWUPKR0MPOU8FGXBT394",
				Attributes: LEIRecord{Registration: Registration{Status: "ISSUED"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	const lei = "HWUPKR0MPOU8FGXBT394"
	if _, err := client.ValidateLEI(context.Background(), lei); err != nil {
		t.Fatalf("first ValidateLEI failed: %v", err)
	}
	callsAfterFirst := calls
	if _, err := client.ValidateLEI(context.Background(), lei); err != nil {
		t.Fatalf("second ValidateLEI failed: %v", err)
	}
	if calls != callsAfterFirst {
		t.Errorf("API called again (%d -> %d); cached validation should short-circuit", callsAfterFirst, calls)
	}
}
