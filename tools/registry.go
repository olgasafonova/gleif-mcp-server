package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	handlers map[string]registrationFunc
}

// registrationFunc registers one tool with the server. It erases the per-tool
// Args/Result type parameters so all tools can live in a single dispatch map.
type registrationFunc func(server *mcp.Server, spec ToolSpec)

// NewRegistry creates a new tool registry.
func NewRegistry(client *gleif.Client, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	r := &Registry{client: client, logger: logger}
	r.handlers = r.buildHandlerMap()
	return r
}

// buildHandlerMap returns the name → registration dispatch table. makeHandler
// captures each typed handler method and adapts it to the uniform
// registrationFunc signature.
func (r *Registry) buildHandlerMap() map[string]registrationFunc {
	return map[string]registrationFunc{
		"lei_lookup":               makeHandler(r, r.handleLEILookup),
		"validate_lei":             makeHandler(r, r.handleValidateLEI),
		"batch_lei_lookup":         makeHandler(r, r.handleBatchLEILookup),
		"search_entity":            makeHandler(r, r.handleSearchEntity),
		"search_by_bic":            makeHandler(r, r.handleSearchByBIC),
		"search_by_isin":           makeHandler(r, r.handleSearchByISIN),
		"search_by_country":        makeHandler(r, r.handleSearchByCountry),
		"get_relationships":        makeHandler(r, r.handleGetRelationships),
		"autocomplete":             makeHandler(r, r.handleAutocomplete),
		"get_lei_issuer":           makeHandler(r, r.handleGetLEIIssuer),
		"list_lei_issuers":         makeHandler(r, r.handleListLEIIssuers),
		"get_reporting_exceptions": makeHandler(r, r.handleGetReportingExceptions),
	}
}

// RegisterAll registers all tools with the MCP server.
func (r *Registry) RegisterAll(server *mcp.Server) {
	for _, spec := range AllTools {
		register, ok := r.handlers[spec.Name]
		if !ok {
			r.logger.Warn("No handler registered for tool, skipping", "tool", spec.Name)
			continue
		}
		register(server, spec)
	}
	r.logger.Debug("Registered MCP tools", "count", len(AllTools))
}

// makeHandler adapts a typed handler method into a uniform registrationFunc.
// This is the only place per-tool type parameters are needed.
func makeHandler[Args, Result any](r *Registry, method func(context.Context, Args) (Result, error)) registrationFunc {
	return func(server *mcp.Server, spec ToolSpec) {
		registerTyped(r, server, spec, method)
	}
}

// registerTyped wires one typed handler into the server via the generic
// mcp.AddTool. The Args struct drives the input schema; the Result struct drives
// the output schema and structuredContent (both auto-derived by the go-sdk). The
// named returns plus the deferred recoverPanic are load-bearing for HG-1: a
// panic must surface as a structured error, not as a silent (nil, zero, nil).
func registerTyped[Args, Result any](
	r *Registry,
	server *mcp.Server,
	spec ToolSpec,
	method func(context.Context, Args) (Result, error),
) {
	tool := buildTool(spec)
	mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args Args) (res *mcp.CallToolResult, out Result, err error) {
		defer r.recoverPanic(spec.Name, &err)

		result, methodErr := method(ctx, args)
		if methodErr != nil {
			var zero Result
			return nil, zero, fmt.Errorf("%s: %w", spec.Name, methodErr)
		}
		return nil, result, nil
	})
}

// buildTool constructs the tool metadata. InputSchema is left nil so the go-sdk
// infers it from the handler's Args type.
func buildTool(spec ToolSpec) *mcp.Tool {
	return &mcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		Annotations: &mcp.ToolAnnotations{
			Title:          spec.Title,
			ReadOnlyHint:   spec.ReadOnly,
			IdempotentHint: spec.Idempotent,
		},
	}
}

// recoverPanic converts a recovered panic into a structured tool error with a
// correlation ID (HG-1). The panic value and stack are logged server-side only;
// the MCP caller sees just the tool name and correlation ID, never the panic
// value. Writing through errPtr is what turns the panic into an error rather
// than a silent empty success.
func (r *Registry) recoverPanic(toolName string, errPtr *error) {
	rec := recover()
	if rec == nil {
		return
	}
	corrID := newCorrelationID()
	r.logger.Error("Panic recovered in tool handler",
		"tool", toolName,
		"correlation_id", corrID,
		"panic", rec,
		"stack", string(debug.Stack()))
	if errPtr != nil {
		*errPtr = fmt.Errorf("%s: internal error (correlation_id=%s)", toolName, corrID)
	}
}

// newCorrelationID returns a short random hex ID for correlating a sanitized
// caller-facing error with the full server-side log entry.
func newCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
