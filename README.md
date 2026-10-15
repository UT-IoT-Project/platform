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

Compose waits for Mosquitto and InfluxDB healthchecks to pass before starting
Telegraf. The Mosquitto check only connects and subscribes to a dedicated
health topic; it publishes no MQTT message. InfluxDB is checked with its
`/health` endpoint. Telegraf's healthcheck confirms its process is alive, not
that readings are currently arriving or being stored.

Create the InfluxDB database before Telegraf can store readings:

```sh
docker compose exec influxdb3 influxdb3 create database busyness_sensing
```

Telegraf subscribes to `sensors/<buildingName>/<roomNumber>` at QoS 1. For
example, Building BBB, room 212 publishes to `sensors/BBB/212`. The ESP32
payload does not include a timestamp; Telegraf timestamps readings when it
parses the message. A payload looks like this:

```json
{
  "pressure": 1013.25,
  "hum": 48.2,
  "temp": 22.5,
  "co2equivalent": 615.0,
  "voc": 0.42,
  "airresistance": 12345.6,
  "mic_vol": 0.18,
  "busy_level": 2,
  "PIRs": [
    {"id": "pir-1", "value": 1},
    {"id": "pir-2", "value": 0}
  ]
}
```

`pressure`, `hum`, `temp`, `co2equivalent`, `voc`, `airresistance`, and
`mic_vol` are float32 values from the ESP32; `co2equivalent` is optional.
`busy_level` is an int8 value (-128 through 127). Telegraf converts the
floating-point readings to its `float` field type and `busy_level` to `int`;
its parser does not enforce the source's float32 precision or int8 range.
`PIRs` is an array of objects with a stable string `id` and numeric `value`.
Telegraf writes two measurements into the `busyness_sensing` database:

| Measurement | Tags | Fields |
| --- | --- | --- |
| `room_telemetry` | `building`, `room` | `pressure`, `hum`, `temp`, optional `co2equivalent`, `voc`, `airresistance`, `mic_vol`, `busy_level` |
| `pir_reading` | `building`, `room`, `pir_id` | `value` |

Room and building identifiers are tags so queries can filter by them. Each PIR
is written as its own point so its ID remains attached to the numeric value.
Service config lives in `configs/influxdb3.env` and
`configs/telegraf.conf`. Set
`INFLUXDB3_AUTH_TOKEN` in the environment if InfluxDB authentication is enabled.
Because the payload has no device timestamp, a QoS 1 redelivery can create
another point at a later time; ingestion is not deduplicated.

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

Run the backend alone with `go run . start`. The OpenAPI contract is
documented in [`docs/api/openapi.yaml`](docs/api/openapi.yaml). The contract
uses `GET /api/v1/readings/{building}/{roomNumber}` with `start_time`,
`end_time`, and repeated `sensors` query parameters. Supported sensor names are
`PIR`, `pressure`, `hum`, `temp`, `co2equivalent`, `voc`, `airresistance`,
`mic_vol`, and `busy_level`. Requests for a building or room absent from the
database return HTTP 404. The HTTP server currently has no application routes,
so the documented readings route remains unimplemented.
