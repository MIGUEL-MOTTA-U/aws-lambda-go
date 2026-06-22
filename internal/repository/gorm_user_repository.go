package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"rs-lambda-go/internal/model"
)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{
		db: db,
	}
}

func (r *GormUserRepository) FindAll(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) FindByID(ctx context.Context, id string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (r *GormUserRepository) Create(ctx context.Context, user model.User) error {
	var existing model.User
	err := r.db.WithContext(ctx).First(&existing, "id = ?", user.ID).Error
	if err == nil {
		return ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	err = r.db.WithContext(ctx).Create(&user).Error
	return err
}

func (r *GormUserRepository) Update(ctx context.Context, user model.User) error {
	var existing model.User
	err := r.db.WithContext(ctx).First(&existing, "id = ?", user.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	err = r.db.WithContext(ctx).Save(&user).Error
	return err
}

func (r *GormUserRepository) Delete(ctx context.Context, id string) error {
	var existing model.User
	err := r.db.WithContext(ctx).First(&existing, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	err = r.db.WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
	return err
}
