package tools

import "github.com/olgasafonova/gleif-mcp-server/internal/gleif"

// This file defines the typed Args and Result structs that drive schema
// generation under the generic mcp.AddTool[Args, Result] registration path
// (see registry.go). The go-sdk derives the input schema from the Args struct
// and the output schema + structuredContent from the Result struct.
//
// Tag convention (verified against google/jsonschema-go v0.4.3, which the
// go-sdk uses): a field is REQUIRED unless it carries `,omitempty`; the
// `jsonschema:"..."` tag value becomes the property description. (Note: the
// `jsonschema_description` tag used by some sibling servers is NOT read by this
// library — it silently no-ops, and `jsonschema:"required"` sets the
// description to the literal word "required". This file uses the convention
// the library actually honours.) Format/range/enum constraints are enforced in
// the handlers via the gleif.Parse* validators and default clamping, and
// described in the tag text, since the library does not derive them from tags.

// --- Args structs (one per tool) ---

// LEILookupArgs are the arguments for lei_lookup.
type LEILookupArgs struct {
	LEI string `json:"lei" jsonschema:"20-character LEI code, e.g. HWUPKR0MPOU8FGXBT394"`
}

// ValidateLEIArgs are the arguments for validate_lei.
type ValidateLEIArgs struct {
	LEI string `json:"lei" jsonschema:"LEI code to validate. Any format is accepted; the tool reports valid=false rather than erroring"`
}

// BatchLEIArgs are the arguments for batch_lei_lookup.
type BatchLEIArgs struct {
	LEIs string `json:"leis" jsonschema:"Comma-separated LEI codes (max 100), e.g. HWUPKR0MPOU8FGXBT394,7LTWFZYICNSX8D621K86"`
}

// SearchEntityArgs are the arguments for search_entity.
type SearchEntityArgs struct {
	Query string `json:"query" jsonschema:"Company name to search"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results per page, 1-100 (default 20)"`
	Page  int    `json:"page,omitempty" jsonschema:"Page number, minimum 1 (default 1)"`
	Fuzzy *bool  `json:"fuzzy,omitempty" jsonschema:"Use fuzzy matching (default true)"`
}

// SearchByBICArgs are the arguments for search_by_bic.
type SearchByBICArgs struct {
	BIC string `json:"bic" jsonschema:"BIC/SWIFT code, 8 or 11 characters, e.g. DEUTDEFF"`
}

// SearchByISINArgs are the arguments for search_by_isin.
type SearchByISINArgs struct {
	ISIN string `json:"isin" jsonschema:"12-character ISIN code, e.g. US0378331005"`
}

// SearchByCountryArgs are the arguments for search_by_country.
type SearchByCountryArgs struct {
	Country string `json:"country" jsonschema:"2-letter ISO 3166-1 alpha-2 country code, e.g. US, GB, DE"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max results, 1-100 (default 20). This tool does not page; raise limit for more"`
}

// GetRelationshipsArgs are the arguments for get_relationships.
type GetRelationshipsArgs struct {
	LEI  string `json:"lei" jsonschema:"20-character LEI code"`
	Type string `json:"type,omitempty" jsonschema:"Relationship type: direct-parent, ultimate-parent, children, fund-manager, umbrella-fund, sub-funds (default direct-parent)"`
}

// AutocompleteArgs are the arguments for autocomplete.
type AutocompleteArgs struct {
	Prefix string `json:"prefix" jsonschema:"Name prefix to complete (minimum 2 characters)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max suggestions, 1-50 (default 10)"`
}

// GetLEIIssuerArgs are the arguments for get_lei_issuer.
type GetLEIIssuerArgs struct {
	IssuerID string `json:"issuer_id" jsonschema:"LEI issuer (Local Operating Unit) ID, 4-32 alphanumeric characters"`
}

// ListLEIIssuersArgs are the arguments for list_lei_issuers (none).
type ListLEIIssuersArgs struct{}

// GetReportingExceptionsArgs are the arguments for get_reporting_exceptions.
type GetReportingExceptionsArgs struct {
	LEI string `json:"lei" jsonschema:"20-character LEI code"`
}

// --- Result structs (for the handlers that previously returned map[string]any) ---

// SimpleRecord is the trimmed entity shape shared by batch and search results.
type SimpleRecord struct {
	LEI       string `json:"lei"`
	LegalName string `json:"legalName"`
	Country   string `json:"country"`
	City      string `json:"city"`
	Status    string `json:"status"`
	RegStatus string `json:"regStatus"`
}

// BatchResult is the result of batch_lei_lookup.
type BatchResult struct {
	Requested int            `json:"requested"`
	Found     int            `json:"found"`
	Results   []SimpleRecord `json:"results"`
	NotFound  []string       `json:"notFound,omitempty"`
}

// PageInfo carries pagination metadata for search results.
type PageInfo struct {
	CurrentPage int `json:"currentPage"`
	PerPage     int `json:"perPage"`
	Total       int `json:"total"`
	LastPage    int `json:"lastPage"`
}

// SearchEntityResult is the result of search_entity.
type SearchEntityResult struct {
	Count      int            `json:"count"`
	Results    []SimpleRecord `json:"results"`
	Pagination *PageInfo      `json:"pagination,omitempty"`
	HasMore    *bool          `json:"hasMore,omitempty"`
}

// IDSearchResult is the result of search_by_bic and search_by_isin. The full
// LEI records are returned (not the trimmed SimpleRecord), matching the prior
// behaviour of these two tools.
type IDSearchResult struct {
	Found   bool              `json:"found"`
	Count   int               `json:"count"`
	Records []gleif.LEIRecord `json:"records"`
	Message string            `json:"message,omitempty"`
}

// CountryRecord is the trimmed entity shape used by search_by_country (four
// fields; deliberately narrower than SimpleRecord).
type CountryRecord struct {
	LEI       string `json:"lei"`
	LegalName string `json:"legalName"`
	City      string `json:"city"`
	Status    string `json:"status"`
}

// CountrySearchResult is the result of search_by_country.
type CountrySearchResult struct {
	Country string          `json:"country"`
	Count   int             `json:"count"`
	Results []CountryRecord `json:"results"`
}

// RelationshipsResult is the result of get_relationships.
type RelationshipsResult struct {
	LEI           string               `json:"lei"`
	Type          string               `json:"type"`
	Relationships []gleif.Relationship `json:"relationships"`
}

// AutocompleteToolResult is the result of autocomplete.
type AutocompleteToolResult struct {
	Prefix      string                     `json:"prefix"`
	Suggestions []gleif.AutocompleteResult `json:"suggestions"`
}

// IssuersResult is the result of list_lei_issuers.
type IssuersResult struct {
	Count   int               `json:"count"`
	Issuers []gleif.LEIIssuer `json:"issuers"`
}

// ReportingExceptionsResult is the result of get_reporting_exceptions.
type ReportingExceptionsResult struct {
	LEI        string                     `json:"lei"`
	Count      int                        `json:"count"`
	Exceptions []gleif.ReportingException `json:"exceptions"`
}
