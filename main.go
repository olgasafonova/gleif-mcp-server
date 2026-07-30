// GLEIF MCP Server - Access GLEIF LEI data via MCP
// Provides tools for looking up Legal Entity Identifiers worldwide
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
	"github.com/olgasafonova/gleif-mcp-server/tools"
	"github.com/olgasafonova/mcp-cache-go/mcpcache"
)

const (
	ServerName    = "gleif-mcp-server"
	ServerVersion = "0.7.0"
)

// toolListTTL is how long a client may cache this server's tool list.
//
// The tool set is fixed at compile time and only changes when a release ships,
// so an hour is conservative rather than aggressive. Without this the SDK
// advertises ttlMs:0, which the MCP 2026-07-28 spec defines as immediately
// stale, and a compliant client re-fetches the list every turn.
const toolListTTL = time.Hour

// cacheConfig is the cache-hint policy for this server.
//
// Only the two list-shaped methods are stamped. Default stays zero on purpose:
// a blanket default would also cover resources/read, which returns live content.
// This server registers no resources or prompts today, so listing the methods
// explicitly keeps that true if it ever does.
func cacheConfig() mcpcache.Config {
	return mcpcache.Config{
		TTLs: map[string]time.Duration{
			mcpcache.MethodListTools: toolListTTL,
			mcpcache.MethodDiscover:  toolListTTL,
		},
	}
}

// newServer builds the configured MCP server: tools registered and middleware
// attached. Extracted from main so a test can drive a real tools/list through it
// rather than asserting on the config in isolation.
func newServer(gleifClient *gleif.Client, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, &mcp.ServerOptions{
		Logger:       logger,
		Instructions: serverInstructions,
		// Suppress pre-initialize notifications/tools/list_changed from go-sdk.
		// Without this, AddTool triggers a notification before the client completes
		// the initialize handshake, causing intermittent connection failures.
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})

	// Advertise a cache TTL on list results. The SDK leaves ttlMs at 0, so
	// without this a compliant client treats every tools/list as stale.
	server.AddReceivingMiddleware(mcpcache.Middleware(cacheConfig()))

	toolRegistry := tools.NewRegistry(gleifClient, logger)
	toolRegistry.RegisterAll(server)

	return server
}

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
	server := newServer(gleifClient, logger)

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
