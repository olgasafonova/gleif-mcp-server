package gleif

import (
	"regexp"
	"strings"
)

// Domain identifier types. These are string-underlying wrappers whose only
// constructor (Parse*) runs the format validation that the GLEIF API expects.
// Once a value of one of these types exists, downstream code can interpolate
// it into a URL or pass it to the API client without re-validating.
//
// The validation lives at the MCP boundary: handlers parse caller-supplied
// strings exactly once, and the rest of the call graph is type-safe. This
// closes the Primitive Obsession smell that CodeScene flagged on the prior
// shape (client methods taking bare `string` and re-running the regex check
// at every call site).
type (
	LEI      string
	BIC      string
	ISIN     string
	Country  string
	IssuerID string
)

const (
	leiPattern      = `^[A-Z0-9]{4}[A-Z0-9]{2}[A-Z0-9]{12}[0-9]{2}$`
	bicPattern      = `^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?$`
	isinPattern     = `^[A-Z]{2}[A-Z0-9]{10}$`
	countryPattern  = `^[A-Z]{2}$`
	issuerIDPattern = `^[A-Z0-9]{4,32}$`
)

var (
	leiRegex      = regexp.MustCompile(leiPattern)
	bicRegex      = regexp.MustCompile(bicPattern)
	isinRegex     = regexp.MustCompile(isinPattern)
	countryRegex  = regexp.MustCompile(countryPattern)
	issuerIDRegex = regexp.MustCompile(issuerIDPattern)
)

// LEIPattern, BICPattern, ISINPattern, CountryPattern, IssuerIDPattern remain
// exported for external consumers (tests, ToolSpec definitions). They mirror
// the unexported lowercase constants above.
const (
	LEIPattern      = leiPattern
	BICPattern      = bicPattern
	ISINPattern     = isinPattern
	CountryPattern  = countryPattern
	IssuerIDPattern = issuerIDPattern
)

// normalize uppercases and trims whitespace. The GLEIF API treats identifiers
// case-insensitively but rejects whitespace.
func normalize(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// parseIdentifier is the shared shape behind every Parse* constructor:
// normalize, reject empty, regex-check, and convert to the target type.
// Generics let each typed wrapper reuse the body without per-type duplication.
func parseIdentifier[T ~string](raw, label, formatReason string, re *regexp.Regexp) (T, error) {
	s := normalize(raw)
	if s == "" {
		return "", NewInvalidFormatError(label, "is required")
	}
	if !re.MatchString(s) {
		return "", NewInvalidFormatError(label, formatReason)
	}
	return T(s), nil
}

// ParseLEI validates and constructs an LEI from a caller-supplied string.
// Format only (regex). Check-digit validation lives on (LEI).HasValidCheckDigits
// because callers can trust GLEIF's API to reject malformed records, but the
// validate_lei tool wants to surface check-digit failures explicitly.
func ParseLEI(s string) (LEI, error) {
	return parseIdentifier[LEI](s, "LEI", "must be 20 alphanumeric characters", leiRegex)
}

// ParseBIC validates and constructs a BIC from a caller-supplied string.
func ParseBIC(s string) (BIC, error) {
	return parseIdentifier[BIC](s, "BIC", "must be 8 or 11 characters: 4 letters + 2 letters + 2 alphanumeric, optional 3 alphanumeric", bicRegex)
}

// ParseISIN validates and constructs an ISIN from a caller-supplied string.
func ParseISIN(s string) (ISIN, error) {
	return parseIdentifier[ISIN](s, "ISIN", "must be 2 letters followed by 10 alphanumeric characters (12 total)", isinRegex)
}

// ParseCountry validates and constructs a Country from an ISO 3166-1 alpha-2 code.
func ParseCountry(s string) (Country, error) {
	return parseIdentifier[Country](s, "country", "must be a 2-letter ISO 3166-1 alpha-2 code", countryRegex)
}

// ParseIssuerID validates and constructs an IssuerID. Without this check, an
// adversarial caller could send issuer_id="../lei-records/SOMELEI" and pivot
// the URL away from /lei-issuers/ to a different GLEIF endpoint.
func ParseIssuerID(s string) (IssuerID, error) {
	return parseIdentifier[IssuerID](s, "issuer_id", "must be 4-32 alphanumeric characters", issuerIDRegex)
}

// ValidateIssuerID is a thin wrapper kept for backward compatibility with
// any external caller; new code should use ParseIssuerID.
func ValidateIssuerID(id string) error {
	_, err := ParseIssuerID(id)
	return err
}

// HasValidCheckDigits validates the ISO 17442 check digits on an LEI.
// The format-validity precondition (20 chars, alphanumeric) is enforced at
// construction time by ParseLEI, so this method only re-runs the mod-97 check.
func (l LEI) HasValidCheckDigits() bool {
	s := string(l)
	if len(s) != 20 {
		return false
	}

	remainder := 0
	for _, ch := range s {
		digit, ok := leiCharToDigit(ch)
		if !ok {
			return false
		}
		remainder = foldDigit(remainder, digit)
	}
	return remainder == 1
}

// leiCharToDigit maps an LEI character to its mod-97 numeric value.
// Letters A-Z become 10-35; digits 0-9 keep their value. Anything else is
// rejected. Extracted from HasValidCheckDigits to flatten the per-character
// branching that CodeScene flagged as a Complex Conditional.
func leiCharToDigit(ch rune) (int, bool) {
	switch {
	case ch >= 'A' && ch <= 'Z':
		return int(ch-'A') + 10, true
	case ch >= '0' && ch <= '9':
		return int(ch - '0'), true
	default:
		return 0, false
	}
}

// foldDigit incorporates the next digit into the running mod-97 remainder.
// Two-digit values (letters) shift by 100; single digits shift by 10.
func foldDigit(remainder, digit int) int {
	if digit >= 10 {
		return (remainder*100 + digit) % 97
	}
	return (remainder*10 + digit) % 97
}
