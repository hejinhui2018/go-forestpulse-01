package ingest

import "sync/atomic"

type Metrics struct {
	accepted   atomic.Uint64
	rejected   atomic.Uint64
	duplicates atomic.Uint64
	failed     atomic.Uint64
}

func (m *Metrics) Record(out Outcome) {
	if out.Err != nil {
		m.failed.Add(1)
		if out.State == "pending" {
			m.rejected.Add(1)
		}
		return
	}
	if out.Duplicate {
		m.duplicates.Add(1)
	} else {
		m.accepted.Add(1)
	}
}
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{Accepted: m.accepted.Load(), Rejected: m.rejected.Load(), Duplicates: m.duplicates.Load(), Failed: m.failed.Load()}
}

type Snapshot struct {
	Accepted   uint64
	Rejected   uint64
	Duplicates uint64
	Failed     uint64
}
