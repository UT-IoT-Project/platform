# Wireless Counts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add exact integer `wifiPackets` and `bleAdvertisements` readings from ESP32 MQTT payloads through InfluxDB to the historical HTTP API.

**Architecture:** Telegraf writes the two JSON integers as `room_telemetry` integer fields. The Go adapter keeps them as `int64` through core result types to integer JSON responses, while existing float and PIR paths remain. Query retries omit either new column if an older InfluxDB table does not have it yet.

**Tech Stack:** Telegraf JSON v2 TOML, Bash, Python unittest, Go, OpenAPI 3.1 YAML.

**Spec:** `docs/superpowers/specs/2026-10-30-wireless-counts-design.md`

## Global Constraints

- Field and API names are exactly `wifiPackets` and `bleAdvertisements` (case sensitive).
- Both are required signed JSON integers in the ESP32 message; the broker adds Unix millisecond `timestamp`.
- Telegraf stores both as `type = "int"` fields in `room_telemetry` with the existing `building` and `room` tags.
- Go must preserve these readings as `int64`, including values above 2^53, and return JSON integer values.
- HTTP accepts both in repeated `sensors` parameters and returns an array for each requested name; keep existing envelope, time format, partial/empty behavior, and range validation.
- If either new InfluxDB column is absent in a preexisting table, return an empty series for that sensor by retrying without that column; do not hide unrelated query errors.
- Update the MQTT helper, OpenAPI, README, AGENTS.md, and broker test fixture. Firmware lives outside this repository.

## Review Focus

- Generated MQTT JSON has both exact names as integers and no device timestamp: Task 1 test.
- Helper rejects fractional or non-integer counter input: Task 1 test.
- A large int64 from InfluxDB reaches HTTP unchanged as a JSON integer: Task 2 repository and response tests.
- A response with only integer readings is 200; all-empty integer readings yield 404: Task 2 service and handler tests.
- Missing one or both new columns yields empty arrays, while unrelated planning errors still propagate: Task 2 repository tests.

---

### Task 1: MQTT ingestion and publish helper

**Files:** Modify `configs/telegraf.conf`, `utils/insert_mqtt.sh`, `tests/test_insert_mqtt.py`, `tests/mosquitto_ingress_timestamp.sh`.

**Interfaces:** Produces `room_telemetry.wifiPackets` and `room_telemetry.bleAdvertisements` as integer fields. Task 2 queries these exact columns.

- [ ] Update `test_generated_payload_uses_current_contract_without_device_timestamp` to assert both integer keys; add a test rejecting fractional/non-integer counter flags.
- [ ] Run `python3 -m unittest discover -s tests -p 'test_insert_mqtt.py' -v` and observe contract failures before changing production files.
- [ ] Add required `--wifi-packets` and `--ble-advertisements` helper flags, signed integer syntax checks, usage/example text, and corresponding JSON keys. Preserve uppercase PIR `ID` and no device timestamp.
- [ ] Add both Telegraf JSON v2 fields with `type = "int"`; keep topic tags, broker timestamp, QoS, PIR consumer, and optional CO2 unchanged.
- [ ] Update broker ingress test publication to include both new integers.
- [ ] Run the Python suite, `bash -n utils/insert_mqtt.sh tests/mosquitto_ingress_timestamp.sh`, and `git diff --check`; expect exit 0. Commit.

### Task 2: Go integer readings path

**Files:** Create `internal/core/domain/integer_reading.go`. Modify `internal/core/domain/sensors.go`, `internal/core/port/readings.go`, `internal/core/service/readings.go`, `internal/core/service/readings_test.go`, `internal/driven/database/influxdb/readings.go`, `internal/driven/database/influxdb/readings_test.go`, `internal/driver/http/request/readings.go`, `internal/driver/http/request/readings_test.go`, `internal/driver/http/response/readings_response.go`, `internal/driver/http/response/readings_response_test.go`, `internal/driver/http/readings_handler_test.go`.

**Interfaces:** Add `WIFI_PACKETS_SENSOR` and `BLE_ADVERTISEMENTS_SENSOR` domain values. Add `domain.IntegerReading{Timestamp time.Time, Value int64}` and `ReadingsResult.Integers map[domain.SensorType][]domain.IntegerReading`. Keep existing service/repository signatures. Task 3 documents the external API names and integer response schema.

- [ ] Write tests first: new request names accepted; repository SQL selects both fields, decodes Arrow `int64` including 9007199254740993 exactly, and retries on recognized missing-column planning errors; service distinguishes populated and all-empty integer results; response and handler emit integer JSON values under the exact names.
- [ ] Run focused Go tests before implementation with `GOCACHE=/private/tmp/busyness-go-build go test ./internal/core/service ./internal/driven/database/influxdb ./internal/driver/http/...`; observe expected failures. Local httptest sockets may require sandbox escalation.
- [ ] Add the integer domain/result path, Telegraf column selection and strict `int64` decoding, missing-column retries, request names, service emptiness check, and integer response DTO mapping. Keep old float/PIR behavior.
- [ ] Run `gofmt` on changed Go files, focused tests, `GOCACHE=/private/tmp/busyness-go-build go test ./...`, `GOCACHE=/private/tmp/busyness-go-build go vet ./...`, and `git diff --check`; expect exit 0. Commit.

### Task 3: Public contract and examples

**Files:** Modify `docs/api/openapi.yaml`, `README.md`, `AGENTS.md`.

**Interfaces:** Documents the exact Task 1/2 MQTT and HTTP names, integer values, storage fields, and firmware coordination.

- [ ] Add both names to the OpenAPI `sensors` enum, descriptions, response examples, and `SensorData` properties. Add an integer reading schema with `timestamp` and integer `value`; keep existing status and time-range schemas.
- [ ] Update README payload, field table, API list, and helper command. Update AGENTS.md's telemetry contract and current implementation status, noting firmware changes are external and earlier smoke tests predate the counters.
- [ ] Parse the YAML, verify the documented MQTT payload and helper output agree, audit active files for both names, and run `git diff --check`; expect exit 0. Commit.
