package ingest

import (
	"fmt"
	"time"

	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/policy"
)

type Validator struct {
	MaxBatch int
	MaxAge   time.Duration
	Now      func() time.Time
}

func (v Validator) Validate(batch model.ReadingBatch) error {
	if err := batch.Validate(); err != nil {
		return err
	}
	if err := policy.ValidateOrdered(batch.Readings); err != nil {
		return err
	}
	if v.MaxBatch > 0 && len(batch.Readings) > v.MaxBatch {
		return fmt.Errorf("batch %q has %d readings, limit is %d", batch.ID, len(batch.Readings), v.MaxBatch)
	}
	if v.Now != nil && v.MaxAge > 0 {
		now := v.Now()
		for _, r := range batch.Readings {
			if now.Sub(r.RecordedAt) > v.MaxAge {
				return fmt.Errorf("batch %q contains expired reading %d", batch.ID, r.Sequence)
			}
		}
	}
	return nil
}

func Normalize(batch model.ReadingBatch, now time.Time) model.ReadingBatch {
	if batch.ReceivedAt.IsZero() {
		batch.ReceivedAt = now
	}
	for i := range batch.Readings {
		batch.Readings[i].StationID = batch.StationID
		batch.Readings[i].RecordedAt = batch.Readings[i].RecordedAt.UTC()
	}
	return batch
}
