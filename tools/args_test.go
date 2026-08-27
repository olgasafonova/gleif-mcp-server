package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Tests for the typed Args/Result structs in args.go. These structs drive
// schema generation under the generic mcp.AddTool[Args, Result] path
// (registry_test.go covers schema resolution end-to-end); here we pin down the
// JSON wire shape: field tags, omitempty behaviour, and lossless round-trips.

func boolPtr(b bool) *bool { return &b }

// TestAutocompleteSchemaMatchesClientClamp pins the limit bound the tool
// schema advertises to the bound the client actually enforces
// (gleif.AutocompleteMaxLimit), so the two cannot drift apart again. The
// schema once promised 1-50 while the client clamped anything above 20 down
// to 10; an agent asking for 50 silently got 10.
func TestAutocompleteSchemaMatchesClientClamp(t *testing.T) {
	field, ok := reflect.TypeOf(AutocompleteArgs{}).FieldByName("Limit")
	if !ok {
		t.Fatal("AutocompleteArgs has no Limit field")
	}
	tag := field.Tag.Get("jsonschema")

	wantBound := fmt.Sprintf("1-%d", gleif.AutocompleteMaxLimit)
	if !strings.Contains(tag, wantBound) {
		t.Errorf("AutocompleteArgs.Limit jsonschema tag %q does not advertise the range %q enforced by gleif.AutocompleteMaxLimit", tag, wantBound)
	}

	// The client also uses the bound as the default for omitted/zero limits
	// (clampAutocompleteLimit), so the advertised default must match too.
	wantDefault := fmt.Sprintf("default %d", gleif.AutocompleteMaxLimit)
	if !strings.Contains(tag, wantDefault) {
		t.Errorf("AutocompleteArgs.Limit jsonschema tag %q does not advertise %q", tag, wantDefault)
	}
}

// TestArgsResultJSONRoundTrip marshals a populated instance of every Args and
// Result type, checks the expected JSON keys appear (and no others), and
// verifies unmarshalling the bytes back yields an identical value.
func TestArgsResultJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		wantKeys []string
	}{
		// --- Args structs ---
		{"LEILookupArgs", LEILookupArgs{LEI: "HWUPKR0MPOU8FGXBT394"},
			[]string{"lei"}},
		{"ValidateLEIArgs", ValidateLEIArgs{LEI: "not-a-lei"},
			[]string{"lei"}},
		{"BatchLEIArgs", BatchLEIArgs{LEIs: "HWUPKR0MPOU8FGXBT394,7LTWFZYICNSX8D621K86"},
			[]string{"leis"}},
		{"SearchEntityArgs", SearchEntityArgs{Query: "Deutsche Bank", Limit: 20, Page: 2, Fuzzy: boolPtr(false)},
			[]string{"query", "limit", "page", "fuzzy"}},
		{"SearchByBICArgs", SearchByBICArgs{BIC: "DEUTDEFF"},
			[]string{"bic"}},
		{"SearchByISINArgs", SearchByISINArgs{ISIN: "US0378331005"},
			[]string{"isin"}},
		{"SearchByCountryArgs", SearchByCountryArgs{Country: "DE", Limit: 50},
			[]string{"country", "limit"}},
		{"GetRelationshipsArgs", GetRelationshipsArgs{LEI: "HWUPKR0MPOU8FGXBT394", Type: "ultimate-parent"},
			[]string{"lei", "type"}},
		{"AutocompleteArgs", AutocompleteArgs{Prefix: "App", Limit: 5},
			[]string{"prefix", "limit"}},
		{"GetLEIIssuerArgs", GetLEIIssuerArgs{IssuerID: "EVK05KS7XY1DEII3R011"},
			[]string{"issuer_id"}},
		{"ListLEIIssuersArgs", ListLEIIssuersArgs{Limit: 200},
			[]string{"limit"}},
		{"GetReportingExceptionsArgs", GetReportingExceptionsArgs{LEI: "5493006MHB84DD0ZWV18"},
			[]string{"lei"}},

		// --- Result structs ---
		{"SimpleRecord", SimpleRecord{
			LEI: "HWUPKR0MPOU8FGXBT394", LegalName: "Apple Inc.", Country: "US",
			City: "Cupertino", Status: "ACTIVE", RegStatus: "ISSUED"},
			[]string{"lei", "legalName", "country", "city", "status", "regStatus"}},
		{"BatchResult", BatchResult{
			Requested: 2, Found: 1,
			Results:  []SimpleRecord{{LEI: "HWUPKR0MPOU8FGXBT394"}},
			NotFound: []string{"7LTWFZYICNSX8D621K86"}},
			[]string{"requested", "found", "results", "notFound"}},
		{"PageInfo", PageInfo{CurrentPage: 1, PerPage: 20, Total: 156, LastPage: 8},
			[]string{"currentPage", "perPage", "total", "lastPage"}},
		{"SearchEntityResult", SearchEntityResult{
			Count:      1,
			Results:    []SimpleRecord{{LEI: "HWUPKR0MPOU8FGXBT394"}},
			Pagination: &PageInfo{CurrentPage: 1, PerPage: 20, Total: 156, LastPage: 8},
			HasMore:    boolPtr(true)},
			[]string{"count", "results", "pagination", "hasMore"}},
		{"IDSearchResult", IDSearchResult{
			Found: true, Count: 1,
			Results: []SimpleRecord{{LEI: "7LTWFZYICNSX8D621K86"}},
			Message: "1 match"},
			[]string{"found", "count", "results", "message"}},
		{"CountryRecord", CountryRecord{
			LEI: "HWUPKR0MPOU8FGXBT394", LegalName: "Apple Inc.", City: "Cupertino", Status: "ACTIVE"},
			[]string{"lei", "legalName", "city", "status"}},
		{"CountrySearchResult", CountrySearchResult{
			Country: "US", Count: 1,
			Results: []CountryRecord{{LEI: "HWUPKR0MPOU8FGXBT394"}}},
			[]string{"country", "count", "results"}},
		{"RelationshipsResult", RelationshipsResult{
			LEI: "HWUPKR0MPOU8FGXBT394", Type: "direct-parent",
			Relationships: []gleif.Relationship{{RelationshipType: "IS_DIRECTLY_CONSOLIDATED_BY"}}},
			[]string{"lei", "type", "relationships"}},
		{"AutocompleteToolResult", AutocompleteToolResult{
			Prefix: "App",
			Suggestions: []gleif.AutocompleteResult{
				{LEI: "HWUPKR0MPOU8FGXBT394", LegalName: "Apple Inc.", Country: "US"}}},
			[]string{"prefix", "suggestions"}},
		{"IssuersResult", IssuersResult{
			Count: 1, Total: 39,
			Issuers:   []gleif.LEIIssuer{{ID: "EVK05KS7XY1DEII3R011", Name: "Bloomberg", Country: "US"}},
			Truncated: true},
			[]string{"count", "total", "issuers", "truncated"}},
		{"ReportingExceptionsResult", ReportingExceptionsResult{
			LEI: "5493006MHB84DD0ZWV18", Count: 1,
			Exceptions: []gleif.ReportingException{
				{LEI: "5493006MHB84DD0ZWV18", ExceptionCategory: "DIRECT_ACCOUNTING_CONSOLIDATION_PARENT", ExceptionReason: "NON_CONSOLIDATING"}}},
			[]string{"lei", "count", "exceptions"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var keys map[string]json.RawMessage
			if err := json.Unmarshal(data, &keys); err != nil {
				t.Fatalf("unmarshal into map: %v", err)
			}
			for _, key := range tc.wantKeys {
				if _, ok := keys[key]; !ok {
					t.Errorf("marshalled JSON missing key %q; got %s", key, data)
				}
			}
			if len(keys) != len(tc.wantKeys) {
				t.Errorf("expected %d keys %v, got %d in %s", len(tc.wantKeys), tc.wantKeys, len(keys), data)
			}

			out := reflect.New(reflect.TypeOf(tc.value))
			if err := json.Unmarshal(data, out.Interface()); err != nil {
				t.Fatalf("unmarshal round-trip: %v", err)
			}
			if got := out.Elem().Interface(); !reflect.DeepEqual(got, tc.value) {
				t.Errorf("round-trip mismatch:\n got  %#v\n want %#v", got, tc.value)
			}
		})
	}
}

// TestOmitemptyKeysAbsentWhenZero verifies every field declared with
// `,omitempty` in args.go is actually dropped from the wire when zero. Under
// jsonschema-go, omitempty is also what marks a field optional in the derived
// schema, so a missing tag would silently make an optional parameter required.
func TestOmitemptyKeysAbsentWhenZero(t *testing.T) {
	cases := []struct {
		name       string
		value      any
		absentKeys []string
	}{
		{"SearchEntityArgs", SearchEntityArgs{Query: "x"}, []string{"limit", "page", "fuzzy"}},
		{"SearchByCountryArgs", SearchByCountryArgs{Country: "US"}, []string{"limit"}},
		{"GetRelationshipsArgs", GetRelationshipsArgs{LEI: "x"}, []string{"type"}},
		{"AutocompleteArgs", AutocompleteArgs{Prefix: "xy"}, []string{"limit"}},
		{"ListLEIIssuersArgs", ListLEIIssuersArgs{}, []string{"limit"}},
		{"BatchResult", BatchResult{}, []string{"notFound"}},
		{"SearchEntityResult", SearchEntityResult{}, []string{"pagination", "hasMore"}},
		{"IDSearchResult", IDSearchResult{}, []string{"message"}},
		{"IssuersResult", IssuersResult{}, []string{"truncated"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(data, &keys); err != nil {
				t.Fatalf("unmarshal into map: %v", err)
			}
			for _, key := range tc.absentKeys {
				if _, ok := keys[key]; ok {
					t.Errorf("zero-value field %q should be omitted, got %s", key, data)
				}
			}
		})
	}
}

// TestArgsFieldTags reflects over every Args struct and enforces the tag
// convention documented at the top of args.go: each exported field carries a
// json tag (lowercase name) and a non-empty jsonschema tag, which
// jsonschema-go turns into the property description. The jsonschema_description
// tag used by some sibling servers no-ops silently at runtime, so this is the
// only place it fails loudly.
func TestArgsFieldTags(t *testing.T) {
	argsTypes := []any{
		LEILookupArgs{},
		ValidateLEIArgs{},
		BatchLEIArgs{},
		SearchEntityArgs{},
		SearchByBICArgs{},
		SearchByISINArgs{},
		SearchByCountryArgs{},
		GetRelationshipsArgs{},
		AutocompleteArgs{},
		GetLEIIssuerArgs{},
		ListLEIIssuersArgs{},
		GetReportingExceptionsArgs{},
	}

	for _, v := range argsTypes {
		typ := reflect.TypeOf(v)
		t.Run(typ.Name(), func(t *testing.T) {
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				jsonTag := field.Tag.Get("json")
				if jsonTag == "" || jsonTag == "-" {
					t.Errorf("field %s: missing json tag", field.Name)
					continue
				}
				name := strings.Split(jsonTag, ",")[0]
				if name == "" {
					t.Errorf("field %s: json tag has empty name (%q)", field.Name, jsonTag)
				}
				if name != strings.ToLower(name) {
					t.Errorf("field %s: json name %q should be lowercase", field.Name, name)
				}
				if field.Tag.Get("jsonschema") == "" {
					t.Errorf("field %s: missing jsonschema description tag", field.Name)
				}
				if field.Tag.Get("jsonschema_description") != "" {
					t.Errorf("field %s: jsonschema_description tag is not read by jsonschema-go; use jsonschema", field.Name)
				}
			}
		})
	}
}
