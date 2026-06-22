package repository

import (
	"context"
	"errors"

	"rs-lambda-go/internal/model"
)

var (
	ErrListingAlreadyExists = errors.New("listing already exists")
	ErrListingNotFound      = errors.New("listing not found")
)

type ListingRepository interface {
	FindAll(ctx context.Context) ([]model.Listing, error)
	FindByID(ctx context.Context, id string) (model.Listing, error)
	Create(ctx context.Context, listing model.Listing) error
	Update(ctx context.Context, listing model.Listing) error
	Delete(ctx context.Context, id string) error
}
