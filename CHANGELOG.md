# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
