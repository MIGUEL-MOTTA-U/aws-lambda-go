package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/storage"
)

const (
	// maxConcurrentUploads bounds the number of goroutines uploading to R2
	// simultaneously within a single request. Prevents overloading R2 and
	// exhausting Lambda memory when processing large batches.
	maxConcurrentUploads = 5

	// maxFilesPerRequest is the maximum number of files accepted in a single
	// upload request. Keeps Lambda execution time predictable.
	maxFilesPerRequest = 10

	// privateURLTTL is the lifetime of presigned GET URLs for private assets.
	privateURLTTL = 1 * time.Hour
)

var (
	ErrInvalidUpload = fmt.Errorf("invalid upload")
	ErrForbidden     = fmt.Errorf("forbidden")
	ErrUploadFailed  = fmt.Errorf("upload failed")

	// safeIDRegexp strips any character that is not alphanumeric or a hyphen,
	// preventing path traversal when building R2 object keys.
	safeIDRegexp = regexp.MustCompile(`[^a-zA-Z0-9\-]`)
)

// ParsedFile holds the decoded content of a single file extracted from a
// multipart request. The controller is responsible for parsing; the service
// only receives validated structs.
type ParsedFile struct {
	Filename    string
	Extension   string // lower-cased, includes the leading dot (e.g. ".jpg")
	ContentType string
	Data        []byte
}

// UploadFilesRequest contains the contextual metadata for a batch upload.
type UploadFilesRequest struct {
	EntityType model.AssetEntityType
	EntityID   string
}

// UploadService handles all file-upload business logic.
// It orchestrates concurrent uploads to R2 and persists metadata to the database.
type UploadService struct {
	assetRepo   repository.AssetRepository
	storage     storage.StorageClient
	idGenerator IDGenerator
}

func NewUploadService(
	assetRepo repository.AssetRepository,
	storageClient storage.StorageClient,
	idGenerator IDGenerator,
) *UploadService {
	return &UploadService{
		assetRepo:   assetRepo,
		storage:     storageClient,
		idGenerator: idGenerator,
	}
}

// UploadFiles validates a batch of files and uploads them concurrently to R2.
//
// Concurrency design:
//   - An errgroup with a bounded limit (maxConcurrentUploads) fans out one
//     goroutine per file. Each goroutine uploads its file to R2 and, on
//     success, immediately saves the asset record to the database.
//   - If any goroutine fails, the errgroup cancels all remaining goroutines
//     via the shared context. Already-uploaded objects are then deleted from
//     R2 concurrently in a best-effort rollback goroutine fan-out.
//   - A sync.Mutex protects the shared results slice and the successKeys list
//     since multiple goroutines write to them.
func (s *UploadService) UploadFiles(
	ctx context.Context,
	files []ParsedFile,
	req UploadFilesRequest,
	ownerID string,
) ([]model.Asset, error) {
	// ── 1. Validate request shape ────────────────────────────────────────────
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: at least one file is required", ErrInvalidUpload)
	}
	if len(files) > maxFilesPerRequest {
		return nil, fmt.Errorf("%w: maximum %d files per request, got %d",
			ErrInvalidUpload, maxFilesPerRequest, len(files))
	}
	if strings.TrimSpace(string(req.EntityType)) == "" {
		return nil, fmt.Errorf("%w: entity_type is required", ErrInvalidUpload)
	}
	if strings.TrimSpace(req.EntityID) == "" {
		return nil, fmt.Errorf("%w: entity_id is required", ErrInvalidUpload)
	}

	constraint, ok := model.AssetConstraints[req.EntityType]
	if !ok {
		return nil, fmt.Errorf("%w: unknown entity_type %q", ErrInvalidUpload, req.EntityType)
	}

	// ── 2. Validate every file before touching R2 ────────────────────────────
	for i, f := range files {
		if err := validateFile(f, constraint); err != nil {
			return nil, fmt.Errorf("file[%d] %q: %w", i, f.Filename, err)
		}
	}

	// ── 3. Enforce per-entity count limit ────────────────────────────────────
	if constraint.MaxCountPerEntity > 0 {
		current, err := s.assetRepo.CountByEntity(ctx, req.EntityType, req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("counting existing assets: %w", err)
		}
		if int(current)+len(files) > constraint.MaxCountPerEntity {
			return nil, fmt.Errorf(
				"%w: entity %q already has %d asset(s); adding %d would exceed the limit of %d",
				ErrInvalidUpload, req.EntityID, current, len(files), constraint.MaxCountPerEntity,
			)
		}
	}

	// ── 4. Concurrent upload fan-out ─────────────────────────────────────────
	results := make([]model.Asset, len(files))
	var (
		mu          sync.Mutex
		successKeys []string // tracks R2 keys of successful uploads for rollback
	)

	// errgroup propagates the first error and cancels all sibling goroutines.
	// SetLimit caps the parallelism to avoid memory spikes inside Lambda.
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentUploads)

	for i, file := range files {
		i, file := i, file // capture loop variables for the closure
		g.Go(func() error {
			asset, err := s.uploadOne(gCtx, file, req, ownerID, constraint.IsPublic)
			if err != nil {
				return err
			}

			mu.Lock()
			results[i] = asset
			successKeys = append(successKeys, asset.ObjectKey)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// ── 5. Concurrent rollback on failure ─────────────────────────────────
		// Delete objects that were already written to R2 before the failure.
		// This runs in a fresh context because the original one may be cancelled.
		s.rollbackUploads(successKeys)
		return nil, fmt.Errorf("%w: %s", ErrUploadFailed, err.Error())
	}

	return results, nil
}

// uploadOne handles a single file: uploads bytes to R2, then saves the asset
// record in the database. If the DB write fails after a successful R2 upload,
// the object is deleted from R2 to keep both stores consistent.
func (s *UploadService) uploadOne(
	ctx context.Context,
	file ParsedFile,
	req UploadFilesRequest,
	ownerID string,
	isPublic bool,
) (model.Asset, error) {
	assetID := s.idGenerator()
	key := buildObjectKey(req.EntityType, req.EntityID, assetID, file.Extension)

	// Upload to R2
	if err := s.storage.PutObject(ctx, key, file.Data, file.ContentType); err != nil {
		return model.Asset{}, fmt.Errorf("R2 upload: %w", err)
	}

	// Persist metadata in the database
	asset := model.Asset{
		ID:          assetID,
		EntityType:  req.EntityType,
		EntityID:    req.EntityID,
		ObjectKey:   key,
		ContentType: file.ContentType,
		FileSize:    int64(len(file.Data)),
		Status:      model.AssetStatusConfirmed,
		IsPublic:    isPublic,
		OwnerID:     ownerID,
	}
	if err := s.assetRepo.Create(ctx, asset); err != nil {
		// R2 write succeeded but DB failed — delete the orphaned R2 object.
		if delErr := s.storage.DeleteObject(context.Background(), key); delErr != nil {
			log.Printf("[ERROR] uploadOne: DB save failed AND rollback of R2 object %q failed: %v", key, delErr)
		}
		return model.Asset{}, fmt.Errorf("saving asset to database: %w", err)
	}

	return asset, nil
}

// rollbackUploads deletes a list of R2 object keys concurrently (best-effort).
// Errors are only logged — they do not surface to the caller because the
// original upload error has already been returned.
func (s *UploadService) rollbackUploads(keys []string) {
	if len(keys) == 0 {
		return
	}

	g := &errgroup.Group{}
	for _, key := range keys {
		key := key
		g.Go(func() error {
			return s.storage.DeleteObject(context.Background(), key)
		})
	}
	if err := g.Wait(); err != nil {
		log.Printf("[WARN] rollbackUploads: could not delete all R2 objects during rollback: %v", err)
	}
}

// GetAssetURL resolves the URL for a given asset.
// For public assets it returns the Cloudflare CDN URL immediately (no network call).
// For private assets it generates a time-limited presigned GET URL from R2.
func (s *UploadService) GetAssetURL(ctx context.Context, id string, ownerID string) (model.Asset, error) {
	asset, err := s.assetRepo.FindByID(ctx, id)
	if err != nil {
		return model.Asset{}, err
	}
	if asset.OwnerID != ownerID {
		return model.Asset{}, ErrForbidden
	}

	if asset.IsPublic {
		asset.URL = s.storage.PublicURL(asset.ObjectKey)
	} else {
		url, err := s.storage.GenerateGetURL(ctx, asset.ObjectKey, privateURLTTL)
		if err != nil {
			return model.Asset{}, fmt.Errorf("generating signed URL: %w", err)
		}
		asset.URL = url
	}

	return asset, nil
}

// ListEntityAssets returns all confirmed assets for an entity and resolves their URLs.
// Public assets get CDN URLs; private assets get presigned URLs.
// URL generation for each asset runs sequentially — presign is a CPU-only operation
// and the list is bounded by MaxCountPerEntity, so parallelism is not needed here.
func (s *UploadService) ListEntityAssets(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error) {
	assets, err := s.assetRepo.FindByEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}

	for i := range assets {
		if assets[i].IsPublic {
			assets[i].URL = s.storage.PublicURL(assets[i].ObjectKey)
		} else {
			url, err := s.storage.GenerateGetURL(ctx, assets[i].ObjectKey, privateURLTTL)
			if err != nil {
				log.Printf("[WARN] ListEntityAssets: could not generate URL for asset %q: %v", assets[i].ID, err)
				continue
			}
			assets[i].URL = url
		}
	}

	return assets, nil
}

// DeleteAsset soft-deletes the DB record and removes the object from R2.
// Only the owner may delete an asset.
func (s *UploadService) DeleteAsset(ctx context.Context, id string, ownerID string) error {
	asset, err := s.assetRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if asset.OwnerID != ownerID {
		return ErrForbidden
	}

	// Soft-delete in DB first so the asset is immediately invisible to reads.
	if err := s.assetRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	// Delete from R2. If this fails, the record is already soft-deleted in DB
	// and can be cleaned up by a background job.
	if err := s.storage.DeleteObject(ctx, asset.ObjectKey); err != nil {
		log.Printf("[WARN] DeleteAsset: soft-deleted asset %q from DB but R2 deletion failed: %v", id, err)
	}

	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// buildObjectKey constructs the R2 object key for an asset.
// The key is always generated internally — clients never propose or see it.
//
// Pattern: <entity-prefix>/<entity-id>/<subfolder>/<asset-id><ext>
// Example: listings/abc123/photos/f3a1b2c4d5e6.webp
func buildObjectKey(entityType model.AssetEntityType, entityID, assetID, ext string) string {
	prefix := entityTypeToPrefix(entityType)
	sub := entityTypeToSubfolder(entityType)
	safeID := safeIDRegexp.ReplaceAllString(entityID, "")
	return fmt.Sprintf("%s/%s/%s/%s%s", prefix, safeID, sub, assetID, ext)
}

func entityTypeToPrefix(t model.AssetEntityType) string {
	switch t {
	case model.EntityTypeListingPhoto, model.EntityTypeListingPDF:
		return "listings"
	case model.EntityTypeUserAvatar:
		return "users"
	default:
		return "misc"
	}
}

func entityTypeToSubfolder(t model.AssetEntityType) string {
	switch t {
	case model.EntityTypeListingPhoto:
		return "photos"
	case model.EntityTypeListingPDF:
		return "documents"
	case model.EntityTypeUserAvatar:
		return "avatar"
	default:
		return "files"
	}
}

// validateFile checks a single parsed file against the constraint for its entity type.
func validateFile(f ParsedFile, c model.AssetConstraint) error {
	if len(f.Data) == 0 {
		return fmt.Errorf("%w: file is empty", ErrInvalidUpload)
	}
	if int64(len(f.Data)) > c.MaxSizeBytes {
		return fmt.Errorf("%w: file size %d bytes exceeds maximum of %d bytes",
			ErrInvalidUpload, len(f.Data), c.MaxSizeBytes)
	}
	if !contains(c.AllowedMIMEs, f.ContentType) {
		return fmt.Errorf("%w: content type %q is not allowed (allowed: %v)",
			ErrInvalidUpload, f.ContentType, c.AllowedMIMEs)
	}

	// Detect MIME type from file magic numbers (prevents MIME spoofing)
	detected := http.DetectContentType(f.Data)
	// DetectContentType can return e.g. "image/jpeg; charset=..." — strip params
	if idx := strings.Index(detected, ";"); idx != -1 {
		detected = strings.TrimSpace(detected[:idx])
	}
	if !contains(c.AllowedMIMEs, detected) {
		return fmt.Errorf("%w: detected file type %q does not match declared type %q",
			ErrInvalidUpload, detected, f.ContentType)
	}

	ext := strings.ToLower(filepath.Ext(f.Filename))
	if ext == "" || !contains(c.AllowedExts, ext) {
		return fmt.Errorf("%w: file extension %q is not allowed (allowed: %v)",
			ErrInvalidUpload, ext, c.AllowedExts)
	}

	return nil
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
