package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

var ErrInvalidUser = errors.New("invalid user")

type IDGenerator func() string

type Clock func() time.Time

type UserService struct {
	repository  repository.UserRepository
	idGenerator IDGenerator
	clock       Clock
}

func NewUserService(repository repository.UserRepository) *UserService {
	return NewUserServiceWithDependencies(repository, NewID, func() time.Time {
		return time.Now().UTC()
	})
}

func NewUserServiceWithDependencies(repository repository.UserRepository, idGenerator IDGenerator, clock Clock) *UserService {
	return &UserService{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

func (s UserService) ListUsers(ctx context.Context) ([]model.User, error) {
	return s.repository.FindAll(ctx)
}

func (s UserService) GetUser(ctx context.Context, id string) (model.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.User{}, validationError("id is required")
	}

	return s.repository.FindByID(ctx, id)
}

func (s UserService) CreateUser(ctx context.Context, user model.User) (model.User, error) {
	if strings.TrimSpace(user.ID) == "" {
		user.ID = s.idGenerator()
	}
	user.CreationDate = s.clock().Format(time.RFC3339)

	if err := validateUser(user); err != nil {
		return model.User{}, err
	}

	if err := s.repository.Create(ctx, user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

func (s UserService) UpdateUser(ctx context.Context, id string, user model.User) (model.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.User{}, validationError("id is required")
	}

	existing, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}

	user.ID = id
	user.CreationDate = existing.CreationDate
	if strings.TrimSpace(user.CreationDate) == "" {
		user.CreationDate = s.clock().Format(time.RFC3339)
	}

	if err := validateUser(user); err != nil {
		return model.User{}, err
	}

	if err := s.repository.Update(ctx, user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

func (s UserService) DeleteUser(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return validationError("id is required")
	}

	return s.repository.Delete(ctx, id)
}

func validateUser(user model.User) error {
	switch {
	case strings.TrimSpace(user.ID) == "":
		return validationError("id is required")
	case strings.TrimSpace(user.Name) == "":
		return validationError("name is required")
	case strings.TrimSpace(user.Email) == "":
		return validationError("email is required")
	case !strings.Contains(user.Email, "@"):
		return validationError("email is invalid")
	case strings.TrimSpace(user.Username) == "":
		return validationError("username is required")
	case strings.TrimSpace(user.Birthdate) == "":
		return validationError("birthdate is required")
	case strings.TrimSpace(user.CreationDate) == "":
		return validationError("creationdate is required")
	default:
		return nil
	}
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidUser, message)
}

func NewID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}

	return hex.EncodeToString(bytes)
}
