package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/forestpulse/internal/alerts"
	"example.com/forestpulse/internal/domain"
	"example.com/forestpulse/internal/ingest"
	"example.com/forestpulse/internal/model"
	"example.com/forestpulse/internal/query"
	"example.com/forestpulse/internal/recovery"
	"example.com/forestpulse/internal/stations"
)

type Dependencies struct {
	Ingest   *ingest.Service
	Stations *stations.Service
	Query    *query.Service
	Alerts   *alerts.Queue
	Recovery *recovery.Runner
	Logger   *log.Logger
	Timeout  time.Duration
}
type Server struct {
	ingest   *ingest.Service
	stations *stations.Service
	query    *query.Service
	alerts   *alerts.Queue
	recovery *recovery.Runner
	logger   *log.Logger
	timeout  time.Duration
}

func NewServer(d Dependencies) *Server {
	if d.Logger == nil {
		d.Logger = log.Default()
	}
	if d.Timeout <= 0 {
		d.Timeout = 3 * time.Second
	}
	// Connect the alert queue to the shared ingestion service so alert
	// evaluation runs for every successfully committed batch, regardless of
	// whether it arrived over HTTP or was replayed by the recovery worker.
	// The same ingest.Service instance is shared by the recovery runner, so
	// replayed batches generate alerts through the same path.
	if d.Alerts != nil && d.Ingest != nil {
		d.Ingest.AttachAlerts(d.Alerts)
	}
	return &Server{ingest: d.Ingest, stations: d.Stations, query: d.Query, alerts: d.Alerts, recovery: d.Recovery, logger: d.Logger, timeout: d.Timeout}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/v1/stations", s.stationsRoot)
	mux.HandleFunc("/v1/stations/", s.stationPath)
	mux.HandleFunc("/v1/recovery/run", s.runRecovery)
	return headers(accessLog(s.logger, requestTimeout(mux, s.timeout)))
}
func requestTimeout(next http.Handler, duration time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), duration)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "recovery_running": s.recovery.Running()})
}
func (s *Server) stationsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.stations.List(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var req registerRequest
		if !decode(w, r, &req) {
			return
		}
		station, err := s.stations.Register(r.Context(), req.ID, req.Name, req.Region)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, station)
	default:
		methodNotAllowed(w)
	}
}
func (s *Server) stationPath(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "v1" || parts[1] != "stations" {
		http.NotFound(w, r)
		return
	}
	id := parts[2]
	if len(parts) == 3 {
		s.station(w, r, id)
		return
	}
	if len(parts) == 4 && parts[3] == "batches" {
		s.batch(w, r, id)
		return
	}
	if len(parts) == 4 && parts[3] == "readings" {
		s.readings(w, r, id)
		return
	}
	if len(parts) == 4 && (parts[3] == "pause" || parts[3] == "resume") {
		s.status(w, r, id, parts[3])
		return
	}
	http.NotFound(w, r)
}
func (s *Server) station(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	snap, err := s.query.Snapshot(r.Context(), id, 20)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
func (s *Server) batch(w http.ResponseWriter, r *http.Request, stationID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req batchRequest
	if !decode(w, r, &req) {
		return
	}
	out := s.ingest.Receive(r.Context(), model.ReadingBatch{ID: req.ID, StationID: stationID, Readings: req.Readings})
	if out.Err != nil {
		writeJSON(w, http.StatusBadRequest, batchResponse{BatchID: out.BatchID, StationID: stationID, State: out.State, Error: out.Err.Error()})
		return
	}
	// Alert evaluation now happens inside ingest.Receive, once per
	// successfully committed (non-duplicate) batch. This keeps the recovery
	// replay path on equal footing with the HTTP path and prevents duplicate
	// alerts on resubmission.
	writeJSON(w, http.StatusAccepted, batchResponse{BatchID: out.BatchID, StationID: stationID, State: out.State, Duplicate: out.Duplicate, Cursor: out.Cursor})
}
func (s *Server) readings(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	rows, err := s.query.Readings(r.Context(), id, after, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
func (s *Server) status(w http.ResponseWriter, r *http.Request, id, action string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req statusRequest
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	var station model.Station
	var err error
	if action == "pause" {
		station, err = s.stations.Pause(r.Context(), id, req.Reason)
	} else {
		station, err = s.stations.Resume(r.Context(), id)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, station)
}
func (s *Server) runRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	count, err := s.recovery.RunOnce(r.Context(), 100)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"replayed": count})
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON: " + err.Error()})
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST")
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var de *domain.Error
	if errors.As(err, &de) {
		switch de.Kind {
		case domain.KindValidation:
			status = http.StatusBadRequest
		case domain.KindNotFound:
			status = http.StatusNotFound
		case domain.KindConflict:
			status = http.StatusConflict
		case domain.KindUnavailable:
			status = http.StatusServiceUnavailable
		}
	}
	writeJSON(w, status, errorResponse{Error: fmt.Sprint(err)})
}
