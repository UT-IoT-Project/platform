package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
		{"valid request", validQuery + "&sensors=PIR&sensors=temp", 500, "Internal server error."},
		{"bad timestamp", "start_time=2026-07-01T12:00:00Z&end_time=2026-07-01%2013:00:00.000&sensors=temp", 400, "Invalid timestamp format."},
		{"missing sensors", validQuery, 400, "Empty list of sensors are not allowed."},
		{"empty sensors", validQuery + "&sensors=", 400, "Empty list of sensors are not allowed."},
		{"unknown repeated sensor", validQuery + "&sensors=temp&sensors=CO2", 400, "Unknown sensor name."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer()
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
