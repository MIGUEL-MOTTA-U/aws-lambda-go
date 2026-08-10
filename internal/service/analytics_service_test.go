package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"rs-lambda-go/internal/model"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type fakeAnalyticsRepo struct {
	mu     sync.Mutex
	events []model.Visit
}

func (r *fakeAnalyticsRepo) Create(ctx context.Context, v model.Visit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, v)
	return nil
}

func (r *fakeAnalyticsRepo) ListByVisitorSince(ctx context.Context, visitorID string, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.events {
		if v.VisitorID == visitorID && !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *fakeAnalyticsRepo) ListByListingSince(ctx context.Context, listingID string, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.events {
		if v.ListingID == listingID && !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *fakeAnalyticsRepo) ListAllSince(ctx context.Context, since time.Time) ([]model.Visit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Visit
	for _, v := range r.events {
		if !v.CreatedAt.Before(since) {
			out = append(out, v)
		}
	}
	return out, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// eventFactory returns a helper that builds events relative to a base time.
func eventFactory(base time.Time) func(listing, visitor string, offsetMs int, eventType model.VisitEventType) model.Visit {
	visitorCounter := make(map[string]int)
	return func(listing, visitor string, offsetMs int, eventType model.VisitEventType) model.Visit {
		visitorCounter[visitor]++
		return model.Visit{
			ID:        "evt-" + visitor + "-" + listing + "-" + string(eventType),
			VisitorID:  visitor,
			ListingID:  listing,
			EventType:  eventType,
			Source:     model.VisitSourcePublic,
			CreatedAt: base.Add(time.Duration(offsetMs) * time.Millisecond),
		}
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestAnalyticsService_RejectsInvalidWindow(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	_, err := svc.TopListings(context.Background(), "60d", 50)
	if !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("expected ErrInvalidWindow, got %v", err)
	}
}

func TestAnalyticsService_DefaultWindowIs7d(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	// Empty window must be accepted; no events → empty result, no error.
	stats, err := svc.TopListings(context.Background(), "", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %d", len(stats))
	}
}

func TestAnalyticsService_OrdersByTotalViews(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	// Listing-A: 3 views, 2 visitors; Listing-B: 5 views, 1 visitor; Listing-C: 1 view.
	repo.events = append(repo.events,
		ev("A", "v1", 0, model.VisitEventStart), ev("A", "v1", 1000, model.VisitEventEnd),
		ev("A", "v2", 2000, model.VisitEventStart), ev("A", "v2", 3000, model.VisitEventEnd),
		ev("A", "v1", 4000, model.VisitEventStart), ev("A", "v1", 5000, model.VisitEventEnd),
		ev("B", "v1", 0, model.VisitEventStart), ev("B", "v1", 1000, model.VisitEventEnd),
		ev("B", "v1", 2000, model.VisitEventStart), ev("B", "v1", 3000, model.VisitEventEnd),
		ev("B", "v1", 4000, model.VisitEventStart), ev("B", "v1", 5000, model.VisitEventEnd),
		ev("B", "v1", 6000, model.VisitEventStart), ev("B", "v1", 7000, model.VisitEventEnd),
		ev("B", "v1", 8000, model.VisitEventStart), ev("B", "v1", 9000, model.VisitEventEnd),
		ev("C", "v3", 0, model.VisitEventStart),
	)

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 listings, got %d", len(stats))
	}
	if stats[0].ListingID != "B" || stats[0].TotalViews != 5 {
		t.Errorf("rank 1: got %s (%d views), want B (5)", stats[0].ListingID, stats[0].TotalViews)
	}
	if stats[1].ListingID != "A" || stats[1].TotalViews != 3 {
		t.Errorf("rank 2: got %s (%d views), want A (3)", stats[1].ListingID, stats[1].TotalViews)
	}
	if stats[2].ListingID != "C" || stats[2].TotalViews != 1 {
		t.Errorf("rank 3: got %s (%d views), want C (1)", stats[2].ListingID, stats[2].TotalViews)
	}
}

func TestAnalyticsService_UniqueVisitorsCountedPerListing(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	repo.events = append(repo.events,
		ev("A", "v1", 0, model.VisitEventStart),
		ev("A", "v2", 1000, model.VisitEventStart),
		ev("A", "v1", 2000, model.VisitEventStart), // same visitor, second view
		ev("A", "v3", 3000, model.VisitEventStart),
	)

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(stats))
	}
	if stats[0].UniqueVisitors != 3 {
		t.Errorf("expected 3 unique visitors, got %d", stats[0].UniqueVisitors)
	}
	if stats[0].TotalViews != 4 {
		t.Errorf("expected 4 total views, got %d", stats[0].TotalViews)
	}
}

func TestAnalyticsService_PairsDurationsWithinWindow(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	// Two visits: 1000ms and 5000ms.
	repo.events = append(repo.events,
		ev("A", "v1", 0, model.VisitEventStart), ev("A", "v1", 1000, model.VisitEventEnd),
		ev("A", "v1", 2000, model.VisitEventStart), ev("A", "v1", 7000, model.VisitEventEnd),
	)

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if stats[0].AvgDurationMs != 3000 {
		t.Errorf("expected avg 3000ms, got %v", stats[0].AvgDurationMs)
	}
	if stats[0].MedianDurationMs != 1000 {
		t.Errorf("expected p50 1000ms, got %d", stats[0].MedianDurationMs)
	}
	if stats[0].P95DurationMs != 5000 {
		t.Errorf("expected p95 5000ms, got %d", stats[0].P95DurationMs)
	}
}

func TestAnalyticsService_DropsOrphanedEndEvents(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	// view_end without matching start (e.g. page closed before emit).
	repo.events = append(repo.events,
		ev("A", "v1", 0, model.VisitEventEnd),
		// Start with end more than pairing window later (31m > 30m cap).
		ev("A", "v1", 1000, model.VisitEventStart),
		ev("A", "v1", 1000+31*60*1000, model.VisitEventEnd),
	)

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if stats[0].AvgDurationMs != 0 {
		t.Errorf("orphan ends must not contribute to avg, got %v", stats[0].AvgDurationMs)
	}
	if stats[0].TotalViews != 1 {
		t.Errorf("still 1 view start, expected 1, got %d", stats[0].TotalViews)
	}
}

func TestAnalyticsService_UsesDurationMsFallbackWhenStartUnpaired(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	dur := 15000

	repo.events = []model.Visit{
		{
			ID:         "e1",
			VisitorID:  "v1",
			ListingID:  "listing-A",
			EventType:  model.VisitEventEnd,
			DurationMs: &dur,
			CreatedAt:  now,
		},
	}

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) == 0 {
		t.Fatalf("expected stats for listing-A")
	}
	if stats[0].AvgDurationMs != 15000 {
		t.Errorf("expected avg duration 15000ms from fallback, got %v", stats[0].AvgDurationMs)
	}
}

func TestAnalyticsService_RespectsLimit(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	// 5 listings, each with 1 view.
	for i := 0; i < 5; i++ {
		repo.events = append(repo.events,
			ev(listingID(i), "v1", i*1000, model.VisitEventStart),
		)
	}

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 2)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 stats (limit), got %d", len(stats))
	}
}

func TestAnalyticsService_EmptyResultIsClean(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	stats, err := svc.TopListings(context.Background(), "7d", 50)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty, got %d", len(stats))
	}
}

func TestAnalyticsService_PropagatesRepoError(t *testing.T) {
	// A repo whose only ListAllSince always errors, satisfying the
	// interface by embedding the existing fakeAnalyticsRepo for the
	// methods we don't care about.
	repo := &failingAnalyticsRepo{err: errors.New("db exploded")}
	svc := NewAnalyticsService(repo)

	_, err := svc.TopListings(context.Background(), "7d", 50)
	if err == nil {
		t.Fatalf("expected error from repo")
	}
	if !errors.Is(err, repo.err) {
		t.Errorf("expected wrapped error to include original, got %v", err)
	}
}

func TestAnalyticsService_LimitClampedToMax(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	now := time.Now().UTC()
	ev := eventFactory(now)

	// 250 distinct listings × 1 view each.
	for i := 0; i < 250; i++ {
		repo.events = append(repo.events, ev(fmt.Sprintf("listing-%04d", i), "v1", i*1000, model.VisitEventStart))
	}

	svc := NewAnalyticsService(repo)
	stats, err := svc.TopListings(context.Background(), "7d", 1000)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(stats) != 200 {
		t.Errorf("limit must clamp to 200, got %d", len(stats))
	}
}

func TestAnalyticsService_ExcludesEventsOlderThan90Days(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	baseTime := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	// Event 1: 100 days ago (old event)
	oldEvent := model.Visit{
		ID:        "evt-old",
		VisitorID: "v-old",
		ListingID: "listing-100d",
		EventType: model.VisitEventStart,
		Source:    model.VisitSourcePublic,
		CreatedAt: baseTime.Add(-100 * 24 * time.Hour),
	}
	// Event 2: 5 days ago (recent event)
	recentEvent := model.Visit{
		ID:        "evt-recent",
		VisitorID: "v-recent",
		ListingID: "listing-5d",
		EventType: model.VisitEventStart,
		Source:    model.VisitSourcePublic,
		CreatedAt: baseTime.Add(-5 * 24 * time.Hour),
	}
	repo.events = []model.Visit{oldEvent, recentEvent}

	svc := NewAnalyticsService(repo)
	svc.now = func() time.Time { return baseTime }

	windows := []string{"7d", "30d"}
	for _, w := range windows {
		t.Run("window_"+w, func(t *testing.T) {
			stats, err := svc.TopListings(context.Background(), w, 50)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, s := range stats {
				if s.ListingID == "listing-100d" {
					t.Errorf("event from 100 days ago appeared in window %s aggregate", w)
				}
			}
			foundRecent := false
			for _, s := range stats {
				if s.ListingID == "listing-5d" {
					foundRecent = true
				}
			}
			if !foundRecent {
				t.Errorf("expected recent event (5d ago) to appear in window %s aggregate", w)
			}
		})
	}
}

// uniqueListingID returns a unique string for an index up to 999.
func uniqueListingID(i int) string {
	return fmt.Sprintf("listing-%04d", i)
}

// listingID returns a deterministic string for a test index.
func listingID(i int) string {
	const letters = "ABCDEFGHIJKLMNOP"
	if i < len(letters) {
		return string(letters[i])
	}
	return "X" + string(letters[i%len(letters)])
}

// failingAnalyticsRepo embeds fakeAnalyticsRepo (satisfying the
// interface) and overrides ListAllSince to return an error.
type failingAnalyticsRepo struct {
	*fakeAnalyticsRepo
	err error
}

func (r *failingAnalyticsRepo) ListAllSince(ctx context.Context, since time.Time) ([]model.Visit, error) {
	return nil, r.err
}