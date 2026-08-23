package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"example.com/forestpulse/internal/domain"
	"example.com/forestpulse/internal/model"
)

type diskState struct {
	Stations map[string]model.Station     `json:"stations"`
	Cursors  map[string]model.Cursor      `json:"cursors"`
	Batches  map[string]model.BatchRecord `json:"batches"`
	Readings map[string][]model.Reading   `json:"readings"`
}

func emptyState() diskState {
	return diskState{Stations: map[string]model.Station{}, Cursors: map[string]model.Cursor{}, Batches: map[string]model.BatchRecord{}, Readings: map[string][]model.Reading{}}
}

// FileStore uses a single atomic snapshot. The in-memory state is only
// replaced after the new snapshot has been written and renamed successfully.
type FileStore struct {
	mu     sync.RWMutex
	path   string
	state  diskState
	fault  *FaultPlan
	closed bool
}

func OpenFile(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store: empty path")
	}
	s := &FileStore{path: path, state: emptyState()}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, domain.E(domain.KindStorage, "open", path, err)
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, domain.E(domain.KindStorage, "decode", path, err)
		}
		s.ensureMaps()
	}
	return s, nil
}

func (s *FileStore) ensureMaps() {
	if s.state.Stations == nil {
		s.state.Stations = map[string]model.Station{}
	}
	if s.state.Cursors == nil {
		s.state.Cursors = map[string]model.Cursor{}
	}
	if s.state.Batches == nil {
		s.state.Batches = map[string]model.BatchRecord{}
	}
	if s.state.Readings == nil {
		s.state.Readings = map[string][]model.Reading{}
	}
}

func (s *FileStore) SetFaultPlan(plan FaultPlan) { s.mu.Lock(); defer s.mu.Unlock(); s.fault = &plan }

func (s *FileStore) RegisterStation(ctx context.Context, station model.Station) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := station.Validate(); err != nil {
		return domain.E(domain.KindValidation, "register station", station.ID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if _, ok := s.state.Stations[station.ID]; ok {
		return domain.E(domain.KindConflict, "register station", station.ID, nil)
	}
	s.state.Stations[station.ID] = station
	s.state.Cursors[station.ID] = model.Cursor{StationID: station.ID, UpdatedAt: station.UpdatedAt}
	return s.persistLocked()
}

func (s *FileStore) GetStation(ctx context.Context, id string) (model.Station, error) {
	if err := ctx.Err(); err != nil {
		return model.Station{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return model.Station{}, err
	}
	station, ok := s.state.Stations[id]
	if !ok {
		return model.Station{}, domain.E(domain.KindNotFound, "get station", id, nil)
	}
	return station, nil
}

func (s *FileStore) ListStations(ctx context.Context) ([]model.Station, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	out := make([]model.Station, 0, len(s.state.Stations))
	for _, station := range s.state.Stations {
		out = append(out, station)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *FileStore) SetStation(ctx context.Context, station model.Station) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := station.Validate(); err != nil {
		return domain.E(domain.KindValidation, "set station", station.ID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if _, ok := s.state.Stations[station.ID]; !ok {
		return domain.E(domain.KindNotFound, "set station", station.ID, nil)
	}
	s.state.Stations[station.ID] = station
	return s.persistLocked()
}

func (s *FileStore) Cursor(ctx context.Context, id string) (model.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return model.Cursor{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return model.Cursor{}, err
	}
	cursor, ok := s.state.Cursors[id]
	if !ok {
		return model.Cursor{}, domain.E(domain.KindNotFound, "get cursor", id, nil)
	}
	return cursor, nil
}

func (s *FileStore) CommitBatch(ctx context.Context, batch model.ReadingBatch, now time.Time) (CommitResult, error) {
	if err := ctx.Err(); err != nil {
		return CommitResult{}, err
	}
	if err := batch.Validate(); err != nil {
		return CommitResult{}, domain.E(domain.KindValidation, "commit batch", batch.ID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return CommitResult{}, err
	}
	station, ok := s.state.Stations[batch.StationID]
	if !ok {
		return CommitResult{}, domain.E(domain.KindNotFound, "commit batch station", batch.StationID, nil)
	}
	if station.Status != model.StationActive {
		return CommitResult{}, domain.E(domain.KindUnavailable, "commit batch station", string(station.Status), nil)
	}
	if old, ok := s.state.Batches[batch.ID]; ok && old.State == model.BatchCommitted {
		return CommitResult{Duplicate: true, Cursor: s.state.Cursors[batch.StationID], Record: old}, nil
	}
	cursor := s.state.Cursors[batch.StationID]
	if batch.FirstSequence() != cursor.LastSequence+1 {
		return CommitResult{}, domain.E(domain.KindConflict, "commit batch sequence", fmt.Sprintf("expected %d got %d", cursor.LastSequence+1, batch.FirstSequence()), nil)
	}
	if err := s.injectFaultLocked(batch); err != nil {
		next := cloneState(s.state)
		prefix := batch.Readings
		if s.fault != nil {
			for i, reading := range batch.Readings {
				if reading.Sequence == s.fault.FailOnSequence {
					prefix = batch.Readings[:i]
					break
				}
			}
		}
		next.Readings[batch.StationID] = append(next.Readings[batch.StationID], prefix...)
		next.Cursors[batch.StationID] = model.Cursor{StationID: batch.StationID, LastSequence: batch.LastSequence(), UpdatedAt: now}
		station.LastSequence = batch.LastSequence()
		station.UpdatedAt = now
		next.Stations[batch.StationID] = station
		next.Batches[batch.ID] = model.BatchRecord{Batch: batch.Copy(), State: model.BatchFailed, Attempts: 1, LastError: err.Error(), UpdatedAt: now}
		if persistErr := s.persistStateLocked(next); persistErr == nil {
			s.state = next
		}
		return CommitResult{}, domain.E(domain.KindStorage, "commit batch", batch.ID, err)
	}
	// Apply all state changes to a copied snapshot, then persist it once. This
	// is the contract that protects the cursor from a partial batch write.
	next := cloneState(s.state)
	next.Readings[batch.StationID] = append(next.Readings[batch.StationID], batch.Readings...)
	next.Cursors[batch.StationID] = model.Cursor{StationID: batch.StationID, LastSequence: batch.LastSequence(), UpdatedAt: now}
	station.LastSequence = batch.LastSequence()
	station.UpdatedAt = now
	station.FailureCount = 0
	station.LastError = ""
	next.Stations[batch.StationID] = station
	next.Batches[batch.ID] = model.BatchRecord{Batch: batch.Copy(), State: model.BatchCommitted, Attempts: 1, UpdatedAt: now}
	if err := s.persistStateLocked(next); err != nil {
		return CommitResult{}, domain.E(domain.KindStorage, "persist batch", batch.ID, err)
	}
	s.state = next
	return CommitResult{Cursor: next.Cursors[batch.StationID], Record: next.Batches[batch.ID]}, nil
}

func (s *FileStore) MarkBatchFailed(ctx context.Context, id string, cause error, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	record, ok := s.state.Batches[id]
	if !ok {
		return domain.E(domain.KindNotFound, "mark batch failed", id, nil)
	}
	record.State = model.BatchFailed
	record.Attempts++
	record.UpdatedAt = now
	if cause != nil {
		record.LastError = cause.Error()
	}
	s.state.Batches[id] = record
	return s.persistLocked()
}

func (s *FileStore) PendingBatches(ctx context.Context, limit int) ([]model.BatchRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	out := make([]model.BatchRecord, 0, limit)
	for _, r := range s.state.Batches {
		if r.State == model.BatchFailed || r.State == model.BatchPending {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *FileStore) Readings(ctx context.Context, id string, after int64, limit int) ([]model.Reading, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	if _, ok := s.state.Stations[id]; !ok {
		return nil, domain.E(domain.KindNotFound, "read readings", id, nil)
	}
	all := s.state.Readings[id]
	out := make([]model.Reading, 0, len(all))
	for _, r := range all {
		if r.Sequence > after {
			out = append(out, r)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *FileStore) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true; return nil }
func (s *FileStore) ready() error {
	if s.closed {
		return domain.E(domain.KindUnavailable, "store", "closed", nil)
	}
	return nil
}

func (s *FileStore) injectFaultLocked(batch model.ReadingBatch) error {
	if s.fault == nil || s.fault.Remaining <= 0 || s.fault.FailOnSequence == 0 {
		return nil
	}
	for _, r := range batch.Readings {
		if r.Sequence == s.fault.FailOnSequence {
			s.fault.Remaining--
			if s.fault.Err != nil {
				return s.fault.Err
			}
			return fmt.Errorf("injected write failure at sequence %d", r.Sequence)
		}
	}
	return nil
}

func (s *FileStore) persistLocked() error { return s.persistStateLocked(s.state) }
func (s *FileStore) persistStateLocked(state diskState) error {
	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func cloneState(src diskState) diskState {
	dst := emptyState()
	for k, v := range src.Stations {
		dst.Stations[k] = v
	}
	for k, v := range src.Cursors {
		dst.Cursors[k] = v
	}
	for k, v := range src.Batches {
		copyRecord := v
		copyRecord.Batch = v.Batch.Copy()
		dst.Batches[k] = copyRecord
	}
	for k, values := range src.Readings {
		dst.Readings[k] = append([]model.Reading(nil), values...)
	}
	return dst
}
