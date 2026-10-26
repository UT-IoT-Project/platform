package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitParsesInfluxDBConfig(t *testing.T) {
	previous := C
	t.Cleanup(func() { C = previous })

	path := filepath.Join(t.TempDir(), "config.yml")
	data := []byte(`logger:
  level: debug
influxdb:
  host: http://test-influxdb:8181
  database: test_database
  token: test-token
`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Init(path); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if C.InfluxDB.Host != "http://test-influxdb:8181" {
		t.Errorf("InfluxDB.Host = %q, want http://test-influxdb:8181", C.InfluxDB.Host)
	}
	if C.InfluxDB.Database != "test_database" {
		t.Errorf("InfluxDB.Database = %q, want test_database", C.InfluxDB.Database)
	}
	if C.InfluxDB.Token != "test-token" {
		t.Errorf("InfluxDB.Token = %q, want test-token", C.InfluxDB.Token)
	}
}
