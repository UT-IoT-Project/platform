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
	healthy := false
	defer func() {
		if !healthy {
			_ = client.Close()
		}
	}()
	host.Path = strings.TrimRight(host.Path, "/") + "/health"
	request, err := http.NewRequest(http.MethodGet, host.String(), nil)
	if err != nil {
		return nil, errors.New("create influxdb health request: invalid host")
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	healthClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := healthClient.Do(request)
	if err != nil {
		return nil, errors.New("influxdb health check failed: request could not complete")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("influxdb health check failed: HTTP %d", response.StatusCode)
	}
	healthy = true
	return client, nil
}
