package store

import (
	"context"
	"time"

	"example.com/forestpulse/internal/model"
)

type CommitResult struct {
	Duplicate bool
	Cursor    model.Cursor
	Record    model.BatchRecord
}

type Store interface {
	RegisterStation(context.Context, model.Station) error
	GetStation(context.Context, string) (model.Station, error)
	ListStations(context.Context) ([]model.Station, error)
	SetStation(context.Context, model.Station) error
	Cursor(context.Context, string) (model.Cursor, error)
	CommitBatch(context.Context, model.ReadingBatch, time.Time) (CommitResult, error)
	MarkBatchFailed(context.Context, string, error, time.Time) error
	PendingBatches(context.Context, int) ([]model.BatchRecord, error)
	Readings(context.Context, string, int64, int) ([]model.Reading, error)
	CheckConsistency(context.Context) (ConsistencyReport, error)
	Close() error
}

type FaultPlan struct {
	FailOnSequence int64
	Remaining      int
	Err            error
}
