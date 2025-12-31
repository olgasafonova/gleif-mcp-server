# GLEIF MCP Server

A Model Context Protocol (MCP) server for accessing the Global Legal Entity Identifier (LEI) database via GLEIF's public API.

## What is LEI?

The Legal Entity Identifier (LEI) is a 20-character alphanumeric code that uniquely identifies legal entities participating in financial transactions worldwide. It's mandated by 200+ regulations including MiFID II, EMIR, Dodd-Frank, and DORA.

## Features

- **LEI Lookup**: Get full entity details by LEI code
- **Entity Search**: Find companies by name with fuzzy matching
- **BIC/SWIFT Lookup**: Find bank LEIs from BIC codes
- **ISIN Lookup**: Find security issuer LEIs from ISIN codes
- **Country Browse**: List entities by jurisdiction
- **Relationship Mapping**: Explore corporate ownership structures
- **LEI Validation**: Verify LEI format, check digits, and registration status
- **Autocomplete**: Entity name suggestions for search UIs

## Installation

### Prerequisites

- Go 1.23 or later

### Build from source

```bash
git clone https://github.com/olgasafonova/gleif-mcp-server.git
cd gleif-mcp-server
go build -o gleif-mcp-server .
```

### Download binary

Pre-built binaries are available on the [releases page](https://github.com/olgasafonova/gleif-mcp-server/releases).

## Configuration

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "gleif": {
      "command": "/path/to/gleif-mcp-server"
    }
  }
}
```

### Claude Code

```bash
claude mcp add gleif /path/to/gleif-mcp-server
```

## Tools

| Tool | Description |
|------|-------------|
| `lei_lookup` | Get full details for a specific LEI code |
| `validate_lei` | Check if an LEI is valid and active |
| `search_entity` | Search for legal entities by name |
| `search_by_bic` | Find LEI from a bank's BIC/SWIFT code |
| `search_by_isin` | Find issuer LEI from securities ISIN |
| `search_by_country` | Find entities by jurisdiction |
| `get_relationships` | Get corporate ownership relationships |
| `autocomplete` | Get entity name suggestions |

## Example Usage

### Look up an LEI

```
"Look up LEI HWUPKR0MPOU8FGXBT394"
```

Returns Apple Inc.'s full entity details including legal name, address, jurisdiction, registration status, and renewal dates.

### Search for a company

```
"Search for Deutsche Bank"
```

Returns matching entities with LEI, name, country, and status.

### Find bank LEI from BIC

```
"Find LEI for BIC DEUTDEFF"
```

Returns Deutsche Bank AG's LEI record.

### Explore corporate structure

```
"Who owns Apple?"
```

Returns parent/child entity relationships.

### Validate an LEI

```
"Is this LEI valid: HWUPKR0MPOU8FGXBT394"
```

Returns validation result with format check, check digit verification, and database lookup.

## LEI Format

LEIs follow ISO 17442 format:
- 4 characters: LOU (Local Operating Unit) prefix
- 14 characters: Entity-specific identifier
- 2 digits: Check digits (mod 97 validation)

Example: `HWUPKR0MPOU8FGXBT394` (Apple Inc.)

## API

This server uses the public GLEIF API (https://api.gleif.org/api/v1). No authentication required. Rate limit: 60 requests/minute.

## License

MIT License
