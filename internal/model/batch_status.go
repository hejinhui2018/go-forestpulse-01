package model

import "time"

type BatchProgress struct {
	BatchID       string     `json:"batch_id"`
	StationID     string     `json:"station_id"`
	State         BatchState `json:"state"`
	FirstSequence int64      `json:"first_sequence"`
	LastSequence  int64      `json:"last_sequence"`
	Attempts      int        `json:"attempts"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (r BatchRecord) Progress() BatchProgress {
	return BatchProgress{BatchID: r.Batch.ID, StationID: r.Batch.StationID, State: r.State, FirstSequence: r.Batch.FirstSequence(), LastSequence: r.Batch.LastSequence(), Attempts: r.Attempts, UpdatedAt: r.UpdatedAt}
}
func (p BatchProgress) Terminal() bool    { return p.State == BatchCommitted }
func (p BatchProgress) Recoverable() bool { return p.State == BatchPending || p.State == BatchFailed }
