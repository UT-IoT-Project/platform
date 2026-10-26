package influxdb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
)

func TestInitHealth(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/health" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			config.C.InfluxDB = config.InfluxDBConfig{Host: server.URL, Database: "test", Token: "secret-token"}
			client, err := Init()
			if status == http.StatusOK {
				if err != nil || client == nil {
					t.Fatalf("Init() = %v, %v; want client and no error", client, err)
				}
				defer client.Close()
			} else if err == nil || client != nil {
				t.Fatalf("Init() = %v, %v; want nil and error", client, err)
			}
		})
	}
}

func TestInitMissingSettings(t *testing.T) {
	original := config.C
	t.Cleanup(func() { config.C = original })
	for _, field := range []string{"host", "database", "token"} {
		t.Run(field, func(t *testing.T) {
			c := config.InfluxDBConfig{Host: "http://localhost:8181", Database: "test", Token: "secret-token"}
			switch field {
			case "host":
				c.Host = ""
			case "database":
				c.Database = ""
			case "token":
				c.Token = ""
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
