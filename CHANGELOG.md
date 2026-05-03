# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
