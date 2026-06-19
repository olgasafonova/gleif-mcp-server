package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/olgasafonova/gleif-mcp-server/internal/gleif"
)

// Handler implementations
//
// Each handler is a typed method `func(ctx, Args) (Result, error)` registered
// via the generic mcp.AddTool path (see registry.go). The go-sdk validates the
// arguments against the Args-derived input schema before the handler runs, so
// required-field presence is enforced upstream; handlers validate value-level
// constraints (format, range) via the gleif.Parse* validators. On error the
// handler returns a plain error, which registry.go wraps with the tool name
// and the go-sdk surfaces as a structured tool error. On success it returns the
// typed Result, which the go-sdk emits as structuredContent + a JSON text block.

// --- Clean tools: return existing client types directly ---

func (r *Registry) handleLEILookup(ctx context.Context, args LEILookupArgs) (*gleif.LEIRecord, error) {
	lei, err := gleif.ParseLEI(args.LEI)
	if err != nil {
		return nil, fmt.Errorf("invalid LEI: %w", err)
	}
	record, err := r.client.GetLEI(ctx, lei)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch LEI: %w", err)
	}
	return record, nil
}

func (r *Registry) handleValidateLEI(ctx context.Context, args ValidateLEIArgs) (*gleif.ValidationResult, error) {
	// ValidateLEI accepts raw input on purpose: the tool's job is to report
	// format/check-digit failures as validation results, not errors.
	result, err := r.client.ValidateLEI(ctx, args.LEI)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	return result, nil
}

func (r *Registry) handleGetLEIIssuer(ctx context.Context, args GetLEIIssuerArgs) (*gleif.LEIIssuer, error) {
	issuerID, err := gleif.ParseIssuerID(args.IssuerID)
	if err != nil {
		return nil, fmt.Errorf("invalid issuer_id: %w", err)
	}
	issuer, err := r.client.GetLEIIssuer(ctx, issuerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch LEI issuer: %w", err)
	}
	return issuer, nil
}

// --- Tools with structured Result types ---

func (r *Registry) handleBatchLEILookup(ctx context.Context, args BatchLEIArgs) (BatchResult, error) {
	leis, err := parseBatchLEIs(args.LEIs)
	if err != nil {
		return BatchResult{}, fmt.Errorf("invalid LEI in batch: %w", err)
	}
	if len(leis) == 0 {
		return BatchResult{}, fmt.Errorf("no valid LEIs provided; pass one or more comma-separated 20-character LEI codes")
	}
	records, err := r.client.GetBatchLEI(ctx, leis)
	if err != nil {
		return BatchResult{}, fmt.Errorf("batch lookup failed: %w", err)
	}
	return buildBatchResult(leis, records), nil
}

// parseBatchLEIs splits the comma-separated input and parses each entry.
// Returns the first parse error so the caller can surface it.
func parseBatchLEIs(raw string) ([]gleif.LEI, error) {
	parts := strings.Split(raw, ",")
	leis := make([]gleif.LEI, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lei, err := gleif.ParseLEI(part)
		if err != nil {
			return nil, err
		}
		leis = append(leis, lei)
	}
	return leis, nil
}

// buildBatchResult shapes the batch results, including the not-found set.
func buildBatchResult(requested []gleif.LEI, records []gleif.LEIRecord) BatchResult {
	results := make([]SimpleRecord, len(records))
	foundLEIs := make(map[string]bool, len(records))
	for i, rec := range records {
		results[i] = simplifyRecord(rec)
		foundLEIs[rec.LEI] = true
	}

	var notFound []string
	for _, lei := range requested {
		if !foundLEIs[string(lei)] {
			notFound = append(notFound, string(lei))
		}
	}

	return BatchResult{
		Requested: len(requested),
		Found:     len(results),
		Results:   results,
		NotFound:  notFound,
	}
}

func (r *Registry) handleSearchEntity(ctx context.Context, args SearchEntityArgs) (SearchEntityResult, error) {
	if args.Query == "" {
		return SearchEntityResult{}, fmt.Errorf("query parameter is required")
	}

	limit := args.Limit
	if limit == 0 {
		limit = 20
	}
	page := args.Page
	if page == 0 {
		page = 1
	}
	fuzzy := true
	if args.Fuzzy != nil {
		fuzzy = *args.Fuzzy
	}

	records, pagination, searchErr := r.executeSearch(ctx, searchRequest{
		query: args.Query,
		limit: limit,
		page:  page,
		fuzzy: fuzzy,
	})
	if searchErr != nil {
		return SearchEntityResult{}, fmt.Errorf("search failed: %w", searchErr)
	}

	return buildSearchResult(records, pagination), nil
}

// searchRequest groups parameters for executeSearch. Keeping them together
// avoids the long argument list that codescene flagged.
type searchRequest struct {
	query string
	limit int
	page  int
	fuzzy bool
}

// executeSearch picks the search backend based on the fuzzy flag.
func (r *Registry) executeSearch(ctx context.Context, req searchRequest) ([]gleif.LEIRecord, *gleif.Pagination, error) {
	if req.fuzzy {
		return r.client.FuzzySearch(ctx, req.query, req.limit, req.page)
	}
	return r.client.SearchEntities(ctx, req.query, req.limit, req.page)
}

func buildSearchResult(records []gleif.LEIRecord, pagination *gleif.Pagination) SearchEntityResult {
	results := make([]SimpleRecord, len(records))
	for i, rec := range records {
		results[i] = simplifyRecord(rec)
	}
	result := SearchEntityResult{
		Count:   len(results),
		Results: results,
	}
	if pagination != nil {
		result.Pagination = &PageInfo{
			CurrentPage: pagination.CurrentPage,
			PerPage:     pagination.PerPage,
			Total:       pagination.Total,
			LastPage:    pagination.LastPage,
		}
		hasMore := pagination.CurrentPage < pagination.LastPage
		result.HasMore = &hasMore
	}
	return result
}

func (r *Registry) handleSearchByBIC(ctx context.Context, args SearchByBICArgs) (IDSearchResult, error) {
	bic, err := gleif.ParseBIC(args.BIC)
	if err != nil {
		return IDSearchResult{}, fmt.Errorf("invalid BIC: %w", err)
	}
	records, err := r.client.SearchByBIC(ctx, bic)
	if err != nil {
		return IDSearchResult{}, fmt.Errorf("BIC search failed: %w", err)
	}
	return buildIDSearchResult(records, "No LEI found for this BIC code"), nil
}

func (r *Registry) handleSearchByISIN(ctx context.Context, args SearchByISINArgs) (IDSearchResult, error) {
	isin, err := gleif.ParseISIN(args.ISIN)
	if err != nil {
		return IDSearchResult{}, fmt.Errorf("invalid ISIN: %w", err)
	}
	records, err := r.client.SearchByISIN(ctx, isin)
	if err != nil {
		return IDSearchResult{}, fmt.Errorf("ISIN search failed: %w", err)
	}
	return buildIDSearchResult(records, "No issuer LEI found for this ISIN"), nil
}

// buildIDSearchResult shapes BIC/ISIN search results uniformly. The empty case
// returns a non-nil (empty) records slice plus a message; the found case omits
// the message.
func buildIDSearchResult(records []gleif.LEIRecord, emptyMessage string) IDSearchResult {
	if len(records) == 0 {
		return IDSearchResult{
			Found:   false,
			Count:   0,
			Records: []gleif.LEIRecord{},
			Message: emptyMessage,
		}
	}
	return IDSearchResult{
		Found:   true,
		Count:   len(records),
		Records: records,
	}
}

func (r *Registry) handleSearchByCountry(ctx context.Context, args SearchByCountryArgs) (CountrySearchResult, error) {
	country, err := gleif.ParseCountry(args.Country)
	if err != nil {
		return CountrySearchResult{}, fmt.Errorf("invalid country: %w", err)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 20
	}
	records, err := r.client.SearchByCountry(ctx, country, limit)
	if err != nil {
		return CountrySearchResult{}, fmt.Errorf("country search failed: %w", err)
	}

	results := make([]CountryRecord, len(records))
	for i, rec := range records {
		results[i] = CountryRecord{
			LEI:       rec.LEI,
			LegalName: rec.Entity.LegalName.Name,
			City:      rec.Entity.LegalAddress.City,
			Status:    rec.Entity.Status,
		}
	}
	return CountrySearchResult{
		Country: string(country),
		Count:   len(results),
		Results: results,
	}, nil
}

func (r *Registry) handleGetRelationships(ctx context.Context, args GetRelationshipsArgs) (RelationshipsResult, error) {
	lei, err := gleif.ParseLEI(args.LEI)
	if err != nil {
		return RelationshipsResult{}, fmt.Errorf("invalid LEI: %w", err)
	}

	relType := "direct-parent"
	if args.Type != "" {
		relType = args.Type
	}

	relationships, err := r.client.GetRelationships(ctx, lei, relType)
	if err != nil {
		return RelationshipsResult{}, fmt.Errorf("relationship lookup failed: %w", err)
	}

	return RelationshipsResult{
		LEI:           string(lei),
		Type:          relType,
		Relationships: relationships,
	}, nil
}

func (r *Registry) handleAutocomplete(ctx context.Context, args AutocompleteArgs) (AutocompleteToolResult, error) {
	if len(args.Prefix) < 2 {
		return AutocompleteToolResult{}, fmt.Errorf("prefix must be at least 2 characters")
	}

	limit := args.Limit
	if limit == 0 {
		limit = 10
	}
	suggestions, err := r.client.Autocomplete(ctx, args.Prefix, limit)
	if err != nil {
		return AutocompleteToolResult{}, fmt.Errorf("autocomplete failed: %w", err)
	}

	return AutocompleteToolResult{
		Prefix:      args.Prefix,
		Suggestions: suggestions,
	}, nil
}

func (r *Registry) handleListLEIIssuers(ctx context.Context, _ ListLEIIssuersArgs) (IssuersResult, error) {
	issuers, err := r.client.ListLEIIssuers(ctx)
	if err != nil {
		return IssuersResult{}, fmt.Errorf("failed to list LEI issuers: %w", err)
	}
	return IssuersResult{
		Count:   len(issuers),
		Issuers: issuers,
	}, nil
}

func (r *Registry) handleGetReportingExceptions(ctx context.Context, args GetReportingExceptionsArgs) (ReportingExceptionsResult, error) {
	lei, err := gleif.ParseLEI(args.LEI)
	if err != nil {
		return ReportingExceptionsResult{}, fmt.Errorf("invalid LEI: %w", err)
	}
	exceptions, err := r.client.GetReportingExceptions(ctx, lei)
	if err != nil {
		return ReportingExceptionsResult{}, fmt.Errorf("failed to fetch reporting exceptions: %w", err)
	}
	return ReportingExceptionsResult{
		LEI:        string(lei),
		Count:      len(exceptions),
		Exceptions: exceptions,
	}, nil
}

// simplifyRecord builds the trimmed-down representation used by batch and
// search responses. Centralized to keep the field list in one place.
func simplifyRecord(rec gleif.LEIRecord) SimpleRecord {
	return SimpleRecord{
		LEI:       rec.LEI,
		LegalName: rec.Entity.LegalName.Name,
		Country:   rec.Entity.LegalAddress.Country,
		City:      rec.Entity.LegalAddress.City,
		Status:    rec.Entity.Status,
		RegStatus: rec.Registration.Status,
	}
}
