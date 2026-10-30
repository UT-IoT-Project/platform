package response

import (
	"encoding/json"
	"fmt"
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
			domain.BUSY_LEVEL:         {{Timestamp: timestamp, Value: 255}},
			domain.IAQ_SENSOR:         {{Timestamp: timestamp, Value: 50}},
		},
		PIR: []domain.PIRReading{{Timestamp: timestamp, Values: []domain.PIRValue{{ID: "1", Value: 1}, {ID: "2", Value: 0}}}},
	}
	got, err := MapReadings(query, result)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"status":200,"message":"data retrieved successfully","data":{"busynessLabel":[{"timestamp":"2026-07-01 12:15:30.250","value":255}],"humidity":[],"iaq":[{"timestamp":"2026-07-01 12:15:30.250","value":50}],"pirs":[{"timestamp":"2026-07-01 12:15:30.250","values":[{"ID":1,"activity":1},{"ID":2,"activity":0}]}],"temperature":[{"timestamp":"2026-07-01 12:15:30.250","value":21.7}]}}`
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
				domain.BUSY_LEVEL: {{Value: 256}},
			}},
		},
		{
			name:   "low busy",
			sensor: domain.BUSY_LEVEL,
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.BUSY_LEVEL: {{Value: -1}},
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
		domain.WIFI_PACKETS_SENSOR:       {{Timestamp: stamp, Value: 0}, {Timestamp: stamp, Value: 2147483648}},
		domain.BLE_ADVERTISEMENTS_SENSOR: {{Timestamp: stamp, Value: math.MaxUint32}, {Timestamp: stamp, Value: 0}},
	}}
	got, err := MapReadings(query, result)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"status":200,"message":"data retrieved successfully","data":{"bleAdverts":[{"timestamp":"2026-07-01 12:15:30.250","value":4294967295},{"timestamp":"2026-07-01 12:15:30.250","value":0}],"wifiPackets":[{"timestamp":"2026-07-01 12:15:30.250","value":0},{"timestamp":"2026-07-01 12:15:30.250","value":2147483648}]}}`
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
	const empty = `{"status":200,"message":"data retrieved successfully","data":{"bleAdverts":[],"wifiPackets":[]}}`
	if string(encoded) != empty {
		t.Fatalf("empty response = %s, want %s", encoded, empty)
	}
}

func TestMapWirelessCountReadingsRejectsOutsideUint32Range(t *testing.T) {
	for _, sensor := range []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR} {
		for _, value := range []int64{-1, math.MaxUint32 + 1, math.MinInt64, math.MaxInt64, 9007199254740993} {
			t.Run(fmt.Sprintf("sensor_%d_value_%d", sensor, value), func(t *testing.T) {
				result := port.ReadingsResult{Integers: map[domain.SensorType][]domain.IntegerReading{
					sensor: {{Value: value}},
				}}
				if _, err := MapReadings(port.ReadingsQuery{Sensors: []domain.SensorType{sensor}}, result); err == nil {
					t.Fatal("expected mapping error for wireless count outside unsigned 32-bit range")
				}
			})
		}
	}
}

func TestMapPIRFirmwareTypes(t *testing.T) {
	query := port.ReadingsQuery{Sensors: []domain.SensorType{domain.PIR_SENSOR}}
	result := port.ReadingsResult{PIR: []domain.PIRReading{{Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), Values: []domain.PIRValue{{ID: "0", Value: 0}, {ID: "4294967295", Value: 255}}}}}
	got, err := MapReadings(query, result)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got.Data)
	if err != nil {
		t.Fatal(err)
	}
	const expected = `{"pirs":[{"timestamp":"2026-07-01 12:00:00.000","values":[{"ID":0,"activity":0},{"ID":4294967295,"activity":255}]}]}`
	if string(encoded) != expected {
		t.Fatalf("response = %s, want %s", encoded, expected)
	}
	for _, value := range []domain.PIRValue{
		{ID: "pir-1", Value: 1}, {ID: "1.5", Value: 1}, {ID: "-1", Value: 1}, {ID: "4294967296", Value: 1}, {ID: "", Value: 1},
		{ID: "1", Value: 1.5}, {ID: "1", Value: -1}, {ID: "1", Value: 256},
	} {
		result.PIR[0].Values = []domain.PIRValue{value}
		if _, err := MapReadings(query, result); err == nil {
			t.Errorf("accepted invalid PIR %#v", value)
		}
	}
}
