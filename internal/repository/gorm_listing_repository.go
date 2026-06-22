package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"rs-lambda-go/internal/model"
)

type GormListingRepository struct {
	db *gorm.DB
}

func NewGormListingRepository(db *gorm.DB) *GormListingRepository {
	return &GormListingRepository{
		db: db,
	}
}

func (r *GormListingRepository) FindAll(ctx context.Context) ([]model.Listing, error) {
	var listings []model.Listing
	err := r.db.WithContext(ctx).Find(&listings).Error
	if err != nil {
		return nil, err
	}
	return listings, nil
}

func (r *GormListingRepository) FindByID(ctx context.Context, id string) (model.Listing, error) {
	var listing model.Listing
	err := r.db.WithContext(ctx).First(&listing, "listing_id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Listing{}, ErrListingNotFound
		}
		return model.Listing{}, err
	}
	return listing, nil
}

func (r *GormListingRepository) Create(ctx context.Context, listing model.Listing) error {
	var existing model.Listing
	err := r.db.WithContext(ctx).First(&existing, "listing_id = ?", listing.ListingID).Error
	if err == nil {
		return ErrListingAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	err = r.db.WithContext(ctx).Create(&listing).Error
	return err
}

func (r *GormListingRepository) Update(ctx context.Context, listing model.Listing) error {
	var existing model.Listing
	err := r.db.WithContext(ctx).First(&existing, "listing_id = ?", listing.ListingID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListingNotFound
		}
		return err
	}

	err = r.db.WithContext(ctx).Save(&listing).Error
	return err
}

func (r *GormListingRepository) Delete(ctx context.Context, id string) error {
	var existing model.Listing
	err := r.db.WithContext(ctx).First(&existing, "listing_id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListingNotFound
		}
		return err
	}

	err = r.db.WithContext(ctx).Delete(&model.Listing{}, "listing_id = ?", id).Error
	return err
}
