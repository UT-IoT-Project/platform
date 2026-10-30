package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

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
			query:   validQuery + "&sensors=PIR&sensors=temp",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "exactly 24 hours",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-02%2012:00:00.000&sensors=temp",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "more than 24 hours",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-02%2012:00:00.001&sensors=temp",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "end before start",
			query:   "start_time=2026-07-01%2012:00:00.000&end_time=2026-07-01%2011:59:59.999&sensors=temp",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "24 elapsed hours across spring clock change",
			query:   "start_time=2026-03-28%2012:00:00.000&end_time=2026-03-29%2013:00:00.000&sensors=temp",
			status:  500,
			message: "Internal server error.",
		},
		{
			name:    "25 elapsed hours across autumn clock change",
			query:   "start_time=2026-10-24%2012:00:00.000&end_time=2026-10-25%2012:00:00.000&sensors=temp",
			status:  400,
			message: "Time range must be between 0 and 24 hours.",
		},
		{
			name:    "bad timestamp",
			query:   "start_time=2026-07-01T12:00:00Z&end_time=2026-07-01%2013:00:00.000&sensors=temp",
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
			query:   validQuery + "&sensors=temp&sensors=CO2",
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
