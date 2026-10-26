# InfluxDB Connection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Have the API load InfluxDB settings and initialize a live InfluxDB 3 client on startup.

**Architecture:** Add a dedicated Viper config parser and use the InfluxDB 3 Go client in the outbound adapter. Verify the server through `/health` and keep the client open for the API process lifetime.

**Tech Stack:** Go 1.27, Viper, InfluxDB 3 Core, `github.com/InfluxCommunity/influxdb3-go/v2/influxdb3` v2.17.0.

**Spec:** `docs/superpowers/specs/2026-10-26-influxdb-connection-design.md`

## Global Constraints

- Use the existing `configs/configs.webserver.yml` filename and config parser style.
- Compose InfluxDB URL: `http://influxdb3:8181`; database: `busyness_sensing`.
- Local Compose disables auth. Use a documented nonempty placeholder token in YAML because the SDK requires it; the user expressly permits plaintext config.
- Run Go commands with `GOCACHE=/private/tmp/codex-influx-go-build` to avoid sandbox cache denial.
- Do not disturb the primary checkout's uncommitted changes.

## Review Focus

- Missing or malformed InfluxDB settings should fail initialization with a useful error.
- A non-200 health response should fail and close the constructed client.
- A reachable health endpoint should yield a nonnil client that callers can close.
- The Compose backend should wait for InfluxDB to become healthy.
- Existing API config behavior should remain unchanged.

---

### Task 1: Parse InfluxDB configuration

**Files:**
- Modify: `configs/configs.webserver.yml`
- Create: `internal/app/config/influxdb.go`
- Modify: `internal/app/config/init.go`
- Create: `internal/app/config/influxdb_test.go`

**Interfaces:**
- Produces: `config.C.InfluxDB` of type `InfluxDBConfig{Host, Database, Token string}`.
- Consumed by Task 2 `influxdb.Init()`.

- [ ] Write a failing test that loads a temporary YAML file containing `influxdb.host`, `database`, and `token`, then asserts all three fields in `config.C.InfluxDB`.
- [ ] Run `GOCACHE=/private/tmp/codex-influx-go-build go test ./internal/app/config` and confirm the test fails because the fields are not parsed.
- [ ] Add `influxdb` YAML values (`host: http://influxdb3:8181`, `database: busyness_sensing`, `token: local-dev-token`) with a comment that auth is disabled and the token is an SDK placeholder; add `InfluxDBConfig` and `parseInfluxDBConfig(*viper.Viper) (InfluxDBConfig, error)` using `UnmarshalKey`; call it in `config.Init`.
- [ ] Rerun the focused test and `go test ./...`; commit the task.

### Task 2: Initialize and hold the live client

**Files:**
- Create: `internal/driven/database/influxdb/init.go`
- Create: `internal/driven/database/influxdb/init_test.go`
- Modify: `cmd/start.go`
- Modify: `docker-compose.yml`
- Modify: `go.mod`, `go.sum`
- Modify: `README.md`

**Interfaces:**
- Consumes: `config.C.InfluxDB` from Task 1.
- Produces: `func Init() (*influxdb3.Client, error)`.

- [ ] Write failing initializer tests with `httptest.Server`: a 200 `/health` response returns a nonnil client; a 503 response returns an error and nil client; missing settings return a clear error. Confirm the focused tests fail before implementation.
- [ ] Add the v2.17.0 SDK dependency. Implement `Init` using the three config values, `influxdb3.New`, and a `GET /health` request with a five-second timeout. Set `Authorization: Bearer <token>` when a token is present. On any health failure, close the SDK client and return the error. Do not expose credentials in errors.
- [ ] Call `influxdb.Init()` after config and logging initialization in `cmd/start.go`, defer `Close()`, then start HTTP. Add `depends_on: influxdb3: condition: service_healthy` for backend in Compose. Update README local backend instructions to describe the Compose host URL and startup dependency.
- [ ] Run focused tests, `go test ./...`, `go vet ./...`, and `docker compose config` if Docker is available; commit the task.
