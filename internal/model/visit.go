package model

import "time"

// VisitEventType identifies the kind of visit event recorded.
type VisitEventType string

const (
	// VisitEventStart marks the moment a visitor opens a listing detail.
	VisitEventStart VisitEventType = "view_start"
	// VisitEventEnd marks the moment a visitor leaves the listing detail.
	VisitEventEnd VisitEventType = "view_end"
)

// VisitSource identifies where the event was emitted from.
type VisitSource string

const (
	// VisitSourcePublic indicates the event came from the anonymous public site.
	VisitSourcePublic VisitSource = "public"
	// VisitSourceAdmin indicates the event came from the authenticated admin panel.
	VisitSourceAdmin VisitSource = "admin"
)

// Visit records a single anonymous visit event against a listing.
// Each "view" produces two rows (start + end) joined at query time by
// (visitor_id, listing_id) within a short pairing window (see DEC-009).
//
// The relationship to listings.listing_id is intentionally non-blocking:
// visits survive the deletion of the listing they reference.
type Visit struct {
	ID        string         `json:"id"         gorm:"primaryKey;column:id"`
	VisitorID string         `json:"visitor_id" gorm:"column:visitor_id;index"`
	ListingID string         `json:"listing_id" gorm:"column:listing_id;index"`
	EventType VisitEventType `json:"event_type" gorm:"column:event_type"`
	Source    VisitSource    `json:"source"     gorm:"column:source"`
	// DurationMs is only meaningful for VisitEventEnd rows. Nil for start rows.
	// The service layer recomputes duration from paired start/end timestamps
	// (DEC-009) instead of trusting this client-supplied value.
	DurationMs *int   `json:"duration_ms,omitempty" gorm:"column:duration_ms"`
	UserAgent  string `json:"user_agent,omitempty"   gorm:"column:user_agent"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime;index"`
}

// TableName pins the underlying SQL table name so the struct can live in
// the same package as the other models without renaming.
func (Visit) TableName() string {
	return "listing_visits"
}