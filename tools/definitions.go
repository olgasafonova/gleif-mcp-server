// Package tools provides MCP tool definitions and handlers for the GLEIF server.
package tools

// ToolSpec defines a tool's metadata for registration.
type ToolSpec struct {
	Name        string
	Title       string
	Description string
	Category    string
	ReadOnly    bool
	Parameters  []ParameterSpec
}

// ParameterSpec defines a tool parameter.
type ParameterSpec struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
}

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
		Description: `Get full details for a specific LEI code (legal name, address, jurisdiction, status, managing LOU, renewal dates). USE WHEN: "look up LEI", "LEI details", "who is HWUPKR0MPOU8FGXBT394?" For validating format and status only, use validate_lei. For multiple LEIs at once, use batch_lei_lookup.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "20-character LEI code", Required: true},
		},
	},

	{
		Name:        "validate_lei",
		Title:       "Validate LEI",
		Category:    "validation",
		ReadOnly:    true,
		Description: `Check if an LEI code is valid and active. USE WHEN: "is this LEI valid?", "verify LEI", "check LEI format". Performs format (20 chars), check digit (ISO 17442), and database status checks. For full entity details, use lei_lookup instead.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code to validate", Required: true},
		},
	},

	{
		Name:        "batch_lei_lookup",
		Title:       "Batch LEI Lookup",
		Category:    "lookup",
		ReadOnly:    true,
		Description: `Look up multiple LEI records in one request (max 100). USE WHEN: "look up these LEIs", "batch lookup", user provides 2+ LEI codes. Faster than calling lei_lookup repeatedly.`,
		Parameters: []ParameterSpec{
			{Name: "leis", Type: "string", Description: "Comma-separated LEI codes (max 100)", Required: true},
		},
	},

	// =========================================================================
	// Search Tools
	// =========================================================================
	{
		Name:     "search_entity",
		Title:    "Search Entities",
		Category: "search",
		ReadOnly: true,
		Description: `Search for legal entities by name with fuzzy matching and pagination. For quick name suggestions, use autocomplete instead.

USE WHEN: "find company X", "search for X", "look up company X"`,
		Parameters: []ParameterSpec{
			{Name: "query", Type: "string", Description: "Company name to search", Required: true},
			{Name: "limit", Type: "integer", Description: "Max results per page (default 20)", Required: false},
			{Name: "page", Type: "integer", Description: "Page number (default 1)", Required: false},
			{Name: "fuzzy", Type: "boolean", Description: "Use fuzzy matching (default true)", Required: false},
		},
	},

	{
		Name:        "search_by_bic",
		Title:       "Search by BIC/SWIFT",
		Category:    "search",
		ReadOnly:    true,
		Description: `Find a bank's LEI from its BIC/SWIFT code (8 or 11 characters). USE WHEN: "find LEI from BIC", "BIC to LEI", "SWIFT code lookup", user provides a BIC code.`,
		Parameters: []ParameterSpec{
			{Name: "bic", Type: "string", Description: "BIC/SWIFT code (8 or 11 chars)", Required: true},
		},
	},

	{
		Name:        "search_by_isin",
		Title:       "Search by ISIN",
		Category:    "search",
		ReadOnly:    true,
		Description: `Find the issuer's LEI from a securities ISIN code (12 characters). USE WHEN: "who issued ISIN US0378331005?", "ISIN to LEI", user provides an ISIN.`,
		Parameters: []ParameterSpec{
			{Name: "isin", Type: "string", Description: "12-character ISIN code", Required: true},
		},
	},

	{
		Name:        "search_by_country",
		Title:       "Search by Country",
		Category:    "search",
		ReadOnly:    true,
		Description: `List entities registered in a specific country. USE WHEN: "companies in Germany", "LEIs from US", "entities in country X". Pass ISO 2-letter code (US, GB, DE).`,
		Parameters: []ParameterSpec{
			{Name: "country", Type: "string", Description: "2-letter ISO country code", Required: true},
			{Name: "limit", Type: "integer", Description: "Max results (default 20)", Required: false},
		},
	},

	// =========================================================================
	// Relationship Tools
	// =========================================================================
	{
		Name:     "get_relationships",
		Title:    "Get Relationships",
		Category: "ownership",
		ReadOnly: true,
		Description: `Get corporate ownership and fund relationships for an entity.

USE WHEN: "who owns X?", "parent company", "subsidiaries", "fund manager"

Types: direct-parent, ultimate-parent, children, fund-manager, umbrella-fund, sub-funds.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code", Required: true},
			{Name: "type", Type: "string", Description: "Relationship type", Required: false, Enum: []string{"direct-parent", "ultimate-parent", "children", "fund-manager", "umbrella-fund", "sub-funds"}},
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
		Description: `Get entity name suggestions from a prefix (min 2 characters). For full search results with pagination, use search_entity instead.`,
		Parameters: []ParameterSpec{
			{Name: "prefix", Type: "string", Description: "Name prefix to complete", Required: true},
			{Name: "limit", Type: "integer", Description: "Max suggestions (default 10)", Required: false},
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
		Description: `Get details about a specific LEI issuer (Local Operating Unit / LOU) by ID. USE WHEN: "details on this LOU", "issuer info". Returns name, country, status, and sponsored LEI count. For all issuers worldwide, use list_lei_issuers.`,
		Parameters: []ParameterSpec{
			{Name: "issuer_id", Type: "string", Description: "LEI issuer ID", Required: true},
		},
	},

	{
		Name:        "list_lei_issuers",
		Title:       "List All LEI Issuers",
		Category:    "issuers",
		ReadOnly:    true,
		Description: `List all LEI issuers (Local Operating Units / LOUs) worldwide. USE WHEN: "show all LOUs", "which organizations issue LEIs?", "LEI issuer directory". Returns name, country, status for each. For one specific issuer, use get_lei_issuer.`,
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
		Description: `Explains why parent/ownership data may be missing for an entity. USE WHEN: "why no parent info?", "reporting exceptions", "missing ownership data". Returns Level 2 exception types: NON_CONSOLIDATING, NO_KNOWN_PERSON, NATURAL_PERSONS, NON_PUBLIC.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code", Required: true},
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
