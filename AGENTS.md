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
       │ publish JSON over MQTT
       ▼
Mosquitto broker (gateway)
       │ Telegraf subscribes and parses messages
       ▼
InfluxDB (time-series storage)
       ▲ read/query through the InfluxDB adapter
       │
Go HTTP API ─────────────► UI, Python/ML clients, other API consumers
```

Responsibilities:

- **ESP32 firmware** is maintained in a separate repository. It samples sensors
  and publishes telemetry; it should not know about InfluxDB or the Go API.
- **Mosquitto** routes MQTT publications to subscribers. It is a broker, not
  the historical data store.
- **Telegraf** is the ingestion bridge. It subscribes to MQTT, parses and maps
  message/topic data, then writes points to InfluxDB. Keep ingestion out of the
  Go HTTP request path.
- **InfluxDB** stores timestamped readings and serves time-range queries to the
  Go API. The selected InfluxDB major version/edition and its client/query API
  must be confirmed when the adapter is implemented.
- **Go backend** exposes a controlled HTTP API and owns request validation,
  authorization, query limits, response formats, and application use cases. It
  reads from InfluxDB; Telegraf owns MQTT ingestion.
- **UI and Python/ML clients** call the Go API. The API is the public boundary
  for reading project data.

### Telemetry contract and cross-repository changes

The firmware repository is external to this backend repository, so this guide
does not claim a current MQTT topic or payload shape. Before implementing or
changing ingestion or API queries, confirm the current firmware contract with
its owner and align the Telegraf mapping and any sample data with it. Treat
proposed payloads as proposals until those components agree. Use a device
measurement timestamp in UTC; consider recording a separate gateway receipt
time if late or clock-skewed readings need diagnosis.

If multiple devices can publish for one room, include a stable `device_id` in
the topic or payload so readings remain distinguishable. InfluxDB fields are
scalar values. For a variable-length PIR array, preserve sensor identity
explicitly (for example, array entries with stable `id` and `value`
properties), or transform the data into named fields before writing. Do not
flatten anonymous values in a way that loses which PIR produced them. For
InfluxDB, use tags for string identifiers such as building, room, device, and
PIR ID; use fields for numeric measurements. Keep timestamps as timestamps,
not fields or tags.

MQTT QoS 1 delivery can repeat messages. Ingestion mappings should make
retransmission behavior deliberate and retain enough timestamp precision to
distinguish real samples. Do not publish historical sensor readings as retained
MQTT messages; retained messages represent the latest value on a topic, not a
reading history.

## Backend architecture: hexagonal boundaries

Keep business behavior in the core and make external technologies adapters.
Dependencies point inward: adapters may depend on core interfaces and domain
types, while core packages must not import Echo, Viper, zerolog, InfluxDB
clients, or MQTT libraries.

| Path | Boundary and responsibility |
| --- | --- |
| `cmd/` | CLI entry points and dependency wiring. Construct adapters and services here. |
| `configs/` | Local YAML configuration defaults. Never commit real credentials. |
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
- `Dockerfile` packages the backend, and `docker-compose.yml` defines a backend
  service. The Compose file does not configure the broker, Telegraf, or
  InfluxDB.
- `internal/core/domain/`, `internal/core/port/`, and
  `internal/core/service/` contain package placeholders only.
- `internal/driven/database/influxdb/` is a package placeholder; there is no
  InfluxDB client, connection setup, repository, or query yet.
- There is no Go MQTT subscriber. Telegraf is intended to own that role.

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
  `go.mod` currently targets Go 1.26.0. Keep imports consistent with the module
  path.
- The default config file is `configs/configs.general.yaml`, resolved relative
  to the current working directory. `go run . start --config <path>` selects a
  different file.
- Startup is currently `cmd.Execute()` → config load → logger initialization
  → HTTP server start.
- Keep credentials out of committed YAML. Add environment/secret-backed
  configuration for InfluxDB and MQTT-related credentials if the backend ever
  needs them; the backend should not need MQTT credentials for read-only API
  queries.
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
