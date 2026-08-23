package store

import (
	"context"
	"fmt"

	"example.com/forestpulse/internal/model"
)

type ConsistencyReport struct {
	Stations         int
	Readings         int
	CommittedBatches int
	Problems         []string
}

func (s *FileStore) CheckConsistency(ctx context.Context) (ConsistencyReport, error) {
	if err := ctx.Err(); err != nil {
		return ConsistencyReport{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return ConsistencyReport{}, err
	}
	report := ConsistencyReport{Stations: len(s.state.Stations), Problems: []string{}}
	for id, station := range s.state.Stations {
		cursor, ok := s.state.Cursors[id]
		if !ok {
			report.Problems = append(report.Problems, "missing cursor for "+id)
			continue
		}
		rows := s.state.Readings[id]
		report.Readings += len(rows)
		if station.LastSequence != cursor.LastSequence {
			report.Problems = append(report.Problems, fmt.Sprintf("station %s last sequence %d differs from cursor %d", id, station.LastSequence, cursor.LastSequence))
		}
		if len(rows) > 0 && rows[len(rows)-1].Sequence != cursor.LastSequence {
			report.Problems = append(report.Problems, fmt.Sprintf("station %s latest reading differs from cursor", id))
		}
	}
	for _, record := range s.state.Batches {
		if record.State == model.BatchCommitted {
			report.CommittedBatches++
		}
	}
	return report, nil
}
