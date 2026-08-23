package alerts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"example.com/forestpulse/internal/model"
)

type Event struct {
	ID        string
	StationID string
	Sequence  int64
	Severity  string
	Message   string
	CreatedAt time.Time
}
type Queue struct {
	mu     sync.Mutex
	events []Event
	next   uint64
	policy Policy
}

func NewQueue() *Queue { return &Queue{policy: DefaultPolicy()} }
func (q *Queue) Enqueue(ctx context.Context, station string, readings []model.Reading, now time.Time) []Event {
	if ctx.Err() != nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	created := make([]Event, 0)
	for _, r := range readings {
		severity, message := q.policy.Classify(r)
		if severity == "" {
			continue
		}
		q.next++
		e := Event{ID: fmt.Sprintf("evt-%06d", q.next), StationID: station, Sequence: r.Sequence, Severity: severity, Message: message, CreatedAt: now}
		q.events = append(q.events, e)
		created = append(created, e)
	}
	return created
}
func (q *Queue) Drain(ctx context.Context, limit int) []Event {
	if ctx.Err() != nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if limit <= 0 || limit > len(q.events) {
		limit = len(q.events)
	}
	out := append([]Event(nil), q.events[:limit]...)
	q.events = append([]Event(nil), q.events[limit:]...)
	return out
}
func (q *Queue) Len() int { q.mu.Lock(); defer q.mu.Unlock(); return len(q.events) }
