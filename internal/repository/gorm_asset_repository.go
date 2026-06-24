package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"rs-lambda-go/internal/model"
)

// GormAssetRepository implements AssetRepository using GORM.
type GormAssetRepository struct {
	db *gorm.DB
}

func NewGormAssetRepository(db *gorm.DB) *GormAssetRepository {
	return &GormAssetRepository{db: db}
}

func (r *GormAssetRepository) Create(ctx context.Context, asset model.Asset) error {
	return r.db.WithContext(ctx).Create(&asset).Error
}

func (r *GormAssetRepository) FindByID(ctx context.Context, id string) (model.Asset, error) {
	var asset model.Asset
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Asset{}, ErrAssetNotFound
		}
		return model.Asset{}, err
	}
	return asset, nil
}

// FindByEntity returns all confirmed assets for a given entity, ordered by creation date.
func (r *GormAssetRepository) FindByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error) {
	var assets []model.Asset
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND status = ? AND deleted_at IS NULL",
			entityType, entityID, model.AssetStatusConfirmed).
		Order("created_at ASC").
		Find(&assets).Error
	if err != nil {
		return nil, err
	}
	return assets, nil
}

// CountByEntity returns the number of confirmed (non-deleted) assets for a given entity.
// Used to enforce per-entity upload limits before starting a new upload.
func (r *GormAssetRepository) CountByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Asset{}).
		Where("entity_type = ? AND entity_id = ? AND status = ? AND deleted_at IS NULL",
			entityType, entityID, model.AssetStatusConfirmed).
		Count(&count).Error
	return count, err
}

// SoftDelete marks an asset as deleted without removing the database record.
// The caller is responsible for deleting the object from R2 separately.
func (r *GormAssetRepository) SoftDelete(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.Asset{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":     model.AssetStatusDeleted,
			"deleted_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}
