// GLEIF MCP Server - Access GLEIF LEI data via MCP
// Provides tools for looking up Legal Entity Identifiers worldwide
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
	"github.com/olgasafonova/gleif-mcp-server/tools"
)

const (
	ServerName    = "gleif-mcp-server"
	ServerVersion = "0.1.0"
)

func main() {
	// Parse command-line flags
	verbose := flag.Bool("verbose", false, "Enable verbose debug logging")
	flag.Parse()

	// Configure logging to stderr (stdout is used for MCP protocol)
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Create GLEIF client
	gleifClient := gleif.NewClient(gleif.DefaultConfig(), logger)

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, &mcp.ServerOptions{
		Logger:       logger,
		Instructions: serverInstructions,
	})

	// Register tools
	toolRegistry := tools.NewRegistry(gleifClient, logger)
	toolRegistry.RegisterAll(server)

	logger.Info("Starting GLEIF MCP Server",
		"name", ServerName,
		"version", ServerVersion,
		"verbose", *verbose,
	)

	// Run server with stdio transport
	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

const serverInstructions = `# GLEIF LEI MCP Server

Access the Global Legal Entity Identifier (LEI) database via GLEIF's public API.

## What is LEI?

The Legal Entity Identifier (LEI) is a 20-character, alpha-numeric code that uniquely identifies legal entities participating in financial transactions worldwide. It's mandated by 200+ regulations including MiFID II, EMIR, Dodd-Frank, and DORA.

## Quick Reference

### Lookup Tools
- lei_lookup: Get full details for a specific LEI code
- validate_lei: Check if an LEI is valid and active

### Search Tools
- search_entity: Find companies by name (fuzzy matching)
- search_by_bic: Find LEI from bank BIC/SWIFT code
- search_by_isin: Find issuer LEI from securities ISIN
- search_by_country: Browse entities by jurisdiction

### Relationship Tools
- get_relationships: Get corporate ownership (parent/child entities)

### Utility Tools
- autocomplete: Entity name suggestions

## LEI Format

LEIs are 20 characters: 4-char LOU prefix + 14-char entity ID + 2-digit checksum
Example: HWUPKR0MPOU8FGXBT394 (Apple Inc.)

## Example Queries

"Look up LEI HWUPKR0MPOU8FGXBT394" → lei_lookup
"Search for Deutsche Bank" → search_entity
"Find LEI for BIC DEUTDEFF" → search_by_bic
"Who owns Apple?" → get_relationships
"Is this LEI valid: ABC123..." → validate_lei

## Tips

1. Use search_entity for company lookups - it handles partial names well
2. Use validate_lei before processing LEIs from external sources
3. get_relationships reveals corporate structures and ultimate parents
4. BIC/ISIN lookups are great for cross-referencing financial data
`
