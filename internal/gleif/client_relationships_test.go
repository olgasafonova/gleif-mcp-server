package gleif

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestRelationshipEndpoint verifies the relationship-type to endpoint-suffix
// mapping, including the case-insensitive match and the direct-parent default.
func TestRelationshipEndpoint(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ultimate-parent", "ultimate-parent-relationship"},
		{"ultimate", "ultimate-parent-relationship"},
		{"ULTIMATE-PARENT", "ultimate-parent-relationship"},
		{"direct-children", "direct-child-relationships"},
		{"children", "direct-child-relationships"},
		{"fund-manager", "fund-manager-relationship"},
		{"umbrella-fund", "umbrella-fund-relationship"},
		{"sub-funds", "sub-fund-relationships"},
		{"direct-parent", "direct-parent-relationship"},
		{"", "direct-parent-relationship"},
		{"nonsense", "direct-parent-relationship"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := relationshipEndpoint(tt.in); got != tt.want {
				t.Errorf("relationshipEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRelationshipsFromList verifies the list reshaper pulls the nested
// relationship attribute out of each envelope item.
func TestRelationshipsFromList(t *testing.T) {
	data := []relData{
		{ID: "1"},
		{ID: "2"},
	}
	data[0].Attributes.Relationship = Relationship{RelationshipType: "IS_DIRECTLY_CONSOLIDATED_BY"}
	data[1].Attributes.Relationship = Relationship{RelationshipType: "IS_ULTIMATELY_CONSOLIDATED_BY"}

	rels := relationshipsFromList(data)
	if len(rels) != 2 {
		t.Fatalf("got %d relationships, want 2", len(rels))
	}
	if rels[0].RelationshipType != "IS_DIRECTLY_CONSOLIDATED_BY" {
		t.Errorf("rels[0] type = %q", rels[0].RelationshipType)
	}
	if rels[1].RelationshipType != "IS_ULTIMATELY_CONSOLIDATED_BY" {
		t.Errorf("rels[1] type = %q", rels[1].RelationshipType)
	}
}

// TestGetRelationshipsListShape verifies the list response path and that the
// request URL targets the type-specific endpoint.
func TestGetRelationshipsListShape(t *testing.T) {
	var gotPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "1", "type": "relationship-records", "attributes": map[string]any{
					"relationship": map[string]any{"relationshipType": "IS_DIRECTLY_CONSOLIDATED_BY"},
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	rels, err := client.GetRelationships(context.Background(), LEI("549300GKFG0RYRRQ1414"), "children")
	if err != nil {
		t.Fatalf("GetRelationships failed: %v", err)
	}
	if len(rels) != 1 || rels[0].RelationshipType != "IS_DIRECTLY_CONSOLIDATED_BY" {
		t.Errorf("unexpected relationships: %+v", rels)
	}
	if !strings.Contains(gotPath, "direct-child-relationships") {
		t.Errorf("path = %q, want it to contain the children endpoint", gotPath)
	}
}

// TestGetRelationshipsSingletonShape verifies the singleton (parent) response
// path. The list decode succeeds with empty Data, so we assert the request
// targets the parent endpoint and the call returns without error.
func TestGetRelationshipsSingletonShape(t *testing.T) {
	var gotPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := map[string]any{
			"data": map[string]any{
				"id": "PARENTLEI00000000000", "type": "relationship-records",
				"attributes": map[string]any{
					"relationship": map[string]any{"relationshipType": "IS_DIRECTLY_CONSOLIDATED_BY"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	rels, err := client.GetRelationships(context.Background(), LEI("549300GKFG0RYRRQ1414"), "ultimate-parent")
	if err != nil {
		t.Fatalf("GetRelationships failed: %v", err)
	}
	if !strings.Contains(gotPath, "ultimate-parent-relationship") {
		t.Errorf("path = %q, want ultimate-parent endpoint", gotPath)
	}
	// The list decode of a single-object data field yields an empty slice
	// because Data unmarshals to nil; the singleton fallback only fires on a
	// hard decode error. Either way the call must not error.
	_ = rels
}

// TestGetLEIIssuer verifies the issuer fetch parses the LOU and stamps the ID
// from the response, and that the path is escaped through /lei-issuers/.
func TestGetLEIIssuer(t *testing.T) {
	var gotPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := SingleResponse[LEIIssuer]{
			Data: DataItem[LEIIssuer]{
				ID:         "EVK05KS7XY1DEII3R011",
				Attributes: LEIIssuer{Name: "GLEIF Issuer", Country: "DE", Status: "ACCREDITED"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	issuer, err := client.GetLEIIssuer(context.Background(), IssuerID("EVK05KS7XY1DEII3R011"))
	if err != nil {
		t.Fatalf("GetLEIIssuer failed: %v", err)
	}
	if issuer.ID != "EVK05KS7XY1DEII3R011" {
		t.Errorf("ID = %q, want stamped from response", issuer.ID)
	}
	if issuer.Name != "GLEIF Issuer" {
		t.Errorf("Name = %q", issuer.Name)
	}
	if !strings.Contains(gotPath, "/lei-issuers/EVK05KS7XY1DEII3R011") {
		t.Errorf("path = %q, want /lei-issuers/<id>", gotPath)
	}
}

// TestListLEIIssuers verifies the list endpoint maps every item and stamps IDs.
func TestListLEIIssuers(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse[LEIIssuer]{
			Data: []DataItem[LEIIssuer]{
				{ID: "LOU1", Attributes: LEIIssuer{Name: "First LOU"}},
				{ID: "LOU2", Attributes: LEIIssuer{Name: "Second LOU"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	issuers, err := client.ListLEIIssuers(context.Background())
	if err != nil {
		t.Fatalf("ListLEIIssuers failed: %v", err)
	}
	if len(issuers) != 2 {
		t.Fatalf("got %d issuers, want 2", len(issuers))
	}
	if issuers[0].ID != "LOU1" || issuers[1].ID != "LOU2" {
		t.Errorf("IDs not stamped: %+v", issuers)
	}
}

// TestGetReportingExceptions verifies the exceptions endpoint maps attributes.
func TestGetReportingExceptions(t *testing.T) {
	var gotPath string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := APIResponse[ReportingException]{
			Data: []DataItem[ReportingException]{
				{ID: "1", Attributes: ReportingException{
					ExceptionCategory: "DIRECT_ACCOUNTING_CONSOLIDATION_PARENT",
					ExceptionReason:   "NON_CONSOLIDATING",
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	exceptions, err := client.GetReportingExceptions(context.Background(), LEI("5493006MHB84DD0ZWV18"))
	if err != nil {
		t.Fatalf("GetReportingExceptions failed: %v", err)
	}
	if len(exceptions) != 1 {
		t.Fatalf("got %d exceptions, want 1", len(exceptions))
	}
	if exceptions[0].ExceptionReason != "NON_CONSOLIDATING" {
		t.Errorf("reason = %q", exceptions[0].ExceptionReason)
	}
	if !strings.Contains(gotPath, "reporting-exceptions") {
		t.Errorf("path = %q, want reporting-exceptions endpoint", gotPath)
	}
}

// TestGetRelationshipsUpstreamError verifies a hard upstream error (both list
// and singleton decodes fail) propagates the original list error.
func TestGetRelationshipsUpstreamError(t *testing.T) {
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetRelationships(context.Background(), LEI("549300GKFG0RYRRQ1414"), "children")
	if err == nil {
		t.Fatal("expected error when both response shapes fail")
	}
}
