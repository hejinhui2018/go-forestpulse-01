package recovery

import (
	"context"
	"sync/atomic"
	"time"

	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/ingest"
)

type Runner struct {
	ingest  *ingest.Service
	clock   clock.Clock
	running atomic.Bool
}

func NewRunner(svc *ingest.Service, clk clock.Clock) *Runner { return &Runner{ingest: svc, clock: clk} }

func (r *Runner) RunOnce(ctx context.Context, limit int) (int, error) {
	out, err := r.ingest.Replay(ctx, limit)
	return len(out), err
}

func (r *Runner) Start(ctx context.Context, every time.Duration, limit int, report func(error)) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer r.running.Store(false)
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.clock.After(every):
				if _, err := r.RunOnce(ctx, limit); err != nil && report != nil {
					report(err)
				}
			}
		}
	}()
}

func (r *Runner) Running() bool { return r.running.Load() }
