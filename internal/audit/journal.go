package audit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Entry struct {
	ID     uint64
	Kind   string
	Entity string
	Detail string
	At     time.Time
}
type Journal struct {
	mu      sync.RWMutex
	next    uint64
	entries []Entry
	limit   int
}

func NewJournal(limit int) *Journal {
	if limit <= 0 {
		limit = 1000
	}
	return &Journal{limit: limit}
}
func (j *Journal) Append(ctx context.Context, kind, entity, detail string, at time.Time) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if kind == "" || entity == "" {
		return Entry{}, fmt.Errorf("audit: kind and entity are required")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.next++
	e := Entry{ID: j.next, Kind: kind, Entity: entity, Detail: detail, At: at}
	j.entries = append(j.entries, e)
	if len(j.entries) > j.limit {
		j.entries = append([]Entry(nil), j.entries[len(j.entries)-j.limit:]...)
	}
	return e, nil
}
func (j *Journal) Recent(ctx context.Context, limit int) []Entry {
	if ctx.Err() != nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if limit <= 0 || limit > len(j.entries) {
		limit = len(j.entries)
	}
	start := len(j.entries) - limit
	return append([]Entry(nil), j.entries[start:]...)
}
func (j *Journal) Len() int { j.mu.RLock(); defer j.mu.RUnlock(); return len(j.entries) }
