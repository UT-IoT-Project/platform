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
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ping" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.Header().Set("X-Influxdb-Version", "3.11.0")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("secret-token"))
			}))
			defer server.Close()
			config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
			client, err := Init()
			if status < http.StatusMultipleChoices {
				if err != nil || client == nil {
					t.Fatalf("Init() = %v, %v; want client and no error", client, err)
				}
				defer client.Close()
			} else {
				if err == nil || client != nil {
					t.Fatalf("Init() = %v, %v; want nil and error", client, err)
				}
				if strings.Contains(err.Error(), "secret-token") {
					t.Fatal("error exposes response credentials")
				}
			}
		})
	}
}

func TestInitMissingSettings(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	for _, missing := range []string{"", " \t\n"} {
		for _, field := range []string{"host", "database", "token"} {
			t.Run(field, func(t *testing.T) {
				c := config.InfluxDBConfig{Host: "http://localhost:8181", Database: "test", Token: "secret-token"}
				switch field {
				case "host":
					c.Host = missing
				case "database":
					c.Database = missing
				case "token":
					c.Token = missing
				}
				config.C.InfluxDB = c
				client, err := Init()
				if client != nil || err == nil || !strings.Contains(err.Error(), field) {
					t.Fatalf("Init() = %v, %v; want error naming %s", client, err, field)
				}
				if strings.Contains(err.Error(), "secret-token") {
					t.Fatal("error exposes token")
				}
			})
		}
	}

}

func TestInitMalformedHost(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	for _, host := range []string{"http://[secret-token", "ftp://secret-token.example", "http://user:secret-token@localhost:8181"} {
		t.Run(host, func(t *testing.T) {
			config.C.InfluxDB = config.InfluxDBConfig{Host: host, Database: "test", Token: "secret-token"}
			client, err := Init()
			if client != nil || err == nil || !strings.Contains(err.Error(), "host") {
				t.Fatalf("Init() = %v, %v; want host error", client, err)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("host error exposes credentials")
			}
		})
	}
}

func TestInitRejectsHealthRedirect(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Error("followed health redirect")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/secret-token", http.StatusFound)
	}))
	defer server.Close()
	config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
	client, err := Init()
	if client != nil || err == nil {
		t.Fatalf("Init() = %v, %v; want redirect failure", client, err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatal("redirect error exposes credentials")
	}
}

func TestInitHealthTransportFailure(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	// The health client must reject an untrusted TLS certificate.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected successful transport") }))
	defer server.Close()
	config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
	client, err := Init()
	if client != nil || err == nil {
		t.Fatalf("Init() = %v, %v; want transport failure", client, err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatal("transport error exposes credentials")
	}
}

func TestIsHealthy(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ping" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.Header().Set("X-Influxdb-Version", "3.11.0")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("secret-token"))
			}))
			defer server.Close()
			client := newHealthTestClient(t, server.URL)
			err := IsHealthy(client)
			if status < http.StatusMultipleChoices {
				if err != nil {
					t.Fatalf("IsHealthy() = %v; want no error", err)
				}
			} else if err == nil || err.Error() != "influxdb health check failed: HTTP 503" {
				t.Fatalf("IsHealthy() = %v; want HTTP 503 error", err)
			}
		})
	}
}

func TestIsHealthyRejectsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Error("followed health redirect")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/secret-token", http.StatusFound)
	}))
	defer server.Close()
	if err := IsHealthy(newHealthTestClient(t, server.URL)); err == nil || !strings.Contains(err.Error(), "HTTP 302") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("IsHealthy() = %v; want sanitized redirect failure", err)
	}
}

func TestIsHealthyTransportFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected successful transport")
	}))
	defer server.Close()
	if err := IsHealthy(newHealthTestClient(t, server.URL)); err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("IsHealthy() = %v; want sanitized transport failure", err)
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
