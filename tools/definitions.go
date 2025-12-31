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
		Name:     "lei_lookup",
		Title:    "Get LEI Details",
		Category: "lookup",
		ReadOnly: true,
		Description: `Get full details for a specific LEI (Legal Entity Identifier).

USE WHEN: User provides an LEI code like "5493006MHB84DD0ZWV18" or asks "look up LEI X"

PARAMETERS:
- lei: 20-character LEI code (required)

RETURNS: Full entity details including legal name, address, jurisdiction, registration status, managing LOU, and renewal dates.

EXAMPLE: lei_lookup with lei="HWUPKR0MPOU8FGXBT394" returns Apple Inc. details.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "20-character LEI code", Required: true},
		},
	},

	{
		Name:     "validate_lei",
		Title:    "Validate LEI",
		Category: "validation",
		ReadOnly: true,
		Description: `Check if an LEI code is valid and active.

USE WHEN: User asks "is this LEI valid?", "check LEI X", "verify this LEI"

Performs three checks:
1. Format validation (20 alphanumeric characters)
2. Check digit validation (ISO 17442)
3. Database lookup (confirms it exists and checks status)

PARAMETERS:
- lei: LEI code to validate (required)

RETURNS: Valid (bool), registration status, entity status, next renewal date.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code to validate", Required: true},
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
		Description: `Search for legal entities by name.

USE WHEN: User asks "find company X", "search for X", "look up company X"

PARAMETERS:
- query: Company name to search for (required)
- limit: Max results to return (default 20, max 100)
- fuzzy: Use fuzzy matching (default true, better for partial names)

RETURNS: List of matching entities with LEI, name, country, and status.

EXAMPLE: search_entity with query="Apple" returns Apple Inc., Apple Bank, etc.`,
		Parameters: []ParameterSpec{
			{Name: "query", Type: "string", Description: "Company name to search", Required: true},
			{Name: "limit", Type: "integer", Description: "Max results (default 20)", Required: false},
			{Name: "fuzzy", Type: "boolean", Description: "Use fuzzy matching (default true)", Required: false},
		},
	},

	{
		Name:     "search_by_bic",
		Title:    "Search by BIC/SWIFT",
		Category: "search",
		ReadOnly: true,
		Description: `Find LEI from a bank's BIC/SWIFT code.

USE WHEN: User has a BIC/SWIFT code and needs the bank's LEI.

PARAMETERS:
- bic: 8 or 11 character BIC/SWIFT code (required)

RETURNS: Entity details for the bank with that BIC.

EXAMPLE: search_by_bic with bic="DEUTDEFF" returns Deutsche Bank AG.`,
		Parameters: []ParameterSpec{
			{Name: "bic", Type: "string", Description: "BIC/SWIFT code (8 or 11 chars)", Required: true},
		},
	},

	{
		Name:     "search_by_isin",
		Title:    "Search by ISIN",
		Category: "search",
		ReadOnly: true,
		Description: `Find the issuer's LEI from a securities ISIN code.

USE WHEN: User has an ISIN and wants to know who issued it.

PARAMETERS:
- isin: 12-character ISIN code (required)

RETURNS: Entity details for the security issuer.

EXAMPLE: search_by_isin with isin="US0378331005" returns Apple Inc. (issuer of AAPL).`,
		Parameters: []ParameterSpec{
			{Name: "isin", Type: "string", Description: "12-character ISIN code", Required: true},
		},
	},

	{
		Name:     "search_by_country",
		Title:    "Search by Country",
		Category: "search",
		ReadOnly: true,
		Description: `Find entities registered in a specific country.

USE WHEN: User wants to browse entities by jurisdiction.

PARAMETERS:
- country: 2-letter ISO country code (required, e.g., US, GB, DE)
- limit: Max results (default 20, max 100)

RETURNS: List of entities in that country.`,
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
		Description: `Get corporate ownership relationships for an entity.

USE WHEN: User asks "who owns X?", "parent company of X", "subsidiaries of X"

PARAMETERS:
- lei: LEI of the entity (required)
- type: Relationship type (optional):
  - "direct-parent": Immediate parent company
  - "ultimate-parent": Top of the ownership chain
  - "children": Direct subsidiaries

RETURNS: List of related entities with relationship details.

EXAMPLE: get_relationships with lei="HWUPKR0MPOU8FGXBT394" returns Apple Inc.'s parent/child entities.`,
		Parameters: []ParameterSpec{
			{Name: "lei", Type: "string", Description: "LEI code", Required: true},
			{Name: "type", Type: "string", Description: "Relationship type", Required: false, Enum: []string{"direct-parent", "ultimate-parent", "children"}},
		},
	},

	// =========================================================================
	// Utility Tools
	// =========================================================================
	{
		Name:     "autocomplete",
		Title:    "Autocomplete Entity Name",
		Category: "utility",
		ReadOnly: true,
		Description: `Get entity name suggestions as user types.

USE WHEN: Building search UI or helping user find correct entity name.

PARAMETERS:
- prefix: Start of company name (required, min 2 chars)
- limit: Max suggestions (default 10, max 20)

RETURNS: List of matching entity names with LEI codes.`,
		Parameters: []ParameterSpec{
			{Name: "prefix", Type: "string", Description: "Name prefix to complete", Required: true},
			{Name: "limit", Type: "integer", Description: "Max suggestions (default 10)", Required: false},
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
