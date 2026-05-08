package gleif

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetRelationships retrieves parent/child relationships for an LEI.
func (c *Client) GetRelationships(ctx context.Context, lei string, relType string) ([]Relationship, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	// Build relationship URL
	var endpoint string
	switch strings.ToLower(relType) {
	case "direct-parent", "parent":
		endpoint = "direct-parent-relationship"
	case "ultimate-parent", "ultimate":
		endpoint = "ultimate-parent-relationship"
	case "direct-children", "children":
		endpoint = "direct-child-relationships"
	case "fund-manager":
		endpoint = "fund-manager-relationship"
	case "umbrella-fund":
		endpoint = "umbrella-fund-relationship"
	case "sub-funds":
		endpoint = "sub-fund-relationships"
	default:
		// Get all relationships
		endpoint = "direct-parent-relationship"
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s/%s", c.baseURL, lei, endpoint)
	c.logger.Debug("Fetching relationships", "lei", lei, "type", relType, "url", reqURL)

	// Relationship responses have a different structure
	type RelData struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Relationship Relationship `json:"relationship"`
		} `json:"attributes"`
	}
	type RelResponse struct {
		Data []RelData `json:"data"`
	}

	var resp RelResponse
	if err := c.doRequestWithRetry(ctx, reqURL, &resp); err != nil {
		// Check if it's a single response (for parent relationships)
		type SingleRelResponse struct {
			Data RelData `json:"data"`
		}
		var singleResp SingleRelResponse
		if err2 := c.doRequestWithRetry(ctx, reqURL, &singleResp); err2 != nil {
			return nil, err
		}
		if singleResp.Data.ID != "" {
			return []Relationship{singleResp.Data.Attributes.Relationship}, nil
		}
		return nil, err
	}

	rels := make([]Relationship, len(resp.Data))
	for i, item := range resp.Data {
		rels[i] = item.Attributes.Relationship
	}
	return rels, nil
}

// GetLEIIssuer retrieves information about an LEI issuer (LOU).
func (c *Client) GetLEIIssuer(ctx context.Context, issuerID string) (*LEIIssuer, error) {
	if err := ValidateIssuerID(issuerID); err != nil {
		return nil, err
	}
	issuerID = strings.ToUpper(strings.TrimSpace(issuerID))
	// url.PathEscape is a no-op for validator-approved IDs (the regex already
	// restricts to URL-safe chars), but stays as belt-and-braces against
	// future regex loosening.
	reqURL := fmt.Sprintf("%s/lei-issuers/%s", c.baseURL, url.PathEscape(issuerID))
	c.logger.Debug("Fetching LEI issuer", "id", issuerID, "url", reqURL)

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
func (c *Client) GetReportingExceptions(ctx context.Context, lei string) ([]ReportingException, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))
	if !leiRegex.MatchString(lei) {
		return nil, NewInvalidFormatError("LEI", "must be 20 alphanumeric characters")
	}

	reqURL := fmt.Sprintf("%s/lei-records/%s/reporting-exceptions", c.baseURL, lei)
	c.logger.Debug("Fetching reporting exceptions", "lei", lei, "url", reqURL)

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
