# CLAUDE.md

Import @README.md

## Build & Test
```bash
make check              # Lint + tests
go test ./...           # Run tests
go build .              # Build binary
```

## Project Notes
- GLEIF API integration for LEI lookups
- No authentication required for GLEIF public API
- Rate limiting handled internally
