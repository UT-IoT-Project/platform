# Readings Endpoint End-to-End Design

## Intent

Complete `GET /api/v1/readings/{building}/{roomNumber}` from the existing HTTP request boundary through a core service and the existing InfluxDB 3 readings repository. UI and Python/ML clients receive the selected raw readings through the HTTP API. The user chose the existing batch result and the current sensor-grouped JSON contract for this first working version.

## Behavior

- Keep the existing Echo binding, case-sensitive sensor names, Europe/Amsterdam input parsing, UTC conversion, daylight saving time checks, and inclusive 24-elapsed-hour maximum. The end must not precede the start.
- The handler passes `port.ReadingsQuery` and the HTTP request context to a `port.ReadingsService`. `cmd/start.go` constructs the InfluxDB repository and core service, then injects the service into the HTTP server.
- The core service calls the repository once for the selected sensors and time range. If the result has no scalar or PIR readings, it returns a distinct no-data error. No room registry is added: 404 means no requested sensor readings in the selected range, whether the room is unknown or simply has no matching data.
- The 404 response has status 404, empty `data`, and message `No data was found for room: {room}. The room may not exist or may have no readings for the requested sensors and time range.`
- If at least one requested sensor has readings, return 200 with `data` keys for every requested sensor and empty arrays for requested sensors without readings. Duplicate requested sensor names produce one response key. Keep the OpenAPI `PIR` shape with each stable string `ID` attached to its value.
- Format response timestamps as `YYYY-MM-DD HH:mm:ss.SSS` in Europe/Amsterdam. Scalar values remain JSON numbers; `busy_level` is a JSON integer. If a database value cannot be represented in the contract (for example an out-of-range or fractional busy level), return the generic 500 response rather than silently changing it.
- Database and mapping failures return the existing generic 500 envelope without exposing internals. Keep the request's 400 messages and 24-hour response intact.

## Architecture and approach

Use the existing `ReadingsResult` batch port. Add a small `internal/core/service` implementation that depends only on `port.ReadingsRepository`. Keep HTTP response DTOs and timestamp formatting in `internal/driver/http`; keep SQL in `internal/driven/database/influxdb`. The alternative was to redesign the port for streaming, which would use less memory for very dense data but would change the current contract and error handling. The user chose the batch approach. The HTTP handler does not query InfluxDB directly.

## Verification and limits

Test the service with a repository fake, the HTTP adapter with a service fake, and startup wiring without starting a network listener. Exercise populated, partial, empty, PIR, busy-level, validation, and error paths. Run the full Go suite and vet. Verify the SQL and ingestion against the configured InfluxDB 3 Core and Telegraf Compose stack when a Docker daemon is available; the daemon was unavailable at design time. The 24-hour maximum bounds time but does not cap row count, so very dense ranges can still use substantial memory. Authentication and pagination are outside this feature's current API contract.
