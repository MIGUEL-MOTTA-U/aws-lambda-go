package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
)

const (
	// analyticsPairingWindow caps the time gap between a view_start
	// and its matching view_end from the same visitor/listing. See DEC-009:
	// if the end event arrives more than this after the start, we treat
	// it as orphaned and skip it from duration stats.
	analyticsPairingWindow = 30 * time.Second
)

var (
	// ErrInvalidWindow is returned when the analytics window is not
	// one of the supported values ("7d", "30d").
	ErrInvalidWindow = errors.New("invalid analytics window")
)

// ListingVisitStats summarizes a listing's view activity in a window.
type ListingVisitStats struct {
	ListingID        string  `json:"listing_id"`
	UniqueVisitors   int     `json:"unique_visitors"`
	TotalViews       int     `json:"total_views"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
	MedianDurationMs int     `json:"p50_duration_ms"`
	P95DurationMs    int     `json:"p95_duration_ms"`
}

// AnalyticsService computes aggregate visit statistics over time windows.
type AnalyticsService struct {
	visitRepo repository.VisitRepository
}

func NewAnalyticsService(visitRepo repository.VisitRepository) *AnalyticsService {
	return &AnalyticsService{visitRepo: visitRepo}
}

// TopListings returns visit statistics for the listings with the most views
// in the requested window, ordered by total_views DESC.
//
// window must be "7d" or "30d" (default 7d if empty).
// Listings with zero views in the window are excluded.
func (s *AnalyticsService) TopListings(ctx context.Context, window string, limit int) ([]ListingVisitStats, error) {
	dur, err := parseWindow(window)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	since := time.Now().UTC().Add(-dur)

	// Strategy: pull every visit event for the window (cap below keeps this
	// safe in V1 — the table is small). Pair starts with ends, compute
	// stats per listing, then sort and trim.
	events, err := s.visitRepo.ListAllSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("loading visits: %w", err)
	}

	stats := computeStatsFromEvents(events)
	out := make([]ListingVisitStats, 0, len(stats))
	for _, v := range stats {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalViews != out[j].TotalViews {
			return out[i].TotalViews > out[j].TotalViews
		}
		return out[i].ListingID < out[j].ListingID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// computeStatsFromEvents groups events by listing, counts unique visitors,
// pairs starts with ends within analyticsPairingWindow, and computes
// average + percentile durations.
func computeStatsFromEvents(events []model.Visit) map[string]ListingVisitStats {
	byListing := make(map[string]*listingAccum)
	for _, e := range events {
		acc := byListing[e.ListingID]
		if acc == nil {
			acc = &listingAccum{listingID: e.ListingID, visitors: make(map[string]struct{})}
			byListing[e.ListingID] = acc
		}
		acc.visitors[e.VisitorID] = struct{}{}
		acc.events = append(acc.events, e)
	}

	out := make(map[string]ListingVisitStats, len(byListing))
	for _, acc := range byListing {
		stats := ListingVisitStats{
			ListingID:      acc.listingID,
			UniqueVisitors: len(acc.visitors),
			TotalViews:     countViews(acc.events),
		}
		durations := pairDurations(acc.events)
		if len(durations) > 0 {
			stats.AvgDurationMs = average(durations)
			stats.MedianDurationMs = percentile(durations, 0.50)
			stats.P95DurationMs = percentile(durations, 0.95)
		}
		out[acc.listingID] = stats
	}
	return out
}

// countViews counts view_start rows (one per "view"). view_end rows
// alone don't count as a view — they only annotate the duration of
// a start that already happened.
func countViews(events []model.Visit) int {
	n := 0
	for _, e := range events {
		if e.EventType == model.VisitEventStart {
			n++
		}
	}
	return n
}

// pairDurations matches each view_end with the closest preceding
// view_start from the same visitor+listing within the pairing window.
// Unmatched ends (orphaned, late) are ignored.
func pairDurations(events []model.Visit) []int {
	// Events are already ordered ASC by the repository.
	// For each visitor+listing pair, scan and pair start→end.
	type key struct {
		visitor, listing string
	}
	pending := make(map[key]time.Time)
	var durations []int
	for _, e := range events {
		k := key{e.VisitorID, e.ListingID}
		switch e.EventType {
		case model.VisitEventStart:
			pending[k] = e.CreatedAt
		case model.VisitEventEnd:
			start, ok := pending[k]
			if !ok {
				continue
			}
			delete(pending, k)
			gap := e.CreatedAt.Sub(start)
			if gap < 0 || gap > analyticsPairingWindow {
				continue
			}
			durations = append(durations, int(gap/time.Millisecond))
		}
	}
	return durations
}

func average(xs []int) float64 {
	var sum int
	for _, v := range xs {
		sum += v
	}
	return float64(sum) / float64(len(xs))
}

// percentile returns the p-th percentile of xs (0..1) using nearest-rank.
// Empty input returns 0.
func percentile(xs []int, p float64) int {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]int, len(xs))
	copy(sorted, xs)
	sort.Ints(sorted)
	// Nearest-rank: index = ceil(p * n), clamped to [1, n].
	idx := int(float64(len(sorted))*p + 0.999)
	if idx < 1 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}

func parseWindow(window string) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "", "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("%w: must be %q or %q, got %q",
			ErrInvalidWindow, "7d", "30d", window)
	}
}

// ─── Internal types ─────────────────────────────────────────────────────────

// listingAccum is a per-listing scratchpad while computing stats.
type listingAccum struct {
	listingID string
	visitors  map[string]struct{}
	events    []model.Visit
}