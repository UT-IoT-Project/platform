# InfluxDB Connection Design

## Intent

Initialize a usable InfluxDB 3 Core client when the API starts, using the local Compose instance and the existing Viper configuration pattern.

## Scope and constraints

- Add an `influxdb` block to the existing `configs/configs.webserver.yml` with the Compose service URL, `busyness_sensing` database, and a plaintext local placeholder token. The local server runs with authentication disabled; the Go client still requires a nonempty token.
- Parse that block into a dedicated `InfluxDBConfig` struct inside `internal/app/config`.
- Add `internal/driven/database/influxdb/init.go` with `Init() (*influxdb3.Client, error)`. It constructs the InfluxDB 3 Go client and checks `/health` with a bounded timeout. Return an error and close the client if the health check fails.
- Call the initializer during API startup and close the client when the server returns. Gate the backend container on the InfluxDB health check so normal Compose startup has a reachable server.
- Keep the future query adapter and HTTP routes outside this change. The health check proves server reachability, not that the database has been created.

## Approaches

- **Chosen:** Use the InfluxDB 3 Go client plus a short HTTP health check. This gives the future query adapter its native client and makes startup failures explicit.
- Use only `net/http` as a custom connection wrapper. This avoids a dependency but would require building a query client later and would not provide an InfluxDB client instance now.
- Construct the SDK client without checking the server. This is smaller, but construction can succeed while the database server is unavailable.

## Verification

Unit tests cover YAML unmarshalling, successful initialization against a local health server, and a failing health response. The full Go test suite and `go vet` must pass.
