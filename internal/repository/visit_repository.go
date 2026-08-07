package repository

import (
	"context"
	"errors"
	"time"

	"rs-lambda-go/internal/model"
)

// VisitRepository defines persistence operations for visit events.
type VisitRepository interface {
	// Create persists a single visit event row.
	Create(ctx context.Context, visit model.Visit) error

	// ListByVisitorSince returns events emitted by a specific visitor
	// after `since`, ordered by created_at DESC. Used by the service
	// layer to enforce a per-visitor rate limit.
	ListByVisitorSince(ctx context.Context, visitorID string, since time.Time) ([]model.Visit, error)

	// ListByListingSince returns events for a listing after `since`,
	// ordered by created_at ASC. Used by P-021's aggregate queries.
	// Not exercised by P-020 tests but declared here so the
	// repository contract is complete.
	ListByListingSince(ctx context.Context, listingID string, since time.Time) ([]model.Visit, error)
}

// ErrVisitInvalid is returned when the visit payload fails validation
// before reaching the database (e.g. missing visitor_id).
var ErrVisitInvalid = errors.New("visit payload is invalid")