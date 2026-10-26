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
	client, err := influxdb3.New(influxdb3.ClientConfig{Host: c.Host, Database: c.Database, Token: c.Token, AuthScheme: "Bearer"})
	if err != nil {
		return nil, errors.New("initialize influxdb client: invalid client configuration")
	}
	if err := IsHealthy(c.Host, c.Token); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// IsHealthy verifies server health with an authenticated, bounded HTTP request.
func IsHealthy(host, token string) error {
	endpoint, err := url.Parse(host)
	if err != nil {
		return errors.New("create influxdb health request: invalid host")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/health"
	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return errors.New("create influxdb health request: invalid host")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	healthClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := healthClient.Do(request)
	if err != nil {
		return errors.New("influxdb health check failed: request could not complete")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("influxdb health check failed: HTTP %d", response.StatusCode)
	}
	return nil
}
