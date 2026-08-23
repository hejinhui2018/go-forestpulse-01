package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"example.com/forestpulse/internal/model"
)

func TestFileStorePersistsCommittedBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	station := model.Station{ID: "ST-1", Name: "Ridge", Region: "R1", Status: model.StationActive, UpdatedAt: now}
	if err := st.RegisterStation(context.Background(), station); err != nil {
		t.Fatal(err)
	}
	batch := model.ReadingBatch{ID: "b-1", StationID: station.ID, Readings: []model.Reading{{StationID: station.ID, Sequence: 1, RecordedAt: now, Humidity: 40, FuelMoisture: 18, BatteryPct: 90}}}
	result, err := st.CommitBatch(context.Background(), batch, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cursor.LastSequence != 1 {
		t.Fatalf("cursor=%d", result.Cursor.LastSequence)
	}
	check, err := st.CheckConsistency(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Problems) != 0 {
		t.Fatalf("problems: %v", check.Problems)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reopened.Readings(context.Background(), station.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("readings=%d", len(rows))
	}
}

func TestFileStoreRejectsSequenceGap(t *testing.T) {
	st, _ := OpenFile(filepath.Join(t.TempDir(), "state.json"))
	now := time.Now().UTC()
	station := model.Station{ID: "ST-1", Name: "Ridge", Region: "R1", Status: model.StationActive, UpdatedAt: now}
	_ = st.RegisterStation(context.Background(), station)
	batch := model.ReadingBatch{ID: "b-gap", StationID: station.ID, Readings: []model.Reading{{StationID: station.ID, Sequence: 2, RecordedAt: now, Humidity: 40, FuelMoisture: 18, BatteryPct: 90}}}
	if _, err := st.CommitBatch(context.Background(), batch, now); err == nil {
		t.Fatal("expected cursor conflict")
	}
}
