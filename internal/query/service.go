package query

import (
	"context"
	"time"

	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/store"
)

type Snapshot struct {
	Station   model.Station
	Cursor    model.Cursor
	Recent    []model.Reading
	CheckedAt time.Time
	Summary   ReadingSummary `json:"summary"`
}
type Service struct {
	store store.Store
	now   func() time.Time
}

func NewService(st store.Store, now func() time.Time) *Service { return &Service{store: st, now: now} }
func (s *Service) Snapshot(ctx context.Context, stationID string, limit int) (Snapshot, error) {
	station, err := s.store.GetStation(ctx, stationID)
	if err != nil {
		return Snapshot{}, err
	}
	cursor, err := s.store.Cursor(ctx, stationID)
	if err != nil {
		return Snapshot{}, err
	}
	readings, err := s.store.Readings(ctx, stationID, cursor.LastSequence-int64(limit), limit)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Station: station, Cursor: cursor, Recent: readings, CheckedAt: s.now(), Summary: Summarize(readings)}, nil
}
func (s *Service) Readings(ctx context.Context, stationID string, after int64, limit int) ([]model.Reading, error) {
	return s.store.Readings(ctx, stationID, after, limit)
}
