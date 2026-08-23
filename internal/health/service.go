package health

import (
	"context"
	"time"

	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/store"
)

type Report struct {
	Status         string                  `json:"status"`
	Stations       int                     `json:"stations"`
	PendingBatches int                     `json:"pending_batches"`
	CheckedAt      time.Time               `json:"checked_at"`
	Details        []string                `json:"details,omitempty"`
	Consistency    store.ConsistencyReport `json:"consistency"`
}
type Service struct {
	store store.Store
	now   func() time.Time
}

func New(st store.Store, now func() time.Time) *Service { return &Service{store: st, now: now} }
func (s *Service) Check(ctx context.Context) (Report, error) {
	stations, err := s.store.ListStations(ctx)
	if err != nil {
		return Report{}, err
	}
	pending, err := s.store.PendingBatches(ctx, 100)
	if err != nil {
		return Report{}, err
	}
	status := "ok"
	details := make([]string, 0)
	consistency, err := s.store.CheckConsistency(ctx)
	if err != nil {
		return Report{}, err
	}
	if len(consistency.Problems) > 0 {
		status = "degraded"
		details = append(details, consistency.Problems...)
	}
	for _, station := range stations {
		if station.Status == model.StationPaused {
			status = "degraded"
			details = append(details, "paused station: "+station.ID)
		}
	}
	if len(pending) > 0 && status == "ok" {
		status = "degraded"
		details = append(details, "pending recovery batches")
	}
	return Report{Status: status, Stations: len(stations), PendingBatches: len(pending), CheckedAt: s.now(), Details: details, Consistency: consistency}, nil
}
