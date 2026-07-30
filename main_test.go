package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// connectToTestServer stands up the real server from newServer and returns a
// connected client session. Driving the assembled server end to end is the point:
// asserting on cacheConfig alone would still pass if the middleware were never
// attached.
func connectToTestServer(t *testing.T) *mcp.ClientSession {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := newServer(gleif.NewClient(gleif.DefaultConfig(), logger), logger)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

// The regression this guards: the SDK leaves ttlMs at 0, which the MCP
// 2026-07-28 spec defines as immediately stale. Removing the
// AddReceivingMiddleware call in newServer must fail this test.
func TestToolsListAdvertisesCacheTTL(t *testing.T) {
	session := connectToTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(res.Tools) == 0 {
		t.Fatal("tools/list returned no tools; the fixture is not exercising the real registry")
	}

	want := int(toolListTTL.Milliseconds())
	if got := res.GetTTLMs(); got != want {
		t.Errorf("tools/list ttlMs = %d, want %d", got, want)
	}

	// The SDK defaults this to "public"; assert it survived the middleware,
	// since a wrong scope on a cached result is a cross-user leak, not a
	// performance nit.
	if got := res.GetCacheScope(); got != "public" {
		t.Errorf("tools/list cacheScope = %q, want %q", got, "public")
	}
}

// tools/call is not a cacheable method and must pass through unstamped, proving
// the middleware is selective rather than blanket.
func TestNonListMethodIsNotStamped(t *testing.T) {
	session := connectToTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// validate_lei is offline: it checks the LEI checksum locally, so this makes
	// no network call to GLEIF.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "validate_lei",
		Arguments: map[string]any{"lei": "HWUPKR0MPOU8FGXBT394"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if _, cacheable := any(res).(mcp.CacheableResult); cacheable {
		t.Error("CallToolResult implements CacheableResult; this test's premise needs revisiting")
	}
}
