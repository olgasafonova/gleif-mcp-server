package gleif

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ValidateLEI checks if an LEI is valid and returns its status.
func (c *Client) ValidateLEI(ctx context.Context, lei string) (*ValidationResult, error) {
	lei = strings.ToUpper(strings.TrimSpace(lei))

	// Check cache first
	if result, ok := c.cache.GetValidation(lei); ok {
		return result, nil
	}

	result := &ValidationResult{LEI: lei}

	// First check format
	if !leiRegex.MatchString(lei) {
		result.Valid = false
		result.Message = "Invalid LEI format: must be 20 alphanumeric characters"
		return result, nil
	}

	// Validate check digits (ISO 17442)
	if !validateLEICheckDigits(lei) {
		result.Valid = false
		result.Message = "Invalid check digits: LEI fails ISO 17442 validation"
		return result, nil
	}

	// Try to fetch the record. Only cache a negative result when GLEIF
	// returned a definitive 404 — caching transient errors (5xx, timeout,
	// rate-limit retry exhaustion) as "not found" would poison the cache
	// for the configured ValidationTTL (24 hours by default), allowing an
	// attacker who can briefly disrupt the upstream path (rate-limiter
	// drain, network blip, GLEIF incident) to mark any LEI as invalid for
	// the rest of the day.
	record, err := c.GetLEI(ctx, lei)
	if err != nil {
		result.Valid = false
		var apiErr *APIError
		isDefinitiveNotFound := errors.As(err, &apiErr) && apiErr.Code == ErrCodeNotFound
		if isDefinitiveNotFound {
			result.Message = "LEI not found in GLEIF database"
			c.cache.SetValidation(lei, result)
		} else {
			// Transient — propagate the error context, do NOT cache.
			result.Message = fmt.Sprintf("Validation unavailable: %s", err.Error())
		}
		return result, nil
	}

	result.Valid = true
	result.Status = record.Registration.Status
	result.EntityStatus = record.Entity.Status
	result.NextRenewal = record.Registration.NextRenewalDate.Format("2006-01-02")
	result.Message = fmt.Sprintf("Valid LEI, registration status: %s", record.Registration.Status)

	c.cache.SetValidation(lei, result)
	return result, nil
}

// validateLEICheckDigits validates the ISO 17442 check digits.
func validateLEICheckDigits(lei string) bool {
	if len(lei) != 20 {
		return false
	}

	// Convert letters to numbers (A=10, B=11, ..., Z=35)
	var numStr strings.Builder
	for _, ch := range lei {
		if ch >= 'A' && ch <= 'Z' {
			fmt.Fprintf(&numStr, "%d", ch-'A'+10)
		} else if ch >= '0' && ch <= '9' {
			numStr.WriteByte(byte(ch))
		} else {
			return false
		}
	}

	// Calculate mod 97 (ISO 7064)
	num := numStr.String()
	remainder := 0
	for _, ch := range num {
		remainder = (remainder*10 + int(ch-'0')) % 97
	}

	return remainder == 1
}
