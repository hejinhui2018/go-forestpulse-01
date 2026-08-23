package alerts

import "example.com/forestpulse/internal/model"

type Policy struct {
	CriticalFuel       float64
	WarningBattery     float64
	WarningTemperature float64
}

func DefaultPolicy() Policy {
	return Policy{CriticalFuel: 8, WarningBattery: 15, WarningTemperature: 48}
}
func (p Policy) Classify(r model.Reading) (string, string) {
	if r.FuelMoisture < p.CriticalFuel {
		return "critical", "fuel moisture is critically low"
	}
	if r.BatteryPct < p.WarningBattery {
		return "warning", "station battery is low"
	}
	if r.Temperature > p.WarningTemperature {
		return "warning", "station temperature is high"
	}
	return "", ""
}
