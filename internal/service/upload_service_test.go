package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type fakeAssetRepository struct {
	assets map[string]model.Asset
}

func newFakeAssetRepository() *fakeAssetRepository {
	return &fakeAssetRepository{assets: make(map[string]model.Asset)}
}

func (r *fakeAssetRepository) Create(ctx context.Context, asset model.Asset) error {
	r.assets[asset.ID] = asset
	return nil
}

func (r *fakeAssetRepository) FindByID(ctx context.Context, id string) (model.Asset, error) {
	asset, ok := r.assets[id]
	if !ok || asset.Status == model.AssetStatusDeleted {
		return model.Asset{}, repository.ErrAssetNotFound
	}
	return asset, nil
}

func (r *fakeAssetRepository) FindByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error) {
	var out []model.Asset
	for _, a := range r.assets {
		if a.EntityType == entityType && a.EntityID == entityID && a.Status != model.AssetStatusDeleted {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *fakeAssetRepository) CountByEntity(ctx context.Context, entityType model.AssetEntityType, entityID string) (int64, error) {
	assets, _ := r.FindByEntity(ctx, entityType, entityID)
	return int64(len(assets)), nil
}

func (r *fakeAssetRepository) SoftDelete(ctx context.Context, id string) error {
	asset, ok := r.assets[id]
	if !ok {
		return repository.ErrAssetNotFound
	}
	asset.Status = model.AssetStatusDeleted
	r.assets[id] = asset
	return nil
}

type fakeStorage struct {
	objects map[string][]byte
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: make(map[string][]byte)}
}

func (s *fakeStorage) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	s.objects[key] = data
	return nil
}

func (s *fakeStorage) DeleteObject(ctx context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func (s *fakeStorage) ObjectExists(ctx context.Context, key string) (bool, int64, error) {
	data, ok := s.objects[key]
	return ok, int64(len(data)), nil
}

func (s *fakeStorage) PublicURL(key string) string {
	return "https://cdn.example.com/" + key
}

func (s *fakeStorage) GenerateGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://signed.example.com/" + key, nil
}

func (s *fakeStorage) DetectContentType(ctx context.Context, key string) (string, error) {
	return "image/jpeg", nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// jpegFile builds a real (tiny) JPEG so magic-number detection passes.
func jpegFile(name string) ParsedFile {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 150, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return ParsedFile{
		Filename:    name,
		Extension:   ".jpg",
		ContentType: "image/jpeg",
		Data:        buf.Bytes(),
	}
}

func newTestUploadService() (*UploadService, *fakeAssetRepository, *fakeStorage) {
	repo := newFakeAssetRepository()
	store := newFakeStorage()
	nextID := 0
	idGen := func() string {
		nextID++
		return fmt.Sprintf("asset-%03d", nextID)
	}
	return NewUploadService(repo, store, idGen), repo, store
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestUserAvatarUploadReplacesExisting(t *testing.T) {
	svc, repo, store := newTestUploadService()
	ctx := context.Background()
	req := UploadFilesRequest{EntityType: model.EntityTypeUserAvatar, EntityID: "user-1"}

	// First upload succeeds.
	first, err := svc.UploadFiles(ctx, []ParsedFile{jpegFile("a.jpg")}, req, "owner-1")
	if err != nil {
		t.Fatalf("first upload: unexpected error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first upload: expected 1 asset, got %d", len(first))
	}

	// Second upload must replace the first, not fail against the limit of 1.
	second, err := svc.UploadFiles(ctx, []ParsedFile{jpegFile("b.jpg")}, req, "owner-1")
	if err != nil {
		t.Fatalf("second upload: expected replace semantics, got error: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("second upload: expected 1 asset, got %d", len(second))
	}

	// Only the new asset remains active for the entity.
	active, _ := repo.FindByEntity(ctx, model.EntityTypeUserAvatar, "user-1")
	if len(active) != 1 || active[0].ID != second[0].ID {
		t.Fatalf("expected only the replacement asset to remain, got %+v", active)
	}

	// The old R2 object is gone; the new one exists.
	if _, ok := store.objects[first[0].ObjectKey]; ok {
		t.Fatal("expected old R2 object to be deleted after replacement")
	}
	if _, ok := store.objects[second[0].ObjectKey]; !ok {
		t.Fatal("expected new R2 object to exist after replacement")
	}

	// Avatar is a public asset: it must carry a permanent CDN URL.
	if second[0].URL == "" || second[0].URL != store.PublicURL(second[0].ObjectKey) {
		t.Fatalf("expected public CDN URL for avatar, got %q", second[0].URL)
	}
}

func TestListingPhotosStillEnforceLimit(t *testing.T) {
	svc, repo, _ := newTestUploadService()
	ctx := context.Background()
	req := UploadFilesRequest{EntityType: model.EntityTypeListingPhoto, EntityID: "listing-1"}

	// Fill the entity up to the limit of 30 photos.
	for i := 0; i < 30; i++ {
		repo.assets[fmt.Sprintf("seed-%03d", i)] = model.Asset{
			ID:         fmt.Sprintf("seed-%03d", i),
			EntityType: model.EntityTypeListingPhoto,
			EntityID:   "listing-1",
			Status:     model.AssetStatusConfirmed,
		}
	}

	// Uploading one more must fail: multi-asset entities keep the hard limit.
	if _, err := svc.UploadFiles(ctx, []ParsedFile{jpegFile("extra.jpg")}, req, "owner-1"); err == nil {
		t.Fatal("expected limit error for listing photos beyond 30, got nil")
	}
}
