# Backend project guide

## Scope and project context

This directory is the Go backend repository and module root. Run Go commands
from here. ESP32 firmware is developed in a separate Git repository by another
maintainer; firmware source is not part of this repository. Coordinate changes
to interfaces shared with firmware with that repository's owner and its current
documentation/configuration rather than assuming a local firmware tree exists.

The system collects room sensor readings, stores them as time-series data, and
serves readings to external clients. Python/ML clients and a future UI should
use the HTTP API for application access. They should not need direct database
credentials.

## System architecture and data flow

```text
ESP32 sensor nodes
       │ publish JSON without a timestamp over MQTT
       ▼
Mosquitto broker (gateway)
       │ adds broker receipt timestamp; Telegraf subscribes and parses
       ▼
InfluxDB (time-series storage)
       ▲ read/query through the InfluxDB adapter
       │
Go HTTP API ─────────────► UI, Python/ML clients, other API consumers
```

Responsibilities:

- **ESP32 firmware** is maintained in a separate repository. It samples sensors
  and publishes telemetry; it should not know about InfluxDB or the Go API.
- **Mosquitto** routes MQTT publications to subscribers. Its incoming-message
  plugin adds a Unix millisecond `timestamp` to accepted sensor JSON at broker
  receipt. It is a broker, not the historical data store.
- **Telegraf** is the ingestion bridge. It subscribes to MQTT, parses and maps
  message/topic data, then writes points to InfluxDB. Keep ingestion out of the
  Go HTTP request path. Compose uses `telegraf:1.40.2-alpine` and the config at
  `configs/telegraf.conf`. Compose starts it after the Mosquitto and InfluxDB
  health checks pass.
- **InfluxDB** stores timestamped readings and serves time-range queries to the
  Go API. Compose uses InfluxDB 3 Core (`influxdb:3.11-core`) with settings in
  `configs/influxdb3.env`; the Go adapter initializes the SDK and verifies server
  health at startup. Its reading query adapter has unit tests and was verified
  in a local Compose smoke test against this version on 2026-10-30.
- **Go backend** exposes a controlled HTTP API and owns request validation,
  authorization, query limits, response formats, and application use cases. It
  reads from InfluxDB; Telegraf owns MQTT ingestion.
- **UI and Python/ML clients** call the Go API. The API is the public boundary
  for reading project data.

### Telemetry contract and cross-repository changes

The firmware repository is external to this backend repository. The currently
agreed telemetry contract is:

- Topic: `sensors/<buildingName>/<roomNumber>`.
- No timestamp in the ESP32 payload. Mosquitto adds a top-level Unix
  millisecond `timestamp` before subscriber delivery. Firmware must not send
  this property; the plugin rejects sensor messages that already contain it,
  as well as malformed sensor JSON.
- Float32 `pressure`, `humidity`, `temperature`, `co2Equivalent`, `voc`, `iaq`,
  and `mic_vol`.
- `busy_level`: an unsigned byte currently emitting labels 0 through 3.
- Required `wifiPackets` and `bleAdverts`: unsigned 32-bit JSON integers
  (0 through 4294967295). They are stored as InfluxDB `int` fields and
  transported internally as `int64`; HTTP validates the range before emitting
  integer JSON values.
- `pirs`: array entries with numeric uppercase `ID` (currently 1, 2, and 3)
  and integer `activity` (0 through 100) properties.

The backend expects these exact field names. The available checkout of the
separate firmware repository still spells `busy_level` as `busynessLabel` in
`esp32/src/main.cpp:preparePayload()`; that checkout must be updated to produce
the agreed event. The database is refreshed with contract updates; no
historical backfill is included.

The Telegraf config uses separate MQTT consumers for room readings and
PIR readings. It writes `room_telemetry` points with `building` and `room`
tags, and `pir_reading` points with `building`, `room`, and `pir_id` tags.
Numeric readings are fields. Keep PIR identity attached to the corresponding
value; do not flatten anonymous array values. Reconfirm this mapping if the
external firmware contract changes. The parser converts float32 source
readings to its `float` field type and `busy_level`, `wifiPackets`, and
`bleAdverts` to `int` in `room_telemetry`; it does not
enforce float32 precision or the integer ranges. The helper validates generated
wireless counts for integer syntax and this range without arithmetic overflow;
`--raw` bypasses generated-payload validation. The broker cJSON parse/reprint
path uses doubles, which exactly preserve the unsigned 32-bit range but cannot
preserve arbitrary int64 values. Both consumers use the broker `timestamp` as
point time. The current firmware publishes at QoS 0, so its publications are
not queued for an offline Telegraf subscriber. The PIR array and each
`ID`/`activity` pair remain unchanged. This is
broker receipt time, not device sampling time: ESP32 buffering and network
delay before receipt can make a reading appear later than it was sampled.
Synchronize the broker clock.

If multiple devices can publish for one room, include a stable `device_id` in
the topic or payload so readings remain distinguishable. InfluxDB fields are
scalar values. For a variable-length PIR array, preserve sensor identity
explicitly (for example, array entries with stable `ID` and `activity`
properties), or transform the data into named fields before writing. Do not
flatten anonymous values in a way that loses which PIR produced them. For
InfluxDB, use tags for string identifiers such as building, room, device, and
PIR ID; use fields for numeric measurements. Keep timestamps as timestamps,
not fields or tags.

MQTT QoS 1 delivery can repeat messages. A queued subscriber redelivery keeps
the timestamp already in its payload, while a repeated publication arriving
at the broker may get a new timestamp. Without a device event ID, ingestion
is not deduplicated. Millisecond collisions within a series can merge fields
at one InfluxDB point. Do not publish historical sensor readings as retained
MQTT messages; retained messages represent the latest value on a topic, not a
reading history.

The current ESP32 firmware publishes with MQTT QoS 0. The Telegraf consumers
subscribe at QoS 1, but the broker delivers these firmware messages at QoS 0;
they are not queued if Telegraf is offline. Mosquitto's
`max_qos 1` caps the listener at QoS 1, but does not force clients to use QoS 1.
The Telegraf consumers use stable client IDs and persistent sessions for QoS 1
publications from other publishers. QoS 1 can redeliver messages; the current
contract does not make writes idempotent.
The current topic does not identify a device. If multiple devices can publish
to the same room, update the agreed topic or payload to include a stable
`device_id` before relying on device-level readings.

## Backend architecture: hexagonal boundaries

Keep business behavior in the core and make external technologies adapters.
Dependencies point inward: adapters may depend on core interfaces and domain
types, while core packages must not import Echo, Viper, zerolog, InfluxDB
clients, or MQTT libraries.

| Path | Boundary and responsibility |
| --- | --- |
| `cmd/` | CLI entry points and dependency wiring. Construct adapters and services here. |
| `configs/` | Backend YAML plus Mosquitto, InfluxDB environment, and Telegraf TOML configs. Never commit real credentials. |
| `internal/app/config/` | Parse application, HTTP, logging, and future database settings. |
| `internal/app/log/` | Initialize the application logger. |
| `internal/core/domain/` | Business entities and domain types; independent of transport and storage. |
| `internal/core/port/` | Interfaces required by core services, including repository/query interfaces. |
| `internal/core/service/` | Use cases and application rules. Depend on ports, not concrete adapters. |
| `internal/driver/http/` | Inbound HTTP adapter: routes, handlers, request/response DTOs, and transport status mapping. |
| `internal/driver/http/request/` | HTTP request DTOs and transport-level validation types. |
| `internal/driver/http/response/` | HTTP response DTOs and mapping from core readings to API data. |
| `internal/driven/database/influxdb/` | Outbound InfluxDB adapter implementing core ports and mapping query results to core types. |

For a new read endpoint, keep the flow in this direction:

1. The HTTP handler binds and validates an HTTP request and extracts the
   request context.
2. The handler calls a core service/use case using core types, not Echo
   request/response objects.
3. The service applies application rules and calls a port from
   `internal/core/port/`.
4. The InfluxDB adapter implements that port, runs a time-range query, and maps
   results to domain/application types.
5. The handler maps the result to an HTTP response DTO and status code.

Handlers should not contain InfluxDB queries. Core services should not depend
on HTTP details. The adapter should not decide public response formats.

## Current backend status

The readings API is implemented through the core service and InfluxDB adapter.
A local Compose smoke test on 2026-10-30 verified MQTT → Telegraf → InfluxDB
3.11 Core → HTTP with scalar and identified PIR readings, partial results,
empty-result 404 responses, and over-24-hour 400 responses. This verifies the
tested local setup, not a production deployment:

- The API accepts `pirs`, `pressure`, `humidity`, `temperature`,
  `co2Equivalent`, `voc`, `iaq`, `mic_vol`, `busy_level`, `wifiPackets`, and
  `bleAdverts` as sensor names. Response keys use the same names; grouped PIR
  activities retain numeric uppercase `ID`. Wireless counts return unsigned
  32-bit integer JSON values; stored values outside that range cause a generic
  500 response. The Go core and adapter retain int64 values internally without
  float conversion. A local Compose smoke test on 2026-10-30 published a
  firmware-shaped QoS 0 event and verified all eleven requested sensor arrays
  through HTTP, including numeric PIR IDs and activities, `wifiPackets`
  4294967295, and `bleAdverts` 2147483648. This used a representative payload,
  not a physical ESP32 or production deployment.
- `cmd/start.go` loads configuration, initializes logging and the InfluxDB
  client, verifies database server health, and starts the Echo HTTP server.
  It wires the InfluxDB readings repository into the core readings service and
  HTTP server, and closes the database client when the server returns.
- `internal/driver/http/server.go` configures Echo middleware and registers
  `GET /api/v1/readings/:building/:roomNumber`. The separate readings handler
  binds and validates the request, converts Europe/Amsterdam timestamps to UTC,
  and calls the core readings service with the request context. It returns 200
  with requested sensor arrays when any readings exist, 404 when all requested
  sensors have no readings, and generic 500 responses for query or mapping
  errors. A 404 does not establish that the room is nonexistent. It rejects
  ambiguous and nonexistent local times during daylight saving time changes,
  reversed ranges, and ranges over 24 elapsed hours.
- `Dockerfile.webserver` packages the backend. `docker-compose.yml` defines backend,
  Mosquitto, InfluxDB 3 Core, and Telegraf services. The derived Mosquitto
  image loads the incoming-message timestamp plugin. Telegraf configuration
  writes telemetry into separate room and PIR measurements. Local InfluxDB
  data is stored under `volumes/influxdb3/data/`. Mosquitto and InfluxDB
  healthchecks gate Telegraf startup; InfluxDB health also gates backend startup; Telegraf also has a process liveness
  healthcheck.
- `internal/core/domain/` defines sensor types and timestamped scalar, integer,
  and PIR reading models. `internal/core/port/` declares batch reading query/result
  types and service/repository interfaces. The InfluxDB adapter implements the
  reading repository. `internal/core/service/` implements the readings use case
  and classifies all-empty results with a no-readings error.
- `internal/driven/database/influxdb/` initializes the InfluxDB SDK client and
  checks `/ping` through that client with a five-second timeout. Compose uses
  `/health` for its separate InfluxDB healthcheck. The reading repository runs
  a parameterized batch time-range query across the requested room and PIR
  measurements and maps results to core types. Known missing-table planning
  errors trigger retries without the absent source; normal successful requests
  use one query. `co2Equivalent` is required in the refreshed database. The
  adapter is wired through the core service into the HTTP handler and was
  exercised against local InfluxDB 3.11 Core, including fresh-database and
  missing-PIR cases. Missing-room handling also has a unit
  regression test. Server health does not verify database existence or query
  authorization.
- There is no Go MQTT subscriber. Telegraf owns ingestion; the local smoke test
  published representative telemetry through Mosquitto, not ESP32 firmware.

Keep this status accurate as files are added. Do not describe an endpoint,
database adapter, or deployment as implemented until its code exists.

## Implementation rules

Before changing code, understand the task and inspect the relevant files. Decide
the expected behavior, affected files, possible side effects, and the smallest
safe implementation plan before editing.

- Prefer the simplest solution that correctly solves the task.
- Keep code, tasks, roles, and plugins easy to read and maintain. Avoid
  over-engineering, unnecessary abstractions, and clever code.
- Reuse existing project patterns unless there is a clear reason to introduce a
  different structure.
- Keep changes small, focused, and easy to review.
- Use descriptive names for variables, tasks, functions, modules, and files.
- Add comments only to explain non-obvious decisions or behavior.
- When several approaches work, choose the one with the least operational risk
  and lowest maintenance cost.

## Configuration, startup, and commands

- The Go module path is `github.com/jouleion/busyness-sensing/backend` and
  `go.mod` currently targets Go 1.27.0. Keep imports consistent with the module
  path.
- The default config file is `configs/configs.webserver.yml`, resolved relative
  to the current working directory. `go run . start --config <path>` selects a
  different file.
- Startup is currently `cmd.Execute()` → config load → logger initialization
  → InfluxDB client initialization and health check → HTTP server start.
- Keep credentials out of committed YAML. Add environment/secret-backed
  configuration for InfluxDB and MQTT-related credentials if the backend ever
  needs them; the backend should not need MQTT credentials for read-only API
  queries.
- Set `influxdb.host` to the InfluxDB origin/root URL with no path prefix; the SDK
  resolves the startup availability check to root-level `/ping`. The supplied
  Compose host `http://influxdb3:8181` already meets this requirement.
- The local InfluxDB Compose config disables authentication and binds its host
  port to loopback. Enable authentication and keep its token in the environment
  before exposing the service beyond the local development machine.
- Create the InfluxDB database before writing with
  `docker compose exec -T influxdb3 influxdb3 create database --token local-dev-token busyness_sensing`.
  In the supplied authentication-disabled local setup, this placeholder avoids
  the malformed authorization header caused by the empty `INFLUXDB3_AUTH_TOKEN`
  environment value. Use a valid token when authentication is enabled. The local
  Telegraf output targets that database.
- Format Go code with `gofmt`. Useful commands from this directory are
  `go run . --help`, `go run . start`, `go test ./...`, and `go vet ./...`.

## Development workflow

For development changes in this repository, use the relevant skills from the
installed Superpowers plugin before implementation. Start with
`superpowers:using-superpowers`, then apply the workflow that fits the task:

- New behavior or features: `superpowers:brainstorming`, followed by the
  applicable planning and implementation skills.
- Bugs or unexpected behavior: `superpowers:systematic-debugging`.
- Multi-step implementation work: `superpowers:writing-plans`, then the
  selected execution workflow.
- Before reporting a change complete: `superpowers:verification-before-completion`.

Read and follow the current skill instructions available in the environment;
the plugin itself is managed outside this repository and should not be copied
or vendored here.
