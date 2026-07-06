package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"rs-lambda-go/internal/controller"
	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
)

// ─── In-memory fakes ─────────────────────────────────────────────────────────

type fakeListingRepository struct {
	listings map[string]model.Listing
}

func newFakeListingRepository() *fakeListingRepository {
	return &fakeListingRepository{listings: make(map[string]model.Listing)}
}

func (r *fakeListingRepository) FindAll(ctx context.Context) ([]model.Listing, error) {
	all := make([]model.Listing, 0, len(r.listings))
	for _, l := range r.listings {
		all = append(all, l)
	}
	return all, nil
}

func (r *fakeListingRepository) FindByID(ctx context.Context, id string) (model.Listing, error) {
	listing, ok := r.listings[id]
	if !ok {
		return model.Listing{}, repository.ErrListingNotFound
	}
	return listing, nil
}

func (r *fakeListingRepository) Create(ctx context.Context, listing model.Listing) error {
	id := string(listing.ListingID)
	if _, ok := r.listings[id]; ok {
		return repository.ErrListingAlreadyExists
	}
	r.listings[id] = listing
	return nil
}

func (r *fakeListingRepository) Update(ctx context.Context, listing model.Listing) error {
	id := string(listing.ListingID)
	if _, ok := r.listings[id]; !ok {
		return repository.ErrListingNotFound
	}
	r.listings[id] = listing
	return nil
}

func (r *fakeListingRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.listings[id]; !ok {
		return repository.ErrListingNotFound
	}
	delete(r.listings, id)
	return nil
}

type fakeUserRepository struct {
	users map[string]model.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[string]model.User)}
}

func (r *fakeUserRepository) FindAll(ctx context.Context) ([]model.User, error) {
	all := make([]model.User, 0, len(r.users))
	for _, u := range r.users {
		all = append(all, u)
	}
	return all, nil
}

func (r *fakeUserRepository) FindByID(ctx context.Context, id string) (model.User, error) {
	user, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) Create(ctx context.Context, user model.User) error {
	if _, ok := r.users[user.ID]; ok {
		return repository.ErrUserAlreadyExists
	}
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepository) Update(ctx context.Context, user model.User) error {
	if _, ok := r.users[user.ID]; !ok {
		return repository.ErrUserNotFound
	}
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return repository.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

// stubUploadService satisfies controller.UploadService for routing tests.
type stubUploadService struct {
	assets []model.Asset
}

func (s *stubUploadService) UploadFiles(ctx context.Context, files []service.ParsedFile, req service.UploadFilesRequest, ownerID string) ([]model.Asset, error) {
	return s.assets, nil
}

func (s *stubUploadService) GetAssetURL(ctx context.Context, id string, ownerID string) (model.Asset, error) {
	return model.Asset{}, repository.ErrAssetNotFound
}

func (s *stubUploadService) ListEntityAssets(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error) {
	return s.assets, nil
}

func (s *stubUploadService) DeleteAsset(ctx context.Context, id string, ownerID string) error {
	return repository.ErrAssetNotFound
}

// ─── Test harness ────────────────────────────────────────────────────────────

func newTestRouter() (Router, *fakeListingRepository, *fakeUserRepository) {
	listingRepo := newFakeListingRepository()
	userRepo := newFakeUserRepository()

	fixedClock := func() time.Time {
		return time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	}
	nextID := 0
	idGenerator := func() string {
		nextID++
		return fmt.Sprintf("test-id-%03d", nextID)
	}

	listingService := service.NewListingServiceWithDependencies(listingRepo, idGenerator, fixedClock)
	userService := service.NewUserServiceWithDependencies(userRepo, idGenerator, fixedClock)

	return Router{
		userController:    controller.NewUserController(userService),
		listingController: controller.NewListingController(listingService),
		uploadController:  controller.NewUploadController(&stubUploadService{}),
	}, listingRepo, userRepo
}

func makeRequest(method, path, body string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		RawPath: path,
		Body:    body,
		Headers: map[string]string{"content-type": "application/json"},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			RequestID: "test-request",
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
				Path:   path,
			},
		},
	}
}

// ─── Listings ────────────────────────────────────────────────────────────────

func TestListingsCRUDFlow(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	// Create.
	createBody := `{
		"title": "Apartamento El Poblado",
		"slug": "apartamento-el-poblado",
		"property_type": "apartment",
		"operation_type": "sale",
		"publication_status": "draft",
		"featured": true,
		"language": "es",
		"location": {"country": "Colombia", "state": "Antioquia", "city": "Medellín", "stratum": 6},
		"pricing": {"sale_price": 850000000, "currency": "COP"}
	}`
	resp, err := router.Route(ctx, makeRequest("POST", "/listings", createBody))
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d (%s)", resp.StatusCode, resp.Body)
	}

	var created model.Listing
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatalf("create: invalid JSON response: %v", err)
	}
	if created.ListingID == "" {
		t.Fatal("create: expected generated listing_id")
	}
	if !created.Featured {
		t.Fatal("create: expected featured flag to persist")
	}
	if created.Metadata.UpdatedAt == "" {
		t.Fatal("create: expected metadata.updated_at to be set")
	}
	if created.Metadata.SourceSystem != "century21colombia" {
		t.Fatalf("create: expected default source_system, got %q", created.Metadata.SourceSystem)
	}

	id := string(created.ListingID)

	// Get by ID.
	resp, _ = router.Route(ctx, makeRequest("GET", "/listings/"+id, ""))
	if resp.StatusCode != 200 {
		t.Fatalf("get: expected 200, got %d (%s)", resp.StatusCode, resp.Body)
	}

	// List.
	resp, _ = router.Route(ctx, makeRequest("GET", "/listings", ""))
	if resp.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}
	var all []model.Listing
	if err := json.Unmarshal([]byte(resp.Body), &all); err != nil {
		t.Fatalf("list: invalid JSON response: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("list: expected 1 listing, got %d", len(all))
	}

	// Update (archive it).
	updateBody := `{
		"title": "Apartamento El Poblado",
		"property_type": "apartment",
		"operation_type": "sale",
		"publication_status": "archived",
		"pricing": {"sale_price": 850000000, "currency": "COP"}
	}`
	resp, _ = router.Route(ctx, makeRequest("PUT", "/listings/"+id, updateBody))
	if resp.StatusCode != 200 {
		t.Fatalf("update: expected 200, got %d (%s)", resp.StatusCode, resp.Body)
	}
	var updated model.Listing
	if err := json.Unmarshal([]byte(resp.Body), &updated); err != nil {
		t.Fatalf("update: invalid JSON response: %v", err)
	}
	if updated.PublicationStatus != "archived" {
		t.Fatalf("update: expected archived status, got %q", updated.PublicationStatus)
	}

	// Delete.
	resp, _ = router.Route(ctx, makeRequest("DELETE", "/listings/"+id, ""))
	if resp.StatusCode != 204 {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}
	resp, _ = router.Route(ctx, makeRequest("GET", "/listings/"+id, ""))
	if resp.StatusCode != 404 {
		t.Fatalf("get after delete: expected 404, got %d", resp.StatusCode)
	}
}

func TestListingValidationErrors(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	cases := []struct {
		name string
		body string
	}{
		{"invalid json", `{not json`},
		{"invalid currency", `{"title":"x","pricing":{"sale_price":100,"currency":"GBP"}}`},
		{"invalid language", `{"title":"x","language":"fr"}`},
		{"invalid stratum", `{"title":"x","location":{"stratum":9}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := router.Route(ctx, makeRequest("POST", "/listings", tc.body))
			if resp.StatusCode != 400 {
				t.Fatalf("expected 400, got %d (%s)", resp.StatusCode, resp.Body)
			}
			var apiErr controller.APIError
			if err := json.Unmarshal([]byte(resp.Body), &apiErr); err != nil {
				t.Fatalf("expected structured error body, got %s", resp.Body)
			}
			if apiErr.Code != "BAD_REQUEST" {
				t.Fatalf("expected BAD_REQUEST code, got %q", apiErr.Code)
			}
		})
	}
}

func TestListingNotFoundAndMethodNotAllowed(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	resp, _ := router.Route(ctx, makeRequest("GET", "/listings/missing-id", ""))
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 for missing listing, got %d", resp.StatusCode)
	}

	resp, _ = router.Route(ctx, makeRequest("PATCH", "/listings", ""))
	if resp.StatusCode != 405 {
		t.Fatalf("expected 405 for PATCH /listings, got %d", resp.StatusCode)
	}

	resp, _ = router.Route(ctx, makeRequest("GET", "/unknown", ""))
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 for unknown route, got %d", resp.StatusCode)
	}
}

// ─── Users ───────────────────────────────────────────────────────────────────

func TestUsersCRUDFlow(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	createBody := `{
		"name": "Aura Urrea",
		"email": "aura.urrea@century21.com.co",
		"username": "aura.urrea",
		"birthdate": "1985-04-12",
		"phone": "+57 300 123 4567",
		"license": "LONJA 12847"
	}`
	resp, err := router.Route(ctx, makeRequest("POST", "/users", createBody))
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d (%s)", resp.StatusCode, resp.Body)
	}
	var created model.User
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatalf("create: invalid JSON response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create: expected generated id")
	}
	if created.CreationDate == "" {
		t.Fatal("create: expected creation date to be set")
	}

	// Partial update: only phone; other fields must survive.
	resp, _ = router.Route(ctx, makeRequest("PUT", "/users/"+created.ID, `{"phone":"+57 311 000 0000"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("update: expected 200, got %d (%s)", resp.StatusCode, resp.Body)
	}
	var updated model.User
	if err := json.Unmarshal([]byte(resp.Body), &updated); err != nil {
		t.Fatalf("update: invalid JSON response: %v", err)
	}
	if updated.Phone != "+57 311 000 0000" {
		t.Fatalf("update: phone not updated, got %q", updated.Phone)
	}
	if updated.Name != "Aura Urrea" {
		t.Fatalf("update: name should be preserved, got %q", updated.Name)
	}

	// List.
	resp, _ = router.Route(ctx, makeRequest("GET", "/users", ""))
	if resp.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}

	// Delete.
	resp, _ = router.Route(ctx, makeRequest("DELETE", "/users/"+created.ID, ""))
	if resp.StatusCode != 204 {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}
	resp, _ = router.Route(ctx, makeRequest("GET", "/users/"+created.ID, ""))
	if resp.StatusCode != 404 {
		t.Fatalf("get after delete: expected 404, got %d", resp.StatusCode)
	}
}

func TestUserValidationErrors(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	cases := []struct {
		name string
		body string
	}{
		{"invalid email", `{"name":"x","email":"not-an-email"}`},
		{"invalid birthdate", `{"name":"x","birthdate":"12/04/1985"}`},
		{"invalid phone", `{"name":"x","phone":"call me"}`},
		{"invalid url", `{"name":"x","instagram_url":"not-a-url"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := router.Route(ctx, makeRequest("POST", "/users", tc.body))
			if resp.StatusCode != 400 {
				t.Fatalf("expected 400, got %d (%s)", resp.StatusCode, resp.Body)
			}
		})
	}
}

// ─── Uploads routing ─────────────────────────────────────────────────────────

func TestListingMediaRouteIsHandled(t *testing.T) {
	router, _, _ := newTestRouter()
	ctx := context.Background()

	resp, _ := router.Route(ctx, makeRequest("GET", "/listings/abc/media", ""))
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 from media route, got %d (%s)", resp.StatusCode, resp.Body)
	}
}

func TestUploadWithoutOwnerIsUnauthorized(t *testing.T) {
	t.Setenv("ALLOW_UNAUTHENTICATED_UPLOADS", "false")

	router, _, _ := newTestRouter()
	ctx := context.Background()

	req := makeRequest("POST", "/uploads", "")
	req.Headers["content-type"] = "multipart/form-data; boundary=x"
	resp, _ := router.Route(ctx, req)
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 without authorizer, got %d (%s)", resp.StatusCode, resp.Body)
	}
}
