package response

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

func TestMapReadings(t *testing.T) {
	timestamp := time.Date(2026, 7, 1, 10, 15, 30, 250000000, time.UTC)
	query := port.ReadingsQuery{Sensors: []domain.SensorType{
		domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR, domain.BUSY_LEVEL, domain.HUMIDITY_SENSOR, domain.IAQ_SENSOR, domain.TEMPERATURE_SENSOR,
	}}
	result := port.ReadingsResult{
		Scalars: map[domain.SensorType][]domain.ScalarReading{
			domain.TEMPERATURE_SENSOR: {{Timestamp: timestamp, Value: 21.7}},
			domain.BUSY_LEVEL:         {{Timestamp: timestamp, Value: -128}},
			domain.IAQ_SENSOR:         {{Timestamp: timestamp, Value: 50}},
		},
		PIR: []domain.PIRReading{{Timestamp: timestamp, Values: []domain.PIRValue{{ID: "pir-1", Value: 1}, {ID: "pir-2", Value: 0}}}},
	}
	got, err := MapReadings(query, result)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"status":200,"message":"data retrieved successfully","data":{"PIRs":[{"timestamp":"2026-07-01 12:15:30.250","values":[{"ID":"pir-1","value":1},{"ID":"pir-2","value":0}]}],"busy_level":[{"timestamp":"2026-07-01 12:15:30.250","value":-128}],"humidity":[],"iaq":[{"timestamp":"2026-07-01 12:15:30.250","value":50}],"temperature":[{"timestamp":"2026-07-01 12:15:30.250","value":21.7}]}}`
	if string(encoded) != want {
		t.Fatalf("response = %s, want %s", encoded, want)
	}
}

func TestMapReadingsRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sensor domain.SensorType
		result port.ReadingsResult
	}{
		{
			name:   "fractional busy",
			sensor: domain.BUSY_LEVEL,
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.BUSY_LEVEL: {{Value: 1.5}},
			}},
		},
		{
			name:   "high busy",
			sensor: domain.BUSY_LEVEL,
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.BUSY_LEVEL: {{Value: 128}},
			}},
		},
		{
			name:   "low busy",
			sensor: domain.BUSY_LEVEL,
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.BUSY_LEVEL: {{Value: -129}},
			}},
		},
		{
			name:   "nan",
			sensor: domain.TEMPERATURE_SENSOR,
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.TEMPERATURE_SENSOR: {{Value: math.NaN()}},
			}},
		},
		{
			name:   "infinite PIR",
			sensor: domain.PIR_SENSOR,
			result: port.ReadingsResult{PIR: []domain.PIRReading{{
				Values: []domain.PIRValue{{ID: "p", Value: math.Inf(1)}},
			}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MapReadings(port.ReadingsQuery{Sensors: []domain.SensorType{tc.sensor}}, tc.result); err == nil {
				t.Fatal("expected mapping error")
			}
		})
	}
}

func TestMapWirelessCountReadings(t *testing.T) {
	stamp := time.Date(2026, 7, 1, 10, 15, 30, 250000000, time.UTC)
	query := port.ReadingsQuery{Sensors: []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR, domain.WIFI_PACKETS_SENSOR}}
	result := port.ReadingsResult{Integers: map[domain.SensorType][]domain.IntegerReading{
		domain.WIFI_PACKETS_SENSOR:       {{Timestamp: stamp, Value: 9007199254740993}, {Timestamp: stamp, Value: math.MinInt64}},
		domain.BLE_ADVERTISEMENTS_SENSOR: {{Timestamp: stamp, Value: math.MaxInt64}, {Timestamp: stamp, Value: 0}},
	}}
	got, err := MapReadings(query, result)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"status":200,"message":"data retrieved successfully","data":{"bleAdvertisements":[{"timestamp":"2026-07-01 12:15:30.250","value":9223372036854775807},{"timestamp":"2026-07-01 12:15:30.250","value":0}],"wifiPackets":[{"timestamp":"2026-07-01 12:15:30.250","value":9007199254740993},{"timestamp":"2026-07-01 12:15:30.250","value":-9223372036854775808}]}}`
	if string(encoded) != want {
		t.Fatalf("response = %s, want %s", encoded, want)
	}
	got, err = MapReadings(query, port.ReadingsResult{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const empty = `{"status":200,"message":"data retrieved successfully","data":{"bleAdvertisements":[],"wifiPackets":[]}}`
	if string(encoded) != empty {
		t.Fatalf("empty response = %s, want %s", encoded, empty)
	}
}
