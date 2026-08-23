package app

import (
	"context"
	"log"
	"net/http"

	"example.com/forestpulse/internal/alerts"
	"example.com/forestpulse/internal/clock"
	"example.com/forestpulse/internal/config"
	"example.com/forestpulse/internal/httpapi"
	"example.com/forestpulse/internal/ingest"
	"example.com/forestpulse/internal/query"
	"example.com/forestpulse/internal/recovery"
	"example.com/forestpulse/internal/stations"
	"example.com/forestpulse/internal/store"
)

type App struct {
	Config   config.Config
	Store    *store.FileStore
	Stations *stations.Service
	Ingest   *ingest.Service
	Alerts   *alerts.Queue
	Recovery *recovery.Runner
	Handler  http.Handler
}

func New(cfg config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	st, err := store.OpenFile(cfg.DataPath)
	if err != nil {
		return nil, err
	}
	clk := clock.Real{}
	stationSvc := stations.NewService(st, clk)
	ingestSvc := ingest.NewService(st, clk, cfg.MaxBatchSize)
	queue := alerts.NewQueue()
	querySvc := query.NewService(st, clk.Now)
	runner := recovery.NewRunner(ingestSvc, clk)
	server := httpapi.NewServer(httpapi.Dependencies{Ingest: ingestSvc, Stations: stationSvc, Query: querySvc, Alerts: queue, Recovery: runner, Logger: log.Default(), Timeout: cfg.RequestTimeout})
	return &App{Config: cfg, Store: st, Stations: stationSvc, Ingest: ingestSvc, Alerts: queue, Recovery: runner, Handler: server.Handler()}, nil
}
func (a *App) StartRecovery(ctx context.Context) {
	a.Recovery.Start(ctx, a.Config.RecoveryEvery, 100, func(err error) { log.Printf("recovery: %v", err) })
}
func (a *App) Close() error { return a.Store.Close() }
