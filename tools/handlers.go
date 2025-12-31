package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

	server.AddTool(&mcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: inputSchema,
	}, r.getHandler(spec.Name))
}

func (r *Registry) getHandler(name string) mcp.ToolHandler {
	switch name {
	case "lei_lookup":
		return r.handleLEILookup
	case "validate_lei":
		return r.handleValidateLEI
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
		return errorResult(fmt.Sprintf("Failed to fetch LEI: %v", err))
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
		return errorResult(fmt.Sprintf("Validation error: %v", err))
	}

	return jsonResult(result)
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

	// Default to fuzzy search
	fuzzy := true
	if f, ok := args["fuzzy"].(bool); ok {
		fuzzy = f
	}

	var records []gleif.LEIRecord
	var searchErr error

	if fuzzy {
		records, searchErr = r.client.FuzzySearch(ctx, query, limit)
	} else {
		records, searchErr = r.client.SearchEntities(ctx, query, limit)
	}

	if searchErr != nil {
		return errorResult(fmt.Sprintf("Search failed: %v", searchErr))
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

	return jsonResult(map[string]any{
		"count":   len(results),
		"results": results,
	})
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
		return errorResult(fmt.Sprintf("BIC search failed: %v", err))
	}

	if len(records) == 0 {
		return jsonResult(map[string]any{
			"found":   false,
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
		return errorResult(fmt.Sprintf("ISIN search failed: %v", err))
	}

	if len(records) == 0 {
		return jsonResult(map[string]any{
			"found":   false,
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
		return errorResult(fmt.Sprintf("Country search failed: %v", err))
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
		return errorResult(fmt.Sprintf("Relationship lookup failed: %v", err))
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
		return errorResult(fmt.Sprintf("Autocomplete failed: %v", err))
	}

	return jsonResult(map[string]any{
		"prefix":      prefix,
		"suggestions": suggestions,
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
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
		IsError: true,
	}, nil
}
