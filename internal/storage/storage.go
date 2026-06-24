package storage

import (
	"context"
	"time"
)

// StorageClient abstracts file storage operations.
// The production implementation uses Cloudflare R2 via the S3-compatible API.
// Using an interface allows unit testing with mocks and future provider changes.
type StorageClient interface {
	// PutObject uploads raw bytes directly to storage.
	// Used by the Lambda to proxy file uploads — clients never access R2 directly.
	PutObject(ctx context.Context, key string, data []byte, contentType string) error

	// DeleteObject removes an object from storage.
	// Used during rollback when a concurrent batch upload partially fails.
	DeleteObject(ctx context.Context, key string) error

	// ObjectExists checks whether an object exists and returns its byte size.
	ObjectExists(ctx context.Context, key string) (exists bool, size int64, err error)

	// PublicURL returns the Cloudflare CDN URL for a publicly accessible object.
	// Only valid for assets where IsPublic = true.
	PublicURL(key string) string

	// GenerateGetURL produces a time-limited signed URL for a private object.
	// The Lambda generates this on demand; it is never stored in the database.
	GenerateGetURL(ctx context.Context, key string, ttl time.Duration) (string, error)

	// DetectContentType downloads the first 512 bytes of an object and uses
	// net/http magic-number detection to return its real MIME type.
	// Called during upload confirmation to prevent MIME spoofing.
	DetectContentType(ctx context.Context, key string) (string, error)
}
