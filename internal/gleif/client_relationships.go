package gleif

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetRelationships retrieves parent/child relationships for an LEI.
func (c *Client) GetRelationships(ctx context.Context, lei LEI, relType string) ([]Relationship, error) {
	endpoint := relationshipEndpoint(relType)
	reqURL := fmt.Sprintf("%s/lei-records/%s/%s", c.baseURL, string(lei), endpoint)
	c.logger.Debug("Fetching relationships", "lei", string(lei), "type", relType, "url", reqURL)

	return c.fetchRelationships(ctx, reqURL)
}

// relationshipEndpoint maps a relationship type to its GLEIF endpoint
// suffix. Unknown types fall through to direct-parent (matches the prior
// switch's default arm).
func relationshipEndpoint(relType string) string {
	switch strings.ToLower(relType) {
	case "ultimate-parent", "ultimate":
		return "ultimate-parent-relationship"
	case "direct-children", "children":
		return "direct-child-relationships"
	case "fund-manager":
		return "fund-manager-relationship"
	case "umbrella-fund":
		return "umbrella-fund-relationship"
	case "sub-funds":
		return "sub-fund-relationships"
	default:
		return "direct-parent-relationship"
	}
}

// relData mirrors GLEIF's relationship envelope. Hoisted out of GetRelationships
// so both array and single-shape decode paths share the same struct.
type relData struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Relationship Relationship `json:"relationship"`
	} `json:"attributes"`
}

// fetchRelationships handles GLEIF's two response shapes for relationship
// endpoints: list (children/funds) and singleton (parent). Tries list first,
// falls through to singleton on decode failure.
func (c *Client) fetchRelationships(ctx context.Context, reqURL string) ([]Relationship, error) {
	type relListResponse struct {
		Data []relData `json:"data"`
	}
	type relSingleResponse struct {
		Data relData `json:"data"`
	}

	var listResp relListResponse
	listErr := c.doRequestWithRetry(ctx, reqURL, &listResp)
	if listErr == nil {
		return relationshipsFromList(listResp.Data), nil
	}

	var singleResp relSingleResponse
	if err := c.doRequestWithRetry(ctx, reqURL, &singleResp); err != nil {
		return nil, listErr
	}
	if singleResp.Data.ID == "" {
		return nil, listErr
	}
	return []Relationship{singleResp.Data.Attributes.Relationship}, nil
}

func relationshipsFromList(data []relData) []Relationship {
	rels := make([]Relationship, len(data))
	for i, item := range data {
		rels[i] = item.Attributes.Relationship
	}
	return rels
}

// GetLEIIssuer retrieves information about an LEI issuer (LOU).
func (c *Client) GetLEIIssuer(ctx context.Context, issuerID IssuerID) (*LEIIssuer, error) {
	// url.PathEscape is a no-op for validator-approved IDs (the regex already
	// restricts to URL-safe chars), but stays as belt-and-braces against
	// future regex loosening.
	reqURL := fmt.Sprintf("%s/lei-issuers/%s", c.baseURL, url.PathEscape(string(issuerID)))
	c.logger.Debug("Fetching LEI issuer", "id", string(issuerID), "url", reqURL)

	var resp SingleResponse[LEIIssuer]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	issuer := resp.Data.Attributes
	issuer.ID = resp.Data.ID
	return &issuer, nil
}

// ListLEIIssuers lists all LEI issuers (LOUs).
func (c *Client) ListLEIIssuers(ctx context.Context) ([]LEIIssuer, error) {
	reqURL := fmt.Sprintf("%s/lei-issuers", c.baseURL)
	c.logger.Debug("Listing LEI issuers", "url", reqURL)

	var resp APIResponse[LEIIssuer]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	issuers := make([]LEIIssuer, len(resp.Data))
	for i, item := range resp.Data {
		issuers[i] = item.Attributes
		issuers[i].ID = item.ID
	}
	return issuers, nil
}

// GetReportingExceptions retrieves reporting exceptions for an LEI.
func (c *Client) GetReportingExceptions(ctx context.Context, lei LEI) ([]ReportingException, error) {
	reqURL := fmt.Sprintf("%s/lei-records/%s/reporting-exceptions", c.baseURL, string(lei))
	c.logger.Debug("Fetching reporting exceptions", "lei", string(lei), "url", reqURL)

	var resp APIResponse[ReportingException]
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		return nil, err
	}

	exceptions := make([]ReportingException, len(resp.Data))
	for i, item := range resp.Data {
		exceptions[i] = item.Attributes
	}
	return exceptions, nil
}
