# Telegraf Contract Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align ESP32 telemetry examples, Telegraf ingestion, the Go readings API, and OpenAPI with one sensor naming contract.

**Architecture:** Keep Telegraf as the MQTT to InfluxDB bridge and the Go service as a read-only historical API. Change external sensor names and database field selections in their existing layers without changing time-range semantics.

**Tech Stack:** Telegraf JSON v2 TOML, Bash, Python unittest, Go, OpenAPI YAML.

**Spec:** `docs/superpowers/specs/2026-10-30-telegraf-contract-alignment-design.md`

## Global Constraints

- Required scalar names: `temperature`, `pressure`, `humidity`, `voc`, `iaq`, `mic_vol`, `busy_level`; `co2equivalent` optional.
- MQTT payload uses `PIRs` entries with lowercase `id` and numeric `value`; broker adds Unix millisecond `timestamp`.
- Telegraf stores `room_telemetry` scalar fields and `pir_reading` with `pir_id` tag and `value` field.
- HTTP external sensor names are `PIRs`, `temperature`, `pressure`, `humidity`, `co2equivalent`, `voc`, `iaq`, `mic_vol`, `busy_level`.
- Reject old HTTP names `PIR`, `temp`, `hum`, `airresistance`; output PIR entry key is lowercase `id`.
- Preserve existing API time range, response envelope, Amsterdam formatted timestamps, grouped PIR results, and status behavior.
- Firmware is external; document the required coordinated change and do not claim firmware was updated.

## Review Focus

- A normal helper payload must have every Telegraf required field and no device timestamp: Task 1 contract test.
- Optional `co2equivalent` omission must still produce a valid helper payload: Task 1 existing test.
- Old HTTP sensor names must be rejected: Task 2 request test.
- New scalar names must select actual InfluxDB columns: Task 2 repository SQL assertion.
- PIR values must retain IDs with lowercase API key: Task 2 HTTP and response tests.

---

### Task 1: Ingestion contract and publisher helper

**Files:** Modify `configs/telegraf.conf`, `utils/insert_mqtt.sh`, `tests/test_insert_mqtt.py`, `tests/mosquitto_ingress_timestamp.sh`.

**Interfaces:** Produces the payload keys and InfluxDB field names in Global Constraints. Task 2 queries those field names.

- [ ] Update `test_generated_payload_uses_current_contract_without_device_timestamp` to assert the new exact payload keys and values; add an assertion that old helper flags are rejected.
- [ ] Run `python3 -m unittest discover -s tests -p 'test_insert_mqtt.py' -v`; confirm the new contract test fails on the old helper.
- [ ] Update the helper's flags, required fields, validation, usage, and JSON construction to emit `temperature`, `humidity`, and `iaq`, with no `temp`, `hum`, or `airresistance` aliases.
- [ ] Remove only the redundant `temp` field from Telegraf's room JSON v2 parser; keep broker timestamp, QoS, and PIR mapping.
- [ ] Change the ingress timestamp test sample payload to the new keys.
- [ ] Run the Python helper suite and `bash -n utils/insert_mqtt.sh tests/mosquitto_ingress_timestamp.sh`; expect exit 0. Commit the task.

### Task 2: Go historical readings API

**Files:** Modify `internal/core/domain/sensors.go`, `internal/driver/http/request/readings.go`, `internal/driver/http/request/readings_test.go`, `internal/driven/database/influxdb/readings.go`, `internal/driven/database/influxdb/readings_test.go`, `internal/driver/http/response/readings_response.go`, `internal/driver/http/response/readings_response_test.go`, `internal/driver/http/readings_handler_test.go`.

**Interfaces:** Consumes Task 1's InfluxDB field names. Produces external query and response names in Global Constraints. Keep core service and port signatures unchanged; rename the domain air-resistance enum to `IAQ_SENSOR` and use it throughout.

- [ ] First change/add tests: request accepts each new name and rejects all old names; repository SQL selects `temperature`, `humidity`, `iaq`; response/handler JSON uses `PIRs`, `temperature`, `humidity`, `iaq`, and lowercase `id`.
- [ ] Run focused Go tests with `GOCACHE=/private/tmp/busyness-go-build go test ./internal/driver/http/... ./internal/driven/database/influxdb/...`; confirm contract failures before code changes. Local socket tests may need sandbox escalation.
- [ ] Update request mapping, domain enum, repository column mapping, response keys, and PIR JSON tag. Keep result grouping and validation unchanged.
- [ ] Run `gofmt` on modified Go files, focused tests, then `GOCACHE=/private/tmp/busyness-go-build go test ./...` and `GOCACHE=/private/tmp/busyness-go-build go vet ./...`; expect exit 0. Commit the task.

### Task 3: Public contract documentation

**Files:** Modify `docs/api/openapi.yaml`, `README.md`, `AGENTS.md`.

**Interfaces:** Documents the exact Task 1/2 contract and external firmware coordination.

- [ ] Update OpenAPI parameter enum, descriptions, examples, SensorData keys, and PIR value key to the names in Global Constraints. Keep its envelope, time-range, and status schemas.
- [ ] Update README's MQTT payload, stored fields, API sensor list, and helper example. Update AGENTS.md's agreed contract and current status without implying firmware has changed in its separate repository.
- [ ] Search active code and docs for legacy names with `rg -n 'airresistance|\bhum\b|\btemp\b|"PIR"|\bPIR:\b' configs internal utils tests README.md AGENTS.md docs/api/openapi.yaml`; confirm no stale external contract names. Run `git diff --check`; expect exit 0. Commit the task.
