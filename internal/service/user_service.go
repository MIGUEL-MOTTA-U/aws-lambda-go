package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
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

	// Create a copy of the existing user to update
	updated := existing

	// Only update fields that are provided (non-empty) in the request
	if strings.TrimSpace(user.Name) != "" {
		updated.Name = user.Name
	}
	if strings.TrimSpace(user.Email) != "" {
		updated.Email = user.Email
	}
	if strings.TrimSpace(user.Username) != "" {
		updated.Username = user.Username
	}
	if strings.TrimSpace(user.Birthdate) != "" {
		updated.Birthdate = user.Birthdate
	}
	if strings.TrimSpace(user.CreationDate) != "" {
		updated.CreationDate = user.CreationDate
	}
	if strings.TrimSpace(user.Phone) != "" {
		updated.Phone = user.Phone
	}
	if strings.TrimSpace(user.Role) != "" {
		updated.Role = user.Role
	}
	if strings.TrimSpace(user.Company) != "" {
		updated.Company = user.Company
	}
	if strings.TrimSpace(user.OfficeName) != "" {
		updated.OfficeName = user.OfficeName
	}
	if strings.TrimSpace(user.OfficeAddress) != "" {
		updated.OfficeAddress = user.OfficeAddress
	}
	if strings.TrimSpace(user.License) != "" {
		updated.License = user.License
	}
	if strings.TrimSpace(user.Bio) != "" {
		updated.Bio = user.Bio
	}
	if strings.TrimSpace(user.Headline) != "" {
		updated.Headline = user.Headline
	}
	if strings.TrimSpace(user.AvatarURL) != "" {
		updated.AvatarURL = user.AvatarURL
	}
	if strings.TrimSpace(user.AvatarAssetID) != "" {
		updated.AvatarAssetID = user.AvatarAssetID
	}
	if strings.TrimSpace(user.WhatsAppLink) != "" {
		updated.WhatsAppLink = user.WhatsAppLink
	}
	if strings.TrimSpace(user.InstagramURL) != "" {
		updated.InstagramURL = user.InstagramURL
	}
	if strings.TrimSpace(user.LinkedInURL) != "" {
		updated.LinkedInURL = user.LinkedInURL
	}
	if strings.TrimSpace(user.FacebookURL) != "" {
		updated.FacebookURL = user.FacebookURL
	}

	// Handle metadata - if provided, merge with existing
	if userMetadataProvided(user.Metadata) {
		// If we want to merge, we'd need to handle each field
		// For simplicity, we'll replace if provided
		updated.Metadata = user.Metadata
	}

	// Ensure ID is set
	updated.ID = id

	// Ensure CreationDate is set (should already be from existing)
	if strings.TrimSpace(updated.CreationDate) == "" {
		updated.CreationDate = s.clock().Format(time.RFC3339)
	}

	// Validate the updated user
	if err := validateUser(updated); err != nil {
		return model.User{}, err
	}

	if err := s.repository.Update(ctx, updated); err != nil {
		return model.User{}, err
	}

	return updated, nil
}

func (s UserService) DeleteUser(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return validationError("id is required")
	}

	return s.repository.Delete(ctx, id)
}

func validateUser(user model.User) error {
	// Trim spaces for string fields
	id := strings.TrimSpace(user.ID)
	if id == "" {
		return validationError("id is required")
	}

	// Email validation
	email := strings.TrimSpace(user.Email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return validationError("email is invalid")
		}
	}

	// Birthdate validation (expected format: YYYY-MM-DD)
	birthdate := strings.TrimSpace(user.Birthdate)
	if birthdate != "" {
		if len(birthdate) != 10 || birthdate[4] != '-' || birthdate[7] != '-' {
			return validationError("birthdate must be in YYYY-MM-DD format")
		}
		if _, err := time.Parse("2006-01-02", birthdate); err != nil {
			return validationError("birthdate is invalid")
		}
	}

	// Phone validation (optional, but if set should contain only digits, spaces, +, -, (, ))
	phone := strings.TrimSpace(user.Phone)
	if phone != "" {
		// Allow digits, spaces, plus, hyphen, parentheses
		matched, _ := regexp.MatchString(`^[\d\s\+\-\(\)]+$`, phone)
		if !matched {
			return validationError("phone number contains invalid characters")
		}
		// Optionally, check length? We'll skip for simplicity.
	}

	// URL fields validation
	urlFields := []struct {
		value string
		field string
	}{
		{strings.TrimSpace(user.WhatsAppLink), "whatsapp_link"},
		{strings.TrimSpace(user.InstagramURL), "instagram_url"},
		{strings.TrimSpace(user.LinkedInURL), "linkedin_url"},
		{strings.TrimSpace(user.FacebookURL), "facebook_url"},
		{strings.TrimSpace(user.AvatarURL), "avatar_url"},
	}

	for _, f := range urlFields {
		if f.value != "" {
			if err := validateURL(f.value); err != nil {
				return validationError(fmt.Sprintf("%s is invalid: %v", f.field, err))
			}
		}
	}

	// Other string fields: we don't validate beyond trimming, but we can check for empty if they are required?
	// According to the frontend, many are optional, so we skip.

	return nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidUser, message)
}

func validateURL(urlStr string) error {
	if u, err := url.Parse(urlStr); err != nil {
		return err
	} else if !(u.Scheme == "http" || u.Scheme == "https") || u.Host == "" {
		return fmt.Errorf("URL must have http or https scheme and a host")
	}
	return nil
}

func userMetadataProvided(metadata model.UserMetadata) bool {
	return metadata.Stats != nil ||
		metadata.Badges != nil ||
		metadata.Services != nil ||
		strings.TrimSpace(metadata.HeroImageURL) != "" ||
		strings.TrimSpace(metadata.HeroVideoURL) != ""
}
func NewID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}

	return hex.EncodeToString(bytes)
}
