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

type fakeUserRepo struct {
	users map[string]model.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]model.User)}
}

func (r *fakeUserRepo) FindAll(ctx context.Context) ([]model.User, error) {
	var all []model.User
	for _, u := range r.users {
		all = append(all, u)
	}
	return all, nil
}

func (r *fakeUserRepo) FindByID(ctx context.Context, id string) (model.User, error) {
	user, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *fakeUserRepo) Create(ctx context.Context, user model.User) error {
	if _, ok := r.users[user.ID]; ok {
		return repository.ErrUserAlreadyExists
	}
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepo) Update(ctx context.Context, user model.User) error {
	if _, ok := r.users[user.ID]; !ok {
		return repository.ErrUserNotFound
	}
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return repository.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func newTestUserService() (*UserService, *fakeUserRepo) {
	repo := newFakeUserRepo()
	nextID := 0
	idGen := func() string {
		nextID++
		return fmt.Sprintf("user-%03d", nextID)
	}
	fixedClock := func() time.Time {
		return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	}
	return NewUserServiceWithDependencies(repo, idGen, fixedClock), repo
}

func TestUserService_Create(t *testing.T) {
	svc, repo := newTestUserService()
	ctx := context.Background()

	valid := model.User{
		Name:      "Aura Urrea",
		Email:     "aura.urrea@example.com",
		Phone:     "+57 300 123 4567",
		Birthdate: "1990-05-15",
	}

	created, err := svc.CreateUser(ctx, valid)
	if err != nil {
		t.Fatalf("unexpected error creating user: %v", err)
	}

	if created.ID == "" {
		t.Error("expected generated ID")
	}
	if created.CreationDate == "" {
		t.Error("expected CreationDate to be set")
	}

	if _, ok := repo.users[created.ID]; !ok {
		t.Error("expected user to exist in repository")
	}
}

func TestUserService_ValidationErrors(t *testing.T) {
	svc, _ := newTestUserService()
	ctx := context.Background()

	cases := []struct {
		name string
		user model.User
	}{
		{
			name: "invalid email",
			user: model.User{Name: "User", Email: "invalid-email"},
		},
		{
			name: "invalid birthdate format",
			user: model.User{Name: "User", Birthdate: "15-05-1990"},
		},
		{
			name: "invalid phone characters",
			user: model.User{Name: "User", Phone: "abc-def"},
		},
		{
			name: "invalid instagram url",
			user: model.User{Name: "User", InstagramURL: "not-a-url"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateUser(ctx, tc.user)
			if err == nil || !errors.Is(err, ErrInvalidUser) {
				t.Errorf("expected ErrInvalidUser, got %v", err)
			}
		})
	}
}

func TestUserService_GetUpdateDelete(t *testing.T) {
	svc, repo := newTestUserService()
	ctx := context.Background()

	repo.users["user-001"] = model.User{
		ID:    "user-001",
		Name:  "Original Name",
		Email: "orig@example.com",
	}

	// Get
	got, err := svc.GetUser(ctx, "user-001")
	if err != nil || got.Name != "Original Name" {
		t.Fatalf("failed to get user: %v", err)
	}

	// Update (partial)
	updated, err := svc.UpdateUser(ctx, "user-001", model.User{
		Phone: "+57 311 999 8877",
	})
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	if updated.Name != "Original Name" {
		t.Errorf("expected existing fields to be preserved, got Name=%q", updated.Name)
	}
	if updated.Phone != "+57 311 999 8877" {
		t.Errorf("expected Phone to be updated, got %q", updated.Phone)
	}

	// Delete
	if err := svc.DeleteUser(ctx, "user-001"); err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	_, err = svc.GetUser(ctx, "user-001")
	if err == nil || !errors.Is(err, repository.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound after delete, got %v", err)
	}
}
