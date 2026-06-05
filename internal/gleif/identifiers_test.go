package gleif

import "testing"

// TestNormalize verifies the shared normalize helper uppercases and trims.
func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already normalized", "ABC123", "ABC123"},
		{"lowercase", "abc123", "ABC123"},
		{"leading/trailing space", "  abc  ", "ABC"},
		{"tabs and newlines", "\tabc\n", "ABC"},
		{"mixed case with inner space kept", "a b", "A B"},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalize(tt.in); got != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseLEI verifies LEI construction normalizes input and rejects bad
// formats with an invalid_format APIError.
func TestParseLEI(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantErr   bool
		wantValue LEI
	}{
		{"valid uppercase", "HWUPKR0MPOU8FGXBT394", false, "HWUPKR0MPOU8FGXBT394"},
		{"valid lowercase normalized", "hwupkr0mpou8fgxbt394", false, "HWUPKR0MPOU8FGXBT394"},
		{"valid with surrounding space", "  HWUPKR0MPOU8FGXBT394  ", false, "HWUPKR0MPOU8FGXBT394"},
		{"empty", "", true, ""},
		{"too short", "HWUPKR0MPOU8FGXBT39", true, ""},
		{"too long", "HWUPKR0MPOU8FGXBT3940", true, ""},
		{"non-numeric check digits", "HWUPKR0MPOU8FGXBT3XX", true, ""},
		{"special char", "HWUPKR0MPOU8FGX-T394", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLEI(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLEI(%q) = nil error, want error", tt.in)
				}
				assertInvalidFormat(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseLEI(%q) returned error: %v", tt.in, err)
			}
			if got != tt.wantValue {
				t.Errorf("ParseLEI(%q) = %q, want %q", tt.in, got, tt.wantValue)
			}
		})
	}
}

// TestParseBIC verifies BIC construction for 8 and 11 char codes.
func TestParseBIC(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    BIC
	}{
		{"8-char", "DEUTDEFF", false, "DEUTDEFF"},
		{"11-char", "DEUTDEFFXXX", false, "DEUTDEFFXXX"},
		{"lowercase normalized", "deutdeff", false, "DEUTDEFF"},
		{"7 chars", "DEUTDEF", true, ""},
		{"9 chars", "DEUTDEFFF", true, ""},
		{"10 chars", "DEUTDEFFFX", true, ""},
		{"12 chars", "DEUTDEFFXXXX", true, ""},
		{"digits in country part", "DEU1DEFF", true, ""},
		{"empty", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBIC(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseBIC(%q) = nil error, want error", tt.in)
				}
				assertInvalidFormat(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseBIC(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseBIC(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseISIN verifies ISIN construction (2 letters + 10 alphanumeric).
func TestParseISIN(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    ISIN
	}{
		{"valid US ISIN", "US0378331005", false, "US0378331005"},
		{"valid German ISIN", "DE0007164600", false, "DE0007164600"},
		{"lowercase normalized", "us0378331005", false, "US0378331005"},
		{"11 chars", "US037833100", true, ""},
		{"13 chars", "US03783310050", true, ""},
		{"leading digits", "10378331005A", true, ""},
		{"empty", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseISIN(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseISIN(%q) = nil error, want error", tt.in)
				}
				assertInvalidFormat(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseISIN(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseISIN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseCountry verifies ISO 3166-1 alpha-2 country code construction.
func TestParseCountry(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    Country
	}{
		{"valid US", "US", false, "US"},
		{"valid DE", "DE", false, "DE"},
		{"lowercase normalized", "us", false, "US"},
		{"3 chars", "USA", true, ""},
		{"1 char", "U", true, ""},
		{"digit", "U1", true, ""},
		{"empty", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCountry(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseCountry(%q) = nil error, want error", tt.in)
				}
				assertInvalidFormat(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseCountry(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseCountry(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseIssuerID verifies issuer ID construction and the path-traversal
// rejection that the regex enforces.
func TestParseIssuerID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    IssuerID
	}{
		{"valid 4-char", "EVK0", false, "EVK0"},
		{"valid long alphanumeric", "529900T8BM49AURSDO55", false, "529900T8BM49AURSDO55"},
		{"lowercase normalized", "evk0", false, "EVK0"},
		{"path traversal attempt", "../lei-records/SOMELEI", true, ""},
		{"too short", "ABC", true, ""},
		{"slash", "ABC/DEF", true, ""},
		{"empty", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIssuerID(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseIssuerID(%q) = nil error, want error", tt.in)
				}
				assertInvalidFormat(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseIssuerID(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseIssuerID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestValidateIssuerID verifies the backward-compatible wrapper delegates to
// ParseIssuerID for its error decision.
func TestValidateIssuerID(t *testing.T) {
	if err := ValidateIssuerID("EVK0"); err != nil {
		t.Errorf("ValidateIssuerID(valid) returned error: %v", err)
	}
	if err := ValidateIssuerID("../etc"); err == nil {
		t.Error("ValidateIssuerID(invalid) returned nil, want error")
	}
}

// TestHasValidCheckDigits verifies the ISO 17442 mod-97 check.
func TestHasValidCheckDigits(t *testing.T) {
	tests := []struct {
		name string
		lei  LEI
		want bool
	}{
		{"valid Apple", "HWUPKR0MPOU8FGXBT394", true},
		{"valid Deutsche Bank", "7LTWFZYICNSX8D621K86", true},
		{"valid Google", "5493006MHB84DD0ZWV18", true},
		{"wrong check digit", "HWUPKR0MPOU8FGXBT395", false},
		{"too short", "HWUPKR0MPOU8FGXBT39", false},
		{"too long", "HWUPKR0MPOU8FGXBT3940", false},
		{"empty", "", false},
		{"invalid char", "HWUPKR0MPOU8FGX!T394", false},
		{"lowercase fails", "hwupkr0mpou8fgxbt394", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lei.HasValidCheckDigits(); got != tt.want {
				t.Errorf("LEI(%q).HasValidCheckDigits() = %v, want %v", tt.lei, got, tt.want)
			}
		})
	}
}

// TestLEICharToDigit verifies the per-character mod-97 mapping.
func TestLEICharToDigit(t *testing.T) {
	tests := []struct {
		ch       rune
		wantVal  int
		wantOK   bool
		wantName string
	}{
		{'0', 0, true, "digit 0"},
		{'9', 9, true, "digit 9"},
		{'A', 10, true, "letter A"},
		{'Z', 35, true, "letter Z"},
		{'a', 0, false, "lowercase rejected"},
		{'-', 0, false, "dash rejected"},
		{' ', 0, false, "space rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			val, ok := leiCharToDigit(tt.ch)
			if ok != tt.wantOK {
				t.Fatalf("leiCharToDigit(%q) ok = %v, want %v", tt.ch, ok, tt.wantOK)
			}
			if ok && val != tt.wantVal {
				t.Errorf("leiCharToDigit(%q) = %d, want %d", tt.ch, val, tt.wantVal)
			}
		})
	}
}

// TestFoldDigit verifies the running mod-97 fold for single and two-digit
// values.
func TestFoldDigit(t *testing.T) {
	tests := []struct {
		name      string
		remainder int
		digit     int
		want      int
	}{
		{"single digit shifts by 10", 0, 5, 5},
		{"single digit with carry", 9, 7, (9*10 + 7) % 97},
		{"two-digit letter shifts by 100", 0, 10, 10},
		{"two-digit with carry wraps", 50, 35, (50*100 + 35) % 97},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := foldDigit(tt.remainder, tt.digit); got != tt.want {
				t.Errorf("foldDigit(%d, %d) = %d, want %d", tt.remainder, tt.digit, got, tt.want)
			}
		})
	}
}

// TestExportedPatternsMirrorUnexported guards against the exported pattern
// constants drifting from the unexported ones the regexes compile from.
func TestExportedPatternsMirrorUnexported(t *testing.T) {
	pairs := []struct {
		name       string
		exported   string
		unexported string
	}{
		{"LEI", LEIPattern, leiPattern},
		{"BIC", BICPattern, bicPattern},
		{"ISIN", ISINPattern, isinPattern},
		{"Country", CountryPattern, countryPattern},
		{"IssuerID", IssuerIDPattern, issuerIDPattern},
	}
	for _, p := range pairs {
		if p.exported != p.unexported {
			t.Errorf("%sPattern exported %q != unexported %q", p.name, p.exported, p.unexported)
		}
	}
}

// assertInvalidFormat asserts err is an *APIError with the invalid_format code.
func assertInvalidFormat(t *testing.T, err error) {
	t.Helper()
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.Code != ErrCodeInvalidFormat {
		t.Errorf("error code = %q, want %q", apiErr.Code, ErrCodeInvalidFormat)
	}
}
