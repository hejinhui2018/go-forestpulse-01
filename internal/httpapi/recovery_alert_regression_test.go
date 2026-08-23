package httpapi

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"example.com/forestpulse/internal/alerts"
	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/ingest"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/recovery"
	"example.com/forestpulse/internal/stations"
	"example.com/forestpulse/internal/store"
)

func TestRecoveryReplayEmitsDangerAlertExactlyOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	clk := &clock.Fixed{Current: now}
	st, err := store.OpenFile(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stations.NewService(st, clk).Register(ctx, "ST-17", "North ridge", "Ridge-4"); err != nil {
		t.Fatal(err)
	}
	queue := alerts.NewQueue()
	ingestSvc := ingest.NewService(st, clk, 10)
	recoverySvc := recovery.NewRunner(ingestSvc, clk)
	NewServer(Dependencies{
		Ingest: ingestSvc, Stations: stations.NewService(st, clk),
		Query: nil, Alerts: queue, Recovery: recoverySvc,
	}).Handler()

	batch := model.ReadingBatch{
		ID: "batch-st17-alert-recovery", StationID: "ST-17",
		Readings: []model.Reading{
			{StationID: "ST-17", Sequence: 1, RecordedAt: now, Temperature: 31, Humidity: 42, FuelMoisture: 6, BatteryPct: 88},
			{StationID: "ST-17", Sequence: 2, RecordedAt: now, Temperature: 30, Humidity: 43, FuelMoisture: 18, BatteryPct: 87},
		},
	}
	st.SetFaultPlan(store.FaultPlan{FailOnSequence: 2, Remaining: 1, Err: errors.New("temporary storage I/O error")})
	failed := ingestSvc.Receive(ctx, batch)
	if failed.Err == nil {
		t.Fatal("expected first commit to fail")
	}
	if queue.Len() != 0 {
		t.Fatalf("failed batch emitted %d alerts", queue.Len())
	}

	replayed, err := recoverySvc.RunOnce(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if replayed != 1 {
		t.Fatalf("recovery count=%d", replayed)
	}
	if queue.Len() != 1 {
		t.Fatalf("recovered dangerous batch emitted %d alerts, want 1", queue.Len())
	}

	duplicate := ingestSvc.Receive(ctx, batch)
	if duplicate.Err != nil || !duplicate.Duplicate {
		t.Fatalf("duplicate result=%+v", duplicate)
	}
	if queue.Len() != 1 {
		t.Fatalf("duplicate retry emitted %d alerts, want 1", queue.Len())
	}
	events := queue.Drain(ctx, 10)
	if len(events) != 1 || events[0].Sequence != 1 || events[0].Severity != "critical" {
		t.Fatalf("events=%+v", events)
	}
}
