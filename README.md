# GLEIF MCP Server

Verify any company's legal identity in one question. Look up LEI codes, validate counterparties, and trace corporate ownership structures using the official GLEIF database. Covers 2.8M+ entities across 200+ jurisdictions.

[![CI](https://github.com/olgasafonova/gleif-mcp-server/actions/workflows/ci.yml/badge.svg)](https://github.com/olgasafonova/gleif-mcp-server/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/olgasafonova/gleif-mcp-server?v=2)](https://goreportcard.com/report/github.com/olgasafonova/gleif-mcp-server)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**12 tools** for LEI lookup, entity search, BIC/SWIFT cross-referencing, corporate ownership, and batch validation. Works with Claude Desktop, Claude Code, Cursor, VS Code, Windsurf, and other MCP-compatible tools.

## Use Cases

- **KYC & Onboarding**: Verify counterparty identities before signing contracts
- **Compliance Checks**: Validate LEIs for MiFID II, EMIR, or DORA reporting
- **Due Diligence**: Research corporate ownership chains and ultimate parents
- **Financial Analysis**: Cross-reference securities (ISIN) and banks (BIC/SWIFT) with their legal entities
- **Data Enrichment**: Batch-process company lists to add LEI data

## What is LEI?

The Legal Entity Identifier (LEI) is a 20-character alphanumeric code that uniquely identifies legal entities participating in financial transactions worldwide. It's mandated by 200+ regulations including MiFID II, EMIR, Dodd-Frank, and DORA.

**LEI Format (ISO 17442):**
- Characters 1-4: LOU (Local Operating Unit) prefix
- Characters 5-18: Entity-specific identifier
- Characters 19-20: Check digits (mod 97 validation)

Example: `HWUPKR0MPOU8FGXBT394` (Apple Inc.)

## Features

### Core Capabilities
- **LEI Lookup**: Get full entity details by LEI code
- **Batch Lookup**: Look up multiple LEIs in a single request (up to 100)
- **Entity Search**: Find companies by name with fuzzy matching and pagination
- **LEI Validation**: Verify format, check digits (ISO 17442), and registration status

### Financial Identifiers
- **BIC/SWIFT Lookup**: Find bank LEIs from BIC codes
- **ISIN Lookup**: Find security issuer LEIs from ISIN codes
- **Country Browse**: List entities by jurisdiction

### Relationships & Compliance
- **Corporate Ownership**: Parent companies, subsidiaries, ultimate parents
- **Fund Relationships**: Fund managers, umbrella funds, sub-funds
- **Reporting Exceptions**: Level 2 data exceptions with reasons
- **LEI Issuers**: List and details of all Local Operating Units (LOUs)

### Performance & Reliability
- **Fast Responses**: Results are cached locally, so repeat queries return instantly
- **No API Key Needed**: Works out of the box with GLEIF's public API
- **Handles Errors Gracefully**: Automatic retries on timeouts or temporary failures
- **Stays Within Limits**: Built-in rate limiting prevents hitting GLEIF's quotas

## Installation

### Download Binary

Pre-built binaries for all platforms on the [releases page](https://github.com/olgasafonova/gleif-mcp-server/releases):

| Platform | Binary |
|----------|--------|
| macOS (Apple Silicon) | `gleif-mcp-server-darwin-arm64` |
| macOS (Intel) | `gleif-mcp-server-darwin-amd64` |
| Linux (x64) | `gleif-mcp-server-linux-amd64` |
| Linux (ARM64) | `gleif-mcp-server-linux-arm64` |
| Windows (x64) | `gleif-mcp-server-windows-amd64.exe` |

```bash
# macOS/Linux - download and make executable
chmod +x gleif-mcp-server-darwin-arm64
```

### Build from Source

Requires Go 1.25+:

```bash
git clone https://github.com/olgasafonova/gleif-mcp-server.git
cd gleif-mcp-server
go build -o gleif-mcp-server .
```

### Install via Go

```bash
go install github.com/olgasafonova/gleif-mcp-server@latest
```

## AI Agent Setup

> **Quickest start:** Using Claude Desktop? Just add the config below and restart. Using an IDE like Cursor? Same idea, different config file. Pick your tool below.

### Claude Desktop

#### Step 1: Download the binary

Go to the [releases page](https://github.com/olgasafonova/gleif-mcp-server/releases) and download the binary for your system:
- **Mac (Apple Silicon M1/M2/M3/M4):** `gleif-mcp-server-darwin-arm64`
- **Mac (Intel):** `gleif-mcp-server-darwin-amd64`
- **Windows:** `gleif-mcp-server-windows-amd64.exe`

#### Step 2: Mac only - allow the file to run

macOS blocks downloaded files. Open Terminal and run:

```bash
chmod +x ~/Downloads/gleif-mcp-server-darwin-arm64
xattr -d com.apple.quarantine ~/Downloads/gleif-mcp-server-darwin-arm64
```

#### Step 3: Open the config file

**Mac:** Open Finder, press `Cmd + Shift + G`, paste this path:
```
~/Library/Application Support/Claude/
```

**Windows:** Press `Win + R`, paste this path:
```
%APPDATA%\Claude
```

Open `claude_desktop_config.json`. If it doesn't exist, create it.

#### Step 4: Add the config

**Mac** (replace YOUR_USERNAME with your actual username):
```json
{
  "mcpServers": {
    "gleif": {
      "command": "/Users/YOUR_USERNAME/Downloads/gleif-mcp-server-darwin-arm64"
    }
  }
}
```

**Windows** (replace YOUR_USERNAME - note the double backslashes):
```json
{
  "mcpServers": {
    "gleif": {
      "command": "C:\\Users\\YOUR_USERNAME\\Downloads\\gleif-mcp-server-windows-amd64.exe"
    }
  }
}
```

To find your username: Mac - open Terminal and type `whoami`. Windows - look at `C:\Users\`.

#### Step 5: Restart Claude Desktop

Quit completely (`Cmd + Q` on Mac) and reopen.

#### Step 6: Test it

Type in Claude Desktop:
```
Look up Apple's LEI using GLEIF
```

You should see Claude call the GLEIF tool and return company data.

### Claude Code (CLI)

```bash
# Add the server
claude mcp add gleif /path/to/gleif-mcp-server

# Or with scope for all projects
claude mcp add --scope user gleif /path/to/gleif-mcp-server
```

### Cursor IDE

Add to `.cursor/mcp.json` in your project or `~/.cursor/mcp.json` for global config:

```json
{
  "mcpServers": {
    "gleif": {
      "command": "/path/to/gleif-mcp-server"
    }
  }
}
```

### VS Code with Continue Extension

Add to `.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "name": "gleif",
        "transport": {
          "type": "stdio",
          "command": "/path/to/gleif-mcp-server"
        }
      }
    ]
  }
}
```

### Windsurf

Add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "gleif": {
      "command": "/path/to/gleif-mcp-server"
    }
  }
}
```

### Cline (VS Code Extension)

Add via Cline's MCP settings or in `.vscode/cline_mcp_settings.json`:

```json
{
  "mcpServers": {
    "gleif": {
      "command": "/path/to/gleif-mcp-server",
      "args": []
    }
  }
}
```

### Antigravity

Add to `~/.antigravity/mcp.json`:

```json
{
  "mcpServers": {
    "gleif": {
      "command": "/path/to/gleif-mcp-server"
    }
  }
}
```

> **Not working?** [Tell us what made it hard](https://github.com/olgasafonova/gleif-mcp-server/issues/new?template=bug_report.yml) — even one sentence helps.

## Tools Reference

### Core Lookup Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `lei_lookup` | Get full details for a specific LEI | `lei` (required): 20-char LEI code |
| `validate_lei` | Check format, check digits, and status | `lei` (required): LEI to validate |
| `batch_lei_lookup` | Look up multiple LEIs at once | `leis` (required): Comma-separated LEIs (max 100) |

### Search Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_entity` | Search by company name | `query` (required), `limit` (default 20), `page` (default 1), `fuzzy` (default true) |
| `search_by_bic` | Find LEI from BIC/SWIFT | `bic` (required): 8 or 11 char code |
| `search_by_isin` | Find issuer LEI from ISIN | `isin` (required): 12-char ISIN |
| `search_by_country` | List entities by country | `country` (required): ISO 2-letter code, `limit` (default 20) |
| `autocomplete` | Entity name suggestions | `prefix` (required): min 2 chars, `limit` (default 10) |

### Relationship Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_relationships` | Get corporate/fund relationships | `lei` (required), `type`: direct-parent, ultimate-parent, children, fund-manager, umbrella-fund, sub-funds |

### LEI Issuer Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_lei_issuer` | Get details about an LOU | `issuer_id` (required): LOU identifier |
| `list_lei_issuers` | List all LOUs worldwide | None |

### Compliance Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_reporting_exceptions` | Get Level 2 reporting exceptions | `lei` (required): LEI to check |

## Usage Examples

### Basic LEI Lookup

**Prompt:** "Look up LEI HWUPKR0MPOU8FGXBT394"

**Returns:** Full entity details for Apple Inc. including legal name, headquarters address, jurisdiction (US-CA), entity status, registration status, managing LOU, and next renewal date.

### Company Search with Pagination

**Prompt:** "Search for Deutsche Bank, show page 2"

**Tool call:**
```json
{
  "name": "search_entity",
  "arguments": {
    "query": "Deutsche Bank",
    "limit": 20,
    "page": 2,
    "fuzzy": true
  }
}
```

**Returns:** List of matching entities with pagination info (total results, current page, has more).

### Batch LEI Lookup

**Prompt:** "Look up these LEIs: HWUPKR0MPOU8FGXBT394, 5493006MHB84DD0ZWV18, 549300GKFG0RYRRQ1414"

**Returns:** Summary of all three entities with LEI, legal name, country, city, and status.

### Find Bank by BIC

**Prompt:** "Find the LEI for BIC DEUTDEFF"

**Returns:** Deutsche Bank AG's LEI record with full details.

### Find Security Issuer

**Prompt:** "Who issued ISIN US0378331005?"

**Returns:** Apple Inc. (the issuer of AAPL stock).

### Corporate Structure

**Prompt:** "Who is the ultimate parent of this subsidiary?"

**Tool call:**
```json
{
  "name": "get_relationships",
  "arguments": {
    "lei": "549300GKFG0RYRRQ1414",
    "type": "ultimate-parent"
  }
}
```

### LEI Validation

**Prompt:** "Is LEI HWUPKR0MPOU8FGXBT394 valid?"

**Returns:**
```json
{
  "lei": "HWUPKR0MPOU8FGXBT394",
  "valid": true,
  "status": "ISSUED",
  "entityStatus": "ACTIVE",
  "nextRenewal": "2025-08-15"
}
```

### Check Reporting Exceptions

**Prompt:** "Why is parent info missing for this company?"

**Tool call:**
```json
{
  "name": "get_reporting_exceptions",
  "arguments": {
    "lei": "5493006MHB84DD0ZWV18"
  }
}
```

**Returns:** Exception categories and reasons (e.g., NON_CONSOLIDATING, NATURAL_PERSONS).

### List All LEI Issuers

**Prompt:** "Show me all LEI issuers"

**Returns:** Complete list of LOUs (Local Operating Units) with name, country, status, and number of sponsored LEIs.

## Response Format

All tools return JSON with relevant fields. Example entity record:

```json
{
  "lei": "HWUPKR0MPOU8FGXBT394",
  "legalName": "Apple Inc.",
  "country": "US",
  "city": "Cupertino",
  "status": "ACTIVE",
  "regStatus": "ISSUED"
}
```

Search results include pagination:

```json
{
  "count": 20,
  "results": [...],
  "pagination": {
    "currentPage": 1,
    "perPage": 20,
    "total": 156,
    "lastPage": 8
  },
  "hasMore": true
}
```

## Error Handling

The server returns structured errors:

| Error Code | Description | Retryable |
|------------|-------------|-----------|
| `not_found` | LEI/entity not in GLEIF database | No |
| `invalid_format` | Invalid LEI/BIC/ISIN format | No |
| `rate_limited` | GLEIF API rate limit exceeded | Yes |
| `timeout` | Request timed out | Yes |
| `server_error` | GLEIF API error | Depends on status |
| `network_error` | Connection failed | Yes |

Example error response:
```json
{
  "code": "not_found",
  "message": "LEI not found in GLEIF database",
  "statusCode": 404,
  "retryable": false
}
```

## Architecture

```
gleif-mcp-server/
├── main.go                 # Entry point, MCP server setup
├── internal/gleif/
│   ├── client.go          # GLEIF API client with caching & rate limiting
│   ├── client_test.go     # Client and validation tests
│   ├── cache.go           # LRU cache with TTL
│   ├── types.go           # Data structures for API responses
│   └── errors.go          # Structured error types
└── tools/
    ├── definitions.go     # Tool metadata and parameter specs
    ├── handlers.go        # MCP tool implementations
    └── handlers_test.go   # Handler tests with mock server
```

### Technical Details

For developers who want the specifics:

| Setting | Value |
|---------|-------|
| Cache duration | 15 minutes |
| Cache capacity | 1000 entities, 500 searches |
| Rate limit | 50 req/min (GLEIF allows 60) |
| Retry strategy | 3 attempts with exponential backoff |
| Connection pool | 100 max idle, 10 per host |

## API Reference

This server wraps the public GLEIF API:
- **Base URL**: https://api.gleif.org/api/v1
- **Authentication**: None required
- **Rate Limit**: 60 requests/minute
- **Documentation**: https://www.gleif.org/en/lei-data/gleif-api

## Troubleshooting

### Server won't start
- Check the binary has execute permissions: `chmod +x gleif-mcp-server`
- Verify the path in your MCP config is absolute, not relative

### "Rate limit exceeded" errors
- The server automatically handles rate limiting with retries
- If persistent, reduce concurrent requests or wait a few minutes

### "LEI not found" for valid LEI
- The GLEIF database updates daily; recently issued LEIs may not appear immediately
- Verify the LEI format (exactly 20 alphanumeric characters)

### Slow responses
- First requests may be slower (cache warming)
- GLEIF API occasionally has latency spikes; retries handle this automatically

### Claude Desktop doesn't show the server
- Restart Claude Desktop after editing config
- Check JSON syntax in config file
- Verify the binary path exists and is executable

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detector
go test -race ./...

# Build
go build -o gleif-mcp-server .
```

## Contributing

Contributions welcome. Please open an issue first to discuss proposed changes.

## License

MIT License - see [LICENSE](LICENSE) for details.

## More MCP Servers

Check out my other MCP servers:

| Server | Description | Stars |
|--------|-------------|-------|
| [mediawiki-mcp-server](https://github.com/olgasafonova/mediawiki-mcp-server) | Connect AI to any MediaWiki wiki. Search, read, edit wiki content. | ![GitHub stars](https://img.shields.io/github/stars/olgasafonova/mediawiki-mcp-server?style=flat) |
| [miro-mcp-server](https://github.com/olgasafonova/miro-mcp-server) | Control Miro whiteboards with AI. Boards, diagrams, mindmaps, and more. | ![GitHub stars](https://img.shields.io/github/stars/olgasafonova/miro-mcp-server?style=flat) |
| [nordic-registry-mcp-server](https://github.com/olgasafonova/nordic-registry-mcp-server) | Access Nordic business registries. Look up companies across Norway, Denmark, Finland, Sweden. | ![GitHub stars](https://img.shields.io/github/stars/olgasafonova/nordic-registry-mcp-server?style=flat) |
| [productplan-mcp-server](https://github.com/olgasafonova/productplan-mcp-server) | Talk to your ProductPlan roadmaps. Query OKRs, ideas, launches. | ![GitHub stars](https://img.shields.io/github/stars/olgasafonova/productplan-mcp-server?style=flat) |
| [tilbudstrolden-mcp](https://github.com/olgasafonova/tilbudstrolden-mcp) | Nordic grocery deal hunting. Find offers, plan meals, track spending. | ![GitHub stars](https://img.shields.io/github/stars/olgasafonova/tilbudstrolden-mcp?style=flat) |

---

## Acknowledgments

- [GLEIF](https://www.gleif.org/) for the public LEI API
- [Model Context Protocol](https://modelcontextprotocol.io/) for the MCP specification
