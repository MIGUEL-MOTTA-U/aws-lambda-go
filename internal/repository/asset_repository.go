package repository

import (
	"context"
	"errors"

	"rs-lambda-go/internal/model"
)

var (
	ErrAssetNotFound = errors.New("asset not found")
)

// AssetRepository defines persistence operations for assets.
type AssetRepository interface {
	Create(ctx context.Context, asset model.Asset) error
	FindByID(ctx context.Context, id string) (model.Asset, error)
	FindByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error)
	CountByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) (int64, error)
	SoftDelete(ctx context.Context, id string) error
}
