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
  `configs/influxdb3.env`; the Go adapter and its client/query API still need
  to be implemented and verified against this version.
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
- Float32 `pressure`, `hum`, `temp`, `voc`, `airresistance`, and `mic_vol`, plus
  optional float32 `co2equivalent`.
- `busy_level`: int8 (-128 through 127).
- `PIRs`: array entries with stable string `id` and numeric `value` properties.

The starter Telegraf config uses separate MQTT consumers for room readings and
PIR readings. It writes `room_telemetry` points with `building` and `room`
tags, and `pir_reading` points with `building`, `room`, and `pir_id` tags.
Numeric readings are fields. Keep PIR identity attached to the corresponding
value; do not flatten anonymous array values. Reconfirm this mapping if the
external firmware contract changes. The parser converts float32 source
readings to its `float` field type and `busy_level` to `int`; it does not
enforce float32 precision or the int8 range. Both consumers use the broker
`timestamp` as point time, including for messages queued while Telegraf is
offline. The PIR array and each `id`/`value` pair remain unchanged. This is
broker receipt time, not device sampling time: ESP32 buffering and network
delay before receipt can make a reading appear later than it was sampled.
Synchronize the broker clock.

If multiple devices can publish for one room, include a stable `device_id` in
the topic or payload so readings remain distinguishable. InfluxDB fields are
scalar values. For a variable-length PIR array, preserve sensor identity
explicitly (for example, array entries with stable `id` and `value`
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

Project decision: use MQTT QoS 1 for sensor telemetry. Configure firmware
publishers and the Telegraf MQTT consumers to request QoS 1. Mosquitto's
`max_qos 1` caps the listener at QoS 1, but does not force clients to use QoS 1;
QoS 0 remains allowed. The Telegraf consumers use stable client IDs and
persistent sessions so messages can be queued while they are offline. QoS 1
can redeliver messages; the current contract does not make writes idempotent.
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
| `internal/driven/database/influxdb/` | Outbound InfluxDB adapter implementing core ports and mapping query results to core types. |

For a new read endpoint, keep the flow in this direction:

1. The HTTP handler binds and validates an HTTP request and extracts the
   request context.
2. The handler calls a core service/use case using core types, not Echo
   request/response objects.
3. The service applies application rules and calls a port from
   `internal/core/port/`.
4. The InfluxDB adapter implements that port, runs a bounded query, and maps
   results to domain/application types.
5. The handler maps the result to an HTTP response DTO and status code.

Handlers should not contain InfluxDB queries. Core services should not depend
on HTTP details. The adapter should not decide public response formats.

## Current backend status

This module is scaffolding, not a completed API:

- `cmd/start.go` loads configuration, initializes logging, and starts the Echo
  HTTP server.
- `internal/driver/http/server.go` configures Echo middleware but currently
  registers no application routes.
- `Dockerfile.webserver` packages the backend. `docker-compose.yml` defines backend,
  Mosquitto, InfluxDB 3 Core, and Telegraf services. The derived Mosquitto
  image loads the incoming-message timestamp plugin. Telegraf configuration
  writes telemetry into separate room and PIR measurements. Local InfluxDB
  data is stored under `volumes/influxdb3/data/`. Mosquitto and InfluxDB
  healthchecks gate Telegraf startup; Telegraf also has a process liveness
  healthcheck.
- `internal/core/domain/`, `internal/core/port/`, and
  `internal/core/service/` contain package placeholders only.
- `internal/driven/database/influxdb/` is a package placeholder; there is no
  InfluxDB client, connection setup, repository, or query yet.
- There is no Go MQTT subscriber. Telegraf owns ingestion; the Go HTTP server
  and InfluxDB adapter remain scaffolding.

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
  → HTTP server start.
- Keep credentials out of committed YAML. Add environment/secret-backed
  configuration for InfluxDB and MQTT-related credentials if the backend ever
  needs them; the backend should not need MQTT credentials for read-only API
  queries.
- The local InfluxDB Compose config disables authentication and binds its host
  port to loopback. Enable authentication and keep its token in the environment
  before exposing the service beyond the local development machine.
- Create the InfluxDB database before writing with
  `docker compose exec influxdb3 influxdb3 create database busyness_sensing`.
  The local Telegraf output targets that database.
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
