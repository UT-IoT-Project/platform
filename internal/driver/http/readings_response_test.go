package http

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
	"github.com/jouleion/busyness-sensing/backend/internal/core/service"
)

type readingsServiceFunc func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error)

func (f readingsServiceFunc) GetReadings(c context.Context, q port.ReadingsQuery) (port.ReadingsResult, error) {
	return f(c, q)
}

func TestReadingsSuccess(t *testing.T) {
	timestamp := time.Date(2026, 7, 1, 10, 15, 30, 250000000, time.UTC)
	result := port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
		domain.TEMPERATURE_SENSOR: {{Timestamp: timestamp, Value: 21.7}},
		domain.BUSY_LEVEL:         {{Timestamp: timestamp, Value: -128}},
	}, PIR: []domain.PIRReading{{Timestamp: timestamp, Values: []domain.PIRValue{{ID: "pir-1", Value: 1}, {ID: "pir-2", Value: 0}}}}}
	server := NewServer(readingsServiceFunc(func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error) { return result, nil }))
	recorder := httptest.NewRecorder()
	server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temp&sensors=PIR&sensors=busy_level&sensors=hum&sensors=temp", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	var got any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var want any
	json.Unmarshal([]byte(`{"status":200,"message":"data retrieved successfully","data":{"temp":[{"timestamp":"2026-07-01 12:15:30.250","value":21.7}],"busy_level":[{"timestamp":"2026-07-01 12:15:30.250","value":-128}],"PIR":[{"timestamp":"2026-07-01 12:15:30.250","values":[{"ID":"pir-1","value":1},{"ID":"pir-2","value":0}]}],"hum":[]}}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestReadingsErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		result  port.ReadingsResult
		err     error
		status  int
		message string
	}{
		{"empty", port.ReadingsResult{}, service.ErrNoReadings, 404, "No data was found for room: Z2402. The room may not exist or may have no readings for the requested sensors and time range."},
		{"repository", port.ReadingsResult{}, errors.New("private database details"), 500, "Internal server error."},
		{"fractional busy", port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.BUSY_LEVEL: {{Value: 1.5}}}}, nil, 500, "Internal server error."},
		{"high busy", port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.BUSY_LEVEL: {{Value: 128}}}}, nil, 500, "Internal server error."},
		{"low busy", port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.BUSY_LEVEL: {{Value: -129}}}}, nil, 500, "Internal server error."},
		{"nan", port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {{Value: math.NaN()}}}}, nil, 500, "Internal server error."},
		{"infinite PIR", port.ReadingsResult{PIR: []domain.PIRReading{{Values: []domain.PIRValue{{ID: "p", Value: math.Inf(1)}}}}}, nil, 500, "Internal server error."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(readingsServiceFunc(func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error) { return tc.result, tc.err }))
			recorder := httptest.NewRecorder()
			server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temp&sensors=PIR&sensors=busy_level", nil))
			var got errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || got.Status != tc.status || got.Message != tc.message || got.Data == nil || len(got.Data) != 0 {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestReadingsForwardsContextAndQuery(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	called := false
	server := NewServer(readingsServiceFunc(func(got context.Context, q port.ReadingsQuery) (port.ReadingsResult, error) {
		called = true
		want := port.ReadingsQuery{Building: "zilverling", Room: "Z2402", Start: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC), Sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR}}
		if got != ctx || !reflect.DeepEqual(q, want) {
			t.Errorf("context/query not forwarded: %v", q)
		}
		return port.ReadingsResult{}, service.ErrNoReadings
	}))
	recorder := httptest.NewRecorder()
	server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temp", nil).WithContext(ctx))
	if !called {
		t.Fatal("service was not called")
	}
}
