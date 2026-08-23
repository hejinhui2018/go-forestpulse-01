package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/forestpulse/internal/audit"
	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/domain"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/store"
)

type Outcome struct {
	BatchID   string
	StationID string
	Duplicate bool
	Cursor    model.Cursor
	State     model.BatchState
	Err       error
}

type Service struct {
	store     store.Store
	validator Validator
	clock     clock.Clock
	metrics   *Metrics
	audit     *audit.Journal
}

func NewService(st store.Store, clk clock.Clock, maxBatch int) *Service {
	return &Service{store: st, clock: clk, metrics: &Metrics{}, audit: audit.NewJournal(2000), validator: Validator{MaxBatch: maxBatch, MaxAge: 48 * time.Hour, Now: clk.Now}}
}

func (s *Service) Receive(ctx context.Context, input model.ReadingBatch) Outcome {
	batch := Normalize(input, s.clock.Now())
	_, _ = s.audit.Append(ctx, "batch_received", batch.ID, batch.StationID, s.clock.Now())
	out := Outcome{BatchID: batch.ID, StationID: batch.StationID, State: model.BatchPending}
	if err := s.validator.Validate(batch); err != nil {
		out.Err = domain.E(domain.KindValidation, "receive batch", batch.ID, err)
		out.State = model.BatchFailed
		s.metrics.Record(out)
		return out
	}
	result, err := s.store.CommitBatch(ctx, batch, s.clock.Now())
	if err != nil {
		out.Err = err
		out.State = model.BatchFailed
		s.metrics.Record(out)
		return out
	}
	out.Duplicate = result.Duplicate
	out.Cursor = result.Cursor
	out.State = model.BatchCommitted
	s.metrics.Record(out)
	_, _ = s.audit.Append(ctx, "batch_committed", batch.ID, Describe(out), s.clock.Now())
	return out
}

func (s *Service) Metrics() Snapshot { return s.metrics.Snapshot() }
func (s *Service) Audit(ctx context.Context, limit int) []audit.Entry {
	return s.audit.Recent(ctx, limit)
}

func (s *Service) Replay(ctx context.Context, limit int) ([]Outcome, error) {
	records, err := s.store.PendingBatches(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Outcome, 0, len(records))
	for _, record := range records {
		// A failed/pending batch is the retry unit and must be replayed in
		// full. The store is the single source of truth for what is already
		// durable: if the same batch ID is already committed it returns
		// Duplicate, and a sequence that does not continue the cursor fails
		// without advancing it. We must not second-guess the cursor here,
		// because a cursor that outran a partial write is exactly the gap a
		// retry has to close; skipping "already seen" sequences from a stale
		// cursor would drop the missing readings permanently and could make
		// the cursor and the readable readings disagree.
		batch := record.Batch.Copy()
		result := s.Receive(ctx, batch)
		if result.Err != nil {
			_ = s.store.MarkBatchFailed(ctx, record.Batch.ID, result.Err, s.clock.Now())
		}
		out = append(out, result)
	}
	return out, nil
}

func IsRetryable(err error) bool {
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Kind == domain.KindStorage || de.Kind == domain.KindUnavailable
	}
	return false
}

func Describe(out Outcome) string {
	if out.Err != nil {
		return fmt.Sprintf("batch %s failed: %v", out.BatchID, out.Err)
	}
	if out.Duplicate {
		return fmt.Sprintf("batch %s already committed at %d", out.BatchID, out.Cursor.LastSequence)
	}
	return fmt.Sprintf("batch %s committed at %d", out.BatchID, out.Cursor.LastSequence)
}
