# GLEIF MCP Server Constitution

This document holds the governance articles for the GLEIF MCP Server. These articles are **non-negotiable** and **not subject to per-feature override**. They apply to every commit, pull request, and release regardless of urgency or scope.

This document does not change without an explicit constitutional amendment: a dedicated pull request that modifies only this file, reviewed by the maintainer. A feature pull request that would violate an article does not get an exception; it either changes to comply, or it waits behind an amendment.

**Every article below codifies something the repository already does.** No article invents a new requirement. Each names the file or pattern it is drawn from, and each states honestly whether a linter, a test, or a CI job enforces it, or whether it rests on review alone. An article that claims enforcement it does not have is worse than one that admits it has none, because the false claim stops anyone from adding the missing check.

Written 27-08-2026 against `main` at go-sdk v1.7.0, 12 registered tools.

---

## Article I: Tool registration is declarative and single-entry

Adding a tool means adding one `ToolSpec` to `AllTools` in `tools/definitions.go` and one entry to the dispatch map in `tools/registry.go` (`buildHandlerMap`, lines 39-54). Handlers MUST NOT be registered by hand-written boilerplate: they go through `makeHandler` / `registerTyped`, which is what attaches panic recovery and uniform error wrapping to every tool. A tool wired around that path gets neither and MUST NOT be merged.

Every spec MUST carry a non-empty `Name`, `Title`, `Category`, and `Description`. Names are bare (`lei_lookup`, not `gleif_lei_lookup`); renaming to a prefixed scheme would be a breaking change under Article XIV. Names are unique, and categories come from the fixed set in `TestToolCategories`.

Why: one generic wrapper is the reason a claim like "every tool recovers from panics" can be true at all, and the dispatch map is what makes tool count and handler count checkable against each other.

Codifies: `tools/definitions.go` (`ToolSpec`, `AllTools`), `tools/registry.go` (`buildHandlerMap`, `RegisterAll`, `makeHandler`, `registerTyped`).

**Enforcement: mechanically checked, both directions.** `tools/definitions_test.go` runs `TestAllToolsHaveRequiredFields`, `TestToolCategories`, `TestToolCount` (pinned at 12), `TestToolNamesUnique`, `TestToolDescriptionsHaveUseWhen`. `TestEveryToolHasHandler` in `tools/handlers_test.go` asserts every `AllTools` entry has a dispatch-map entry AND that the map has no orphans, so the runtime skip path in `RegisterAll` (`registry.go:61` logs a warning and continues) is unreachable for committed code. All run in CI via `go test -v -race ./...` in `.github/workflows/ci.yml`.

---

## Article II: Schemas are derived from typed Args and Result structs

Every tool's input schema comes from its handler's typed Args struct and its output schema from its typed Result struct, via the generic `mcp.AddTool[Args, Result]` path. A handler MUST NOT accept `map[string]any` or return an untyped blob. Required-ness is expressed by omitting `,omitempty`; the `jsonschema:"..."` tag value is the property description.

The tag convention is documented at the top of `tools/args.go` (lines 10-18) with the verified library behaviour it rests on: `google/jsonschema-go v0.4.3` does NOT read the `jsonschema_description` tag some sibling servers use, and `jsonschema:"required"` sets the description to the literal word "required". This file uses the convention the library actually honours; a new field using the wrong tag ships a silently broken schema.

Why: the output schema is what gives Code Mode callers typed, structured tool results, and the input schema is what rejects a missing required field before the handler runs.

Codifies: `tools/args.go` (all 12 Args structs, all Result structs), `tools/registry.go` `registerTyped` (`InputSchema` left nil so the SDK infers it).

**Enforcement: mechanically checked.** `TestRegisterAllSchemas` in `tools/registry_test.go` drives a real `tools/list` over an in-memory transport and fails on any nil `InputSchema` or `OutputSchema`; `RegisterAll` itself panics at startup if a struct fails to resolve to a schema. `TestCallToolMissingRequired` asserts schema-level rejection of a missing required field. `TestCallToolStructuredContent` asserts results carry `structuredContent`. `TestArgsResultJSONRoundTrip`, `TestOmitemptyKeysAbsentWhenZero`, and `TestArgsFieldTags` in `tools/args_test.go` pin the tag convention.

---

## Article III: Handlers never panic out; startup may panic loudly

Every tool handler runs behind `defer r.recoverPanic(spec.Name, &err)`, and the enclosing closure in `registerTyped` uses **named return values** (`tools/registry.go:89`). Without named returns the deferred reassignment is a no-op and a recovered panic reaches the caller as a silent empty success. The panic value and stack are logged server-side with a correlation ID; the caller sees only the tool name and the correlation ID, never the panic value.

Startup is the one place a panic is correct: `RegisterAll` panics if a tool's Args or Result type cannot resolve to a JSON schema, before any request is served.

Why: a fake-success response is the most expensive failure an agent can receive, because it carries no signal to retry or report on.

Codifies: `tools/registry.go` (`registerTyped` lines 82-99, `recoverPanic` lines 120-134, `newCorrelationID`); HG-1 in the portfolio's `rules/code-review-prompts.md`.

**Enforcement: mechanically checked.** `TestPanicRecovery` in `tools/registry_test.go` registers a deliberately panicking handler, drives it end-to-end, and asserts the result is a structured error containing `correlation_id=` and NOT containing the panic value. Nothing checks named returns as such; that rests on `registerTyped` being the only registration path (Article I).

---

## Article IV: Caller-supplied identifiers are parsed into typed values at the boundary

Every identifier a caller supplies (LEI, BIC, ISIN, country code, issuer ID) is validated exactly once, at the MCP boundary, by a typed `Parse*` constructor in `internal/gleif/identifiers.go`. Once a value of type `LEI`, `BIC`, `ISIN`, `Country`, or `IssuerID` exists, downstream code may interpolate it into a URL without re-validating. A handler or client method that accepts a raw `string` where a typed identifier exists is a violation. The one deliberate exception is `validate_lei`, whose job is to report format failures as results rather than errors (`tools/handlers.go:71-79`), and it still routes through the same validators.

This article is incident-born twice, both recorded in `CHANGELOG.md` 0.8.0: `issuer_id` was interpolated into the URL path unvalidated, opening a path-traversal pivot from `/lei-issuers/` to other GLEIF endpoints; and ISIN/BIC/country inputs were length-checked only, leaving a parameter-smuggling vector that full-regex matching plus `url.Values.Encode()` closed.

Codifies: `internal/gleif/identifiers.go` (`parseIdentifier`, `ParseLEI`, `ParseBIC`, `ParseISIN`, `ParseCountry`, `ParseIssuerID`), `tools/handlers.go` (`fetchByID`, `searchByID` route every handler through a parser), `CHANGELOG.md` 0.8.0.

**Enforcement: mechanically checked.** `internal/gleif/identifiers_test.go` (`TestParseLEI`, `TestParseBIC`, `TestParseISIN`, `TestParseCountry`, `TestParseIssuerID`, `TestExportedPatternsMirrorUnexported`) and `internal/gleif/validate_paths_test.go` (`TestValidateIssuerID_RejectsInjection`, `TestParseIssuerID_RejectsInjection`, `TestParseISIN_QuerySmugglingRejected`). Nothing mechanically prevents a future client method from taking a bare `string`; that rests on review.

---

## Article V: Anything that does I/O takes `context.Context` first

Every method on `gleif.Client` that performs a network call, and every tool handler, MUST accept `context.Context` as its first parameter and propagate it to the underlying request. Cancellation and deadlines are not optional; the retry loop's backoff sleep respects context cancellation (`waitForRetry`, `internal/gleif/client.go:156-165`).

The only exempt methods are the in-process accessors that touch no network: `CacheStats` (`internal/gleif/client.go:236`), `ClearCache` (`internal/gleif/client.go:241`), and `partitionCachedLEIs` (`internal/gleif/client_lookup.go:83`, a pure cache read). This exemption is exhaustive. A new method that reaches the GLEIF API without a context is a violation, not a new exemption.

Codifies: all network-touching methods across `internal/gleif/client.go`, `client_lookup.go`, `client_search.go`, `client_relationships.go`, `client_validate.go`.

**Enforcement: none mechanical.** No linter in `.golangci.yml` checks parameter ordering. This rests on review.

---

## Article VI: Errors are never silently discarded

An operation MUST NOT swallow an error. If an error cannot be handled where it occurs it is propagated, wrapped with the tool name (`registerTyped`) or the failing operation. A best-effort path that discards an error writes the discard explicitly (`_ =`) so the reader can see it was a decision, as in `_ = resp.Body.Close()` at `internal/gleif/client.go:202`.

Codifies: the explicit-discard idiom throughout; `mapStatusToError` (`internal/gleif/client.go:215-233`), which converts every non-200 status into a structured `APIError` rather than a nil.

**Enforcement: mechanically checked.** `errcheck` is in the enabled linter list in `.golangci.yml` and runs on every push to `main` and every pull request via `.github/workflows/lint.yml` (golangci-lint v2.12.2). Verified locally on 27-08-2026: `golangci-lint run ./...` reports **0 issues**, and the unconfigured probe `golangci-lint run --no-config --default=none --enable=errcheck ./...` also reports **0 issues**, so the clean result is not an artifact of the config's exclusions. The `gosec` exclusion for `G104` in `.golangci.yml` carries the comment "Covered by errcheck", and unlike the sister repository where that same comment sat over a disabled check, here the claim is true.

---

## Article VII: Upstream response bodies never reach the caller

An error surfaced to the MCP caller MUST NOT contain the raw GLEIF response body. `mapStatusToError` maps status codes to structured `APIError` values built from `http.StatusText` and fixed messages; `NewServerError` drops the body argument. The reason is not cosmetic: GLEIF fronting infrastructure can return HTML error pages, and a body echoed into an error string is a prompt-injection channel straight into the agent's context.

This article is incident-born: `CHANGELOG.md` 0.8.0 records that 4xx responses were previously echoed verbatim to MCP callers, and the fix restored compliance with the portfolio's HG-2 gate.

Codifies: `internal/gleif/client.go` (`mapStatusToError`, lines 211-233), `internal/gleif/errors.go` (`APIError` and every `New*Error` constructor), `CHANGELOG.md` 0.8.0.

**Enforcement: mechanically checked.** `internal/gleif/sanitize_error_test.go`: `TestRequest4xxDoesNotLeakResponseBody`, `TestNewServerErrorDropsBody`, `TestNewServerError_NoLeakInJSON`. All run in CI.

---

## Article VIII: The API client refuses all redirects

The HTTP client sets `CheckRedirect` to return `http.ErrUseLastResponse` (`internal/gleif/client.go:99-101`). The configured `BaseURL` is the only legitimate target; the GLEIF API does not redirect under normal operation. Without this guard Go follows up to 10 redirects, and a misconfigured deployment combined with a proxy returning `Location: http://169.254.169.254/...` would pivot a lookup into a fetch against cloud metadata or other link-local services, with the body landing in the agent context.

This is the full-refuse form of the portfolio's HG-4 gate, and this file is cited as its reference implementation in `rules/code-review-prompts.md`. A future client that follows even one redirect is a change to this article's terms and needs an amendment, not a feature pull request.

Codifies: `internal/gleif/client.go:93-101` (the guard and its rationale comment), `CHANGELOG.md` 0.9.0 (security entry).

**Enforcement: mechanically checked.** `internal/gleif/client_redirect_test.go`: `TestAPIClientRefusesRedirectToInternalIP`, `TestAPIClientRefusesRedirectChain`. Both run in CI.

---

## Article IX: The cache never lies

Three properties, each incident-born, each with its own tests:

1. **No aliased pointers.** `SetLEI` deep-copies on store and `GetLEI` deep-copies on retrieval. Before 0.9.0 the cache handed the same pointer to every hit, so any future handler mutating a returned field would silently corrupt cached state for every concurrent caller.
2. **Transient failures are never cached as definitive answers.** Only a definitive 404 may enter the 24-hour validation cache. Before 0.8.0, a single client saturating the rate limiter could mark a targeted LEI as `Valid=false` for the rest of the day.
3. **A cold-cache stampede costs one upstream call, not N.** `GetLEI` collapses concurrent misses for the same key via `singleflight.Group`, closing the rate-limiter-drain amplification that made the poisoning above reachable from one client. Errors are deliberately not shared across singleflight followers.

Codifies: `internal/gleif/cache.go` (clone-on-store, clone-on-load), `internal/gleif/client_validate.go` (definitive-only caching), `internal/gleif/client.go:63-71` (`sfGroup` and its rationale comment), `CHANGELOG.md` 0.8.0 and 0.9.0.

**Enforcement: mechanically checked, the best-covered article in this document.** `TestCacheReturnsIndependentCopies`, `TestCloneLEIRecordIsolation`, `TestGetLEIReturnsCopy`, `TestSetLEIStoresCopy` (isolation); `TestValidateLEI_DoesNotCacheTransientError`, `TestValidateLEI_CachesDefinitiveNotFound`, `TestValidateLEITransientErrorNotCached` (poisoning); `TestGetLEI_SingleflightCollapsesConcurrentMisses`, `TestGetLEI_SingleflightDoesNotShareErrors` (stampede). All run in CI.

---

## Article X: Every response is bounded, projected, and honest about truncation and staleness

No tool returns a raw GLEIF payload. Search and batch results are projected through `simplifyRecord` into the six-field `SimpleRecord`; `search_by_country` projects into the four-field `CountryRecord`. Every list operation has a default and a maximum: search limit clamps to `[1,100]` with default 20 (`clampLimitAndPage`, `internal/gleif/client_search.go:201-208`), batch lookups cap at 100 (`maxBatchSize`), `list_lei_issuers` defaults to 100 with a maximum of 1000 and returns `Count`, `Total`, and a `Truncated` flag when cut short (`tools/handlers.go:320-341`). `search_entity` returns `Pagination` and `HasMore`. Raising a default or a cap is a change to this article's terms and belongs in an amendment.

Staleness is part of the same honesty: the server advertises `ttlMs` of one hour on `tools/list` and `server/discover` results via the `mcp-cache-go` middleware (`main.go:24-45`), because the SDK's default of 0 means "immediately stale" under MCP `2026-07-28` and would make a compliant client re-fetch the tool list every turn. Only the two list-shaped methods are stamped, deliberately, so a future `resources/read` never inherits a blanket TTL.

**Named gaps, stated so they are visible rather than implied.** (1) Zero-result honesty is partial: `search_by_bic` and `search_by_isin` return an explicit `Message` ("No LEI found for this BIC code") on empty results (`buildIDSearchResult`, `tools/handlers.go:226-234`), but `search_entity` and `search_by_country` return a bare `Count: 0`. (2) Resolved 27-08-2026 (`bd028dd`): the `autocomplete` schema/clamp mismatch is fixed at the true upstream bound of 10 — GLEIF's endpoint serves at most 10 suggestions regardless of `page[size]`, verified by live probes — with `AutocompleteMaxLimit` shared between schema text and clamp and pinned by `TestAutocompleteSchemaMatchesClientClamp` and `TestClampAutocompleteLimit`. Gap (1) remains a defect worth fixing; neither was ever policy.

Codifies: `tools/handlers.go` (`simplifyRecord`, `buildIDSearchResult`, `handleListLEIIssuers`), `tools/args.go` (`SimpleRecord`, `CountryRecord`, `IssuersResult.Truncated`), `internal/gleif/client_search.go` (clamps), `main.go` (`toolListTTL`, `cacheConfig`).

**Enforcement: partially mechanical.** `TestClampLimitAndPage` and `TestApplyLimit` pin the clamps; `TestHandleListLEIIssuers` ("caps to limit" subtest) asserts `Count`, `Total`, and `Truncated`; `TestToolsListAdvertisesCacheTTL` and `TestNonListMethodIsNotStamped` in `main_test.go` drive the TTL middleware end-to-end. Nothing asserts that a *new* list tool has a cap or a projection; that rests on review, and it is the article most likely to erode quietly.

---

## Article XI: A tool description is a public contract with the agent

The description on a `ToolSpec` is the only thing an agent reads before deciding whether to call a tool. Changing it changes behaviour for every caller, invisibly, with no version bump and no error.

Descriptions follow the established shape and keep it: what the tool does, a `USE WHEN` clause, a cross-reference to the confusable sibling where one exists (`lei_lookup` vs `validate_lei` vs `batch_lei_lookup`; `search_entity` vs `autocomplete`; `get_lei_issuer` vs `list_lei_issuers`), and on 10 of the 12 tools a `FAILS WHEN` clause telling the agent how to recover. Those cross-references are load-bearing disambiguation and MUST NOT be dropped when a description is shortened. Removing a `USE WHEN` clause, removing a sibling cross-reference, or renaming a tool is a breaking change under Article XIV whatever happened to the code behind it. This repository is the portfolio's named reference for the `USE WHEN` ToolSpec style.

Codifies: `tools/definitions.go` (every spec), the `serverInstructions` block in `main.go` (which restates the tool routing and must be kept consistent with the specs by hand).

**Enforcement: partially mechanical.** `TestToolDescriptionsHaveUseWhen` fails any description missing a `USE WHEN` clause; `TestAllToolsHaveRequiredFields` rejects an empty description. Nothing checks `FAILS WHEN`, cross-reference preservation, or `serverInstructions` consistency; there is no `evals/` confusion-pair suite in this repository. Those rest on review.

---

## Article XII: Annotations tell the truth: this is a read-only server by construction

Every one of the 12 tools is annotated `ReadOnly: true` and `Idempotent: true`, and `buildTool` (`tools/registry.go:103-113`) maps those to `ReadOnlyHint` and `IdempotentHint` on the MCP tool. This is not a per-tool judgment; it is a property of the server: GLEIF is a public, unauthenticated, read-only API, and this server holds no credential and can mutate nothing. The `ToolSpec` struct deliberately has no `Destructive` field, so a destructive tool cannot even be declared. Adding one would require widening the struct, and that widening is an amendment to this article, because it changes what a client may assume about every tool named `gleif_*`-adjacent in its config.

Codifies: `tools/definitions.go` (`ToolSpec` annotation fields, every spec), `tools/registry.go` (`buildTool`).

**Enforcement: mechanically checked.** `TestAllToolsReadOnly` and `TestAllToolsIdempotent` in `tools/definitions_test.go` assert both flags on every tool, with the read-only-API rationale in the failure message. Both run in CI.

---

## Article XIII: The supply chain is verified on every pull request

CI MUST verify, on every push to `main` and every pull request against it: that module checksums match (`go mod verify`), that neither `go.mod` nor `go.sum` drifts from `go mod tidy`, that `go vet` is clean, that tests pass with the race detector, and that `golangci-lint` reports no issues.

The tidy check diffs **both** `go.mod` and `go.sum`. This repository is the one the portfolio's supply-chain rule calls out by name: until PR #63 its check diffed `go.sum` alone, which reports clean while a stale `// indirect` annotation sits in `go.mod`. The two-file diff is deliberate and MUST NOT be "simplified" back.

Codifies: `.github/workflows/ci.yml` (verify, tidy diff, govulncheck, vet, race tests, codecov), `.github/workflows/lint.yml` (golangci-lint v2.12.2), `.golangci.yml` (errcheck, gosec, govet, ineffassign, staticcheck, unused; gosec `G104`/`G101`/`G115`/`G304` excluded with stated reasons).

**Enforcement: mechanically checked, with one step that cannot fail.** `govulncheck` runs with `|| echo "::warning::..."`, so a known vulnerability produces a warning annotation and a green build; the workflow's stated rationale is that stdlib findings resolve with Go patch updates, and the cost is that a vulnerable direct dependency is equally unable to redden CI. Coverage upload is `fail_ci_if_error: false`, correct for a reporting step. Verified locally on 27-08-2026: `golangci-lint run ./...` reports **0 issues**.

---

## Article XIV: Semantic versioning, and the changelog is part of the change

The released binary, the MCPB bundles, the GHCR image, and the MCP Registry entry are versioned artifacts (`.github/workflows/release.yml` builds all of them from one tag and generates `server.json` with per-platform SHA256 hashes). Breaking changes MUST NOT ship in a patch or minor release. On this server, "breaking" means any of: removing a tool, renaming a tool, making an optional argument required, narrowing an argument's accepted values or an identifier's accepted format, removing a field from a Result struct, or the description changes named in Article XI. Adding a tool, adding an optional argument whose default preserves behaviour, and adding a Result field are not breaking.

Every user-visible change is recorded in `CHANGELOG.md` under Keep a Changelog headings, in the same pull request that makes the change. The existing entries set the standard: the 0.8.0 and 0.9.0 security entries name the vulnerable behaviour, the attack it enabled, and what the fix does.

**Standing violations, named rather than hidden — resolved 27-08-2026 (`a8009cf`).** The version constants had drifted from the tags (ServerVersion at 0.7.0 against tag v0.9.0, the User-Agent frozen at 0.2.0). `ServerVersion` is now a stampable `var` wired into the User-Agent, `release.yml`, the Dockerfile, and the Makefile — whose previous `-X main.Version`/`main.BuildTime` flags targeted symbols that never existed and were silently dropped, the same defect class this article recorded. The stamp is proven by a test build and pinned by `TestUserAgentHeader`. The tracked `server.json`/`manifest.json` values remain inert at release time because `release.yml` regenerates or overrides them.

Codifies: `CHANGELOG.md`, `.github/workflows/release.yml`, `.github/workflows/docker.yml`, `server.json`, `manifest.json`, the git tag history (`v0.5.0` through `v0.9.0`).

**Enforcement: none.** No CI job checks that a pull request touching `tools/` also touched `CHANGELOG.md`, and nothing reconciles `ServerVersion` with the release tag; the drift above is the proof. The smallest change that would enforce the latter is injecting the version via `-ldflags` in `release.yml`, as the Makefile already does for `main.Version`.

---

## Articles considered and rejected

**Operations that grant durable access fail closed.** Rejected as not applicable. Every tool reads a public, unauthenticated API; the server holds no token, no OAuth flow, and no tool that shares, invites, or grants anything. There is no access to fail closed on. Article XII records the stronger structural fact: destructive tools cannot even be declared.

**No credentials in version control.** Rejected as an article because there are no credentials in the system to govern: the GLEIF API needs none, and the only secret-shaped files in the working tree are local MCP Registry publishing tokens (`.mcpregistry_*`), which are already covered by `.gitignore:16` and verified untracked. An article would assert discipline over a surface this server does not have. The `gosec` `G101` exclusion in `.golangci.yml` means no scanner stands behind even the residual risk; that is recorded here so nobody reads the exclusion as evidence of coverage elsewhere.

**Fixtures are captured from live responses.** Rejected because it is not what this repository does. Test fixtures are hand-built mock JSON:API envelopes (`mockLEIResponse` in `tools/handlers_test.go`, inline `APIResponse` literals in the client tests), by design, with no capture-date provenance and no `api-tracking/` live-probe layer. Writing the article would describe a discipline this repo has never practiced, and there is no incident receipt here of an imagined wire shape passing tests while live calls failed. If one ever appears, that is the moment to amend.

**Test-first development, or coverage thresholds.** Rejected because coverage is real but uneven (the `tools/` and `internal/gleif` packages are heavily tested; `main.go` has two tests covering only the cache middleware) and there is no `CONTRIBUTING.md` in this repository to carry the honest ">80% on new code" form. Stating a threshold here would make the document a wish.

**Structured output and meaningful exit codes as a standalone article.** Rejected as redundant in this repository. The CLI surface is a single `-verbose` flag, and Go's `flag` package already exits 2 on an unknown flag; the structured-output half is Article II, and the count-on-every-list half is Article X. An article would restate two others to govern one flag.

**Structured logging with `log/slog` everywhere.** Rejected as too close to true to be useful: `slog` is used throughout with a single `log.Fatalf` at the terminal startup-failure site (`main.go:102`), which is the correct behaviour there. An article would exist to bless one line.

**Tool names carry a `gleif_` prefix.** Considered because the portfolio template assumes prefixed names and sibling servers use them. Rejected: this server's names are bare and published, so the article would either invent a requirement (a mass rename is a breaking change to every installed client) or codify the absence of one. Article I records the naming reality; Article XIV makes any rename a major-version event.

**A rule requiring README tool tables to match `AllTools`.** Rejected as too small for a constitution. `TestToolCount` pins the code at 12 and `README.md` quotes "12 tools" by hand; the drift risk is real (the version constants in Article XIV drifted exactly this way) but the fix is a test or a generator, not an article.

---

## Amendment log

| Date | Change |
|------|--------|
| 27-08-2026 | Ratified. Fourteen articles, adapted from the `CONSTITUTION.md` in `gridctl/gridctl` (Apache-2.0, github.com/gridctl/gridctl) via the portfolio template and the miro-mcp-server worked example. |
