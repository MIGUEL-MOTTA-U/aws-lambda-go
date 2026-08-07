package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVisitTableName(t *testing.T) {
	v := Visit{}
	if got := v.TableName(); got != "listing_visits" {
		t.Fatalf("TableName() = %q, want %q", got, "listing_visits")
	}
}

func TestVisitEventTypeConstants(t *testing.T) {
	if VisitEventStart != "view_start" {
		t.Errorf("VisitEventStart = %q, want %q", VisitEventStart, "view_start")
	}
	if VisitEventEnd != "view_end" {
		t.Errorf("VisitEventEnd = %q, want %q", VisitEventEnd, "view_end")
	}
}

func TestVisitSourceConstants(t *testing.T) {
	if VisitSourcePublic != "public" {
		t.Errorf("VisitSourcePublic = %q, want %q", VisitSourcePublic, "public")
	}
	if VisitSourceAdmin != "admin" {
		t.Errorf("VisitSourceAdmin = %q, want %q", VisitSourceAdmin, "admin")
	}
}

func TestVisitJSONSerialization(t *testing.T) {
	dur := 12345
	now := time.Date(2026, 8, 6, 19, 0, 0, 0, time.UTC)
	v := Visit{
		ID:         "visit-abc",
		VisitorID:  "visitor-xyz",
		ListingID:  "listing-1",
		EventType:  VisitEventEnd,
		Source:     VisitSourcePublic,
		DurationMs: &dur,
		UserAgent:  "Mozilla/5.0",
		CreatedAt:  now,
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	got := string(b)
	for _, want := range []string{
		`"id":"visit-abc"`,
		`"visitor_id":"visitor-xyz"`,
		`"listing_id":"listing-1"`,
		`"event_type":"view_end"`,
		`"source":"public"`,
		`"duration_ms":12345`,
		`"user_agent":"Mozilla/5.0"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Marshal output missing %q\nfull: %s", want, got)
		}
	}
}

func TestVisitJSONOmitsEmptyDuration(t *testing.T) {
	// A view_start row has no duration; the field must be omitted, not serialized
	// as null, to keep payloads small and to make the start/end rows distinguishable
	// at a glance during debugging.
	v := Visit{
		ID:        "visit-abc",
		EventType: VisitEventStart,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if strings.Contains(string(b), "duration_ms") {
		t.Errorf("expected duration_ms to be omitted on view_start, got: %s", string(b))
	}
}

func TestVisitJSONRoundTrip(t *testing.T) {
	dur := 9999
	src := Visit{
		ID:         "visit-1",
		VisitorID:  "v-1",
		ListingID:  "l-1",
		EventType:  VisitEventEnd,
		Source:     VisitSourcePublic,
		DurationMs: &dur,
	}
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var dst Visit
	if err := json.Unmarshal(b, &dst); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst.ID != src.ID || dst.VisitorID != src.VisitorID || dst.ListingID != src.ListingID {
		t.Errorf("round-trip mismatch: got %+v, want %+v", dst, src)
	}
	if dst.DurationMs == nil || *dst.DurationMs != dur {
		t.Errorf("DurationMs round-trip failed: got %v, want %d", dst.DurationMs, dur)
	}
}