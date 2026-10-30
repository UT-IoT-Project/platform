# Firmware Payload Contract Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Accept the exact JSON emitted by `preparePayload()` and expose the same sensor names and PIR property types through the historical API.

**Architecture:** Keep Mosquitto timestamping, Telegraf ingestion, InfluxDB querying, and HTTP mapping in their existing layers. Use firmware names for persisted fields and API names; retain internal enum names.

**Tech Stack:** Telegraf JSON v2, Bash/Python helper, Go, InfluxDB 3, OpenAPI 3.1.

**Spec:** `docs/superpowers/specs/2026-10-30-firmware-payload-contract-design.md`

## Global Constraints

- Firmware source of truth: `../esp32/src/main.cpp:preparePayload()`.
- Keep broker receipt `timestamp` as point time and topic `sensors/<building>/<room>`.
- Wireless counts are JSON integers in 0..4294967295.
- HTTP PIR entries use numeric `ID` and numeric `activity`.

## Review Focus

- Upper uint32 boundary survives broker parsing and API mapping.
- Numeric PIR IDs become stable InfluxDB tags and return as numeric JSON.
- All firmware scalar keys are stored and queryable.
- Firmware QoS 0 behavior is documented accurately.
- Old field aliases do not silently appear in the public API.

---

### Task 1: MQTT ingestion and helper

**Files:** `configs/telegraf.conf`, `utils/insert_mqtt.sh`, `tests/test_insert_mqtt.py`, `tests/mosquitto_ingress_timestamp.sh`.

- [x] Write helper tests for exact firmware payload keys, numeric PIR IDs, and unsigned count boundaries; observe failure.
- [x] Update Telegraf field and PIR object paths to firmware names, with PIR `ID` as `pir_id` tag and `activity` as integer field.
- [x] Update helper output, validation, and tests; observe passing tests.
- [x] Verify broker timestamp plugin preserves max uint32 values and PIR objects.

### Task 2: Go historical API

**Files:** `internal/driver/http/request/readings.go`, `internal/driver/http/request/readings_test.go`, `internal/driver/http/response/readings_response.go`, `internal/driver/http/response/readings_response_test.go`, `internal/driven/database/influxdb/readings.go`, `internal/driven/database/influxdb/readings_test.go`, `internal/driver/http/readings_handler_test.go`.

- [x] Write failing tests for firmware sensor names, numeric PIR response, and uint32 count boundary.
- [x] Map room columns and PIR activity, update query names and response DTOs, enforce unsigned boundaries.
- [x] Run focused tests and `go test ./...`.

### Task 3: Contract documentation and integration review

**Files:** `docs/api/openapi.yaml`, `README.md`, `AGENTS.md`.

- [x] Update Swagger names, integer schemas, and examples to match firmware.
- [x] Update current contract prose and describe firmware QoS 0 accurately.
- [x] Run full tests, `go vet ./...`, helper tests, and available MQTT/Telegraf integration checks.
- [x] Review the whole diff for any stale public names or incompatible types.
