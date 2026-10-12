# Backend

This Go module is the API scaffold for the busyness sensing project. It uses a
hexagonal layout: core services depend on ports, while HTTP and database code
live in adapters. Telegraf writes sensor readings to InfluxDB; this backend
will provide read APIs for those readings.

| Path | Responsibility |
| --- | --- |
| `cmd/` | CLI commands and application wiring |
| `configs/` | Application configuration |
| `internal/app/` | Configuration and logging setup |
| `internal/core/domain/` | Business entities |
| `internal/core/port/` | Interfaces for services and adapters |
| `internal/core/service/` | Application use cases |
| `internal/driver/http/` | HTTP API adapter and request types |
| `internal/driven/database/influxdb/` | InfluxDB adapter placeholder |

Run from this directory with `go run . start`. The HTTP server currently has
no application routes; routes and database queries will be added when the API
contract is defined.
