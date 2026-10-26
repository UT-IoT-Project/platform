# InfluxDB Health Split Design

## Intent

Separate client construction from health checking without changing startup behavior.

## Design

- `Init() (*influxdb3.Client, error)` validates connection settings, constructs the SDK client, calls `IsHealthy` in an `if err := ...` statement, closes the client on health failure, and returns the client only on success.
- `IsHealthy(host, token string) error` performs the existing bounded `GET /health` request with Bearer authorization, redirect rejection, sanitized errors, and HTTP 200 requirement.
- Keep the current public startup call, configuration schema, Compose behavior, and error safety. Add direct tests for `IsHealthy` while retaining tests that prove `Init` rejects unhealthy servers and returns a client for healthy ones.

## Approaches

- **Chosen:** Pass host and token explicitly to `IsHealthy`. This keeps its dependencies visible and makes it directly testable.
- Read global config inside `IsHealthy`. This would hide inputs and couple the health check to global state.
- Pass the SDK client. The SDK does not expose the host or a direct health method; this would add unnecessary adapter state.
