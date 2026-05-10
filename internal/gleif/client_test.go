package gleif

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestValidateLEICheckDigits tests the ISO 17442 check digit validation.
func TestValidateLEICheckDigits(t *testing.T) {
	tests := []struct {
		name  string
		lei   string
		valid bool
	}{
		// Valid LEIs (from real GLEIF data)
		{"Apple Inc.", "HWUPKR0MPOU8FGXBT394", true},
		{"Deutsche Bank AG", "7LTWFZYICNSX8D621K86", true},
		{"Google LLC", "5493006MHB84DD0ZWV18", true},
		{"HSBC Holdings", "MLU0ZO3ML4LN2LL2TL39", true},
		{"Microsoft Corp", "INR2EJN1ERAN0W5ZP974", true},

		// Invalid check digits
		{"Wrong check digit 1", "HWUPKR0MPOU8FGXBT395", false},
		{"Wrong check digit 2", "HWUPKR0MPOU8FGXBT300", false},
		{"Wrong check digit 3", "7LTWFZYICNSX8D621K00", false},

		// Wrong length
		{"Too short", "HWUPKR0MPOU8FGXBT39", false},
		{"Too long", "HWUPKR0MPOU8FGXBT3940", false},
		{"Empty", "", false},

		// Invalid characters
		{"Lowercase", "hwupkr0mpou8fgxbt394", false}, // Must be uppercase for validation
		{"Special chars", "HWUPKR0MPOU8FGX!T394", false},
		{"Spaces", "HWUPKR0MPOU8 GXBT394", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LEI(tt.lei).HasValidCheckDigits()
			if got != tt.valid {
				t.Errorf("LEI(%q).HasValidCheckDigits() = %v, want %v", tt.lei, got, tt.valid)
			}
		})
	}
}

// TestLEIRegex tests the LEI format regex pattern.
func TestLEIRegex(t *testing.T) {
	tests := []struct {
		name    string
		lei     string
		matches bool
	}{
		// Valid format (20 alphanumeric, specific pattern)
		{"Valid Apple LEI", "HWUPKR0MPOU8FGXBT394", true},
		{"Valid Deutsche Bank", "7LTWFZYICNSX8D621K86", true},
		{"All digits (valid format)", "12345678901234567890", true},
		{"All letters", "ABCDEFGHIJKLMNOPQR12", true},

		// Invalid format
		{"Too short", "HWUPKR0MPOU8FGXBT39", false},
		{"Too long", "HWUPKR0MPOU8FGXBT3940", false},
		{"Empty", "", false},
		{"Lowercase", "hwupkr0mpou8fgxbt394", false},
		{"Special chars", "HWUPKR0MPOU8FGX-T394", false},
		{"Spaces", "HWUPKR0MPOU8 GXBT394", false},
		{"Non-numeric check digits", "HWUPKR0MPOU8FGXBT3XX", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leiRegex.MatchString(tt.lei)
			if got != tt.matches {
				t.Errorf("leiRegex.MatchString(%q) = %v, want %v", tt.lei, got, tt.matches)
			}
		})
	}
}

// TestBICValidation tests BIC/SWIFT code length validation.
func TestBICValidation(t *testing.T) {
	tests := []struct {
		name  string
		bic   string
		valid bool
	}{
		// Valid BIC codes (8 or 11 characters)
		{"8-char BIC", "DEUTDEFF", true},
		{"11-char BIC", "DEUTDEFFXXX", true},
		{"8-char lowercase", "deutdeff", true}, // Will be normalized to uppercase

		// Invalid lengths
		{"7 chars", "DEUTDEF", false},
		{"9 chars", "DEUTDEFFF", false},
		{"10 chars", "DEUTDEFFFX", false},
		{"12 chars", "DEUTDEFFXXXX", false},
		{"Empty", "", false},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server that returns empty results
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := APIResponse[LEIRecord]{Data: []DataItem[LEIRecord]{}}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			clientWithMock := NewClient(Config{
				BaseURL:     server.URL,
				Timeout:     5 * time.Second,
				RateLimit:   100,
				BurstSize:   10,
				MaxRetries:  0,
				EnableCache: false,
			}, logger)

			parsed, parseErr := ParseBIC(tt.bic)
			if !tt.valid {
				if parseErr == nil {
					t.Errorf("ParseBIC(%q) = nil, expected error", tt.bic)
				}
				return
			}
			if parseErr != nil {
				t.Errorf("ParseBIC(%q) returned error: %v", tt.bic, parseErr)
				return
			}
			_, err := clientWithMock.SearchByBIC(context.Background(), parsed)
			if err != nil {
				var apiErr *APIError
				if ok := isAPIError(err, &apiErr); ok && apiErr.Code == ErrCodeInvalidFormat {
					t.Errorf("SearchByBIC(%q) returned format error, expected valid", tt.bic)
				}
			}
		})
	}
}

// TestISINValidation tests ISIN code length validation.
func TestISINValidation(t *testing.T) {
	tests := []struct {
		name  string
		isin  string
		valid bool
	}{
		// Valid ISIN codes (12 characters)
		{"Valid Apple ISIN", "US0378331005", true},
		{"Valid German ISIN", "DE0007164600", true},
		{"Lowercase", "us0378331005", true}, // Will be normalized

		// Invalid lengths
		{"11 chars", "US037833100", false},
		{"13 chars", "US03783310050", false},
		{"Empty", "", false},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := APIResponse[LEIRecord]{Data: []DataItem[LEIRecord]{}}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			clientWithMock := NewClient(Config{
				BaseURL:     server.URL,
				Timeout:     5 * time.Second,
				RateLimit:   100,
				BurstSize:   10,
				MaxRetries:  0,
				EnableCache: false,
			}, logger)

			parsed, parseErr := ParseISIN(tt.isin)
			if !tt.valid {
				if parseErr == nil {
					t.Errorf("ParseISIN(%q) = nil, expected error", tt.isin)
				}
				return
			}
			if parseErr != nil {
				t.Errorf("ParseISIN(%q) returned error: %v", tt.isin, parseErr)
				return
			}
			_, err := clientWithMock.SearchByISIN(context.Background(), parsed)
			if err != nil {
				var apiErr *APIError
				if ok := isAPIError(err, &apiErr); ok && apiErr.Code == ErrCodeInvalidFormat {
					t.Errorf("SearchByISIN(%q) returned format error, expected valid", tt.isin)
				}
			}
		})
	}
}

// TestCountryValidation tests country code validation.
func TestCountryValidation(t *testing.T) {
	tests := []struct {
		name    string
		country string
		valid   bool
	}{
		{"Valid US", "US", true},
		{"Valid DE", "DE", true},
		{"Lowercase", "us", true}, // Will be normalized

		// Invalid
		{"3 chars", "USA", false},
		{"1 char", "U", false},
		{"Empty", "", false},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := APIResponse[LEIRecord]{Data: []DataItem[LEIRecord]{}}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			clientWithMock := NewClient(Config{
				BaseURL:     server.URL,
				Timeout:     5 * time.Second,
				RateLimit:   100,
				BurstSize:   10,
				MaxRetries:  0,
				EnableCache: false,
			}, logger)

			parsed, parseErr := ParseCountry(tt.country)
			if !tt.valid {
				if parseErr == nil {
					t.Errorf("ParseCountry(%q) = nil, expected error", tt.country)
				}
				return
			}
			if parseErr != nil {
				t.Errorf("ParseCountry(%q) returned error: %v", tt.country, parseErr)
				return
			}
			_, err := clientWithMock.SearchByCountry(context.Background(), parsed, 10)
			if err != nil {
				var apiErr *APIError
				if ok := isAPIError(err, &apiErr); ok && apiErr.Code == ErrCodeInvalidFormat {
					t.Errorf("SearchByCountry(%q) returned format error, expected valid", tt.country)
				}
			}
		})
	}
}

// TestCacheHitMiss tests cache behavior.
func TestCacheHitMiss(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL:    time.Hour,
		ValidationTTL:   time.Hour,
		AutocompleteTTL: time.Hour,
		SearchTTL:       time.Hour,
		MaxEntries:      100,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	t.Run("LEI cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetLEI("NONEXISTENT0000000000")
		if ok {
			t.Error("Expected cache miss for nonexistent LEI")
		}
	})

	t.Run("LEI cache hit after set", func(t *testing.T) {
		lei := "HWUPKR0MPOU8FGXBT394"
		record := &LEIRecord{
			LEI: lei,
			Entity: Entity{
				LegalName: LegalName{Name: "Apple Inc."},
			},
		}

		cache.SetLEI(lei, record)

		got, ok := cache.GetLEI(lei)
		if !ok {
			t.Error("Expected cache hit after SetLEI")
		}
		if got.Entity.LegalName.Name != "Apple Inc." {
			t.Errorf("Got wrong record, expected Apple Inc., got %s", got.Entity.LegalName.Name)
		}
	})

	t.Run("Validation cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetValidation("NONEXISTENT0000000000")
		if ok {
			t.Error("Expected cache miss for nonexistent validation")
		}
	})

	t.Run("Validation cache hit after set", func(t *testing.T) {
		lei := "7LTWFZYICNSX8D621K86"
		result := &ValidationResult{
			LEI:   lei,
			Valid: true,
		}

		cache.SetValidation(lei, result)

		got, ok := cache.GetValidation(lei)
		if !ok {
			t.Error("Expected cache hit after SetValidation")
		}
		if !got.Valid {
			t.Error("Expected Valid=true")
		}
	})

	t.Run("Search cache miss on empty", func(t *testing.T) {
		_, ok := cache.GetSearch("nonexistent:query")
		if ok {
			t.Error("Expected cache miss for nonexistent search")
		}
	})

	t.Run("Search cache hit after set", func(t *testing.T) {
		key := "bic:DEUTDEFF"
		records := []LEIRecord{{LEI: "7LTWFZYICNSX8D621K86"}}

		cache.SetSearch(key, records)

		got, ok := cache.GetSearch(key)
		if !ok {
			t.Error("Expected cache hit after SetSearch")
		}
		if len(got) != 1 {
			t.Errorf("Expected 1 record, got %d", len(got))
		}
	})

	t.Run("Autocomplete cache hit after set", func(t *testing.T) {
		prefix := "Apple"
		results := []AutocompleteResult{{LEI: "HWUPKR0MPOU8FGXBT394", LegalName: "Apple Inc."}}

		cache.SetAutocomplete(prefix, results)

		got, ok := cache.GetAutocomplete(prefix)
		if !ok {
			t.Error("Expected cache hit after SetAutocomplete")
		}
		if len(got) != 1 || got[0].LegalName != "Apple Inc." {
			t.Error("Got wrong autocomplete results")
		}
	})
}

// TestCacheDisabled tests that disabled cache returns misses.
func TestCacheDisabled(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	lei := "HWUPKR0MPOU8FGXBT394"
	record := &LEIRecord{LEI: lei}
	cache.SetLEI(lei, record)

	_, ok := cache.GetLEI(lei)
	if ok {
		t.Error("Expected cache miss when cache is disabled")
	}
}

// TestCacheStats tests cache statistics tracking.
func TestCacheStats(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL: time.Hour,
		MaxEntries:   100,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	// Generate some hits and misses
	cache.GetLEI("MISS1")
	cache.GetLEI("MISS2")

	lei := "HWUPKR0MPOU8FGXBT394"
	cache.SetLEI(lei, &LEIRecord{LEI: lei})
	cache.GetLEI(lei) // hit

	stats := cache.Stats()
	if stats.Misses < 2 {
		t.Errorf("Expected at least 2 misses, got %d", stats.Misses)
	}
	if stats.Hits < 1 {
		t.Errorf("Expected at least 1 hit, got %d", stats.Hits)
	}
}

// TestCacheClear tests cache clearing.
func TestCacheClear(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		LEIRecordTTL: time.Hour,
		MaxEntries:   100,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	lei := "HWUPKR0MPOU8FGXBT394"
	cache.SetLEI(lei, &LEIRecord{LEI: lei})
	cache.SetSearch("test:key", []LEIRecord{{LEI: lei}})

	cache.Clear()

	if _, ok := cache.GetLEI(lei); ok {
		t.Error("Expected cache miss after Clear")
	}
	if _, ok := cache.GetSearch("test:key"); ok {
		t.Error("Expected search cache miss after Clear")
	}
}

// TestGetLEIWithMockServer tests GetLEI with a mock HTTP server.
func TestGetLEIWithMockServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("successful lookup", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := SingleResponse[LEIRecord]{
				Data: DataItem[LEIRecord]{
					ID:   "HWUPKR0MPOU8FGXBT394",
					Type: "lei-records",
					Attributes: LEIRecord{
						Entity: Entity{
							LegalName:    LegalName{Name: "Apple Inc."},
							LegalAddress: Address{Country: "US", City: "Cupertino"},
							Status:       "ACTIVE",
						},
						Registration: Registration{
							Status: "ISSUED",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(Config{
			BaseURL:     server.URL,
			Timeout:     5 * time.Second,
			RateLimit:   100,
			BurstSize:   10,
			MaxRetries:  0,
			EnableCache: false,
		}, logger)

		record, err := client.GetLEI(context.Background(), LEI("HWUPKR0MPOU8FGXBT394"))
		if err != nil {
			t.Fatalf("GetLEI failed: %v", err)
		}

		if record.LEI != "HWUPKR0MPOU8FGXBT394" {
			t.Errorf("Expected LEI HWUPKR0MPOU8FGXBT394, got %s", record.LEI)
		}
		if record.Entity.LegalName.Name != "Apple Inc." {
			t.Errorf("Expected Apple Inc., got %s", record.Entity.LegalName.Name)
		}
	})

	t.Run("invalid LEI format rejected at Parse", func(t *testing.T) {
		_, err := ParseLEI("INVALID")
		if err == nil {
			t.Fatal("ParseLEI(\"INVALID\") returned nil; expected rejection")
		}
		var apiErr *APIError
		if !isAPIError(err, &apiErr) || apiErr.Code != ErrCodeInvalidFormat {
			t.Errorf("Expected invalid_format error, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(Config{
			BaseURL:     server.URL,
			Timeout:     5 * time.Second,
			RateLimit:   100,
			BurstSize:   10,
			MaxRetries:  0,
			EnableCache: false,
		}, logger)

		_, err := client.GetLEI(context.Background(), LEI("HWUPKR0MPOU8FGXBT394"))
		if err == nil {
			t.Error("Expected error for not found")
		}

		var apiErr *APIError
		if !isAPIError(err, &apiErr) || apiErr.Code != ErrCodeNotFound {
			t.Errorf("Expected not_found error, got %v", err)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		client := NewClient(Config{
			BaseURL:     server.URL,
			Timeout:     5 * time.Second,
			RateLimit:   100,
			BurstSize:   10,
			MaxRetries:  0,
			EnableCache: false,
		}, logger)

		_, err := client.GetLEI(context.Background(), LEI("HWUPKR0MPOU8FGXBT394"))
		if err == nil {
			t.Error("Expected error for rate limited")
		}

		var apiErr *APIError
		if !isAPIError(err, &apiErr) || apiErr.Code != ErrCodeRateLimited {
			t.Errorf("Expected rate_limited error, got %v", err)
		}
	})
}

// TestBatchLEIValidation tests batch LEI request validation.
func TestBatchLEIValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(DefaultConfig(), logger)

	t.Run("empty list", func(t *testing.T) {
		_, err := client.GetBatchLEI(context.Background(), []LEI{})
		if err == nil {
			t.Error("Expected error for empty LEI list")
		}
	})

	t.Run("too many LEIs", func(t *testing.T) {
		leis := make([]LEI, 101)
		for i := range leis {
			leis[i] = LEI("HWUPKR0MPOU8FGXBT394")
		}
		_, err := client.GetBatchLEI(context.Background(), leis)
		if err == nil {
			t.Error("Expected error for too many LEIs")
		}
	})

	t.Run("invalid LEI rejected at Parse", func(t *testing.T) {
		// With typed signatures, batch validation is the caller's concern.
		// ParseLEI is the format gate; tools/handlers.parseBatchLEIs
		// surfaces the first failure to the MCP caller.
		if _, err := ParseLEI("INVALID"); err == nil {
			t.Error("ParseLEI(\"INVALID\") returned nil; expected rejection")
		}
	})
}

// TestValidateLEI tests the full validation flow.
func TestValidateLEI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("invalid format returns result without API call", func(t *testing.T) {
		client := NewClient(Config{
			BaseURL:     "http://should-not-be-called",
			EnableCache: false,
		}, logger)

		result, err := client.ValidateLEI(context.Background(), "INVALID")
		if err != nil {
			t.Fatalf("ValidateLEI returned error: %v", err)
		}

		if result.Valid {
			t.Error("Expected Valid=false for invalid format")
		}
		if result.Message == "" {
			t.Error("Expected error message")
		}
	})

	t.Run("invalid check digits returns result without API call", func(t *testing.T) {
		client := NewClient(Config{
			BaseURL:     "http://should-not-be-called",
			EnableCache: false,
		}, logger)

		// Valid format but wrong check digits
		result, err := client.ValidateLEI(context.Background(), "HWUPKR0MPOU8FGXBT300")
		if err != nil {
			t.Fatalf("ValidateLEI returned error: %v", err)
		}

		if result.Valid {
			t.Error("Expected Valid=false for invalid check digits")
		}
		if result.Message == "" {
			t.Error("Expected error message")
		}
	})
}

// TestDefaultConfig tests default configuration values.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BaseURL != BaseURL {
		t.Errorf("Expected BaseURL %s, got %s", BaseURL, config.BaseURL)
	}
	if config.Timeout != DefaultTimeout {
		t.Errorf("Expected Timeout %v, got %v", DefaultTimeout, config.Timeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if !config.EnableCache {
		t.Error("Expected EnableCache=true")
	}
}

// Helper function to check if error is an APIError.
func isAPIError(err error, target **APIError) bool {
	if apiErr, ok := err.(*APIError); ok {
		*target = apiErr
		return true
	}
	return false
}
