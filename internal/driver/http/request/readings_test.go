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
		{"summer", "2026-07-01 12:00:00.000", "2026-07-01 13:00:00.000", "2026-07-01T10:00:00Z", "2026-07-01T11:00:00Z"},
		{"milliseconds", "2026-07-01 12:00:00.123", "2026-07-01 13:00:00.456", "2026-07-01T10:00:00.123Z", "2026-07-01T11:00:00.456Z"},
		{"spring transition", "2026-03-29 01:59:59.999", "2026-03-29 03:00:00.000", "2026-03-29T00:59:59.999Z", "2026-03-29T01:00:00Z"},
		{"autumn transition", "2026-10-25 01:59:59.999", "2026-10-25 03:00:00.000", "2026-10-24T23:59:59.999Z", "2026-10-25T02:00:00Z"},
		{"winter", "2026-01-01 12:00:00.000", "2026-01-01 13:00:00.000", "2026-01-01T11:00:00Z", "2026-01-01T12:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Readings{
				Building: "zilverling", RoomNumber: "Z2402", StartTime: tc.start, EndTime: tc.end,
				Sensors: []string{"PIR", "pressure", "hum", "temp", "co2equivalent", "voc", "airresistance", "mic_vol", "busy_level"},
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
				domain.AIRRESISTANCE_SENSOR, domain.MIC_VOL_SENSOR, domain.BUSY_LEVEL,
			}
			if !reflect.DeepEqual(query.Sensors, wantSensors) {
				t.Errorf("sensors = %v, want %v", query.Sensors, wantSensors)
			}
		})
	}

	valid := Readings{StartTime: "2026-07-01 12:00:00.000", EndTime: "2026-07-01 13:00:00.000", Sensors: []string{"temp"}}
	for _, tc := range []struct {
		name string
		edit func(*Readings)
		want error
	}{
		{"missing start", func(r *Readings) { r.StartTime = "" }, ErrInvalidTimestamp},
		{"missing end", func(r *Readings) { r.EndTime = "" }, ErrInvalidTimestamp},
		{"malformed start", func(r *Readings) { r.StartTime = "2026-07-01T12:00:00Z" }, ErrInvalidTimestamp},
		{"malformed end", func(r *Readings) { r.EndTime = "2026-07-01T13:00:00Z" }, ErrInvalidTimestamp},
		{"missing milliseconds", func(r *Readings) { r.StartTime = "2026-07-01 12:00:00" }, ErrInvalidTimestamp},
		{"extra precision", func(r *Readings) { r.StartTime = "2026-07-01 12:00:00.0001" }, ErrInvalidTimestamp},
		{"comma fraction", func(r *Readings) { r.StartTime = "2026-07-01 12:00:00,000" }, ErrInvalidTimestamp},
		{"impossible date", func(r *Readings) { r.EndTime = "2026-02-30 13:00:00.000" }, ErrInvalidTimestamp},
		{"spring gap", func(r *Readings) { r.StartTime = "2026-03-29 02:30:00.000" }, ErrInvalidTimestamp},
		{"autumn ambiguity", func(r *Readings) { r.StartTime = "2026-10-25 02:30:00.000" }, ErrInvalidTimestamp},
		{"missing sensors", func(r *Readings) { r.Sensors = nil }, ErrEmptySensors},
		{"empty sensor", func(r *Readings) { r.Sensors = []string{""} }, ErrEmptySensors},
		{"unknown sensor", func(r *Readings) { r.Sensors = []string{"CO2"} }, ErrUnknownSensor},
		{"wrong case", func(r *Readings) { r.Sensors = []string{"Temp"} }, ErrUnknownSensor},
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
