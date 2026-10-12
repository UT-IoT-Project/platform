# Backend

This Go module is the API scaffold for the busyness sensing project. It uses a
hexagonal layout: core services depend on ports, while HTTP and database code
live in adapters. Docker Compose runs the Go backend, Mosquitto, Telegraf, and
InfluxDB 3 Core. Telegraf ingests MQTT sensor readings into InfluxDB; the
backend will provide read APIs for those readings.

| Path | Responsibility |
| --- | --- |
| `cmd/` | CLI commands and application wiring |
| `configs/` | Backend, Mosquitto, InfluxDB, and Telegraf configuration |
| `volumes/` | Local persistent state for Mosquitto and InfluxDB; runtime data is ignored by Git |
| `internal/app/` | Configuration and logging setup |
| `internal/core/domain/` | Business entities |
| `internal/core/port/` | Interfaces for services and adapters |
| `internal/core/service/` | Application use cases |
| `internal/driver/http/` | HTTP API adapter and request types |
| `internal/driven/database/influxdb/` | InfluxDB adapter placeholder |

## Local services

Start the stack from this directory with `docker compose up -d`. Service image
versions are pinned in `docker-compose.yml`. InfluxDB is published only on
`127.0.0.1:8181`; its local development config disables authentication. Keep
the stack on a trusted machine and enable authentication before exposing it to
a network.

Create the InfluxDB database before Telegraf can store readings:

```sh
docker compose exec influxdb3 influxdb3 create database busyness_sensing
```

Telegraf subscribes to `sensors/<buildingName>/<roomNumber>` at QoS 1. For
example, Building BBB, room 212 publishes to `sensors/BBB/212`. The parser uses
the payload's `timestamp` as Unix seconds and writes two measurements into the
`busyness_sensing` database:

| Measurement | Tags | Fields |
| --- | --- | --- |
| `room_telemetry` | `building`, `room` | `temprature`, `humidity`, optional `co2` |
| `pir_reading` | `building`, `room`, `pir_id` | `value` |

Room and building identifiers are tags so queries can filter by them. Each PIR
is written as its own point so its ID remains attached to the numeric value.
The supplied `temprature` spelling is retained. Service config lives in
`configs/influxdb3.env` and `configs/telegraf.conf`. Set
`INFLUXDB3_AUTH_TOKEN` in the environment if InfluxDB authentication is enabled.

Example SQL for readings from the past hour in Building BBB, room 212:

```sql
SELECT *
FROM room_telemetry
WHERE building = 'BBB'
  AND room = '212'
  AND time >= now() - INTERVAL '1 hour'
ORDER BY time DESC;
```

Use `pir_reading` as the table name to query PIR values over the same time
range.

## Backend

Run the backend alone with `go run . start`. The HTTP server currently has no
application routes; API routes and database queries will be added when the API
contract is implemented.
