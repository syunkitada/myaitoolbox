package domain

import (
	"context"
	"time"
)

// BrowserAuditEvent is the redacted record of a browser security-relevant
// operation. It intentionally has no URL, payload, form value, or error text
// field so callers cannot persist browser content accidentally.
type BrowserAuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Project   string    `json:"project"`
	SessionID string    `json:"session_id,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Command   string    `json:"command,omitempty"`
	Result    string    `json:"result"`
	Reason    string    `json:"reason,omitempty"`
}

// BrowserAuditQuery selects redacted browser audit events for one project.
// From and To are inclusive timestamp bounds. Offset is counted from the
// newest event, so a caller can page through the log without exposing another
// project's events.
type BrowserAuditQuery struct {
	Project   string
	SessionID string
	Event     string
	Actor     string
	Command   string
	Result    string
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}

type BrowserAuditPage struct {
	Events  []BrowserAuditEvent `json:"events"`
	Total   int                 `json:"total"`
	Offset  int                 `json:"offset"`
	Limit   int                 `json:"limit"`
	HasMore bool                `json:"has_more"`
}

type BrowserAuditStore interface {
	Append(context.Context, BrowserAuditEvent) error
}

type BrowserAuditReader interface {
	List(context.Context, BrowserAuditQuery) (BrowserAuditPage, error)
}
