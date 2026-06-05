package tools

import (
	"context"
	"strings"
	"testing"
)

// Tests for the registration/dispatch machinery in registry.go
// (Registry, NewRegistry, lookupHandler, wrapHandler). Shared test helpers
// (newTestRegistry, makeRequest, getResultText) live in handlers_test.go in
// the same package.

// TestGetHandler tests the getHandler dispatcher.
func TestGetHandler(t *testing.T) {
	registry := newTestRegistry("http://unused")

	tests := []struct {
		name     string
		toolName string
		wantNil  bool
	}{
		{"lei_lookup", "lei_lookup", false},
		{"validate_lei", "validate_lei", false},
		{"batch_lei_lookup", "batch_lei_lookup", false},
		{"search_entity", "search_entity", false},
		{"search_by_bic", "search_by_bic", false},
		{"search_by_isin", "search_by_isin", false},
		{"search_by_country", "search_by_country", false},
		{"get_relationships", "get_relationships", false},
		{"autocomplete", "autocomplete", false},
		{"get_lei_issuer", "get_lei_issuer", false},
		{"list_lei_issuers", "list_lei_issuers", false},
		{"get_reporting_exceptions", "get_reporting_exceptions", false},
		{"unknown_tool", "nonexistent_tool", false}, // Returns error handler
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := registry.lookupHandler(tt.toolName)
			if handler == nil && !tt.wantNil {
				t.Errorf("lookupHandler(%q) returned nil", tt.toolName)
			}
		})
	}
}

// TestUnknownToolHandler tests that unknown tools return an error.
func TestUnknownToolHandler(t *testing.T) {
	registry := newTestRegistry("http://unused")
	handler := registry.lookupHandler("nonexistent_tool")
	req := makeRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Unknown tool handler returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for unknown tool")
	}
	text := getResultText(result)
	if !strings.Contains(text, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' in error message, got: %s", text)
	}
}
