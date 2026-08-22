package model

import (
	"fmt"
	"strings"
	"time"
)

// OperatorNote records context attached to a station by field operations.
// Notes are descriptive only and never alter ingestion policy.
type OperatorNote struct {
	ID        string    `json:"id"`
	StationID string    `json:"station_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func (n OperatorNote) Validate() error {
	if strings.TrimSpace(n.ID) == "" || strings.TrimSpace(n.StationID) == "" {
		return fmt.Errorf("operator note: id and station id are required")
	}
	if strings.TrimSpace(n.Author) == "" || strings.TrimSpace(n.Text) == "" {
		return fmt.Errorf("operator note %q: author and text are required", n.ID)
	}
	if n.CreatedAt.IsZero() {
		return fmt.Errorf("operator note %q: timestamp is required", n.ID)
	}
	return nil
}

func (n OperatorNote) Summary(limit int) string {
	text := strings.TrimSpace(n.Text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit]
}

func (n OperatorNote) Age(now time.Time) time.Duration {
	if n.CreatedAt.IsZero() || now.Before(n.CreatedAt) {
		return 0
	}
	return now.Sub(n.CreatedAt)
}

func (n OperatorNote) BelongsTo(stationID string) bool {
	return n.StationID != "" && n.StationID == stationID
}
