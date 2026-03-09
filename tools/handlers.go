package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Registry manages tool registration and handlers.
type Registry struct {
	client *gleif.Client
	logger *slog.Logger
}

// NewRegistry creates a new tool registry.
func NewRegistry(client *gleif.Client, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		client: client,
		logger: logger,
	}
}

// RegisterAll registers all tools with the MCP server.
func (r *Registry) RegisterAll(server *mcp.Server) {
	for _, spec := range AllTools {
		r.registerTool(server, spec)
	}
	r.logger.Debug("Registered MCP tools", "count", len(AllTools))
}

func (r *Registry) registerTool(server *mcp.Server, spec ToolSpec) {
	// Build input schema as map
	properties := make(map[string]any)
	required := []string{}

	for _, param := range spec.Parameters {
		prop := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.Enum) > 0 {
			prop["enum"] = param.Enum
		}
		if param.MinLength != nil {
			prop["minLength"] = *param.MinLength
		}
		if param.MaxLength != nil {
			prop["maxLength"] = *param.MaxLength
		}
		if param.Minimum != nil {
			prop["minimum"] = *param.Minimum
		}
		if param.Maximum != nil {
			prop["maximum"] = *param.Maximum
		}
		if param.Default != nil {
			prop["default"] = param.Default
		}
		if param.Example != nil {
			prop["examples"] = []any{param.Example}
		}
		if param.Pattern != "" {
			prop["pattern"] = param.Pattern
		}
		properties[param.Name] = prop
		if param.Required {
			required = append(required, param.Name)
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}

	handler := r.getHandler(spec.Name)

	server.AddTool(&mcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: inputSchema,
		Annotations: &mcp.ToolAnnotations{
			Title:          spec.Title,
			ReadOnlyHint:   spec.ReadOnly,
			IdempotentHint: spec.Idempotent,
		},
	}, r.wrapHandler(spec.Name, handler))
}

func (r *Registry) getHandler(name string) mcp.ToolHandler {
	switch name {
	case "lei_lookup":
		return r.handleLEILookup
	case "validate_lei":
		return r.handleValidateLEI
	case "batch_lei_lookup":
		return r.handleBatchLEILookup
	case "search_entity":
		return r.handleSearchEntity
	case "search_by_bic":
		return r.handleSearchByBIC
	case "search_by_isin":
		return r.handleSearchByISIN
	case "search_by_country":
		return r.handleSearchByCountry
	case "get_relationships":
		return r.handleGetRelationships
	case "autocomplete":
		return r.handleAutocomplete
	case "get_lei_issuer":
		return r.handleGetLEIIssuer
	case "list_lei_issuers":
		return r.handleListLEIIssuers
	case "get_reporting_exceptions":
		return r.handleGetReportingExceptions
	default:
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return errorResult(fmt.Sprintf("Unknown tool: %s", name))
		}
	}
}

// Handler implementations

func (r *Registry) handleLEILookup(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	lei, ok := args["lei"].(string)
	if !ok || lei == "" {
		return errorResult("lei parameter is required")
	}

	record, err := r.client.GetLEI(ctx, lei)
	if err != nil {
		return classifyError("Failed to fetch LEI", err)
	}

	return jsonResult(record)
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

	// Parse comma-separated LEIs
	leis := strings.Split(leisStr, ",")
	for i, lei := range leis {
		leis[i] = strings.TrimSpace(lei)
	}

	records, err := r.client.GetBatchLEI(ctx, leis)
	if err != nil {
		return classifyError("Batch lookup failed", err)
	}

	// Return simplified results
	results := make([]map[string]any, len(records))
	foundLEIs := make(map[string]bool, len(records))
	for i, rec := range records {
		results[i] = map[string]any{
			"lei":       rec.LEI,
			"legalName": rec.Entity.LegalName.Name,
			"country":   rec.Entity.LegalAddress.Country,
			"city":      rec.Entity.LegalAddress.City,
			"status":    rec.Entity.Status,
			"regStatus": rec.Registration.Status,
		}
		foundLEIs[rec.LEI] = true
	}

	// Compute set difference: requested LEIs not in results
	var notFound []string
	for _, lei := range leis {
		if !foundLEIs[lei] {
			notFound = append(notFound, lei)
		}
	}

	response := map[string]any{
		"requested": len(leis),
		"found":     len(results),
		"results":   results,
	}
	if len(notFound) > 0 {
		response["notFound"] = notFound
	}

	return jsonResult(response)
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

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	page := 1
	if p, ok := args["page"].(float64); ok {
		page = int(p)
	}

	// Default to fuzzy search
	fuzzy := true
	if f, ok := args["fuzzy"].(bool); ok {
		fuzzy = f
	}

	var records []gleif.LEIRecord
	var pagination *gleif.Pagination
	var searchErr error

	if fuzzy {
		records, pagination, searchErr = r.client.FuzzySearch(ctx, query, limit, page)
	} else {
		records, pagination, searchErr = r.client.SearchEntities(ctx, query, limit, page)
	}

	if searchErr != nil {
		return classifyError("Search failed", searchErr)
	}

	// Return simplified results
	results := make([]map[string]any, len(records))
	for i, rec := range records {
		results[i] = map[string]any{
			"lei":       rec.LEI,
			"legalName": rec.Entity.LegalName.Name,
			"country":   rec.Entity.LegalAddress.Country,
			"city":      rec.Entity.LegalAddress.City,
			"status":    rec.Entity.Status,
			"regStatus": rec.Registration.Status,
		}
	}

	response := map[string]any{
		"count":   len(results),
		"results": results,
	}

	// Add pagination info if available
	if pagination != nil {
		response["pagination"] = map[string]any{
			"currentPage": pagination.CurrentPage,
			"perPage":     pagination.PerPage,
			"total":       pagination.Total,
			"lastPage":    pagination.LastPage,
		}
		response["hasMore"] = pagination.CurrentPage < pagination.LastPage
	}

	return jsonResult(response)
}

func (r *Registry) handleSearchByBIC(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	bic, ok := args["bic"].(string)
	if !ok || bic == "" {
		return errorResult("bic parameter is required")
	}

	records, err := r.client.SearchByBIC(ctx, bic)
	if err != nil {
		return classifyError("BIC search failed", err)
	}

	if len(records) == 0 {
		return jsonResult(map[string]any{
			"found":   false,
			"count":   0,
			"records": []any{},
			"message": "No LEI found for this BIC code",
		})
	}

	return jsonResult(map[string]any{
		"found":   true,
		"count":   len(records),
		"records": records,
	})
}

func (r *Registry) handleSearchByISIN(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	isin, ok := args["isin"].(string)
	if !ok || isin == "" {
		return errorResult("isin parameter is required")
	}

	records, err := r.client.SearchByISIN(ctx, isin)
	if err != nil {
		return classifyError("ISIN search failed", err)
	}

	if len(records) == 0 {
		return jsonResult(map[string]any{
			"found":   false,
			"count":   0,
			"records": []any{},
			"message": "No issuer LEI found for this ISIN",
		})
	}

	return jsonResult(map[string]any{
		"found":   true,
		"count":   len(records),
		"records": records,
	})
}

func (r *Registry) handleSearchByCountry(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	country, ok := args["country"].(string)
	if !ok || country == "" {
		return errorResult("country parameter is required")
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	records, err := r.client.SearchByCountry(ctx, country, limit)
	if err != nil {
		return classifyError("Country search failed", err)
	}

	// Return simplified results
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
		"country": country,
		"count":   len(results),
		"results": results,
	})
}

func (r *Registry) handleGetRelationships(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	lei, ok := args["lei"].(string)
	if !ok || lei == "" {
		return errorResult("lei parameter is required")
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
		"lei":           lei,
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

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	suggestions, err := r.client.Autocomplete(ctx, prefix, limit)
	if err != nil {
		return classifyError("Autocomplete failed", err)
	}

	return jsonResult(map[string]any{
		"prefix":      prefix,
		"suggestions": suggestions,
	})
}

func (r *Registry) handleGetLEIIssuer(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	issuerID, ok := args["issuer_id"].(string)
	if !ok || issuerID == "" {
		return errorResult("issuer_id parameter is required")
	}

	issuer, err := r.client.GetLEIIssuer(ctx, issuerID)
	if err != nil {
		return classifyError("Failed to fetch LEI issuer", err)
	}

	return jsonResult(issuer)
}

func (r *Registry) handleListLEIIssuers(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issuers, err := r.client.ListLEIIssuers(ctx)
	if err != nil {
		return classifyError("Failed to list LEI issuers", err)
	}

	return jsonResult(map[string]any{
		"count":   len(issuers),
		"issuers": issuers,
	})
}

func (r *Registry) handleGetReportingExceptions(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := parseArguments(req)
	if err != nil {
		return errorResult(err.Error())
	}

	lei, ok := args["lei"].(string)
	if !ok || lei == "" {
		return errorResult("lei parameter is required")
	}

	exceptions, err := r.client.GetReportingExceptions(ctx, lei)
	if err != nil {
		return classifyError("Failed to fetch reporting exceptions", err)
	}

	return jsonResult(map[string]any{
		"lei":        lei,
		"count":      len(exceptions),
		"exceptions": exceptions,
	})
}

// Helper functions

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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonBytes),
			},
		},
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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(errJSON),
			},
		},
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

// wrapHandler adds panic recovery around a tool handler.
func (r *Registry) wrapHandler(toolName string, handler mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("Panic recovered in tool handler",
					"tool", toolName,
					"panic", rec,
					"stack", string(debug.Stack()))
				result, err = errorResult(fmt.Sprintf("Internal error in %s", toolName))
			}
		}()
		return handler(ctx, req)
	}
}
