package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type fakeVisitRepository struct {
	mu      sync.Mutex
	visits  []model.Visit
	byID    map[string]model.Visit
	failOn  error
}

func newFakeVisitRepository() *fakeVisitRepository {
	return &fakeVisitRepository{byID: make(map[string]model.Visit)}
}

func (r *fakeVisitRepository) Create(ctx context.Context, v model.Visit) error {
	if r.failOn != nil {
		return r.failOn
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.visits = append(r.visits, v)
	r.byID[v.ID] = v
	return nil
}

func (r *fakeVisitRepository) ListByVisitorSince(ctx context.Context, visitorID string, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.visits {
		if v.VisitorID == visitorID && !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *fakeVisitRepository) ListByListingSince(ctx context.Context, listingID string, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.visits {
		if v.ListingID == listingID && !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *fakeVisitRepository) ListAllSince(ctx context.Context, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.visits {
		if !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestService(t *testing.T, repo *fakeVisitRepository) *VisitService {
	t.Helper()
	s := NewVisitService(repo, func() string { return "test-id" })
	// Pin clock so rate limit tests are deterministic.
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }
	return s
}

func validRequest() VisitRecordRequest {
	return VisitRecordRequest{
		VisitorID: "v-1",
		ListingID: "l-1",
		EventType: model.VisitEventStart,
		Source:    model.VisitSourcePublic,
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestVisitService_RecordEvent_Valid(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	visit, err := svc.RecordEvent(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}
	if visit.ID == "" {
		t.Errorf("expected generated ID, got empty")
	}
	if visit.VisitorID != "v-1" || visit.ListingID != "l-1" || visit.EventType != model.VisitEventStart {
		t.Errorf("visit payload round-trip mismatch: %+v", visit)
	}
	if len(repo.visits) != 1 {
		t.Errorf("expected 1 row persisted, got %d", len(repo.visits))
	}
}

func TestVisitService_RecordEvent_RequiresVisitorID(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	req := validRequest()
	req.VisitorID = ""

	_, err := svc.RecordEvent(context.Background(), req)
	if !errors.Is(err, ErrVisitInvalid) {
		t.Fatalf("expected ErrVisitInvalid, got %v", err)
	}
	if len(repo.visits) != 0 {
		t.Errorf("validation must short-circuit before persistence")
	}
}

func TestVisitService_RecordEvent_RequiresListingID(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	req := validRequest()
	req.ListingID = ""

	_, err := svc.RecordEvent(context.Background(), req)
	if !errors.Is(err, ErrVisitInvalid) {
		t.Fatalf("expected ErrVisitInvalid, got %v", err)
	}
}

func TestVisitService_RecordEvent_RejectsUnknownEventType(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	req := validRequest()
	req.EventType = "view_hover"

	_, err := svc.RecordEvent(context.Background(), req)
	if !errors.Is(err, ErrVisitInvalid) {
		t.Fatalf("expected ErrVisitInvalid, got %v", err)
	}
}

func TestVisitService_RecordEvent_RateLimitsByVisitor(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	// First call lands at the pinned clock; second call is within the
	// 1-second window so it must be rejected.
	if _, err := svc.RecordEvent(context.Background(), validRequest()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	_, err := svc.RecordEvent(context.Background(), validRequest())
	if !errors.Is(err, ErrVisitRateLimited) {
		t.Fatalf("expected ErrVisitRateLimited, got %v", err)
	}
	if len(repo.visits) != 1 {
		t.Errorf("rate-limited event must not be persisted (rows=%d)", len(repo.visits))
	}
}

func TestVisitService_RecordEvent_RateLimitExpiresAfterWindow(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	// Advance the clock past the rate-limit window between calls.
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }

	if _, err := svc.RecordEvent(context.Background(), validRequest()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	base = base.Add(2 * visitRateLimitWindow)

	req := validRequest()
	req.EventType = model.VisitEventEnd
	if _, err := svc.RecordEvent(context.Background(), req); err != nil {
		t.Fatalf("second call after window: %v", err)
	}
	if len(repo.visits) != 2 {
		t.Errorf("expected 2 rows persisted after window, got %d", len(repo.visits))
	}
}

func TestVisitService_RecordEvent_RateLimitIsPerVisitor(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	reqA := validRequest()
	reqA.VisitorID = "visitor-A"
	if _, err := svc.RecordEvent(context.Background(), reqA); err != nil {
		t.Fatalf("visitor A: %v", err)
	}

	reqB := validRequest()
	reqB.VisitorID = "visitor-B"
	if _, err := svc.RecordEvent(context.Background(), reqB); err != nil {
		t.Fatalf("visitor B should not be rate-limited by visitor A: %v", err)
	}
	if len(repo.visits) != 2 {
		t.Errorf("expected 2 rows, got %d", len(repo.visits))
	}
}

func TestVisitService_RecordEvent_PersistsDurationOnlyOnEnd(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }

	// Start with no duration.
	_, err := svc.RecordEvent(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Advance past the rate-limit window so the second event is accepted.
	base = base.Add(2 * visitRateLimitWindow)

	req := validRequest()
	req.EventType = model.VisitEventEnd
	dur := 4500
	req.DurationMs = &dur
	_, err = svc.RecordEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("end: %v", err)
	}

	if repo.visits[0].DurationMs != nil {
		t.Errorf("start row should have nil duration, got %v", *repo.visits[0].DurationMs)
	}
	if repo.visits[1].DurationMs == nil || *repo.visits[1].DurationMs != dur {
		t.Errorf("end row should have duration %d, got %v", dur, repo.visits[1].DurationMs)
	}
}

func TestVisitService_RecordEvent_DefaultsSourceToPublic(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	req := validRequest()
	req.Source = ""
	visit, err := svc.RecordEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if visit.Source != model.VisitSourcePublic {
		t.Errorf("expected default source public, got %q", visit.Source)
	}
}

func TestVisitService_RecordEvent_RejectsNegativeDuration(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	req := validRequest()
	req.EventType = model.VisitEventEnd
	neg := -1
	req.DurationMs = &neg

	_, err := svc.RecordEvent(context.Background(), req)
	if !errors.Is(err, ErrVisitInvalid) {
		t.Fatalf("expected ErrVisitInvalid, got %v", err)
	}
}

func TestVisitService_RecordEvent_WrapsRepositoryError(t *testing.T) {
	repo := newFakeVisitRepository()
	repo.failOn = errors.New("db down")
	svc := newTestService(t, repo)

	_, err := svc.RecordEvent(context.Background(), validRequest())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if errors.Is(err, repository.ErrVisitInvalid) {
		t.Errorf("repository error must not be masked as ErrVisitInvalid")
	}
}

func TestVisitService_GetListingVisits_ClampsTo90Days(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	baseTime := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	oldVisit := model.Visit{
		ID:        "v-old",
		ListingID: "l-1",
		CreatedAt: baseTime.Add(-100 * 24 * time.Hour),
	}
	recentVisit := model.Visit{
		ID:        "v-recent",
		ListingID: "l-1",
		CreatedAt: baseTime.Add(-10 * 24 * time.Hour),
	}
	repo.visits = []model.Visit{oldVisit, recentVisit}

	// Requesting visits since 120 days ago should be clamped to 90 days ago.
	visits, err := svc.GetListingVisits(context.Background(), "l-1", baseTime.Add(-120*24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(visits) != 1 {
		t.Fatalf("expected 1 visit (recent visit only), got %d", len(visits))
	}
	if visits[0].ID != "v-recent" {
		t.Errorf("expected recent visit, got %s", visits[0].ID)
	}
}

func TestVisitService_GetVisitorVisits_ClampsTo90Days(t *testing.T) {
	repo := newFakeVisitRepository()
	svc := newTestService(t, repo)

	baseTime := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return baseTime }

	oldVisit := model.Visit{
		ID:        "v-old",
		VisitorID: "vis-1",
		CreatedAt: baseTime.Add(-100 * 24 * time.Hour),
	}
	recentVisit := model.Visit{
		ID:        "v-recent",
		VisitorID: "vis-1",
		CreatedAt: baseTime.Add(-10 * 24 * time.Hour),
	}
	repo.visits = []model.Visit{oldVisit, recentVisit}

	// Requesting visits since 120 days ago should be clamped to 90 days ago.
	visits, err := svc.GetVisitorVisits(context.Background(), "vis-1", baseTime.Add(-120*24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(visits) != 1 {
		t.Fatalf("expected 1 visit (recent visit only), got %d", len(visits))
	}
	if visits[0].ID != "v-recent" {
		t.Errorf("expected recent visit, got %s", visits[0].ID)
	}
}