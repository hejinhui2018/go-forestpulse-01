package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"example.com/forestpulse/internal/app"
	"example.com/forestpulse/internal/config"
	"example.com/forestpulse/internal/model"
)

func main() {
	dir, err := os.MkdirTemp("", "forestpulse-smoke-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	cfg := config.Default()
	cfg.DataPath = filepath.Join(dir, "state.json")
	cfg.RecoveryEvery = time.Hour
	a, err := app.New(cfg)
	if err != nil {
		panic(err)
	}
	defer a.Close()
	ctx := context.Background()
	if _, err := a.Stations.Register(ctx, "ST-17", "North ridge", "Ridge-4"); err != nil {
		panic(err)
	}
	ts := httptest.NewServer(a.Handler)
	defer ts.Close()
	batch := model.ReadingBatch{ID: "batch-smoke-1", StationID: "ST-17", Readings: []model.Reading{{StationID: "ST-17", Sequence: 1, RecordedAt: time.Now().UTC(), Temperature: 22, Humidity: 55, FuelMoisture: 18, BatteryPct: 91}, {StationID: "ST-17", Sequence: 2, RecordedAt: time.Now().UTC(), Temperature: 23, Humidity: 54, FuelMoisture: 17, BatteryPct: 90}}}
	body, _ := json.Marshal(map[string]any{"id": batch.ID, "readings": batch.Readings})
	resp, err := http.Post(ts.URL+"/v1/stations/ST-17/batches", "application/json", strings.NewReader(string(body)))
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		panic(fmt.Sprintf("batch status %d", resp.StatusCode))
	}
	check, err := http.Get(ts.URL + "/v1/stations/ST-17")
	if err != nil {
		panic(err)
	}
	defer check.Body.Close()
	if check.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("query status %d", check.StatusCode))
	}
	var snapshot struct {
		Cursor model.Cursor `json:"cursor"`
	}
	if err := json.NewDecoder(check.Body).Decode(&snapshot); err != nil {
		panic(err)
	}
	if snapshot.Cursor.LastSequence != 2 {
		panic(fmt.Sprintf("cursor=%d", snapshot.Cursor.LastSequence))
	}
	fmt.Println("accepted batch, persisted readings, cursor=2")
}
