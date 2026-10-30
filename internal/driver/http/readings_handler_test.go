package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
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

func TestGetReadingsRoute(t *testing.T) {
	const path = "/api/v1/readings/zilverling/Z2402"
	const validQuery = "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000"

	for _, tc := range []struct {
		name    string
		query   string
		status  int
		message string
	}{
		{
			name:    "valid request",
			query:   validQuery + "&sensors=PIRs&sensors=temperature",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "exactly 24 hours",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-02%2012:00:00.000&sensors=temperature",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "more than 24 hours",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-02%2012:00:00.001&sensors=temperature",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "end before start",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2011:59:59.999&sensors=temperature",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "24 elapsed hours across spring clock change",
			query:   "start_time=2026-03-28%2012:00:00.000&end_time=2026-03-29%2013:00:00.000&sensors=temperature",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "25 elapsed hours across autumn clock change",
			query:   "start_time=2026-10-24%2012:00:00.000&end_time=2026-10-25%2012:00:00.000&sensors=temperature",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "bad timestamp",
			query:   "start_time=2026-07-01T12:00:00Z&end_time=2026-07-01%2013:00:00.000&sensors=temperature",
			status:  400,
			message: "Invalid timestamp format.",
		},
		{
			name:    "missing sensors",
			query:   validQuery,
			status:  400,
			message: "Empty list of sensors are not allowed.",
		},
		{
			name:    "empty sensors",
			query:   validQuery + "&sensors=",
			status:  400,
			message: "Empty list of sensors are not allowed.",
		},
		{
			name:    "unknown repeated sensor",
			query:   validQuery + "&sensors=temperature&sensors=CO2",
			status:  400,
			message: "Unknown sensor name.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(readingsServiceFunc(func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error) {
				return port.ReadingsResult{}, errors.New("database unavailable")
			}))
			recorder := httptest.NewRecorder()
			server.e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path+"?"+tc.query, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.status, recorder.Body.String())
			}
			wantBody := fmt.Sprintf("{\"status\":%d,\"message\":%q,\"data\":{}}\n", tc.status, tc.message)
			if got := recorder.Body.String(); got != wantBody {
				t.Errorf("body = %q, want %q", got, wantBody)
			}
		})
	}
}

func TestGetReadingsSuccess(t *testing.T) {
	timestamp := time.Date(2026, 7, 1, 10, 15, 30, 250000000, time.UTC)
	result := port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
		domain.TEMPERATURE_SENSOR: {{Timestamp: timestamp, Value: 21.7}},
		domain.BUSY_LEVEL:         {{Timestamp: timestamp, Value: -128}},
		domain.IAQ_SENSOR:         {{Timestamp: timestamp, Value: 50}},
	}, PIR: []domain.PIRReading{{Timestamp: timestamp, Values: []domain.PIRValue{{ID: "pir-1", Value: 1}, {ID: "pir-2", Value: 0}}}}}
	server := NewServer(readingsServiceFunc(func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error) { return result, nil }))
	recorder := httptest.NewRecorder()
	server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temperature&sensors=PIRs&sensors=busy_level&sensors=humidity&sensors=iaq&sensors=temperature", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	var got any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal([]byte(`{"status":200,"message":"data retrieved successfully","data":{"temperature":[{"timestamp":"2026-07-01 12:15:30.250","value":21.7}],"busy_level":[{"timestamp":"2026-07-01 12:15:30.250","value":-128}],"PIRs":[{"timestamp":"2026-07-01 12:15:30.250","values":[{"ID":"pir-1","value":1},{"ID":"pir-2","value":0}]}],"humidity":[],"iaq":[{"timestamp":"2026-07-01 12:15:30.250","value":50}]}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestGetReadingsErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		result  port.ReadingsResult
		err     error
		status  int
		message string
	}{
		{
			name:    "empty",
			result:  port.ReadingsResult{},
			err:     service.ErrNoReadings,
			status:  404,
			message: "No data was found for room: Z2402. The room may not exist or may have no readings for the requested sensors and time range.",
		},
		{
			name:    "repository",
			result:  port.ReadingsResult{},
			err:     errors.New("private database details"),
			status:  500,
			message: "Internal server error.",
		},
		{
			name: "out-of-range wireless count",
			result: port.ReadingsResult{Integers: map[domain.SensorType][]domain.IntegerReading{
				domain.WIFI_PACKETS_SENSOR: {{Value: math.MaxInt32 + 1}},
			}},
			status:  500,
			message: "Internal server error.",
		},
		{
			name: "invalid mapped value",
			result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
				domain.BUSY_LEVEL: {{Value: math.NaN()}},
			}},
			status:  500,
			message: "Internal server error.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(readingsServiceFunc(func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error) { return tc.result, tc.err }))
			recorder := httptest.NewRecorder()
			server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temperature&sensors=PIRs&sensors=busy_level&sensors=wifiPackets", nil))
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

func TestGetReadingsForwardsContextAndQuery(t *testing.T) {
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
	server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=temperature", nil).WithContext(ctx))
	if !called {
		t.Fatal("service was not called")
	}
}

func TestGetWirelessCountReadings(t *testing.T) {
	stamp := time.Date(2026, 7, 1, 10, 15, 30, 250000000, time.UTC)
	result := port.ReadingsResult{Integers: map[domain.SensorType][]domain.IntegerReading{
		domain.WIFI_PACKETS_SENSOR:       {{Timestamp: stamp, Value: 2147483647}},
		domain.BLE_ADVERTISEMENTS_SENSOR: {{Timestamp: stamp, Value: -7}},
	}}
	server := NewServer(readingsServiceFunc(func(_ context.Context, query port.ReadingsQuery) (port.ReadingsResult, error) {
		want := []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR, domain.TEMPERATURE_SENSOR}
		if !reflect.DeepEqual(query.Sensors, want) {
			t.Errorf("sensors = %v, want %v", query.Sensors, want)
		}
		return result, nil
	}))
	recorder := httptest.NewRecorder()
	server.e.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/readings/zilverling/Z2402?start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2013:00:00.000&sensors=wifiPackets&sensors=bleAdvertisements&sensors=temperature", nil))
	const want = `{"status":200,"message":"data retrieved successfully","data":{"bleAdvertisements":[{"timestamp":"2026-07-01 12:15:30.250","value":-7}],"temperature":[],"wifiPackets":[{"timestamp":"2026-07-01 12:15:30.250","value":2147483647}]}}` + "\n"
	if recorder.Code != 200 || recorder.Body.String() != want {
		t.Fatalf("response = %d %s, want 200 %s", recorder.Code, recorder.Body.String(), want)
	}
}
