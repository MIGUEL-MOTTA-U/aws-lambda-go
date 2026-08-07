package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

const (
	// visitRateLimitWindow is the per-visitor cooldown: at most one
	// event accepted every `visitRateLimitWindow`. Mitigates bot spam
	// without a separate counter table.
	visitRateLimitWindow = 1 * time.Second
)

var (
	// ErrVisitRateLimited is returned when the same visitor_id emits
	// events faster than visitRateLimitWindow. The HTTP layer maps
	// this to 429 Too Many Requests.
	ErrVisitRateLimited = errors.New("visit rate limit exceeded")

	// ErrVisitInvalid is the service-level counterpart of
	// repository.ErrVisitInvalid. Same shape so the controller can
	// switch on a single sentinel.
	ErrVisitInvalid = errors.New("visit payload is invalid")
)

// VisitRecordRequest is the input to RecordEvent after the controller
// has decoded and lightly validated the JSON body.
type VisitRecordRequest struct {
	VisitorID  string
	ListingID  string
	EventType  model.VisitEventType
	Source     model.VisitSource
	DurationMs *int
	UserAgent  string
}

// VisitService centralizes validation and the per-visitor rate limit
// before delegating persistence to the repository.
type VisitService struct {
	repo         repository.VisitRepository
	idGenerator  IDGenerator
	now          func() time.Time

	// rateLimitMu guards rateLimitLast. In a Lambda cold start the map
	// is rebuilt per invocation, so the rate limit is approximate
	// across invocations. This is acceptable: the goal is to deter
	// spam, not to enforce strict quotas.
	rateLimitMu  sync.Mutex
	rateLimitLast map[string]time.Time
}

func NewVisitService(repo repository.VisitRepository, idGenerator IDGenerator) *VisitService {
	return &VisitService{
		repo:          repo,
		idGenerator:   idGenerator,
		now:           func() time.Time { return time.Now().UTC() },
		rateLimitLast: make(map[string]time.Time),
	}
}

// RecordEvent validates the request, enforces the per-visitor rate
// limit, and persists the event. It does not require the listing to
// exist (visits survive listing deletion, per DEC-006).
func (s *VisitService) RecordEvent(ctx context.Context, req VisitRecordRequest) (model.Visit, error) {
	if err := validateVisitRequest(req); err != nil {
		return model.Visit{}, err
	}

	// Default an empty source to public — the only source this endpoint
	// serves in V1. Keeps the front payload minimal.
	if req.Source == "" {
		req.Source = model.VisitSourcePublic
	}

	if s.isRateLimited(req.VisitorID) {
		return model.Visit{}, ErrVisitRateLimited
	}

	visit := model.Visit{
		ID:         s.idGenerator(),
		VisitorID:  req.VisitorID,
		ListingID:  req.ListingID,
		EventType:  req.EventType,
		Source:     req.Source,
		DurationMs: req.DurationMs,
		UserAgent:  req.UserAgent,
		CreatedAt:  s.now(),
	}
	if err := s.repo.Create(ctx, visit); err != nil {
		return model.Visit{}, fmt.Errorf("persisting visit: %w", err)
	}

	s.markSeen(req.VisitorID)
	return visit, nil
}

func validateVisitRequest(req VisitRecordRequest) error {
	if req.VisitorID == "" {
		return fmt.Errorf("%w: visitor_id is required", ErrVisitInvalid)
	}
	if req.ListingID == "" {
		return fmt.Errorf("%w: listing_id is required", ErrVisitInvalid)
	}
	switch req.EventType {
	case model.VisitEventStart, model.VisitEventEnd:
		// ok
	default:
		return fmt.Errorf("%w: event_type must be %q or %q, got %q",
			ErrVisitInvalid, model.VisitEventStart, model.VisitEventEnd, req.EventType)
	}
	if req.Source != "" && req.Source != model.VisitSourcePublic && req.Source != model.VisitSourceAdmin {
		return fmt.Errorf("%w: source must be %q, %q, or empty, got %q",
			ErrVisitInvalid, model.VisitSourcePublic, model.VisitSourceAdmin, req.Source)
	}
	if req.DurationMs != nil && *req.DurationMs < 0 {
		return fmt.Errorf("%w: duration_ms cannot be negative", ErrVisitInvalid)
	}
	return nil
}

func (s *VisitService) isRateLimited(visitorID string) bool {
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()
	last, ok := s.rateLimitLast[visitorID]
	if !ok {
		return false
	}
	return s.now().Sub(last) < visitRateLimitWindow
}

func (s *VisitService) markSeen(visitorID string) {
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()
	s.rateLimitLast[visitorID] = s.now()
}