package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Handler implementations
//
// Single-LEI lookups (lei_lookup, get_relationships, get_reporting_exceptions)
// share a parse-and-call shape; the lookups that don't need extra args go
// through handleSimpleLEILookup. Multi-arg handlers below run the parse step
// inline because the extra arguments diverge.

func (r *Registry) handleLEILookup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return r.handleSimpleLEILookup(ctx, req, "Failed to fetch LEI", func(ctx context.Context, lei gleif.LEI) (any, error) {
		return r.client.GetLEI(ctx, lei)
	})
}

func (r *Registry) handleGetReportingExceptions(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return r.handleSimpleLEILookup(ctx, req, "Failed to fetch reporting exceptions", func(ctx context.Context, lei gleif.LEI) (any, error) {
		exceptions, err := r.client.GetReportingExceptions(ctx, lei)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"lei":        string(lei),
			"count":      len(exceptions),
			"exceptions": exceptions,
		}, nil
	})
}

func (r *Registry) handleGetLEIIssuer(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	id, ok := args["issuer_id"].(string)
	if !ok || id == "" {
		return errorResult("issuer_id parameter is required")
	}
	issuerID, err := gleif.ParseIssuerID(id)
	if err != nil {
		return classifyError("Invalid issuer_id", err)
	}
	issuer, err := r.client.GetLEIIssuer(ctx, issuerID)
	if err != nil {
		return classifyError("Failed to fetch LEI issuer", err)
	}
	return jsonResult(issuer)
}

func (r *Registry) handleValidateLEI(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	lei, ok := args["lei"].(string)
	if !ok || lei == "" {
		return errorResult("lei parameter is required")
	}
	// ValidateLEI accepts raw input on purpose: the tool's job is to report
	// format/check-digit failures back as validation results.
	result, err := r.client.ValidateLEI(ctx, lei)
	if err != nil {
		return classifyError("Validation error", err)
	}
	return jsonResult(result)
}

func (r *Registry) handleBatchLEILookup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	leisStr, ok := args["leis"].(string)
	if !ok || leisStr == "" {
		return errorResult("leis parameter is required")
	}

	leis, parseErr := parseBatchLEIs(leisStr)
	if parseErr != nil {
		return classifyError("Invalid LEI in batch", parseErr)
	}

	records, err := r.client.GetBatchLEI(ctx, leis)
	if err != nil {
		return classifyError("Batch lookup failed", err)
	}

	return jsonResult(buildBatchResponse(leis, records))
}

// parseBatchLEIs splits the comma-separated input and parses each entry.
// Returns the first parse error so the caller can surface it.
func parseBatchLEIs(raw string) ([]gleif.LEI, error) {
	parts := strings.Split(raw, ",")
	leis := make([]gleif.LEI, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lei, err := gleif.ParseLEI(part)
		if err != nil {
			return nil, err
		}
		leis = append(leis, lei)
	}
	return leis, nil
}

// buildBatchResponse shapes the batch results, including the not-found set.
func buildBatchResponse(requested []gleif.LEI, records []gleif.LEIRecord) map[string]any {
	results := make([]map[string]any, len(records))
	foundLEIs := make(map[string]bool, len(records))
	for i, rec := range records {
		results[i] = simplifyRecord(rec)
		foundLEIs[rec.LEI] = true
	}

	var notFound []string
	for _, lei := range requested {
		if !foundLEIs[string(lei)] {
			notFound = append(notFound, string(lei))
		}
	}

	response := map[string]any{
		"requested": len(requested),
		"found":     len(results),
		"results":   results,
	}
	if len(notFound) > 0 {
		response["notFound"] = notFound
	}
	return response
}

func (r *Registry) handleSearchEntity(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return errorResult("query parameter is required")
	}

	limit := intArg(args, "limit", 20)
	page := intArg(args, "page", 1)
	fuzzy := boolArg(args, "fuzzy", true)

	records, pagination, searchErr := r.executeSearch(ctx, searchRequest{
		query: query,
		limit: limit,
		page:  page,
		fuzzy: fuzzy,
	})
	if searchErr != nil {
		return classifyError("Search failed", searchErr)
	}

	return jsonResult(buildSearchResponse(records, pagination))
}

// searchRequest groups parameters for executeSearch. Keeping them together
// avoids the long argument list that codescene flagged.
type searchRequest struct {
	query string
	limit int
	page  int
	fuzzy bool
}

// executeSearch picks the search backend based on the fuzzy flag.
func (r *Registry) executeSearch(ctx context.Context, req searchRequest) ([]gleif.LEIRecord, *gleif.Pagination, error) {
	if req.fuzzy {
		return r.client.FuzzySearch(ctx, req.query, req.limit, req.page)
	}
	return r.client.SearchEntities(ctx, req.query, req.limit, req.page)
}

func buildSearchResponse(records []gleif.LEIRecord, pagination *gleif.Pagination) map[string]any {
	results := make([]map[string]any, len(records))
	for i, rec := range records {
		results[i] = simplifyRecord(rec)
	}
	response := map[string]any{
		"count":   len(results),
		"results": results,
	}
	if pagination != nil {
		response["pagination"] = map[string]any{
			"currentPage": pagination.CurrentPage,
			"perPage":     pagination.PerPage,
			"total":       pagination.Total,
			"lastPage":    pagination.LastPage,
		}
		response["hasMore"] = pagination.CurrentPage < pagination.LastPage
	}
	return response
}

func (r *Registry) handleSearchByBIC(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	raw, ok := args["bic"].(string)
	if !ok || raw == "" {
		return errorResult("bic parameter is required")
	}
	bic, err := gleif.ParseBIC(raw)
	if err != nil {
		return classifyError("Invalid BIC", err)
	}
	records, err := r.client.SearchByBIC(ctx, bic)
	if err != nil {
		return classifyError("BIC search failed", err)
	}
	return jsonResult(buildIDSearchResponse(records, "No LEI found for this BIC code"))
}

func (r *Registry) handleSearchByISIN(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	raw, ok := args["isin"].(string)
	if !ok || raw == "" {
		return errorResult("isin parameter is required")
	}
	isin, err := gleif.ParseISIN(raw)
	if err != nil {
		return classifyError("Invalid ISIN", err)
	}
	records, err := r.client.SearchByISIN(ctx, isin)
	if err != nil {
		return classifyError("ISIN search failed", err)
	}
	return jsonResult(buildIDSearchResponse(records, "No issuer LEI found for this ISIN"))
}

// buildIDSearchResponse shapes BIC/ISIN search results uniformly. Replaces
// the duplicate found/empty branches that CodeScene flagged across both
// handlers.
func buildIDSearchResponse(records []gleif.LEIRecord, emptyMessage string) map[string]any {
	if len(records) == 0 {
		return map[string]any{
			"found":   false,
			"count":   0,
			"records": []any{},
			"message": emptyMessage,
		}
	}
	return map[string]any{
		"found":   true,
		"count":   len(records),
		"records": records,
	}
}

func (r *Registry) handleSearchByCountry(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	raw, ok := args["country"].(string)
	if !ok || raw == "" {
		return errorResult("country parameter is required")
	}
	country, err := gleif.ParseCountry(raw)
	if err != nil {
		return classifyError("Invalid country", err)
	}

	limit := intArg(args, "limit", 20)
	records, err := r.client.SearchByCountry(ctx, country, limit)
	if err != nil {
		return classifyError("Country search failed", err)
	}

	results := make([]map[string]any, len(records))
	for i, rec := range records {
		results[i] = map[string]any{
			"lei":       rec.LEI,
			"legalName": rec.Entity.LegalName.Name,
			"city":      rec.Entity.LegalAddress.City,
			"status":    rec.Entity.Status,
		}
	}
	return jsonResult(map[string]any{
		"country": string(country),
		"count":   len(results),
		"results": results,
	})
}

func (r *Registry) handleGetRelationships(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	raw, ok := args["lei"].(string)
	if !ok || raw == "" {
		return errorResult("lei parameter is required")
	}
	lei, err := gleif.ParseLEI(raw)
	if err != nil {
		return classifyError("Invalid LEI", err)
	}

	relType := "direct-parent"
	if t, ok := args["type"].(string); ok && t != "" {
		relType = t
	}

	relationships, err := r.client.GetRelationships(ctx, lei, relType)
	if err != nil {
		return classifyError("Relationship lookup failed", err)
	}

	return jsonResult(map[string]any{
		"lei":           string(lei),
		"type":          relType,
		"relationships": relationships,
	})
}

func (r *Registry) handleAutocomplete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	prefix, ok := args["prefix"].(string)
	if !ok || prefix == "" {
		return errorResult("prefix parameter is required")
	}

	limit := intArg(args, "limit", 10)
	suggestions, err := r.client.Autocomplete(ctx, prefix, limit)
	if err != nil {
		return classifyError("Autocomplete failed", err)
	}

	return jsonResult(map[string]any{
		"prefix":      prefix,
		"suggestions": suggestions,
	})
}

func (r *Registry) handleListLEIIssuers(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issuers, err := r.client.ListLEIIssuers(ctx)
	if err != nil {
		return classifyError("Failed to list LEI issuers", err)
	}
	return jsonResult(map[string]any{
		"count":   len(issuers),
		"issuers": issuers,
	})
}

// Helpers

// handleSimpleLEILookup is the shared shape for handlers whose only argument
// is an LEI: parse args, parse the LEI, call the client, format the response.
// Replaces the duplicate prologue across handleLEILookup, handleGetReporting-
// Exceptions (and previously handleGetRelationships before it grew an extra arg).
func (r *Registry) handleSimpleLEILookup(
	ctx context.Context,
	req *mcp.CallToolRequest,
	errorPrefix string,
	call func(context.Context, gleif.LEI) (any, error),
) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}
	raw, ok := args["lei"].(string)
	if !ok || raw == "" {
		return errorResult("lei parameter is required")
	}
	lei, err := gleif.ParseLEI(raw)
	if err != nil {
		return classifyError("Invalid LEI", err)
	}
	result, err := call(ctx, lei)
	if err != nil {
		return classifyError(errorPrefix, err)
	}
	return jsonResult(result)
}

// simplifyRecord builds the trimmed-down representation used by batch and
// search responses. Centralized to keep the field list in one place.
func simplifyRecord(rec gleif.LEIRecord) map[string]any {
	return map[string]any{
		"lei":       rec.LEI,
		"legalName": rec.Entity.LegalName.Name,
		"country":   rec.Entity.LegalAddress.Country,
		"city":      rec.Entity.LegalAddress.City,
		"status":    rec.Entity.Status,
		"regStatus": rec.Registration.Status,
	}
}

// intArg pulls an int argument out of MCP's float64-typed JSON, falling back
// to the supplied default if missing or wrong-typed.
func intArg(args map[string]any, name string, def int) int {
	if v, ok := args[name].(float64); ok {
		return int(v)
	}
	return def
}

// boolArg pulls a bool argument with a fallback default.
func boolArg(args map[string]any, name string, def bool) bool {
	if v, ok := args[name].(bool); ok {
		return v
	}
	return def
}

func parseArguments(req *mcp.CallToolRequest) (map[string]any, error) {
	if len(req.Params.Arguments) == 0 {
		return make(map[string]any), nil
	}
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}
	return args, nil
}

func jsonResult(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
	}, nil
}

func errorResult(message string) (*mcp.CallToolResult, error) {
	return errorResultWithCode("error", message, false)
}

func errorResultWithCode(code, message string, retryable bool) (*mcp.CallToolResult, error) {
	errJSON, _ := json.MarshalIndent(map[string]any{
		"error":     true,
		"code":      code,
		"message":   message,
		"retryable": retryable,
	}, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(errJSON)}},
		IsError: true,
	}, nil
}

// classifyError inspects an error and returns a structured error result with
// the appropriate code and retryable flag from the underlying APIError, if any.
func classifyError(prefix string, err error) (*mcp.CallToolResult, error) {
	var apiErr *gleif.APIError
	if errors.As(err, &apiErr) {
		return errorResultWithCode(apiErr.Code, fmt.Sprintf("%s: %s", prefix, apiErr.Message), apiErr.Retryable)
	}
	return errorResultWithCode("error", fmt.Sprintf("%s: %v", prefix, err), false)
}
