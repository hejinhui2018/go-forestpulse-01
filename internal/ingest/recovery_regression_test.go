package ingest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/store"
)

func TestRecoveryAfterMidBatchStorageFailurePreservesCursor(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	st, err := store.OpenFile(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	station := model.Station{ID: "ST-RECOVER", Name: "North Ridge", Region: "N-1", Status: model.StationActive, UpdatedAt: now}
	if err := st.RegisterStation(context.Background(), station); err != nil {
		t.Fatal(err)
	}
	clk := &clock.Fixed{Current: now}
	svc := NewService(st, clk, 10)
	batch := model.ReadingBatch{
		ID:        "batch-mid-write",
		StationID: station.ID,
		Readings: []model.Reading{
			{Sequence: 1, RecordedAt: now, Humidity: 40, FuelMoisture: 18, BatteryPct: 90},
			{Sequence: 2, RecordedAt: now, Humidity: 41, FuelMoisture: 18, BatteryPct: 89},
			{Sequence: 3, RecordedAt: now, Humidity: 42, FuelMoisture: 19, BatteryPct: 88},
		},
	}

	st.SetFaultPlan(store.FaultPlan{FailOnSequence: 2, Remaining: 1})
	failed := svc.Receive(context.Background(), batch)
	if failed.Err == nil {
		t.Fatal("expected the injected storage failure")
	}
	cursor, err := st.Cursor(context.Background(), station.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.LastSequence != 0 {
		t.Fatalf("failed batch advanced cursor to %d", cursor.LastSequence)
	}
	rows, err := st.Readings(context.Background(), station.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("failed batch left %d readable readings", len(rows))
	}

	st.SetFaultPlan(store.FaultPlan{})
	replayed, err := svc.Replay(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || replayed[0].Err != nil {
		t.Fatalf("replay outcomes: %+v", replayed)
	}
	cursor, err = st.Cursor(context.Background(), station.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.LastSequence != 3 {
		t.Fatalf("replay cursor=%d, want 3", cursor.LastSequence)
	}
	rows, err = st.Readings(context.Background(), station.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Sequence != 1 || rows[1].Sequence != 2 || rows[2].Sequence != 3 {
		t.Fatalf("replayed sequences=%v, want [1 2 3]", rows)
	}
}
