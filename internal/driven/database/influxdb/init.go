package influxdb

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"
	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
)

// Init constructs the client and verifies that InfluxDB is healthy before startup.
func Init() (*influxdb3.Client, error) {
	c := config.C.InfluxDB
	for _, setting := range []struct{ name, value string }{
		{"host", c.Host}, {"database", c.Database}, {"token", c.Token},
	} {
		if strings.TrimSpace(setting.value) == "" {
			return nil, fmt.Errorf("influxdb %s is required", setting.name)
		}
	}
	host, err := url.Parse(c.Host)
	if err != nil || (host.Scheme != "http" && host.Scheme != "https") || host.Host == "" || host.User != nil || host.RawQuery != "" || host.Fragment != "" {
		return nil, errors.New("influxdb host must be an HTTP or HTTPS URL without credentials, query, or fragment")
	}
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host: c.Host, Database: c.Database, Token: c.Token, AuthScheme: "Bearer",
		HTTPClient: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	})
	if err != nil {
		return nil, errors.New("initialize influxdb client: invalid client configuration")
	}
	if err := IsHealthy(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// IsHealthy verifies server availability through the SDK's authenticated /ping request.
// The caller configures the client's HTTP timeout and redirect policy.
func IsHealthy(client *influxdb3.Client) error {
	if client == nil {
		return errors.New("influxdb health check failed: client is nil")
	}
	if _, err := client.GetServerVersion(); err != nil {
		var serverError *influxdb3.ServerError
		if errors.As(err, &serverError) {
			return fmt.Errorf("influxdb health check failed: HTTP %d", serverError.StatusCode)
		}
		return errors.New("influxdb health check failed: request could not complete")
	}
	return nil
}
