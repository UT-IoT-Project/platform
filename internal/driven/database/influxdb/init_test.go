package influxdb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"
	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
)

func TestInitHealth(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	tests := map[string]struct {
		status     int
		wantClient bool
		wantError  string
	}{
		"HTTP 200 starts with a healthy client": {
			status:     http.StatusOK,
			wantClient: true,
		},
		"HTTP 204 starts with a healthy client": {
			status:     http.StatusNoContent,
			wantClient: true,
		},
		"HTTP 503 rejects an unhealthy server": {
			status:    http.StatusServiceUnavailable,
			wantError: "influxdb health check failed: HTTP 503",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ping" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.Header().Set("X-Influxdb-Version", "3.11.0")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("secret-token"))
			}))
			t.Cleanup(server.Close)
			config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
			client, err := Init()
			if (client != nil) != tc.wantClient {
				t.Fatalf("Init() client = %v; want client: %t", client, tc.wantClient)
			}
			if client != nil {
				t.Cleanup(func() { _ = client.Close() })
			}
			if tc.wantError == "" && err != nil {
				t.Fatalf("Init() error = %v; want no error", err)
			}
			if tc.wantError != "" && (err == nil || err.Error() != tc.wantError) {
				t.Fatalf("Init() error = %v; want %q", err, tc.wantError)
			}
			if err != nil && strings.Contains(err.Error(), "secret-token") {
				t.Fatal("error exposes response credentials")
			}
		})
	}
}

func TestInitMissingSettings(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	tests := map[string]struct {
		config    config.InfluxDBConfig
		wantError string
	}{
		"empty host is rejected": {
			config: config.InfluxDBConfig{
				Database: "test",
				Token:    "secret-token",
			},
			wantError: "influxdb host is required",
		},
		"whitespace host is rejected": {
			config: config.InfluxDBConfig{
				Host:     " \t\n",
				Database: "test",
				Token:    "secret-token",
			},
			wantError: "influxdb host is required",
		},
		"empty database is rejected": {
			config: config.InfluxDBConfig{
				Host:  "http://localhost:8181",
				Token: "secret-token",
			},
			wantError: "influxdb database is required",
		},
		"whitespace database is rejected": {
			config: config.InfluxDBConfig{
				Host:     "http://localhost:8181",
				Database: " \t\n",
				Token:    "secret-token",
			},
			wantError: "influxdb database is required",
		},
		"empty token is rejected": {
			config: config.InfluxDBConfig{
				Host:     "http://localhost:8181",
				Database: "test",
			},
			wantError: "influxdb token is required",
		},
		"whitespace token is rejected": {
			config: config.InfluxDBConfig{
				Host:     "http://localhost:8181",
				Database: "test",
				Token:    " \t\n",
			},
			wantError: "influxdb token is required",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config.C.InfluxDB = tc.config
			client, err := Init()
			if client != nil || err == nil || err.Error() != tc.wantError {
				t.Fatalf("Init() = %v, %v; want nil client and %q", client, err, tc.wantError)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("error exposes token")
			}
		})
	}
}

func TestInitMalformedHost(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	tests := map[string]struct {
		host      string
		wantError string
	}{
		"malformed URL is rejected": {
			host:      "http://[secret-token",
			wantError: "influxdb host must be an HTTP or HTTPS URL without credentials, query, or fragment",
		},
		"non HTTP scheme is rejected": {
			host:      "ftp://secret-token.example",
			wantError: "influxdb host must be an HTTP or HTTPS URL without credentials, query, or fragment",
		},
		"embedded credentials are rejected": {
			host:      "http://user:secret-token@localhost:8181",
			wantError: "influxdb host must be an HTTP or HTTPS URL without credentials, query, or fragment",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config.C.InfluxDB = config.InfluxDBConfig{Host: tc.host, Database: "test", Token: "secret-token"}
			client, err := Init()
			if client != nil || err == nil || err.Error() != tc.wantError {
				t.Fatalf("Init() = %v, %v; want nil client and %q", client, err, tc.wantError)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("host error exposes credentials")
			}
		})
	}
}

func TestInitHealthFailureModes(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	for name, tc := range healthFailureCases() {
		t.Run(name, func(t *testing.T) {
			server := tc.newServer(t)
			t.Cleanup(server.Close)
			config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
			client, err := Init()
			if client != nil || err == nil || err.Error() != tc.wantError {
				t.Fatalf("Init() = %v, %v; want nil client and %q", client, err, tc.wantError)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("health failure exposes credentials")
			}
		})
	}
}

func TestIsHealthy(t *testing.T) {
	tests := map[string]struct {
		status    int
		wantError string
	}{
		"HTTP 200 reports healthy": {
			status: http.StatusOK,
		},
		"HTTP 204 reports healthy": {
			status: http.StatusNoContent,
		},
		"HTTP 503 reports server failure": {
			status:    http.StatusServiceUnavailable,
			wantError: "influxdb health check failed: HTTP 503",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ping" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.Header().Set("X-Influxdb-Version", "3.11.0")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("secret-token"))
			}))
			t.Cleanup(server.Close)
			client := newHealthTestClient(t, server.URL)
			err := IsHealthy(client)
			if tc.wantError == "" && err != nil {
				t.Fatalf("IsHealthy() = %v; want no error", err)
			}
			if tc.wantError != "" && (err == nil || err.Error() != tc.wantError) {
				t.Fatalf("IsHealthy() = %v; want %q", err, tc.wantError)
			}
		})
	}
}

func TestIsHealthyFailureModes(t *testing.T) {
	for name, tc := range healthFailureCases() {
		t.Run(name, func(t *testing.T) {
			server := tc.newServer(t)
			t.Cleanup(server.Close)
			err := IsHealthy(newHealthTestClient(t, server.URL))
			if err == nil || err.Error() != tc.wantError {
				t.Fatalf("IsHealthy() = %v; want %q", err, tc.wantError)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("health failure exposes credentials")
			}
		})
	}
}

type healthFailureCase struct {
	newServer func(*testing.T) *httptest.Server
	wantError string
}

func healthFailureCases() map[string]healthFailureCase {
	return map[string]healthFailureCase{
		"redirect is not followed": {
			newServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/ping" {
						t.Error("followed health redirect")
						w.WriteHeader(http.StatusOK)
						return
					}
					http.Redirect(w, r, "/secret-token", http.StatusFound)
				}))
			},
			wantError: "influxdb health check failed: HTTP 302",
		},
		"untrusted TLS certificate is rejected": {
			newServer: func(t *testing.T) *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Error("unexpected successful transport")
				}))
			},
			wantError: "influxdb health check failed: request could not complete",
		},
	}
}

func newHealthTestClient(t *testing.T, host string) *influxdb3.Client {
	t.Helper()
	client, err := influxdb3.New(influxdb3.ClientConfig{Host: host, Database: "test", Token: "secret-token", AuthScheme: "Bearer", HTTPClient: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestIsHealthyNilClient(t *testing.T) {
	if err := IsHealthy(nil); err == nil {
		t.Fatal("IsHealthy(nil) = nil; want error")
	}
}
