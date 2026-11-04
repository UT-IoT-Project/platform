# Backend

This Go module provides the readings API for the busyness sensing project.
It uses a hexagonal layout: core services depend on ports, while HTTP and database code
live in adapters. Docker Compose runs the Go backend, Mosquitto, Telegraf,
InfluxDB 3 Core, and Grafana. Telegraf ingests MQTT sensor readings into InfluxDB; the
backend serves those readings through its HTTP API.

| Path | Responsibility |
| --- | --- |
| `cmd/` | CLI commands and application wiring |
| `configs/` | Backend, Mosquitto, InfluxDB, and Telegraf configuration |
| `volumes/` | Local persistent state for Mosquitto, InfluxDB, and Grafana; runtime data is ignored by Git |
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
`Dockerfile.mosquitto`; Telegraf, InfluxDB, and Grafana image versions are pinned in
`docker-compose.yml`.
InfluxDB is published only on `127.0.0.1:8181`; its local development config
disables authentication. Keep the stack on a trusted machine and enable
authentication before exposing it to a network.

Compose waits for Mosquitto and InfluxDB healthchecks to pass before starting
Telegraf, and waits for InfluxDB before starting the backend and Grafana. The Mosquitto check only connects and subscribes to a dedicated
health topic; it publishes no MQTT message. InfluxDB is checked with its
`/health` endpoint. Telegraf's healthcheck confirms its process is alive, not
that readings are currently arriving or being stored.

Create the InfluxDB database before Telegraf can store readings:

```sh
docker compose exec -T influxdb3 influxdb3 create database --token local-dev-token busyness_sensing
```

Grafana is available at <http://127.0.0.1:3000>. On first login, use username
`admin` and password `admin`, then follow the prompt to choose a new password.
In **Connections → Data sources**, add the built-in **InfluxDB** data source
and enter these manual connection settings:

| Setting | Value |
| --- | --- |
| Product | `InfluxDB Enterprise 3.x` (the current Grafana UI has no Core entry) |
| Query language | `SQL` |
| URL | `http://influxdb3:8181` |
| Database | `busyness_sensing` |
| Token | `local-dev-token` if the form requires a nonempty token; local InfluxDB authentication is disabled |
| Insecure gRPC connection | Enabled, because the local InfluxDB server has no TLS |

Select **Save & test** to verify the connection. When InfluxDB authentication
is enabled, use a read-only database token instead. Grafana connects directly
to InfluxDB as an internal visualization tool; UI and Python/ML clients
continue to use the Go HTTP API.

The bind mount at `volumes/grafana` preserves Grafana users, data sources, and
dashboards across normal builds and container recreation. Stop Grafana before
deleting `volumes/grafana` to explicitly reset its state. Grafana runs as root
only to keep this local bind mount writable; its host port is limited to
loopback. Data sources and dashboards are configured manually.

Run `sh tests/mosquitto_ingress_timestamp.sh` from this directory to build an
isolated broker image and check timestamping, offline delivery, and payload
validation.

Telegraf subscribes to `sensors/<buildingName>/<roomNumber>` at QoS 1. The
current ESP32 firmware publishes at QoS 0. For
example, Building BBB, room 212 publishes to `sensors/BBB/212`. The ESP32
payload has no timestamp. Its telemetry event looks like this:

```json
{
  "pressure": 1013.25,
  "humidity": 48.2,
  "temperature": 22.5,
  "co2Equivalent": 615.0,
  "voc": 0.42,
  "iaq": 12345.6,
  "mic_vol": 0.18,
  "busy_level": 2,
  "wifiPackets": 42,
  "bleAdverts": 17,
  "pirs": [
    {"ID": 1, "activity": 100},
    {"ID": 2, "activity": 0},
    {"ID": 3, "activity": 40}
  ]
}
```

Firmware is maintained in a separate repository and is not changed here.
Both `wifiPackets` and `bleAdverts` are required unsigned 32-bit JSON integers
(0 through 4294967295). The database is refreshed when the telemetry contract
changes; no historical backfill is included.

Mosquitto's incoming-message plugin adds a top-level `timestamp` containing
Unix milliseconds when the broker receives an accepted sensor publication.
For example, the JSON delivered to Telegraf has the same readings and PIR
array, plus `"timestamp": 1792066530250`. The plugin rejects malformed sensor
JSON and sensor payloads that already contain `timestamp`. Both Telegraf JSON
v2 consumers use this value as the InfluxDB point time. The current firmware's
QoS 0 publications are not queued for Telegraf while it is offline. The timestamp is the
broker receipt time, not the ESP32 sampling time: buffering on the ESP32 and
network delay before broker receipt are not measured. Keep the broker clock
synchronized for useful times.

`pressure`, `humidity`, `temperature`, `co2Equivalent`, `voc`, `iaq`, and
`mic_vol` are float32 values from the ESP32. `busy_level` is a firmware
unsigned byte currently emitting labels 0 through 3. Telegraf converts the
floating-point readings to its `float` field type and `busy_level`,
`wifiPackets`, and `bleAdverts` to `int`;
its parser does not enforce the source's float32 precision or integer ranges.
`pirs` is an array of objects with numeric `ID` and integer `activity` (0 through 100).
Telegraf writes two measurements into the `busyness_sensing` database:

| Measurement | Tags | Fields |
| --- | --- | --- |
| `room_telemetry` | `building`, `room` | `pressure`, `humidity`, `temperature`, `co2Equivalent`, `voc`, `iaq`, `mic_vol`, `busy_level`, `wifiPackets`, `bleAdverts` |
| `pir_reading` | `building`, `room`, `pir_id` | `activity` |

Room and building identifiers are tags so queries can filter by them. Each PIR
is written as its own point so its ID remains attached to the numeric value.
Service config lives in `configs/influxdb3.env` and
`configs/telegraf.conf`. Set
`INFLUXDB3_AUTH_TOKEN` in the environment if InfluxDB authentication is enabled.
QoS 1 publications from other publishers can be redelivered. A queued subscriber
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
`pirs`, `pressure`, `humidity`, `temperature`, `co2Equivalent`, `voc`, `iaq`,
`mic_vol`, `busy_level`, `wifiPackets`, and `bleAdverts`. Requests for a
building or room absent from the database, or without readings for the requested
sensors and time range,
return HTTP 404; this does not establish that a room is nonexistent. The route
is implemented through the core readings service and InfluxDB adapter. Responses
include an array for every requested sensor, with `pirs` grouped by timestamp
and numeric uppercase `ID` preserved for each PIR activity. The two wireless counts use
unsigned 32-bit JSON integer response values. Storage and core types use `int64`
internally without float conversion; the HTTP mapper rejects out-of-range stored
counts with a generic 500 response. The broker parses and reprints JSON numbers
through cJSON doubles, which exactly preserve the unsigned 32-bit range but cannot preserve
arbitrary int64 values. A local Compose smoke test on 2026-10-30 published
the firmware-shaped payload at QoS 0 through Mosquitto, Telegraf, and
InfluxDB 3.11 Core. The API returned all eleven requested sensor arrays,
including numeric PIR IDs and activities, `wifiPackets` 4294967295, and
`bleAdverts` 2147483648. This used a representative payload, not a physical
ESP32 or production deployment.

## Raspberry Pi helper commands

The publish helper validates generated wireless counts as JSON integer syntax
within unsigned 32-bit range before publishing. Its `--raw` mode sends the supplied
payload as-is and bypasses generated-payload validation.

Run these from the repository root on the Raspberry Pi:

```sh
docker compose up -d --build
docker compose down

./utils/insert_mqtt.sh -b BBB -r 212 --pressure 1013.25 -H 48.2 -t 22.5 \
  --voc 0.42 --iaq 12345.6 --mic-vol 0.18 --busy-level 2 \
  --wifi-packets 42 --ble-adverts 17 \
  -c 615.0 -p 1=100,2=0,3=40

./utils/read_mqtt.sh live -b BBB -n 5
./utils/read_mqtt.sh history -b BBB -r 212 -s 30m
./utils/read_mqtt.sh history --pir -s 2h --format csv
```
