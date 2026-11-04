# Grafana Setup Design

## Intent

Add a local Grafana instance to the Compose development stack so a developer
can prove that Grafana reads the existing InfluxDB 3 Core telemetry database
before any dashboard work begins. Grafana is an internal visualization tool;
ordinary UI and Python/ML clients continue to use the Go HTTP API.

## Chosen approach

Use a manually configured InfluxDB data source. Compose starts Grafana and
persists its state, but does not provision a data source, dashboard, alert, or
credentials. This is the smallest way to validate the connection in Grafana's
Explore view and keeps the first Grafana configuration visible to the
developer.

The service uses the official pinned `grafana/grafana:13.2.2` image, waits for
the existing InfluxDB health check, and publishes Grafana only on
`127.0.0.1:3000`. Bind-mount `./volumes/grafana` to `/var/lib/grafana` so the
embedded Grafana database retains users, data sources, dashboards, and other
configuration when containers are rebuilt or recreated. The local service may
run as root only to keep this host bind mount writable across the supported
local Docker environments; it is not an internet-exposed deployment.

## Manual connection settings

After starting the stack, configure Grafana's built-in InfluxDB data source
with the Compose-network URL `http://influxdb3:8181`, product selection
`InfluxDB Enterprise 3.x` (the current Grafana UI has no Core entry), SQL query
language, database `busyness_sensing`, and insecure gRPC enabled because the
local InfluxDB server has no TLS. Supply `local-dev-token` if the SQL form
requires a nonempty token; the current local InfluxDB configuration disables
authentication and accepts it. A later authenticated deployment must use a
read-only database token instead.

## Approaches considered

- **Chosen: manual data source configuration.** It adds only the service and
  durable storage, so the team can validate connectivity before adopting a
  dashboard or provisioning convention.
- Provision an InfluxDB data source through Grafana YAML. This would make
  setup repeatable, but it adds configuration and credential handling that the
  project does not need until the connection is proven.
- Query the Go API from Grafana. This preserves the API as the only application
  data boundary, but would require a JSON API plugin or a purpose-built API
  adapter and is not a simple way to use the existing time-series database.

## Boundaries and verification

No backend, Telegraf, Mosquitto, InfluxDB, or dashboard changes are part of
this work. Document the first-login and data-source steps in the README.
Validate Compose configuration, confirm the Grafana sign-in page is reachable,
manually save and test the SQL data source, and confirm a Compose recreation
retains that data source through the bind mount.
