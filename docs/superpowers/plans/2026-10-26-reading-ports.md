# Reading Ports Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the reading domain models and define batch service and repository interfaces.

**Architecture:** Core models distinguish scalar readings from PIR readings. A query selects several sensors; both ports accept the same query and return one typed result. This work defines contracts only.

**Tech Stack:** Go 1.27.0; standard library only for new code.

**Spec:** `docs/superpowers/specs/2026-10-26-reading-ports-design.md`

## Global Constraints

- Preserve the user's uncommitted additions in `internal/core/domain/sensors.go`.
- No HTTP route, service implementation, InfluxDB adapter, or database query in this change.
- Do not add tests that merely mirror declarations; verify compilation with `GOCACHE=/private/tmp/codex-go-cache go test ./...` and run `go vet ./...` with the same cache.
- Work in the existing `feature/define-project-ports` branch. Do not commit the user's pre-existing uncommitted enum edit.

## Review Focus

- PIR identity remains attached to each value in the domain type.
- The result can hold multiple scalar sensors and PIR values from one batch.
- Core files import no transport or database dependencies.
- Existing enum additions remain present after edits.

---

### Task 1: Replace reading domain models

**Files:**
- Delete: `internal/core/domain/single_value_reading.go`
- Delete: `internal/core/domain/multiple_value_reading.go`
- Create: `internal/core/domain/scalar_reading.go`
- Create: `internal/core/domain/pir_reading.go`
- Modify: `internal/core/domain/sensors.go`

**Interfaces:**
- Produces: `ScalarReading{Timestamp time.Time, Value float64}`, `PIRValue{ID string, Value float64}`, and `PIRReading{Timestamp time.Time, Values []PIRValue}` in `domain`.
- Produces corrected constants `TEMPERATURE_SENSOR`, `AIRRESISTANCE_SENSOR`, and `CO2EQUIVALENT_SENSOR`; preserves all nine sensor values and their current order.

- [x] **Step 1:** Replace the generic models with the three concrete types and correct the enum spelling. Keep core types free of HTTP response tags.
- [x] **Step 2:** Run `gofmt` on changed Go files.
- [x] **Step 3:** Run `GOCACHE=/private/tmp/codex-go-cache go test ./...` and `GOCACHE=/private/tmp/codex-go-cache go vet ./...`; both exit 0.
- [x] **Step 4:** Review the diff to confirm the pre-existing enum additions remain.

### Task 2: Add the reading ports

**Files:**
- Create: `internal/core/port/readings.go`
- Modify: `AGENTS.md` only to update its current-status description.

**Interfaces:**
- Consumes: Task 1's `domain.ScalarReading`, `domain.PIRReading`, and `domain.SensorType`.
- Produces: `ReadingsQuery{Building string, Room string, Start time.Time, End time.Time, Sensors []domain.SensorType}`.
- Produces: `ReadingsResult{Scalars map[domain.SensorType][]domain.ScalarReading, PIR []domain.PIRReading}`.
- Produces: `ReadingsService` and `ReadingsRepository`, each with `GetReadings(ctx context.Context, query ReadingsQuery) (ReadingsResult, error)`.

- [x] **Step 1:** Add the port declarations and a short comment that the repository method represents a batch fetch; the future adapter must use one database query.
- [x] **Step 2:** Update `AGENTS.md` to reflect implemented domain models and port declarations, without claiming implementations exist.
- [x] **Step 3:** Run `gofmt` on the new Go file.
- [x] **Step 4:** Run `GOCACHE=/private/tmp/codex-go-cache go test ./...` and `GOCACHE=/private/tmp/codex-go-cache go vet ./...`; both exit 0.
