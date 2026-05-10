package gleif

import (
	"context"
	"errors"
	"fmt"
)

// ValidateLEI checks if an LEI is valid and returns its status. Unlike the
// other lookups, this method takes a raw caller-supplied string because it
// needs to report format and check-digit failures back to the caller as
// validation results, not refuse to construct.
func (c *Client) ValidateLEI(ctx context.Context, raw string) (*ValidationResult, error) {
	normalized := normalize(raw)

	if result, ok := c.cache.GetValidation(normalized); ok {
		return result, nil
	}

	result := &ValidationResult{LEI: normalized}

	parsed, err := ParseLEI(raw)
	if err != nil {
		result.Valid = false
		result.Message = "Invalid LEI format: must be 20 alphanumeric characters"
		return result, nil
	}

	if !parsed.HasValidCheckDigits() {
		result.Valid = false
		result.Message = "Invalid check digits: LEI fails ISO 17442 validation"
		return result, nil
	}

	return c.validateLEIAgainstAPI(ctx, parsed, result)
}

// validateLEIAgainstAPI fetches the record and shapes the validation result.
// Only caches a definitive 404; transient errors propagate without cache write
// to avoid the 24-hour cache-poisoning window that a network blip could open.
func (c *Client) validateLEIAgainstAPI(ctx context.Context, lei LEI, result *ValidationResult) (*ValidationResult, error) {
	record, err := c.GetLEI(ctx, lei)
	if err != nil {
		result.Valid = false
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == ErrCodeNotFound {
			result.Message = "LEI not found in GLEIF database"
			c.cache.SetValidation(string(lei), result)
			return result, nil
		}
		result.Message = fmt.Sprintf("Validation unavailable: %s", err.Error())
		return result, nil
	}

	result.Valid = true
	result.Status = record.Registration.Status
	result.EntityStatus = record.Entity.Status
	result.NextRenewal = record.Registration.NextRenewalDate.Format("2006-01-02")
	result.Message = fmt.Sprintf("Valid LEI, registration status: %s", record.Registration.Status)

	c.cache.SetValidation(string(lei), result)
	return result, nil
}
