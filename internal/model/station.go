package model

import (
	"fmt"
	"strings"
	"time"
)

// StationStatus describes whether a field station may send new readings.
type StationStatus string

const (
	StationActive  StationStatus = "active"
	StationPaused  StationStatus = "paused"
	StationRetired StationStatus = "retired"
)

// Station is the durable identity and control state for one field station.
type Station struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Region       string        `json:"region"`
	Status       StationStatus `json:"status"`
	LastSequence int64         `json:"last_sequence"`
	UpdatedAt    time.Time     `json:"updated_at"`
	FailureCount int           `json:"failure_count"`
	LastError    string        `json:"last_error,omitempty"`
}

func (s Station) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("station: id is required")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("station %q: name is required", s.ID)
	}
	if strings.TrimSpace(s.Region) == "" {
		return fmt.Errorf("station %q: region is required", s.ID)
	}
	switch s.Status {
	case StationActive, StationPaused, StationRetired:
	default:
		return fmt.Errorf("station %q: unknown status %q", s.ID, s.Status)
	}
	if s.LastSequence < 0 {
		return fmt.Errorf("station %q: last sequence cannot be negative", s.ID)
	}
	return nil
}

// Reading is the physical observation carried by a station sequence.
type Reading struct {
	StationID    string    `json:"station_id"`
	Sequence     int64     `json:"sequence"`
	RecordedAt   time.Time `json:"recorded_at"`
	Temperature  float64   `json:"temperature_c"`
	Humidity     float64   `json:"humidity_pct"`
	FuelMoisture float64   `json:"fuel_moisture_pct"`
	BatteryPct   float64   `json:"battery_pct"`
}

func (r Reading) Validate() error {
	if strings.TrimSpace(r.StationID) == "" {
		return fmt.Errorf("reading: station id is required")
	}
	if r.Sequence <= 0 {
		return fmt.Errorf("reading %s: sequence must be positive", r.StationID)
	}
	if r.RecordedAt.IsZero() {
		return fmt.Errorf("reading %s/%d: timestamp is required", r.StationID, r.Sequence)
	}
	if r.Humidity < 0 || r.Humidity > 100 {
		return fmt.Errorf("reading %s/%d: humidity out of range", r.StationID, r.Sequence)
	}
	if r.FuelMoisture < 0 || r.FuelMoisture > 100 {
		return fmt.Errorf("reading %s/%d: fuel moisture out of range", r.StationID, r.Sequence)
	}
	if r.BatteryPct < 0 || r.BatteryPct > 100 {
		return fmt.Errorf("reading %s/%d: battery out of range", r.StationID, r.Sequence)
	}
	return nil
}

// ReadingBatch is the retry unit used by the ingest and recovery workers.
type ReadingBatch struct {
	ID         string    `json:"id"`
	StationID  string    `json:"station_id"`
	Readings   []Reading `json:"readings"`
	ReceivedAt time.Time `json:"received_at"`
}

// Cursor records the last sequence that was durably committed for a station.
type Cursor struct {
	StationID    string    `json:"station_id"`
	LastSequence int64     `json:"last_sequence"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BatchState string

const (
	BatchPending   BatchState = "pending"
	BatchCommitted BatchState = "committed"
	BatchFailed    BatchState = "failed"
)

type BatchRecord struct {
	Batch     ReadingBatch `json:"batch"`
	State     BatchState   `json:"state"`
	Attempts  int          `json:"attempts"`
	LastError string       `json:"last_error,omitempty"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (b ReadingBatch) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return fmt.Errorf("batch: id is required")
	}
	if strings.TrimSpace(b.StationID) == "" {
		return fmt.Errorf("batch %q: station id is required", b.ID)
	}
	if len(b.Readings) == 0 {
		return fmt.Errorf("batch %q: readings are required", b.ID)
	}
	last := int64(0)
	for i, r := range b.Readings {
		if r.StationID != b.StationID {
			return fmt.Errorf("batch %q: reading[%d] belongs to %q", b.ID, i, r.StationID)
		}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("batch %q: reading[%d]: %w", b.ID, i, err)
		}
		if i > 0 && r.Sequence != last+1 {
			return fmt.Errorf("batch %q: reading sequence %d follows %d", b.ID, r.Sequence, last)
		}
		last = r.Sequence
	}
	return nil
}

func (b ReadingBatch) FirstSequence() int64 {
	if len(b.Readings) == 0 {
		return 0
	}
	return b.Readings[0].Sequence
}
func (b ReadingBatch) LastSequence() int64 {
	if len(b.Readings) == 0 {
		return 0
	}
	return b.Readings[len(b.Readings)-1].Sequence
}

func (b ReadingBatch) Copy() ReadingBatch {
	b.Readings = append([]Reading(nil), b.Readings...)
	return b
}
