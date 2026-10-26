# InfluxDB Health Split Design

## Intent

Separate client construction from health checking without changing startup behavior.

## Design

- `Init() (*influxdb3.Client, error)` validates connection settings, constructs the SDK client with a five-second HTTP timeout and redirect rejection, calls `IsHealthy` in an `if err := ...` statement, closes the client on health failure, and returns the client only on success.
- `IsHealthy(client *influxdb3.Client) error` checks availability through the SDK's `GetServerVersion` method, which makes an authenticated `GET /ping` request. It returns a sanitized error for nil clients, transport failures, and non-success responses.
- Keep the current public startup call, configuration schema, Compose behavior, and error safety. Update tests to exercise `IsHealthy` with a real SDK client and retain tests that prove `Init` rejects unavailable servers and returns a client for available ones. Update documentation to distinguish the backend's `/ping` check from Compose's `/health` check.

## Approaches

- **Chosen after user correction:** Pass the SDK client to `IsHealthy` and call `GetServerVersion`. The SDK exposes no direct `/health` operation, but its `/ping` request uses the client's host and authentication.
- Pass host and token explicitly and send a separate `/health` request. This was the first implementation, but the user wants the client passed directly.
- Wrap the SDK client to retain host/token for a raw `/health` call. This would change the returned client type and add state solely for the check.
