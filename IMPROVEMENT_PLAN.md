# GLEIF MCP Server - Improvement Plan

## Overview

This document outlines comprehensive improvements to the GLEIF MCP Server covering security, performance, API coverage, UX, and code architecture.

---

## 1. Go Architecture & Libraries Update

### Current State
- Go 1.23
- github.com/modelcontextprotocol/go-sdk v0.2.0
- Basic HTTP client with 30s timeout
- Single-file handlers

### Target Architecture
```
gleif-mcp-server/
├── cmd/
│   └── server/
│       └── main.go           # Entry point
├── internal/
│   ├── gleif/
│   │   ├── client.go         # HTTP client with retry/rate-limit
│   │   ├── cache.go          # In-memory cache with TTL
│   │   ├── types.go          # API response types
│   │   └── validation.go     # LEI/BIC/ISIN validation
│   ├── tools/
│   │   ├── registry.go       # Tool registration
│   │   ├── definitions.go    # Tool specs
│   │   ├── handlers.go       # Tool handlers
│   │   └── errors.go         # Custom error types
│   └── config/
│       └── config.go         # Configuration management
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── Makefile
```

### Libraries to Evaluate
- `golang.org/x/time/rate` - Rate limiting
- `github.com/hashicorp/golang-lru/v2` - LRU cache
- `github.com/cenkalti/backoff/v4` - Exponential backoff
- Keep MCP SDK at latest stable

---

## 2. Security Improvements

### 2.1 Client-Side Rate Limiting
- Implement token bucket rate limiter (50 req/min to stay under GLEIF's 60)
- Queue requests when limit approached
- Return clear error when rate limited

### 2.2 Retry with Exponential Backoff
- Retry on 5xx errors and timeouts
- Max 3 retries with 1s, 2s, 4s delays
- Don't retry on 4xx (client errors)

### 2.3 Request Logging
- Log all API requests with timing
- Structured logging with slog
- Debug mode for verbose output

---

## 3. Performance Improvements

### 3.1 In-Memory Cache
- Cache LEI lookups for 1 hour (data updates daily)
- Cache validation results for 24 hours
- Cache autocomplete for 5 minutes
- LRU eviction with 10,000 entry limit

### 3.2 Batch LEI Lookup
- New tool: `batch_lei_lookup`
- Accept up to 100 LEIs in one request
- Use GLEIF's comma-separated filter
- Return results as array

### 3.3 Response Compression
- Add `Accept-Encoding: gzip` header
- Reduces bandwidth significantly

### 3.4 Connection Optimization
- Enable HTTP keep-alive (Go default)
- Set reasonable idle connection limits

---

## 4. GLEIF API Coverage Expansion

### 4.1 New Tools to Add

| Tool | Endpoint | Description |
|------|----------|-------------|
| `batch_lei_lookup` | `lei-records?filter[lei]=...` | Look up multiple LEIs at once |
| `get_lei_issuer` | `lei-issuers/{id}` | Get LEI issuer (LOU) details |
| `list_lei_issuers` | `lei-issuers` | List all LEI issuers |
| `get_reporting_exceptions` | `reporting-exceptions` | Get Level 2 exceptions |
| `get_fund_relationships` | `lei-records/{lei}/...` | Fund manager/sub-fund relations |
| `get_successor` | `lei-records/{lei}/successor-entity` | Get successor for merged entities |

### 4.2 Enhanced Existing Tools

- `search_entity`: Add pagination (page number, page size)
- `search_entity`: Add sorting (by name, date, country)
- `get_relationships`: Add all relationship types
- `search_by_country`: Add entity type filter

---

## 5. UX Improvements

### 5.1 Better Error Messages
```go
type APIError struct {
    Code    string // "not_found", "invalid_format", "rate_limited"
    Message string // Human-readable message
    Details any    // Additional context
}
```

### 5.2 Pagination Support
- Add `page` and `page_size` parameters
- Return `total_count` and `has_more` in results
- Default page size: 20, max: 100

### 5.3 Field Selection
- Add `fields` parameter to reduce response size
- Example: `fields=["lei","legalName","country"]`

---

## 6. README Enhancement

### Sections to Add
- Installation for multiple platforms
- Configuration options
- Setup for Claude Desktop, Claude Code, Cursor, VS Code
- All tools with examples
- Response format documentation
- Rate limits and caching behavior
- Troubleshooting guide
- Contributing guidelines

---

## 7. Implementation Order

1. **Phase 1: Architecture** (foundation)
   - Restructure code
   - Add configuration management
   - Update dependencies

2. **Phase 2: Core Improvements** (reliability)
   - Rate limiting
   - Retry logic
   - Caching
   - Better errors

3. **Phase 3: New Features** (coverage)
   - Batch lookup
   - LEI issuers
   - Reporting exceptions
   - Fund relationships
   - Pagination

4. **Phase 4: Documentation** (usability)
   - Comprehensive README
   - Examples for all AI agents
   - API reference

---

## 8. Testing Strategy

- Unit tests for validation functions
- Integration tests against GLEIF API (with caching)
- Mock tests for error handling
- Benchmark tests for cache performance

---

## 9. Release Plan

- Version: 0.2.0 (significant feature addition)
- Update CHANGELOG
- Build binaries for all platforms
- Create GitHub release
- Update installation instructions
