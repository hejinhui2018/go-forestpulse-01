package stations

import (
	"fmt"

	"example.com/forestpulse/internal/model"
)

func CanTransition(from, to model.StationStatus) error {
	if from == to {
		return nil
	}
	if from == model.StationRetired {
		return fmt.Errorf("retired station has no outgoing transition")
	}
	switch to {
	case model.StationActive, model.StationPaused, model.StationRetired:
		return nil
	default:
		return fmt.Errorf("unsupported destination status %q", to)
	}
}
func Transition(station model.Station, to model.StationStatus, reason string) (model.Station, error) {
	if err := CanTransition(station.Status, to); err != nil {
		return station, err
	}
	station.Status = to
	station.LastError = reason
	return station, nil
}
