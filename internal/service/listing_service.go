package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

var ErrInvalidListing = errors.New("invalid listing")

type ListingService struct {
	repository  repository.ListingRepository
	idGenerator IDGenerator
	clock       Clock
}

func NewListingService(repository repository.ListingRepository) *ListingService {
	return NewListingServiceWithDependencies(repository, NewID, func() time.Time {
		return time.Now().UTC()
	})
}

func NewListingServiceWithDependencies(repository repository.ListingRepository, idGenerator IDGenerator, clock Clock) *ListingService {
	return &ListingService{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

func (s ListingService) ListListings(ctx context.Context) ([]model.Listing, error) {
	return s.repository.FindAll(ctx)
}

func (s ListingService) GetListing(ctx context.Context, id string) (model.Listing, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Listing{}, validationListingError("listing_id is required")
	}

	return s.repository.FindByID(ctx, id)
}

func (s ListingService) CreateListing(ctx context.Context, listing model.Listing) (model.Listing, error) {
	if strings.TrimSpace(string(listing.ListingID)) == "" {
		listing.ListingID = model.ListingID(s.idGenerator())
	}
	listing.Metadata.UpdatedAt = s.clock().Format(time.RFC3339)
	if strings.TrimSpace(listing.Metadata.SourceSystem) == "" {
		listing.Metadata.SourceSystem = "century21colombia"
	}

	if err := validateListing(listing); err != nil {
		return model.Listing{}, err
	}

	if err := s.repository.Create(ctx, listing); err != nil {
		return model.Listing{}, err
	}

	return listing, nil
}

func (s ListingService) UpdateListing(ctx context.Context, id string, listing model.Listing) (model.Listing, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Listing{}, validationListingError("listing_id is required")
	}

	_, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return model.Listing{}, err
	}

	listing.ListingID = model.ListingID(id)
	listing.Metadata.UpdatedAt = s.clock().Format(time.RFC3339)
	if strings.TrimSpace(listing.Metadata.SourceSystem) == "" {
		listing.Metadata.SourceSystem = "century21colombia"
	}

	if err := validateListing(listing); err != nil {
		return model.Listing{}, err
	}

	if err := s.repository.Update(ctx, listing); err != nil {
		return model.Listing{}, err
	}

	return listing, nil
}

func (s ListingService) DeleteListing(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return validationListingError("listing_id is required")
	}

	return s.repository.Delete(ctx, id)
}

func validateListing(listing model.Listing) error {
	// For listing updates, we only require the ListingID to be present
	// Other field validations are handled at the database level or are optional for partial updates
	if strings.TrimSpace(string(listing.ListingID)) == "" {
		return validationListingError("listing_id is required")
	}

	// Validate individual fields only if they are being set (not empty/default)
	// This allows for partial updates where only some fields are provided
	if listing.Slug != "" && strings.TrimSpace(listing.Slug) == "" {
		return validationListingError("slug is required")
	}
	if listing.URL != "" && strings.TrimSpace(listing.URL) == "" {
		return validationListingError("url is required")
	}
	if listing.Language != "" {
		lang := strings.ToLower(strings.TrimSpace(listing.Language))
		if lang != "es" && lang != "en" {
			return validationListingError("language must be 'es' or 'en'")
		}
	}
	if listing.Title != "" && strings.TrimSpace(listing.Title) == "" {
		return validationListingError("title is required")
	}
	if listing.PropertyType != "" && strings.TrimSpace(listing.PropertyType) == "" {
		return validationListingError("property_type is required")
	}
	if listing.OperationType != "" && strings.TrimSpace(listing.OperationType) == "" {
		return validationListingError("operation_type is required")
	}
	if listing.PublicationStatus != "" && strings.TrimSpace(listing.PublicationStatus) == "" {
		return validationListingError("publication_status is required")
	}

	// Validate location fields
	if listing.Location.Country != "" && strings.TrimSpace(listing.Location.Country) == "" {
		return validationListingError("country is required")
	}
	if listing.Location.State != "" && strings.TrimSpace(listing.Location.State) == "" {
		return validationListingError("state is required")
	}
	if listing.Location.City != "" && strings.TrimSpace(listing.Location.City) == "" {
		return validationListingError("city is required")
	}
	// Neighborhood is optional, but if provided should not be just whitespace
	if listing.Location.Neighborhood != "" && strings.TrimSpace(listing.Location.Neighborhood) == "" {
		return validationListingError("neighborhood must not be empty")
	}
	if listing.Location.Address != "" && strings.TrimSpace(listing.Location.Address) == "" {
		return validationListingError("address is required")
	}

	// Validate pricing fields
	// At least one of sale_price or rent_price should be provided if they are being set
	// We check if either field is non-zero to determine if it's being set
	if listing.Pricing.SalePrice != 0 || listing.Pricing.RentPrice != 0 {
		if listing.Pricing.SalePrice <= 0 && listing.Pricing.RentPrice <= 0 {
			return validationListingError("at least one of sale_price or rent_price must be greater than zero")
		}
	}

	// Validate currency if provided
	if listing.Pricing.Currency != "" {
		currency := strings.ToUpper(strings.TrimSpace(listing.Pricing.Currency))
		if currency != "COP" && currency != "USD" && currency != "EUR" {
			return validationListingError("currency must be one of COP, USD, or EUR")
		}
	}

	// Validate stratum if provided (between 1 and 6)
	if listing.Location.Stratum != 0 {
		if listing.Location.Stratum < 1 || listing.Location.Stratum > 6 {
			return validationListingError("stratum must be between 1 and 6")
		}
	}

	// Validate coordinates if provided (non-zero)
	if listing.Location.Coordinates.Lat != 0 || listing.Location.Coordinates.Lng != 0 {
		if listing.Location.Coordinates.Lat < -90 || listing.Location.Coordinates.Lat > 90 {
			return validationListingError("latitude must be between -90 and 90")
		}
		if listing.Location.Coordinates.Lng < -180 || listing.Location.Coordinates.Lng > 180 {
			return validationListingError("longitude must be between -180 and 180")
		}
	}

	return nil
}

func validationListingError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidListing, message)
}
