package httpapi

import "example.com/forestpulse/internal/model"

type registerRequest struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}
type statusRequest struct {
	Reason string `json:"reason"`
}
type batchRequest struct {
	ID       string          `json:"id"`
	Readings []model.Reading `json:"readings"`
}
type batchResponse struct {
	BatchID   string           `json:"batch_id"`
	StationID string           `json:"station_id"`
	State     model.BatchState `json:"state"`
	Duplicate bool             `json:"duplicate"`
	Cursor    model.Cursor     `json:"cursor"`
	Error     string           `json:"error,omitempty"`
}
type errorResponse struct {
	Error string `json:"error"`
}
