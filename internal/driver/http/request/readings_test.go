package request

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
)

func TestReadingsToQuery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		start    string
		end      string
		wantFrom string
		wantTo   string
	}{
		{
			name:     "summer",
			start:    "2026-07-01 12:00:00.000",
			end:      "2026-07-01 13:00:00.000",
			wantFrom: "2026-07-01T10:00:00Z",
			wantTo:   "2026-07-01T11:00:00Z",
		},
		{
			name:     "milliseconds",
			start:    "2026-07-01 12:00:00.123",
			end:      "2026-07-01 13:00:00.456",
			wantFrom: "2026-07-01T10:00:00.123Z",
			wantTo:   "2026-07-01T11:00:00.456Z",
		},
		{
			name:     "spring transition",
			start:    "2026-03-29 01:59:59.999",
			end:      "2026-03-29 03:00:00.000",
			wantFrom: "2026-03-29T00:59:59.999Z",
			wantTo:   "2026-03-29T01:00:00Z",
		},
		{
			name:     "autumn transition",
			start:    "2026-10-25 01:59:59.999",
			end:      "2026-10-25 03:00:00.000",
			wantFrom: "2026-10-24T23:59:59.999Z",
			wantTo:   "2026-10-25T02:00:00Z",
		},
		{
			name:     "winter",
			start:    "2026-01-01 12:00:00.000",
			end:      "2026-01-01 13:00:00.000",
			wantFrom: "2026-01-01T11:00:00Z",
			wantTo:   "2026-01-01T12:00:00Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Readings{
				Building: "zilverling", RoomNumber: "Z2402", StartTime: tc.start, EndTime: tc.end,
				Sensors: []string{"pirs", "pressure", "humidity", "temperature", "co2Equivalent", "voc", "iaq", "mic_vol", "busy_level"},
			}
			query, err := r.ToQuery()
			if err != nil {
				t.Fatal(err)
			}
			if query.Building != "zilverling" || query.Room != "Z2402" {
				t.Fatalf("location = %q/%q", query.Building, query.Room)
			}
			if got := query.Start.Format(time.RFC3339Nano); got != tc.wantFrom {
				t.Errorf("start = %s, want %s", got, tc.wantFrom)
			}
			if got := query.End.Format(time.RFC3339Nano); got != tc.wantTo {
				t.Errorf("end = %s, want %s", got, tc.wantTo)
			}
			if query.Start.Location() != time.UTC || query.End.Location() != time.UTC {
				t.Error("query times must use UTC location")
			}
			wantSensors := []domain.SensorType{
				domain.PIR_SENSOR, domain.PRESSURE_SENSOR, domain.HUMIDITY_SENSOR,
				domain.TEMPERATURE_SENSOR, domain.CO2EQUIVALENT_SENSOR, domain.VOC_SENSOR,
				domain.IAQ_SENSOR, domain.MIC_VOL_SENSOR, domain.BUSY_LEVEL,
			}
			if !reflect.DeepEqual(query.Sensors, wantSensors) {
				t.Errorf("sensors = %v, want %v", query.Sensors, wantSensors)
			}
		})
	}

	valid := Readings{StartTime: "2026-07-01 12:00:00.000", EndTime: "2026-07-01 13:00:00.000", Sensors: []string{"temperature"}}
	for _, tc := range []struct {
		name string
		edit func(*Readings)
		want error
	}{
		{
			name: "missing start",
			edit: func(r *Readings) { r.StartTime = "" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "missing end",
			edit: func(r *Readings) { r.EndTime = "" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "malformed start",
			edit: func(r *Readings) { r.StartTime = "2026-07-01T12:00:00Z" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "malformed end",
			edit: func(r *Readings) { r.EndTime = "2026-07-01T13:00:00Z" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "missing milliseconds",
			edit: func(r *Readings) { r.StartTime = "2026-07-01 12:00:00" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "extra precision",
			edit: func(r *Readings) { r.StartTime = "2026-07-01 12:00:00.0001" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "comma fraction",
			edit: func(r *Readings) { r.StartTime = "2026-07-01 12:00:00,000" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "impossible date",
			edit: func(r *Readings) { r.EndTime = "2026-02-30 13:00:00.000" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "spring gap",
			edit: func(r *Readings) { r.StartTime = "2026-03-29 02:30:00.000" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "autumn ambiguity",
			edit: func(r *Readings) { r.StartTime = "2026-10-25 02:30:00.000" },
			want: ErrInvalidTimestamp,
		},
		{
			name: "missing sensors",
			edit: func(r *Readings) { r.Sensors = nil },
			want: ErrEmptySensors,
		},
		{
			name: "empty sensor",
			edit: func(r *Readings) { r.Sensors = []string{""} },
			want: ErrEmptySensors,
		},
		{
			name: "unknown sensor",
			edit: func(r *Readings) { r.Sensors = []string{"CO2"} },
			want: ErrUnknownSensor,
		},
		{
			name: "wrong case",
			edit: func(r *Readings) { r.Sensors = []string{"Temp"} },
			want: ErrUnknownSensor,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid
			tc.edit(&r)
			_, err := r.ToQuery()
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestReadingsRejectsLegacySensorNames(t *testing.T) {
	for _, name := range []string{"temp", "hum", "airresistance", "PIR", "PIRs", "co2equivalent", "busynessLabel", "bleAdvertisements"} {
		t.Run(name, func(t *testing.T) {
			r := Readings{StartTime: "2026-07-01 12:00:00.000", EndTime: "2026-07-01 13:00:00.000", Sensors: []string{name}}
			if _, err := r.ToQuery(); !errors.Is(err, ErrUnknownSensor) {
				t.Fatalf("error = %v, want %v", err, ErrUnknownSensor)
			}
		})
	}
}

func TestReadingsAcceptsWirelessCountNames(t *testing.T) {
	r := Readings{StartTime: "2026-07-01 12:00:00.000", EndTime: "2026-07-01 13:00:00.000", Sensors: []string{"wifiPackets", "bleAdverts"}}
	query, err := r.ToQuery()
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR}
	if !reflect.DeepEqual(query.Sensors, want) {
		t.Fatalf("sensors = %v, want %v", query.Sensors, want)
	}
	for _, name := range []string{"wifipackets", "WifiPackets", "bleadvertisements", "BLEAdvertisements"} {
		r.Sensors = []string{name}
		if _, err := r.ToQuery(); !errors.Is(err, ErrUnknownSensor) {
			t.Errorf("%s: error = %v, want unknown sensor", name, err)
		}
	}
}
