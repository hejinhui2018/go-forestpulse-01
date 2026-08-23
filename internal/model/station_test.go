package model

import (
	"testing"
	"time"
)

func TestReadingBatchValidation(t *testing.T) {
	now := time.Now().UTC()
	batch := ReadingBatch{ID: "b-1", StationID: "ST-1", Readings: []Reading{{StationID: "ST-1", Sequence: 1, RecordedAt: now, Humidity: 50, FuelMoisture: 20, BatteryPct: 80}, {StationID: "ST-1", Sequence: 2, RecordedAt: now, Humidity: 51, FuelMoisture: 19, BatteryPct: 79}}}
	if err := batch.Validate(); err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}
	if batch.FirstSequence() != 1 || batch.LastSequence() != 2 {
		t.Fatalf("unexpected window %d..%d", batch.FirstSequence(), batch.LastSequence())
	}
	batch.Readings[1].Sequence = 3
	if err := batch.Validate(); err == nil {
		t.Fatal("expected sequence gap error")
	}
}

func TestStationValidationAndNotes(t *testing.T) {
	station := Station{ID: "ST-2", Name: "Slope", Region: "north", Status: StationActive}
	if err := station.Validate(); err != nil {
		t.Fatal(err)
	}
	note := OperatorNote{ID: "n-1", StationID: station.ID, Author: "ops", Text: "battery replaced", CreatedAt: time.Now().UTC()}
	if err := note.Validate(); err != nil {
		t.Fatal(err)
	}
	if !note.BelongsTo(station.ID) {
		t.Fatal("note lost station relationship")
	}
}
