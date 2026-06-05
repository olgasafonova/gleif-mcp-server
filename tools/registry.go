package tools

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Registry manages tool registration and handlers.
type Registry struct {
	client   *gleif.Client
	logger   *slog.Logger
	handlers map[string]mcp.ToolHandler
}

// NewRegistry creates a new tool registry.
func NewRegistry(client *gleif.Client, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	r := &Registry{client: client, logger: logger}
	r.handlers = r.buildHandlerMap()
	return r
}

// buildHandlerMap returns the name → handler dispatch table. A map collapses
// the prior 12-arm switch (cc=13) into a constant-complexity lookup.
func (r *Registry) buildHandlerMap() map[string]mcp.ToolHandler {
	return map[string]mcp.ToolHandler{
		"lei_lookup":               r.handleLEILookup,
		"validate_lei":             r.handleValidateLEI,
		"batch_lei_lookup":         r.handleBatchLEILookup,
		"search_entity":            r.handleSearchEntity,
		"search_by_bic":            r.handleSearchByBIC,
		"search_by_isin":           r.handleSearchByISIN,
		"search_by_country":        r.handleSearchByCountry,
		"get_relationships":        r.handleGetRelationships,
		"autocomplete":             r.handleAutocomplete,
		"get_lei_issuer":           r.handleGetLEIIssuer,
		"list_lei_issuers":         r.handleListLEIIssuers,
		"get_reporting_exceptions": r.handleGetReportingExceptions,
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
	inputSchema := buildInputSchema(spec.Parameters)
	handler := r.lookupHandler(spec.Name)

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

// buildInputSchema converts ParameterSpec slice to MCP input schema map.
// Extracted from registerTool to drop its cyclomatic complexity below the
// Go threshold; the per-property field copying is contained here.
func buildInputSchema(params []ParameterSpec) map[string]any {
	properties := make(map[string]any, len(params))
	required := []string{}

	for _, p := range params {
		properties[p.Name] = buildPropertySchema(p)
		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// buildPropertySchema renders one ParameterSpec as a JSON-Schema property map.
func buildPropertySchema(p ParameterSpec) map[string]any {
	prop := map[string]any{
		"type":        p.Type,
		"description": p.Description,
	}
	addLengthBounds(prop, p)
	addNumericBounds(prop, p)
	addEnumAndExample(prop, p)
	addDefaultAndPattern(prop, p)
	return prop
}

// addLengthBounds copies MinLength / MaxLength into the property map when set.
func addLengthBounds(prop map[string]any, p ParameterSpec) {
	if p.MinLength != nil {
		prop["minLength"] = *p.MinLength
	}
	if p.MaxLength != nil {
		prop["maxLength"] = *p.MaxLength
	}
}

// addNumericBounds copies Minimum / Maximum into the property map when set.
func addNumericBounds(prop map[string]any, p ParameterSpec) {
	if p.Minimum != nil {
		prop["minimum"] = *p.Minimum
	}
	if p.Maximum != nil {
		prop["maximum"] = *p.Maximum
	}
}

// addEnumAndExample copies enum and example values into the property map.
func addEnumAndExample(prop map[string]any, p ParameterSpec) {
	if len(p.Enum) > 0 {
		prop["enum"] = p.Enum
	}
	if p.Example != nil {
		prop["examples"] = []any{p.Example}
	}
}

// addDefaultAndPattern copies Default and Pattern into the property map.
func addDefaultAndPattern(prop map[string]any, p ParameterSpec) {
	if p.Default != nil {
		prop["default"] = p.Default
	}
	if p.Pattern != "" {
		prop["pattern"] = p.Pattern
	}
}

func (r *Registry) lookupHandler(name string) mcp.ToolHandler {
	if h, ok := r.handlers[name]; ok {
		return h
	}
	return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return errorResult(fmt.Sprintf("Unknown tool: %s", name))
	}
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
