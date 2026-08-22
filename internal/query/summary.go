package query

import (
	"math"

	"example.com/forestpulse/internal/model"
)

type ReadingSummary struct {
	Count              int     `json:"count"`
	AverageTemperature float64 `json:"average_temperature_c"`
	AverageHumidity    float64 `json:"average_humidity_pct"`
	MinimumBattery     float64 `json:"minimum_battery_pct"`
	LatestSequence     int64   `json:"latest_sequence"`
}

func Summarize(readings []model.Reading) ReadingSummary {
	out := ReadingSummary{Count: len(readings), MinimumBattery: math.MaxFloat64}
	if len(readings) == 0 {
		out.MinimumBattery = 0
		return out
	}
	for _, r := range readings {
		out.AverageTemperature += r.Temperature
		out.AverageHumidity += r.Humidity
		if r.BatteryPct < out.MinimumBattery {
			out.MinimumBattery = r.BatteryPct
		}
		if r.Sequence > out.LatestSequence {
			out.LatestSequence = r.Sequence
		}
	}
	out.AverageTemperature /= float64(len(readings))
	out.AverageHumidity /= float64(len(readings))
	return out
}
func (s ReadingSummary) Healthy() bool { return s.Count > 0 && s.MinimumBattery >= 15 }
