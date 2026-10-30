# Readings HTTP Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register and validate the Swagger readings request while converting Amsterdam timestamps to UTC for a future core service.

**Architecture:** A request DTO in `internal/driver/http/request` accepts Echo path and query binding, validates the contract, and produces a `port.ReadingsQuery` with UTC times. The HTTP handler owns JSON error mapping; the server registers the route inline from `NewServer`. No service is wired in this change.

**Tech Stack:** Go 1.27.0, Echo v5, standard library time package.

**Spec:** `docs/superpowers/specs/2026-10-30-readings-http-boundary-design.md`

## Global Constraints

- Route: `GET /api/v1/readings/:building/:roomNumber`.
- Request timezone: `Europe/Amsterdam`; core query times must be UTC.
- Preserve the exact case-sensitive sensor names and error messages in `docs/api/openapi.yaml`.
- A valid request returns the generic documented 500 envelope until a service is wired.
- Keep Echo and InfluxDB out of core code; no database access in the handler.

## Review Focus

- Missing `start_time` or `end_time` returns `Invalid timestamp format.`; Task 1 tests both.
- An impossible date or a spring clock gap returns `Invalid timestamp format.`; Task 1 tests both.
- An ambiguous autumn clock hour returns `Invalid timestamp format.`; Task 1 tests it.
- Repeated `sensors` keys preserve all values; Task 2 tests this through an HTTP request.
- An unsupported or differently cased sensor returns `Unknown sensor name.`; Task 1 and Task 2 test this.

---

### Task 1: Bound request and UTC query conversion

**Files:**
- Create: `internal/driver/http/request/readings.go`
- Test: `internal/driver/http/request/readings_test.go`

**Interfaces:**
- Produces: `request.Readings` with `param:"building"`, `param:"roomNumber"`, `query:"start_time"`, `query:"end_time"`, and `query:"sensors"` fields.
- Produces: `func (r Readings) ToQuery() (port.ReadingsQuery, error)` and exported sentinel errors for invalid timestamp, empty sensors, and unknown sensor.

- [ ] **Step 1:** Write `TestReadingsToQuery` with a summer time, winter time, all nine exact sensor names, and literal expected UTC instants and core sensor values. Write table cases for missing or malformed timestamps, impossible date, spring gap, autumn ambiguity, empty sensor list, and unknown sensor.
- [ ] **Step 2:** Run `go test ./internal/driver/http/request -run TestReadingsToQuery -count=1`; expect failure because `Readings` and `ToQuery` do not exist.
- [ ] **Step 3:** Implement `Readings.ToQuery` with `time.LoadLocation("Europe/Amsterdam")`, exact layout `2006-01-02 15:04:05.000`, UTC conversion, case-sensitive sensor mapping, and DST round-trip/ambiguity checks.
- [ ] **Step 4:** Run `go test ./internal/driver/http/request -run TestReadingsToQuery -count=1`; expect PASS.

### Task 2: Route, handler, and contract status

**Files:**
- Modify: `internal/driver/http/server.go`
- Create: `internal/driver/http/readings_handler.go`
- Test: `internal/driver/http/readings_handler_test.go`
- Modify: `docs/api/openapi.yaml`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: `request.Readings.ToQuery()` and its validation errors from Task 1.
- Produces: `func (s *Server) registerRoutes() *Server`, `func (s *Server) getReadings(c *echo.Context) error`.

- [ ] **Step 1:** Write `TestGetReadingsRoute` using `httptest` against `NewServer().e`, asserting exact 400 JSON envelopes for invalid input, that repeated sensor keys bind, and exact 500 JSON envelope for a valid request.
- [ ] **Step 2:** Run `go test ./internal/driver/http -run TestGetReadingsRoute -count=1`; expect route test failure (404 rather than expected response).
- [ ] **Step 3:** Add inline `registerRoutes` call in `NewServer`, register the GET route, and implement the handler in its own file. Bind path and query with Echo, map validation errors, and return the generic 500 envelope for valid input.
- [ ] **Step 4:** Run `go test ./internal/driver/http -run TestGetReadingsRoute -count=1`; expect PASS.
- [ ] **Step 5:** Update OpenAPI to specify `Europe/Amsterdam` and its DST invalid-time behavior, and update `AGENTS.md` to describe the new HTTP boundary without claiming successful reads.
- [ ] **Step 6:** Run `gofmt` on changed Go files, `go test ./...`, and `go vet ./...`; expect all commands to exit 0.
