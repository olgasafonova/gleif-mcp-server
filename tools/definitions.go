// Package tools provides MCP tool definitions and handlers for the GLEIF server.
package tools

// ToolSpec defines a tool's metadata for registration.
type ToolSpec struct {
	Name        string
	Title       string
	Description string
	Category    string
	ReadOnly    bool
	Idempotent  bool
	Parameters  []ParameterSpec
}

// ParameterSpec defines a tool parameter.
type ParameterSpec struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
	MinLength   *int
	MaxLength   *int
	Minimum     *float64
	Maximum     *float64
	Default     any
	Example     any
	Pattern     string
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int { return &v }

// floatPtr returns a pointer to a float64 value.
func floatPtr(v float64) *float64 { return &v }

// AllTools defines all available MCP tools.
var AllTools = []ToolSpec{
	// =========================================================================
	// Core Lookup Tools
	// =========================================================================
	{
		Name:        "lei_lookup",
		Title:       "Get LEI Details",
		Category:    "lookup",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Get full details for a specific LEI code (legal name, address, jurisdiction, status, managing LOU, renewal dates). USE WHEN: "look up LEI", "LEI details", "who is HWUPKR0MPOU8FGXBT394?" For validating format and status only, use validate_lei. For multiple LEIs at once, use batch_lei_lookup. FAILS WHEN: LEI is not exactly 20 alphanumeric characters (fix format and retry), LEI not found in GLEIF database (entity may not have an LEI; try search_entity by name instead).`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "20-character LEI code", Required: true, Pattern: `^[A-Z0-9]{20}$`, MinLength: intPtr(20), MaxLength: intPtr(20), Example: "HWUPKR0MPOU8FGXBT394"},
		},
	},

	{
		Name:        "validate_lei",
		Title:       "Validate LEI",
		Category:    "validation",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Check if an LEI code is valid and active. USE WHEN: "is this LEI valid?", "verify LEI", "check LEI format". Performs format (20 chars), check digit (ISO 17442), and database status checks. For full entity details, use lei_lookup instead. NOTE: Never returns an error for invalid LEIs; returns valid=false with a reason message instead. Safe to call on untrusted input.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code to validate", Required: true, Example: "HWUPKR0MPOU8FGXBT394"},
		},
	},

	{
		Name:        "batch_lei_lookup",
		Title:       "Batch LEI Lookup",
		Category:    "lookup",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Look up multiple LEI records in one request (max 100). USE WHEN: "look up these LEIs", "batch lookup", user provides 2+ LEI codes. Faster than calling lei_lookup repeatedly. FAILS WHEN: list is empty (provide at least one LEI), more than 100 LEIs (split into multiple calls), any individual LEI has invalid format (fix that specific LEI; the error message includes its position).`,
		Parameters: []ParameterSpec{
			{Name: "leis", Type: "string", Description: "Comma-separated LEI codes (max 100)", Required: true, Example: "HWUPKR0MPOU8FGXBT394,7LTWFZYICNSX8D621K86"},
		},
	},

	// =========================================================================
	// Search Tools
	// =========================================================================
	{
		Name:       "search_entity",
		Title:      "Search Entities",
		Category:   "search",
		ReadOnly:   true,
		Idempotent: true,
		Description: `Search for legal entities by name with fuzzy matching and pagination. For quick name suggestions, use autocomplete instead.

USE WHEN: "find company X", "search for X", "look up company X"

FAILS WHEN: no results found (try autocomplete for name suggestions, or check spelling; fuzzy matching is on by default).`,
		Parameters: []ParameterSpec{
			{Name: "query", Type: "string", Description: "Company name to search", Required: true, MinLength: intPtr(1), Example: "Apple"},
			{Name: "limit", Type: "integer", Description: "Max results per page (default 20)", Required: false, Minimum: floatPtr(1), Maximum: floatPtr(100), Default: 20},
			{Name: "page", Type: "integer", Description: "Page number (default 1)", Required: false, Minimum: floatPtr(1), Default: 1},
			{Name: "fuzzy", Type: "boolean", Description: "Use fuzzy matching (default true)", Required: false, Default: true},
		},
	},

	{
		Name:        "search_by_bic",
		Title:       "Search by BIC/SWIFT",
		Category:    "search",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Find a bank's LEI from its BIC/SWIFT code (8 or 11 characters). USE WHEN: "find LEI from BIC", "BIC to LEI", "SWIFT code lookup", user provides a BIC code. Returns matching entity with LEI, legal name, country, and registration status. FAILS WHEN: BIC is not 8 or 11 characters (fix input), no entity mapped to this BIC (not all banks have LEIs; try search_entity with the bank name instead).`,
		Parameters: []ParameterSpec{
			{Name: "bic", Type: "string", Description: "BIC/SWIFT code (8 or 11 chars)", Required: true, Pattern: `^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?$`, Example: "DEUTDEFF"},
		},
	},

	{
		Name:        "search_by_isin",
		Title:       "Search by ISIN",
		Category:    "search",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Find the issuer's LEI from a securities ISIN code (12 characters). USE WHEN: "who issued ISIN US0378331005?", "ISIN to LEI", user provides an ISIN. Returns issuer entity with LEI, legal name, country, and registration status. FAILS WHEN: ISIN is not 12 characters (fix input), no issuer found for this ISIN (some securities lack LEI mapping).`,
		Parameters: []ParameterSpec{
			{Name: "isin", Type: "string", Description: "12-character ISIN code", Required: true, Pattern: `^[A-Z]{2}[A-Z0-9]{10}$`, MinLength: intPtr(12), MaxLength: intPtr(12), Example: "US0378331005"},
		},
	},

	{
		Name:        "search_by_country",
		Title:       "Search by Country",
		Category:    "search",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `List entities registered in a specific country. USE WHEN: "companies in Germany", "LEIs from US", "entities in country X". Pass ISO 2-letter code (US, GB, DE). Returns entity list with LEI, legal name, and status for each. Paginated. FAILS WHEN: country code is not a 2-letter ISO 3166-1 alpha-2 code (use "US" not "USA", "GB" not "UK").`,
		Parameters: []ParameterSpec{
			{Name: "country", Type: "string", Description: "2-letter ISO country code", Required: true, Pattern: `^[A-Z]{2}$`, MinLength: intPtr(2), MaxLength: intPtr(2), Example: "US"},
			{Name: "limit", Type: "integer", Description: "Max results (default 20)", Required: false, Minimum: floatPtr(1), Maximum: floatPtr(100), Default: 20},
		},
	},

	// =========================================================================
	// Relationship Tools
	// =========================================================================
	{
		Name:       "get_relationships",
		Title:      "Get Relationships",
		Category:   "ownership",
		ReadOnly:   true,
		Idempotent: true,
		Description: `Get corporate ownership and fund relationships for an entity.

USE WHEN: "who owns X?", "parent company", "subsidiaries", "fund manager"

Types: direct-parent, ultimate-parent, children, fund-manager, umbrella-fund, sub-funds. Returns relationship records with related entity LEI, legal name, relationship type, and status.

FAILS WHEN: LEI format invalid (must be 20 alphanumeric chars), no relationships of requested type exist (entity may have no parent; check get_reporting_exceptions for why ownership data is missing).`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code", Required: true, Pattern: `^[A-Z0-9]{20}$`, MinLength: intPtr(20), MaxLength: intPtr(20)},
			{Name: "type", Type: "string", Description: "Relationship type", Required: false, Enum: []string{"direct-parent", "ultimate-parent", "children", "fund-manager", "umbrella-fund", "sub-funds"}, Default: "direct-parent"},
		},
	},

	// =========================================================================
	// Utility Tools
	// =========================================================================
	{
		Name:        "autocomplete",
		Title:       "Autocomplete Entity Name",
		Category:    "utility",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Get entity name suggestions from a prefix (min 2 characters). USE WHEN: "suggest companies starting with X", "autocomplete company name", user is typing a name and needs quick suggestions. For full search results with pagination, use search_entity instead. Returns name suggestions with LEI and legal name for each match. FAILS WHEN: prefix is shorter than 2 characters. Falls back to fuzzy search automatically if the autocomplete endpoint is unavailable.`,
		Parameters: []ParameterSpec{
			{Name: "prefix", Type: "string", Description: "Name prefix to complete", Required: true, MinLength: intPtr(2), Example: "Apple"},
			{Name: "limit", Type: "integer", Description: "Max suggestions (default 10)", Required: false, Minimum: floatPtr(1), Maximum: floatPtr(50), Default: 10},
		},
	},

	// =========================================================================
	// LEI Issuer Tools
	// =========================================================================
	{
		Name:        "get_lei_issuer",
		Title:       "Get LEI Issuer Details",
		Category:    "issuers",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Get details about a specific LEI issuer (Local Operating Unit / LOU) by ID. USE WHEN: "details on this LOU", "issuer info". Returns issuer name, country, status, website, and count of sponsored LEIs. For all issuers worldwide, use list_lei_issuers. FAILS WHEN: issuer ID not found (use list_lei_issuers to get valid IDs).`,
		Parameters: []ParameterSpec{
			{Name: "issuer_id", Type: "string", Description: "LEI issuer ID (4-32 alphanumeric characters)", Required: true, Pattern: `^[A-Z0-9]{4,32}$`, MinLength: intPtr(4), MaxLength: intPtr(32), Example: "EVK05KS7XY1DEII3R011"},
		},
	},

	{
		Name:        "list_lei_issuers",
		Title:       "List All LEI Issuers",
		Category:    "issuers",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `List all LEI issuers (Local Operating Units / LOUs) worldwide. USE WHEN: "show all LOUs", "which organizations issue LEIs?", "LEI issuer directory". Returns all issuers with name, country, status, and sponsored LEI count. For one specific issuer, use get_lei_issuer.`,
		Parameters:  []ParameterSpec{},
	},

	// =========================================================================
	// Reporting & Compliance Tools
	// =========================================================================
	{
		Name:        "get_reporting_exceptions",
		Title:       "Get Reporting Exceptions",
		Category:    "compliance",
		ReadOnly:    true,
		Idempotent:  true,
		Description: `Explains why parent/ownership data may be missing for an entity. USE WHEN: "why no parent info?", "reporting exceptions", "missing ownership data". Returns list of Level 2 exceptions, each with type (NON_CONSOLIDATING, NO_KNOWN_PERSON, NATURAL_PERSONS, NON_PUBLIC), category, and reference entity. Empty list when entity has no exceptions (parent data is complete). FAILS WHEN: LEI format invalid (must be 20 alphanumeric chars).`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code", Required: true, Pattern: `^[A-Z0-9]{20}$`, MinLength: intPtr(20), MaxLength: intPtr(20)},
		},
	},
}

// GetToolByName returns a tool spec by name.
func GetToolByName(name string) *ToolSpec {
	for _, tool := range AllTools {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}
