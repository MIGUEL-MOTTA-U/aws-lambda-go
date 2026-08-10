package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"rs-lambda-go/internal/model"
)

// GormVisitRepository implements VisitRepository using GORM.
type GormVisitRepository struct {
	db *gorm.DB
}

func NewGormVisitRepository(db *gorm.DB) *GormVisitRepository {
	return &GormVisitRepository{db: db}
}

func (r *GormVisitRepository) Create(ctx context.Context, visit model.Visit) error {
	return r.db.WithContext(ctx).Create(&visit).Error
}

func (r *GormVisitRepository) ListByVisitorSince(ctx context.Context, visitorID string, since time.Time) ([]model.Visit, error) {
	var visits []model.Visit
	err := r.db.WithContext(ctx).
		Where("visitor_id = ? AND created_at >= ?", visitorID, since).
		Order("created_at DESC").
		Find(&visits).Error
	if err != nil {
		return nil, err
	}
	return visits, nil
}

func (r *GormVisitRepository) ListByListingSince(ctx context.Context, listingID string, since time.Time) ([]model.Visit, error) {
	var visits []model.Visit
	err := r.db.WithContext(ctx).
		Where("listing_id = ? AND created_at >= ?", listingID, since).
		Order("created_at ASC").
		Find(&visits).Error
	if err != nil {
		return nil, err
	}
	return visits, nil
}

func (r *GormVisitRepository) ListAllSince(ctx context.Context, since time.Time) ([]model.Visit, error) {
	var visits []model.Visit
	err := r.db.WithContext(ctx).
		Where("created_at >= ?", since).
		Order("created_at ASC").
		Find(&visits).Error
	if err != nil {
		return nil, err
	}
	return visits, nil
}