# Backend

This Go module provides the readings API for the busyness sensing project.
It uses a hexagonal layout: core services depend on ports, while HTTP and database code
live in adapters. Docker Compose runs the Go backend, Mosquitto, Telegraf, and
InfluxDB 3 Core. Telegraf ingests MQTT sensor readings into InfluxDB; the
backend serves those readings through its HTTP API.

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
| `internal/driven/database/influxdb/` | InfluxDB client, startup health check, and readings queries |

## Local services

Start the stack from this directory with `docker compose up -d --build` so
Compose builds the backend from `Dockerfile.webserver` and the Mosquitto image
with its timestamp plugin. The Mosquitto base image version is pinned in
`Dockerfile.mosquitto`; Telegraf and InfluxDB image versions are pinned in
`docker-compose.yml`.
InfluxDB is published only on `127.0.0.1:8181`; its local development config
disables authentication. Keep the stack on a trusted machine and enable
authentication before exposing it to a network.

Compose waits for Mosquitto and InfluxDB healthchecks to pass before starting
Telegraf, and waits for InfluxDB before starting the backend. The Mosquitto check only connects and subscribes to a dedicated
health topic; it publishes no MQTT message. InfluxDB is checked with its
`/health` endpoint. Telegraf's healthcheck confirms its process is alive, not
that readings are currently arriving or being stored.

Create the InfluxDB database before Telegraf can store readings:

```sh
docker compose exec influxdb3 influxdb3 create database busyness_sensing
```

Run `sh tests/mosquitto_ingress_timestamp.sh` from this directory to build an
isolated broker image and check timestamping, offline delivery, and payload
validation.

Telegraf subscribes to `sensors/<buildingName>/<roomNumber>` at QoS 1. For
example, Building BBB, room 212 publishes to `sensors/BBB/212`. The ESP32
payload has no timestamp. The required telemetry contract looks like this
(`co2equivalent` is optional):

```json
{
  "pressure": 1013.25,
  "humidity": 48.2,
  "temperature": 22.5,
  "co2equivalent": 615.0,
  "voc": 0.42,
  "iaq": 12345.6,
  "mic_vol": 0.18,
  "busy_level": 2,
  "PIRs": [
    {"ID": "pir-1", "value": 1},
    {"ID": "pir-2", "value": 0}
  ]
}
```

Firmware is maintained in a separate repository and is not changed here.
Coordinate adoption of these exact field names and uppercase PIR `ID` with
the firmware owner before deployment. Existing InfluxDB data under previous
column names is not migrated by this change.

Mosquitto's incoming-message plugin adds a top-level `timestamp` containing
Unix milliseconds when the broker receives an accepted sensor publication.
For example, the JSON delivered to Telegraf has the same readings and PIR
array, plus `"timestamp": 1792066530250`. The plugin rejects malformed sensor
JSON and sensor payloads that already contain `timestamp`. Both Telegraf JSON
v2 consumers use this value as the InfluxDB point time, including when a
message waits in the broker while Telegraf is offline. The timestamp is the
broker receipt time, not the ESP32 sampling time: buffering on the ESP32 and
network delay before broker receipt are not measured. Keep the broker clock
synchronized for useful times.

`pressure`, `humidity`, `temperature`, `co2equivalent`, `voc`, `iaq`, and
`mic_vol` are float32 values from the ESP32; `co2equivalent` is optional.
`busy_level` is an int8 value (-128 through 127). Telegraf converts the
floating-point readings to its `float` field type and `busy_level` to `int`;
its parser does not enforce the source's float32 precision or int8 range.
`PIRs` is an array of objects with a stable string `ID` and numeric `value`.
Telegraf writes two measurements into the `busyness_sensing` database:

| Measurement | Tags | Fields |
| --- | --- | --- |
| `room_telemetry` | `building`, `room` | `pressure`, `humidity`, `temperature`, optional `co2equivalent`, `voc`, `iaq`, `mic_vol`, `busy_level` |
| `pir_reading` | `building`, `room`, `pir_id` | `value` |

Room and building identifiers are tags so queries can filter by them. Each PIR
is written as its own point so its ID remains attached to the numeric value.
Service config lives in `configs/influxdb3.env` and
`configs/telegraf.conf`. Set
`INFLUXDB3_AUTH_TOKEN` in the environment if InfluxDB authentication is enabled.
QoS 1 can deliver the same publication more than once. A queued subscriber
redelivery retains the enriched payload and its timestamp; a repeated
publication reaching the broker may receive another timestamp. Ingestion is
not deduplicated. Millisecond precision can also give distinct readings the
same series and timestamp, so InfluxDB may merge their fields.

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

The backend uses the SDK client to check InfluxDB `/ping` with a five-second
timeout before starting HTTP and closes its client when the server stops. Compose config uses
`http://influxdb3:8181`, the InfluxDB service hostname inside the Compose network.
Set `influxdb.host` to the InfluxDB origin/root URL with no path prefix: the SDK
resolves the availability check to root-level `/ping`. The supplied Compose
configuration already meets this requirement.
For a backend running on your host, start InfluxDB with
`docker compose up -d influxdb3`, copy `configs/configs.webserver.yml` to a local
config, change `influxdb.host` to `http://127.0.0.1:8181`, and run
`go run . start --config /path/to/local.yml`. The SDK requires a nonempty token;
the supplied `local-dev-token` is a placeholder for the local server with
authentication disabled. Set a real token in your local configuration when
authentication is enabled. A successful health check confirms server health,
not that the configured database exists or that queries are authorized.

The OpenAPI contract is
documented in [`docs/api/openapi.yaml`](docs/api/openapi.yaml). The contract
uses `GET /api/v1/readings/{building}/{roomNumber}` with `start_time`,
`end_time`, and repeated `sensors` query parameters. Supported sensor names are
`PIRs`, `pressure`, `humidity`, `temperature`, `co2equivalent`, `voc`, `iaq`,
`mic_vol`, and `busy_level`. Requests for a building or room absent from the
database, or without readings for the requested sensors and time range,
return HTTP 404; this does not establish that a room is nonexistent. The route
is implemented through the core readings service and InfluxDB adapter. Responses
include an array for every requested sensor, with `PIRs` grouped by timestamp
and uppercase `ID` preserved for each PIR value.

## Raspberry Pi helper commands

Run these from the repository root on the Raspberry Pi:

```sh
docker compose up -d --build
docker compose down

./utils/insert_mqtt.sh -b BBB -r 212 --pressure 1013.25 -H 48.2 -t 22.5 \
  --voc 0.42 --iaq 12345.6 --mic-vol 0.18 --busy-level 2 \
  -c 615.0 -p pir-1=1,pir-2=0

./utils/read_mqtt.sh live -b BBB -n 5
./utils/read_mqtt.sh history -b BBB -r 212 -s 30m
./utils/read_mqtt.sh history --pir -s 2h --format csv
```
