package repository

import (
	"context"
	"errors"

	"rs-lambda-go/internal/model"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
)

type UserRepository interface {
	FindAll(ctx context.Context) ([]model.User, error)
	FindByID(ctx context.Context, id string) (model.User, error)
	Create(ctx context.Context, user model.User) error
	Update(ctx context.Context, user model.User) error
	Delete(ctx context.Context, id string) error
}
