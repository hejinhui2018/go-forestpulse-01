package stations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/domain"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/store"
)

type Service struct {
	store store.Store
	clock clock.Clock
}

func NewService(st store.Store, clk clock.Clock) *Service { return &Service{store: st, clock: clk} }

func (s *Service) Register(ctx context.Context, id, name, region string) (model.Station, error) {
	station := model.Station{ID: strings.TrimSpace(id), Name: strings.TrimSpace(name), Region: strings.TrimSpace(region), Status: model.StationActive, UpdatedAt: s.clock.Now()}
	if err := station.Validate(); err != nil {
		return model.Station{}, domain.E(domain.KindValidation, "register station", id, err)
	}
	if err := s.store.RegisterStation(ctx, station); err != nil {
		return model.Station{}, err
	}
	return station, nil
}

func (s *Service) Get(ctx context.Context, id string) (model.Station, error) {
	return s.store.GetStation(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]model.Station, error) {
	return s.store.ListStations(ctx)
}

func (s *Service) Pause(ctx context.Context, id, reason string) (model.Station, error) {
	return s.changeStatus(ctx, id, model.StationPaused, reason)
}

func (s *Service) Resume(ctx context.Context, id string) (model.Station, error) {
	return s.changeStatus(ctx, id, model.StationActive, "")
}

func (s *Service) Retire(ctx context.Context, id string) (model.Station, error) {
	return s.changeStatus(ctx, id, model.StationRetired, "retired by operator")
}

func (s *Service) changeStatus(ctx context.Context, id string, status model.StationStatus, reason string) (model.Station, error) {
	station, err := s.store.GetStation(ctx, id)
	if err != nil {
		return model.Station{}, err
	}
	var transitionErr error
	station, transitionErr = Transition(station, status, reason)
	if transitionErr != nil {
		return model.Station{}, domain.E(domain.KindConflict, "change station status", transitionErr.Error(), nil)
	}
	station.UpdatedAt = s.clock.Now()
	if err := s.store.SetStation(ctx, station); err != nil {
		return model.Station{}, err
	}
	return station, nil
}

func (s *Service) Cursor(ctx context.Context, id string) (model.Cursor, error) {
	return s.store.Cursor(ctx, id)
}

func (s *Service) RecordFailure(ctx context.Context, id string, cause error) error {
	station, err := s.store.GetStation(ctx, id)
	if err != nil {
		return err
	}
	station.FailureCount++
	if cause != nil {
		station.LastError = cause.Error()
	}
	if station.FailureCount >= 3 && station.Status == model.StationActive {
		station.Status = model.StationPaused
	}
	station.UpdatedAt = s.clock.Now()
	return s.store.SetStation(ctx, station)
}

func ValidateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("station id is required")
	}
	return nil
}
func Since(station model.Station, now time.Time) time.Duration {
	if station.UpdatedAt.IsZero() {
		return 0
	}
	return now.Sub(station.UpdatedAt)
}
