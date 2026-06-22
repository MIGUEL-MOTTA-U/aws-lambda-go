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
	if strings.TrimSpace(string(listing.ListingID)) == "" {
		return validationListingError("listing_id is required")
	}
	if strings.TrimSpace(listing.Slug) == "" {
		return validationListingError("slug is required")
	}
	if strings.TrimSpace(listing.URL) == "" {
		return validationListingError("url is required")
	}
	lang := strings.ToLower(strings.TrimSpace(listing.Language))
	if lang != "es" && lang != "en" {
		return validationListingError("language must be 'es' or 'en'")
	}
	if strings.TrimSpace(listing.Title) == "" {
		return validationListingError("title is required")
	}
	if strings.TrimSpace(listing.PropertyType) == "" {
		return validationListingError("property_type is required")
	}
	if strings.TrimSpace(listing.OperationType) == "" {
		return validationListingError("operation_type is required")
	}
	if strings.TrimSpace(listing.PublicationStatus) == "" {
		return validationListingError("publication_status is required")
	}
	return nil
}

func validationListingError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidListing, message)
}
