package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"example.com/forestpulse/internal/alerts"
	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/ingest"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/query"
	"example.com/forestpulse/internal/recovery"
	"example.com/forestpulse/internal/stations"
	"example.com/forestpulse/internal/store"
)

func TestStationHTTPWorkflow(t *testing.T) {
	st, err := store.OpenFile(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	clk := &clock.Fixed{Current: time.Now().UTC()}
	stationSvc := stations.NewService(st, clk)
	ingestSvc := ingest.NewService(st, clk, 10)
	querySvc := query.NewService(st, clk.Now)
	recoverySvc := recovery.NewRunner(ingestSvc, clk)
	server := httptest.NewServer(NewServer(Dependencies{Ingest: ingestSvc, Stations: stationSvc, Query: querySvc, Alerts: alerts.NewQueue(), Recovery: recoverySvc, Logger: log.Default()}).Handler())
	defer server.Close()
	postJSON(t, server.URL+"/v1/stations", map[string]any{"id": "ST-9", "name": "Lookout", "region": "R9"}, http.StatusCreated)
	reading := model.Reading{StationID: "ST-9", Sequence: 1, RecordedAt: clk.Now(), Humidity: 45, FuelMoisture: 12, BatteryPct: 88}
	postJSON(t, server.URL+"/v1/stations/ST-9/batches", map[string]any{"id": "b-http", "readings": []model.Reading{reading}}, http.StatusAccepted)
	resp, err := http.Get(server.URL + "/v1/stations/ST-9")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var snapshot query.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Cursor.LastSequence != 1 {
		t.Fatalf("cursor=%d", snapshot.Cursor.LastSequence)
	}
	if _, err := st.GetStation(context.Background(), "ST-9"); err != nil {
		t.Fatal(err)
	}
}

func postJSON(t *testing.T, url string, body any, want int) {
	t.Helper()
	data, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("POST %s status=%d want=%d", url, resp.StatusCode, want)
	}
}
