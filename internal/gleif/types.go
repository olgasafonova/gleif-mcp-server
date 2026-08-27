// Package gleif provides a client for the GLEIF LEI API.
package gleif

import "time"

// LEIRecord represents a complete LEI record from the GLEIF API.
type LEIRecord struct {
	LEI          string       `json:"lei"`
	Entity       Entity       `json:"entity"`
	Registration Registration `json:"registration"`
}

// Entity contains legal entity information.
type Entity struct {
	LegalName              LegalName         `json:"legalName"`
	OtherNames             []OtherName       `json:"otherNames,omitempty"`
	LegalAddress           Address           `json:"legalAddress"`
	HeadquartersAddress    Address           `json:"headquartersAddress"`
	RegisteredAt           RegisteredAt      `json:"registeredAt,omitempty"`
	RegisteredAs           string            `json:"registeredAs,omitempty"`
	Jurisdiction           string            `json:"jurisdiction"`
	Category               string            `json:"category,omitempty"`
	LegalForm              LegalForm         `json:"legalForm,omitempty"`
	AssociatedEntity       *AssociatedEntity `json:"associatedEntity,omitempty"`
	Status                 string            `json:"status"`
	EntityExpirationDate   *string           `json:"entityExpirationDate,omitempty"`
	EntityExpirationReason *string           `json:"entityExpirationReason,omitempty"`
	SuccessorEntity        *SuccessorEntity  `json:"successorEntity,omitempty"`
}

// LegalName represents the official legal name.
type LegalName struct {
	Name     string `json:"name"`
	Language string `json:"language,omitempty"`
}

// OtherName represents alternative names (trading names, etc.).
type OtherName struct {
	Name     string `json:"name"`
	Language string `json:"language,omitempty"`
	Type     string `json:"type,omitempty"`
}

// Address represents a physical address.
type Address struct {
	Language                    string   `json:"language,omitempty"`
	AddressLines                []string `json:"addressLines,omitempty"`
	AddressNumber               string   `json:"addressNumber,omitempty"`
	AddressNumberWithinBuilding string   `json:"addressNumberWithinBuilding,omitempty"`
	MailRouting                 string   `json:"mailRouting,omitempty"`
	City                        string   `json:"city"`
	Region                      string   `json:"region,omitempty"`
	Country                     string   `json:"country"`
	PostalCode                  string   `json:"postalCode,omitempty"`
}

// RegisteredAt contains registration authority information.
type RegisteredAt struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"other,omitempty"`
}

// LegalForm represents the entity's legal form.
type LegalForm struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"other,omitempty"`
}

// AssociatedEntity for fund managers, etc.
type AssociatedEntity struct {
	LEI  string `json:"lei,omitempty"`
	Name string `json:"name,omitempty"`
}

// SuccessorEntity for merged/acquired companies.
type SuccessorEntity struct {
	LEI  string `json:"lei,omitempty"`
	Name string `json:"name,omitempty"`
}

// Registration contains LEI registration metadata.
type Registration struct {
	InitialRegistrationDate time.Time            `json:"initialRegistrationDate"`
	LastUpdateDate          time.Time            `json:"lastUpdateDate"`
	Status                  string               `json:"status"`
	NextRenewalDate         time.Time            `json:"nextRenewalDate"`
	ManagingLOU             string               `json:"managingLou"`
	ValidationSources       string               `json:"validationSources,omitempty"`
	ValidationAuthority     *ValidationAuthority `json:"validationAuthority,omitempty"`
}

// ValidationAuthority contains validation authority details.
type ValidationAuthority struct {
	ValidationAuthorityID       string `json:"validationAuthorityID,omitempty"`
	OtherValidationAuthorityID  string `json:"otherValidationAuthorityID,omitempty"`
	ValidationAuthorityEntityID string `json:"validationAuthorityEntityID,omitempty"`
}

// Relationship represents a parent-child or other relationship between entities.
type Relationship struct {
	StartNode             RelationshipNode     `json:"startNode"`
	EndNode               RelationshipNode     `json:"endNode"`
	RelationshipType      string               `json:"relationshipType"`
	RelationshipPeriods   []RelationshipPeriod `json:"relationshipPeriods,omitempty"`
	RelationshipStatus    string               `json:"relationshipStatus,omitempty"`
	RelationshipQualifier string               `json:"relationshipQualifier,omitempty"`
}

// RelationshipNode represents one side of a relationship.
type RelationshipNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// RelationshipPeriod defines the time period for a relationship.
type RelationshipPeriod struct {
	StartDate  *string `json:"startDate,omitempty"`
	EndDate    *string `json:"endDate,omitempty"`
	PeriodType string  `json:"periodType,omitempty"`
}

// BICMapping represents a BIC to LEI mapping.
type BICMapping struct {
	BIC string `json:"bic"`
	LEI string `json:"lei"`
}

// ISINMapping represents an ISIN to LEI mapping.
type ISINMapping struct {
	ISIN string `json:"isin"`
	LEI  string `json:"lei"`
}

// APIResponse is the generic wrapper for GLEIF API responses.
type APIResponse[T any] struct {
	Meta  Meta          `json:"meta,omitempty"`
	Data  []DataItem[T] `json:"data"`
	Links Links         `json:"links,omitempty"`
}

// SingleResponse is for single-item responses.
type SingleResponse[T any] struct {
	Meta Meta        `json:"meta,omitempty"`
	Data DataItem[T] `json:"data"`
}

// DataItem wraps individual records.
type DataItem[T any] struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	Attributes    T                          `json:"attributes"`
	Relationships map[string]RelationshipRef `json:"relationships,omitempty"`
	Links         Links                      `json:"links,omitempty"`
}

// RelationshipRef points to related resources.
type RelationshipRef struct {
	Links Links `json:"links,omitempty"`
}

// Meta contains pagination info.
type Meta struct {
	GoldenCopy GoldenCopy `json:"goldenCopy,omitempty"`
	Pagination Pagination `json:"pagination,omitempty"`
}

// GoldenCopy contains data quality info.
type GoldenCopy struct {
	PublishDate string `json:"publishDate,omitempty"`
}

// Pagination contains paging info.
type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PerPage     int `json:"perPage"`
	From        int `json:"from"`
	To          int `json:"to"`
	Total       int `json:"total"`
	LastPage    int `json:"lastPage"`
}

// Links contains HATEOAS links.
type Links struct {
	Self    string `json:"self,omitempty"`
	First   string `json:"first,omitempty"`
	Last    string `json:"last,omitempty"`
	Next    string `json:"next,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Related string `json:"related,omitempty"`
}

// AutocompleteResult represents a single autocomplete suggestion, projected
// for tool output. Country is filled only by the fuzzy fallback: the real
// /autocompletions payload does not carry one.
type AutocompleteResult struct {
	LEI       string `json:"lei"`
	LegalName string `json:"value"`
	Country   string `json:"country,omitempty"`
}

// autocompleteAPIResponse is the WIRE shape of /autocompletions, captured from
// the live endpoint 27-08-2026 (bead claude-code-config-jekc). The suggestion
// text sits under attributes.value and the LEI under
// relationships.lei-records.data.id. The previous flat decode was imagined,
// not captured, and was never exercised: without the required `field`
// parameter every call 400'd into the fuzzy fallback, so decoding ten blank
// suggestions at 200 was one parameter away.
type autocompleteAPIResponse struct {
	Data []struct {
		Attributes struct {
			Value        string `json:"value"`
			Highlighting string `json:"highlighting"`
		} `json:"attributes"`
		Relationships struct {
			LEIRecords struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			} `json:"lei-records"`
		} `json:"relationships"`
	} `json:"data"`
}

// ValidationResult contains LEI validation info.
type ValidationResult struct {
	LEI          string `json:"lei"`
	Valid        bool   `json:"valid"`
	Status       string `json:"status,omitempty"`
	EntityStatus string `json:"entityStatus,omitempty"`
	NextRenewal  string `json:"nextRenewal,omitempty"`
	Message      string `json:"message,omitempty"`
}

// LEIIssuer represents an LEI issuer (Local Operating Unit / LOU).
type LEIIssuer struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Country        string `json:"country"`
	Jurisdiction   string `json:"jurisdiction,omitempty"`
	Website        string `json:"website,omitempty"`
	Status         string `json:"status,omitempty"`
	LEI            string `json:"lei,omitempty"`
	ManagingLOU    string `json:"managingLou,omitempty"`
	SponsoredLEIs  int    `json:"sponsoredLeis,omitempty"`
	AccreditedDate string `json:"accreditedDate,omitempty"`
}

// ReportingException represents a Level 2 reporting exception.
type ReportingException struct {
	LEI                string `json:"lei"`
	ExceptionCategory  string `json:"exceptionCategory"`
	ExceptionReason    string `json:"exceptionReason"`
	ExceptionReference string `json:"exceptionReference,omitempty"`
}

// SearchResult wraps search results with pagination info.
type SearchResult struct {
	Records    []LEIRecord `json:"records"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Total      int         `json:"total"`
	HasMore    bool        `json:"hasMore"`
}
