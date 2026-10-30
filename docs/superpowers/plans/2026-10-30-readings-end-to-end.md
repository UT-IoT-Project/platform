# Readings Endpoint End-to-End Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing readings GET route return real InfluxDB readings through a core service and the documented JSON API.

**Architecture:** Keep the current batch repository port and its one-query result. Add a core service to classify an empty result, inject it into the Echo server from `cmd/start.go`, and map core results to transport DTOs. Finish with a live Compose smoke test when Docker is available.

**Tech Stack:** Go 1.27.0, Echo v5, InfluxDB 3 Core 3.11, Telegraf 1.40.2, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-10-30-readings-end-to-end-design.md`

## Global Constraints

- Preserve `GET /api/v1/readings/:building/:roomNumber` and the exact case-sensitive sensor names in `docs/api/openapi.yaml`.
- Inputs use Europe/Amsterdam wall time; `port.ReadingsQuery` times are UTC. The end must not precede the start and the elapsed interval must not exceed 24 hours, inclusive.
- No rows across all requested sensors means 404 with the exact message `No data was found for room: {room}. The room may not exist or may have no readings for the requested sensors and time range.`
- If any requested sensor has rows, return 200 with a key for every requested sensor; requested sensors with no rows have empty arrays. Duplicate requested names produce one key.
- Output timestamps use `YYYY-MM-DD HH:mm:ss.SSS` in Europe/Amsterdam. PIR value entries keep uppercase `ID` and numeric `value`; `busy_level` is an integer from -128 through 127.
- Unexpected database or response-mapping errors return the generic 500 envelope. Keep the existing 400 messages and 24-hour behavior.
- The handler must not query InfluxDB directly; core packages must not import Echo, Viper, zerolog, InfluxDB, or MQTT libraries.

## Review Focus

- Empty scalar slices with map keys still yield 404; Task 1 tests this.
- A partial response preserves requested empty arrays and omits unrequested sensors; Task 2 tests this.
- PIR values preserve each string ID next to its value and group by timestamp; Task 2 tests this.
- Fractional, non-finite, or out-of-int8-range `busy_level` values do not silently become an integer; Task 2 tests this.
- Database/query errors do not leak internals into the HTTP response; Task 2 tests this.

---

### Task 1: Core readings service

**Files:**
- Create: `internal/core/service/readings.go`
- Test: `internal/core/service/readings_test.go`

**Interfaces:**
- Consumes: `port.ReadingsRepository.GetReadings(context.Context, port.ReadingsQuery) (port.ReadingsResult, error)`.
- Produces: `service.NewReadingsService(repo port.ReadingsRepository) *ReadingsService`; `(*ReadingsService).GetReadings(ctx context.Context, query port.ReadingsQuery) (port.ReadingsResult, error)`; `service.ErrNoReadings` sentinel.

- [ ] **Step 1:** Write `TestReadingsServiceGetReadings` with a repository fake. Assert forwarding of context/query and populated/partial results, 404-classifying sentinel for empty scalar/PIR results, and propagation of a repository error.
- [ ] **Step 2:** Run `GOCACHE=/private/tmp/codex-go-cache go test ./internal/core/service -run TestReadingsServiceGetReadings -count=1`; expect RED because the service does not exist.
- [ ] **Step 3:** Implement the minimal service and no-data sentinel. Do not add HTTP status codes to core.
- [ ] **Step 4:** Run the focused test; expect PASS. Run `go test ./...` with loopback permission; expect PASS.
- [ ] **Step 5:** Commit only Task 1 files.

### Task 2: Complete HTTP result and error mapping

**Files:**
- Modify: `internal/driver/http/server.go`
- Modify: `internal/driver/http/readings_handler.go`
- Create: `internal/driver/http/readings_response.go`
- Modify: `internal/driver/http/readings_handler_test.go`
- Test: `internal/driver/http/readings_response_test.go`
- Modify: `cmd/start.go`
- Include existing uncommitted 24-hour handler/test changes in this task's commit.

**Interfaces:**
- Consumes: `port.ReadingsService`, `service.ErrNoReadings`, `port.ReadingsQuery`, and `port.ReadingsResult`.
- Produces: `NewServer(readingsService port.ReadingsService) *Server`; a response mapper from query/result to the Swagger 200 DTO; 200/404/500 handler behavior; startup wiring client → repository → service → HTTP.

- [ ] **Step 1:** Add failing HTTP tests for populated scalar and PIR data, partial data with empty arrays, all-empty 404 with exact room message, repository error to generic 500, request context/UTC query forwarding, and invalid busy levels. Retain and adapt the existing 400/24-hour tests.
- [ ] **Step 2:** Run `GOCACHE=/private/tmp/codex-go-cache go test ./internal/driver/http -count=1`; expect RED for missing 200/404 behavior.
- [ ] **Step 3:** Inject the service into `Server`, call it with the validated query and request context, and map results to the documented JSON DTO. Validate `busy_level` and non-finite scalar values before encoding. Wire the existing SDK client into the repository, service, and HTTP server in `cmd/start.go`, preserving client close behavior. Do not add SQL to HTTP.
- [ ] **Step 4:** Run the HTTP tests; expect PASS. Run `go test ./...` with loopback permission; expect PASS.
- [ ] **Step 5:** Commit Task 2 files, including the pre-existing 24-hour validation change.

### Task 3: API contract and project status

**Files:**
- Modify: `docs/api/openapi.yaml`
- Modify: `AGENTS.md`
- Create: `docs/superpowers/specs/2026-10-30-readings-end-to-end-design.md` and `docs/superpowers/plans/2026-10-30-readings-end-to-end.md` (created during planning; commit with this task).

**Interfaces:**
- Consumes: Task 2's startup chain and HTTP response behavior.
- Produces: OpenAPI documents 200, 400, 404, 500 actual behavior.

- [ ] **Step 1:** Update OpenAPI's planned-status text and 404 dynamic message. Replace the all-empty 200 example with a partial-data example, retain requested empty arrays, and preserve the 24-hour 400 contract. Update `AGENTS.md` status accurately.
- [ ] **Step 2:** Run `go test ./...`, `go vet ./...`, and a YAML parse check; expect all to pass. Do not add a test that merely mirrors constructor forwarding.
- [ ] **Step 3:** Commit Task 3 files.

### Task 4: Live ingestion and query verification

**Files:**
- Modify only the relevant adapter/config/test files if live evidence reveals a defect; no speculative changes.

**Interfaces:**
- Consumes: completed route, service, repository, Compose stack, and telemetry contract.
- Produces: a recorded smoke result for MQTT → Telegraf → InfluxDB 3 Core → API, or a precise environment blocker if the Docker daemon remains unavailable.

- [ ] **Step 1:** Check Docker daemon availability. If available, build/start Compose, ensure database `busyness_sensing` exists, publish a representative sensor message through Mosquitto, and verify the API returns the requested scalar and PIR readings. Verify 404 for an empty range and 400 for a range over 24 hours.
- [ ] **Step 2:** If live verification reveals a repository/SQL/config defect, add a failing regression test, make the smallest fix, and rerun the focused and full Go suites plus the smoke check.
- [ ] **Step 3:** Record exact smoke commands and outputs in the task report. Commit only any code/config changes; if Docker is unavailable, report the blocker and leave the end-to-end verification claim open.
