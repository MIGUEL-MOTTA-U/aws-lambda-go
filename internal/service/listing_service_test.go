package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

type fakeListingRepo struct {
	listings map[string]model.Listing
}

func newFakeListingRepo() *fakeListingRepo {
	return &fakeListingRepo{listings: make(map[string]model.Listing)}
}

func (r *fakeListingRepo) FindAll(ctx context.Context, limit, offset int) ([]model.Listing, error) {
	var all []model.Listing
	for _, l := range r.listings {
		all = append(all, l)
	}
	if offset >= len(all) {
		if offset > 0 {
			return []model.Listing{}, nil
		}
	} else if offset > 0 {
		all = all[offset:]
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (r *fakeListingRepo) FindByID(ctx context.Context, id string) (model.Listing, error) {
	listing, ok := r.listings[id]
	if !ok {
		return model.Listing{}, repository.ErrListingNotFound
	}
	return listing, nil
}

func (r *fakeListingRepo) Create(ctx context.Context, listing model.Listing) error {
	id := string(listing.ListingID)
	if _, ok := r.listings[id]; ok {
		return repository.ErrListingAlreadyExists
	}
	r.listings[id] = listing
	return nil
}

func (r *fakeListingRepo) Update(ctx context.Context, listing model.Listing) error {
	id := string(listing.ListingID)
	if _, ok := r.listings[id]; !ok {
		return repository.ErrListingNotFound
	}
	r.listings[id] = listing
	return nil
}

func (r *fakeListingRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.listings[id]; !ok {
		return repository.ErrListingNotFound
	}
	delete(r.listings, id)
	return nil
}

func newTestListingService() (*ListingService, *fakeListingRepo) {
	repo := newFakeListingRepo()
	nextID := 0
	idGen := func() string {
		nextID++
		return fmt.Sprintf("listing-%03d", nextID)
	}
	fixedClock := func() time.Time {
		return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	}
	return NewListingServiceWithDependencies(repo, idGen, fixedClock), repo
}

func TestListingService_Create(t *testing.T) {
	svc, repo := newTestListingService()
	ctx := context.Background()

	valid := model.Listing{
		Title:         "Casa Campestre",
		PropertyType:  "house",
		OperationType: "sale",
		Pricing:       model.Pricing{SalePrice: 500000000, Currency: "COP"},
		DescriptionShort: "Hermosa casa campestre",
	}

	created, err := svc.CreateListing(ctx, valid)
	if err != nil {
		t.Fatalf("unexpected error creating listing: %v", err)
	}

	if created.ListingID == "" {
		t.Error("expected generated ListingID")
	}
	if created.DescriptionLong != "Hermosa casa campestre" {
		t.Errorf("expected DescriptionLong default to DescriptionShort, got %q", created.DescriptionLong)
	}
	if created.Metadata.SourceSystem != "century21colombia" {
		t.Errorf("expected default SourceSystem century21colombia, got %q", created.Metadata.SourceSystem)
	}

	// Verify repo stored item
	if _, ok := repo.listings[string(created.ListingID)]; !ok {
		t.Error("expected listing to exist in repository")
	}
}

func TestListingService_Create_ValidationErrors(t *testing.T) {
	svc, _ := newTestListingService()
	ctx := context.Background()

	cases := []struct {
		name    string
		listing model.Listing
	}{
		{
			name: "missing price",
			listing: model.Listing{
				Title:        "Apto",
				PropertyType: "apartment",
			},
		},
		{
			name: "invalid property type",
			listing: model.Listing{
				Title:        "Apto",
				PropertyType: "palace",
				Pricing:      model.Pricing{SalePrice: 1000},
			},
		},
		{
			name: "invalid currency",
			listing: model.Listing{
				Title:        "Apto",
				PropertyType: "apartment",
				Pricing:      model.Pricing{SalePrice: 1000, Currency: "EUR"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateListing(ctx, tc.listing)
			if err == nil || !errors.Is(err, ErrInvalidListing) {
				t.Errorf("expected ErrInvalidListing, got %v", err)
			}
		})
	}
}

func TestListingService_GetUpdateDelete(t *testing.T) {
	svc, repo := newTestListingService()
	ctx := context.Background()

	// Seed item
	repo.listings["listing-001"] = model.Listing{
		ListingID:    "listing-001",
		Title:        "Original Title",
		PropertyType: "apartment",
		Pricing:      model.Pricing{SalePrice: 100000},
	}

	// Get
	got, err := svc.GetListing(ctx, "listing-001")
	if err != nil || got.Title != "Original Title" {
		t.Fatalf("failed to get listing: %v", err)
	}

	// Update
	updated, err := svc.UpdateListing(ctx, "listing-001", model.Listing{
		Title:        "Updated Title",
		PropertyType: "apartment",
		Pricing:      model.Pricing{SalePrice: 200000},
	})
	if err != nil || updated.Title != "Updated Title" {
		t.Fatalf("failed to update listing: %v", err)
	}

	// Delete
	if err := svc.DeleteListing(ctx, "listing-001"); err != nil {
		t.Fatalf("failed to delete listing: %v", err)
	}

	// Get after delete
	_, err = svc.GetListing(ctx, "listing-001")
	if err == nil || !errors.Is(err, repository.ErrListingNotFound) {
		t.Errorf("expected ErrListingNotFound after delete, got %v", err)
	}
}
