package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tests for the registration/dispatch machinery in registry.go, exercised
// end-to-end over an in-memory transport. Shared test helpers (newTestServer,
// newTestRegistry, mockLEIResponse) live in handlers_test.go in the same package.

// connectInMemory registers the registry on a fresh server and returns a
// connected in-memory client session. RegisterAll panics if any tool's Args or
// Result type fails to resolve to a JSON schema, so reaching the return is
// itself an assertion that all 12 tools' schemas are valid.
func connectInMemory(t *testing.T, registry *Registry) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "gleif-test", Version: "0.0.0"}, nil)
	registry.RegisterAll(server)

	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return cs
}

// contentText extracts the first text content block from a tool result.
func contentText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// TestRegisterAllSchemas confirms every tool registers with a resolvable input
// AND output schema. The output schema is the point of the migration: it is what
// gives Code Mode callers typed, structured tool results.
func TestRegisterAllSchemas(t *testing.T) {
	registry := newTestRegistry("http://example.invalid")
	cs := connectInMemory(t, registry)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != len(AllTools) {
		t.Fatalf("expected %d tools, got %d", len(AllTools), len(res.Tools))
	}
	for _, tool := range res.Tools {
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil InputSchema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %q has nil OutputSchema (structuredContent will not be typed)", tool.Name)
		}
	}
}

// TestCallToolStructuredContent drives lei_lookup end-to-end and asserts the
// result carries structuredContent (not just a text blob).
func TestCallToolStructuredContent(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	ts.mux.HandleFunc("/lei-records/HWUPKR0MPOU8FGXBT394", func(w http.ResponseWriter, _ *http.Request) {
		resp := mockLEIResponse("HWUPKR0MPOU8FGXBT394", "Apple Inc.", "US", "Cupertino")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	registry := newTestRegistry(ts.URL())
	cs := connectInMemory(t, registry)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "lei_lookup",
		Arguments: map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", contentText(res))
	}
	if res.StructuredContent == nil {
		t.Fatal("expected non-nil StructuredContent")
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(raw), "Apple Inc.") {
		t.Errorf("structuredContent missing 'Apple Inc.': %s", raw)
	}
}

// TestCallToolMissingRequired confirms the input schema rejects a missing
// required field before the handler runs (required-ness is now schema-enforced).
func TestCallToolMissingRequired(t *testing.T) {
	registry := newTestRegistry("http://example.invalid")
	cs := connectInMemory(t, registry)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "lei_lookup",
		Arguments: map[string]any{}, // missing required "lei"
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for missing required 'lei' (schema validation)")
	}
}

// TestPanicRecovery is the HG-1 coverage assertion: a panicking handler must
// surface as a structured tool error with a correlation ID, and the panic value
// must never reach the caller.
func TestPanicRecovery(t *testing.T) {
	registry := newTestRegistry("http://example.invalid")
	server := mcp.NewServer(&mcp.Implementation{Name: "gleif-test", Version: "0.0.0"}, nil)
	spec := ToolSpec{Name: "boom", Title: "Boom", Description: "panics for testing", ReadOnly: true}
	registerTyped(registry, server, spec, func(_ context.Context, _ LEILookupArgs) (IssuersResult, error) {
		panic("kaboom-secret-internal-detail")
	})

	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = ss.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "boom",
		Arguments: map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError after handler panic")
	}
	text := contentText(res)
	if strings.Contains(text, "kaboom-secret-internal-detail") {
		t.Errorf("panic value leaked to caller: %s", text)
	}
	if !strings.Contains(text, "correlation_id=") {
		t.Errorf("expected correlation_id in caller-facing error, got: %s", text)
	}
}
