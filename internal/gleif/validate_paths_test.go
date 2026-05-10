package gleif

import (
	"strings"
	"testing"
)

// TestValidateIssuerID_RejectsInjection is the regression test for the
// path-injection finding. Each payload must be rejected before any URL is
// constructed.
func TestValidateIssuerID_RejectsInjection(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"path traversal", "../lei-records/SOMELEI"},
		{"slash extension", "valid/lei-records/extra"},
		{"query injection", "valid?include=*"},
		{"hash anchor", "valid#anchor"},
		{"semicolon param", "valid;param"},
		{"ampersand", "valid&query=injected"},
		{"newline injection", "valid\nX-Inject: 1"},
		{"null byte", "valid\x00.attacker"},
		{"empty", ""},
		{"only whitespace", "   "},
		{"too short", "abc"},
		{"oversize", strings.Repeat("A", 33)},
		{"lowercase letters preserved", ""}, // empty intentional; covered by "empty"
	}

	for _, c := range cases {
		if c.name == "lowercase letters preserved" {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateIssuerID(c.id); err == nil {
				t.Errorf("ValidateIssuerID(%q) returned nil error; expected rejection", c.id)
			}
		})
	}
}

// TestValidateIssuerID_AcceptsRealisticIDs guards the success path. GLEIF
// issuer IDs (LOU codes) in the wild are alphanumeric tokens, often the LEI
// of the issuing organization (20 chars) or shorter historic codes.
func TestValidateIssuerID_AcceptsRealisticIDs(t *testing.T) {
	realisticIDs := []string{
		"EVK05KS7XY1DEII3R011", // example from tool spec — full LEI shape
		"529900T8BM49AURSDO55",
		"5493001KJTIIGC8Y1R12",
		"ABCD1234EFGH",
		"ABCD",                             // 4 chars (lower bound)
		"529900T8BM49AURSDO55ABCDEFGHIJKL", // 32 chars (upper bound)
	}

	for _, id := range realisticIDs {
		t.Run(id, func(t *testing.T) {
			if err := ValidateIssuerID(id); err != nil {
				t.Errorf("ValidateIssuerID(%q) returned %v; expected nil for realistic ID", id, err)
			}
		})
	}
}

// TestParseIssuerID_RejectsInjection asserts ParseIssuerID rejects path-
// injection payloads before any URL is constructed. The typed signature on
// GetLEIIssuer means a malformed string can't reach the URL builder; the
// type system enforces what was previously a runtime check.
func TestParseIssuerID_RejectsInjection(t *testing.T) {
	maliciousID := "../lei-records/SOMELEI"
	_, err := ParseIssuerID(maliciousID)

	if err == nil {
		t.Fatal("ParseIssuerID accepted path-injection payload; expected rejection")
	}
	if !strings.Contains(err.Error(), "issuer_id") {
		t.Errorf("error did not mention issuer_id: %v", err)
	}
	if strings.Contains(err.Error(), "lei-records") {
		t.Errorf("error leaked the injected payload: %v", err)
	}
}

// TestParseISIN_QuerySmugglingRejected verifies the ISIN format check blocks
// payloads that would smuggle additional query parameters. With typed
// signatures, ParseISIN is the only gate; SearchByISIN can no longer be
// called with an unparsed string.
func TestParseISIN_QuerySmugglingRejected(t *testing.T) {
	// 12-character payload that would have smuggled query params via
	// the previous fmt.Sprintf path: "X&y=z&w=qABC".
	smugglingPayload := "X&y=z&w=qABC"
	_, err := ParseISIN(smugglingPayload)

	if err == nil {
		t.Fatal("ParseISIN accepted smuggling payload; expected rejection")
	}
	if !strings.Contains(err.Error(), "ISIN") {
		t.Errorf("error did not mention ISIN: %v", err)
	}
}
