# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- `autocomplete`: the `limit` schema and the client clamp now agree. The schema promised 1-50 while the client clamped anything above 20 down to 10, so an agent asking for 50 silently got 10. The GLEIF `/autocompletions` endpoint serves at most 10 suggestions and ignores `page[size]` (verified against the live API), so the honest contract is 1-10: the schema now says 1-10 and the clamp caps at the bound (`gleif.AutocompleteMaxLimit`), never below it. A test pins the schema bound to the client constant so they cannot drift apart again.
- Released binaries now self-report the real release version instead of a frozen literal. `ServerVersion` is a package-level `var` (default `dev`) stamped at build time via `-ldflags "-X main.ServerVersion=X.Y.Z"` in release.yml, docker.yml (Dockerfile `VERSION` build arg), and the Makefile; previously the const `0.7.0` shipped in v0.9.0 binaries. The GLEIF API `User-Agent` header derives from the same variable through `Config.UserAgent` instead of the frozen `gleif-mcp-server/0.2.0`. The Makefile's old `-X main.Version`/`-X main.BuildTime` flags pointed at symbols that do not exist and were silently dropped.

## [0.9.0] - 2026-05-10

### Changed
- Internal refactor: domain identifier types (`LEI`, `BIC`, `ISIN`, `Country`, `IssuerID`) with `Parse*` constructors at the MCP boundary. Client method signatures now take typed values instead of raw strings; format validation lives in the type system. Public API of `internal/gleif` package is unchanged for MCP callers (tool surface identical) but Go-package consumers will see typed parameters.
- Internal refactor: `client.go` split into `client.go` (HTTP plumbing), `client_lookup.go`, `client_search.go`, `client_relationships.go`, `client_validate.go` by concern. Plus decomposed complex methods (`doRequest`, `Autocomplete`, `GetRelationships`, `GetBatchLEI`) into single-responsibility helpers and replaced the 13-arm handler `switch` with a map dispatch. Result: every source file scores at Green (≥9.0) or Optimal (10.0) on CodeScene Code Health.
- Bumped `github.com/modelcontextprotocol/go-sdk` to v1.6.0.

### Fixed
- API client refuses all redirects via `CheckRedirect` returning `http.ErrUseLastResponse`. The GLEIF API does not redirect under normal operation; without this guard, a misconfigured `BaseURL` or a wiki/proxy returning `Location: http://169.254.169.254/...` would pivot a lookup into a fetch against cloud metadata or other link-local internal services, with the body landing in the agent context. (security)
- LEI record cache no longer returns aliased pointers. Previously `SetLEI` stored the caller's pointer as-is and `GetLEI` handed the same pointer to every cache-hit caller; any future handler mutating a returned field (redaction, normalization, locale fix-ups) would silently corrupt cached state for every other concurrent caller. Cache now deep-copies on both store and retrieval. (correctness)
- `GetLEI` cold-path fetches now collapse via `singleflight.Group`. Previously, N concurrent callers asking for the same uncached LEI each consumed a slot in the shared rate limiter (50 rpm, burst 10), trivially draining it; now one leader does the upstream round-trip and followers receive the leader's result. Closes the rate-limit-amplification surface that made the (now-fixed) 24-hour validation cache poison reachable from a single client. (security)

## [0.8.0] - 2026-05-03

### Fixed
- `validate_lei`: only cache definitive 404 responses; transient upstream errors (5xx, timeout, rate-limit retry exhaustion) no longer poison the 24-hour validation cache. Previously a single concurrent client saturating the rate limiter could mark a targeted LEI as `Valid=false` for the rest of the day. (security)
- `get_lei_issuer`: validate `issuer_id` against `^[A-Z0-9]{4,32}$` and `url.PathEscape` before URL interpolation; closes a path-traversal pivot from `/lei-issuers/` to other GLEIF endpoints. Tool spec gains `Pattern`, `MinLength=4`, `MaxLength=32` so the MCP framework rejects malformed IDs client-side too. (security)
- ISIN, BIC, and country tool inputs now validated as full-regex matches (length-only checks were insufficient); `search_by_isin` routes input through `url.Values.Encode()` instead of raw `fmt.Sprintf`, closing a parameter-smuggling vector. (security)
- API error responses no longer echo raw 4xx response body verbatim to MCP callers; replaces with `http.StatusText`. Restores compliance with hard gate HG-2 (no raw response body in errors). (security)

## [0.7.0] - 2026-03-03

### Added
- FAILS WHEN clauses in tool descriptions for LLM error recovery
- Panic recovery and ToolAnnotations (ReadOnlyHint, OpenWorldHint) in handler wrapper
- Feedback link in format validation errors
- ToolSpec validation tests and fix for missing USE WHEN clause
- golangci-lint config with gosec security linter

### Changed
- Bumped go-sdk from 1.2.0 to 1.3.1 (security patch)
- Reframed README pitch and added causal feedback templates

### Fixed
- Suppress pre-initialize `notifications/tools/list_changed` from go-sdk, preventing intermittent connection failures when many MCP servers start simultaneously
