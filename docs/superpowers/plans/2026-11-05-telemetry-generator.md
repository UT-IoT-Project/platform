# Telemetry Generator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the manual MQTT payload helper with a five-second, room-scoped fake telemetry generator.

**Architecture:** Keep the existing MQTT topic, publisher selection, authentication environment overrides, and Telegraf ingestion path. The Bash helper owns generated state and serializes the agreed telemetry JSON contract; Python tests inspect dry-run output and use a temporary publisher stub to prove repeated live publication.

**Tech Stack:** Bash, Python `unittest`, Mosquitto CLI, Docker Compose fallback.

**Spec:** `docs/superpowers/specs/2026-11-05-telemetry-generator-design.md`

## Global Constraints

- The live command accepts only required building and room identifiers; it generates every sensor value and publishes every five seconds until interrupted.
- Retain `--dry-run` as the one-payload, no-publish inspection path. Remove raw-payload and per-sensor-value inputs.
- Produce the exact telemetry keys `pressure`, `humidity`, `temperature`, `co2Equivalent`, `voc`, `iaq`, `mic_vol`, `busy_level`, `wifiPackets`, `bleAdverts`, and `pirs`; never generate `timestamp`.
- Preserve PIR IDs 1, 2, and 3, numeric activities 0 through 100, unsigned 32-bit integer wireless counts, valid topic identifier validation, MQTT environment overrides, and the local-or-Compose publisher fallback.
- Update README and AGENTS.md so they no longer describe per-reading input or `--raw`.
- Work only in the existing isolated `feature/grafana-setup` worktree and use subagent-driven development with task and final review.

## Review Focus

- A dry-run must never invoke the MQTT publisher, while normal mode must continue beyond one point at a five-second cadence; Task 1 tests both.
- Every generated value must remain valid JSON and satisfy the telemetry type/range contract; Task 1 tests the full payload shape and bounds.
- No generated payload may contain `timestamp`; Task 1 tests its absence.
- Missing/invalid building or room names and removed sensor flags must fail before publishing; Task 1 tests these inputs.
- Documentation must say the generator is development-only and stopped with Ctrl-C; Task 1 updates and reviews the relevant project guidance.

---

### Task 1: Replace manual payload publishing with generated telemetry

**Files:**
- Modify: `utils/insert_mqtt.sh`
- Modify: `tests/test_insert_mqtt.py`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Create during planning: `docs/superpowers/specs/2026-11-05-telemetry-generator-design.md`
- Create during planning: `docs/superpowers/plans/2026-11-05-telemetry-generator.md`

**Interfaces:**
- Consumes: `./utils/insert_mqtt.sh -b BUILDING -r ROOM [--dry-run]` and current MQTT environment overrides.
- Produces: continuous complete payloads to `sensors/<building>/<room>` every five seconds, or one printed payload in dry-run mode.

- [ ] **Step 1:** Replace the existing contract tests with failing tests that call only building and room plus `--dry-run`, assert the complete contract-valid generated JSON shape/ranges and no timestamp, assert invalid identifiers and removed sensor flags fail, and use a temporary `mosquitto_pub` stub to observe at least two normal-mode publications across the five-second interval.
- [ ] **Step 2:** Run `python3 -m unittest tests/test_insert_mqtt.py -v`; expect RED because the existing helper requires explicit sensor values and does not continuously generate them.
- [ ] **Step 3:** Replace `utils/insert_mqtt.sh` with the generated-state loop. Keep topic validation, MQTT host/port/auth overrides, and local-or-Compose publisher selection. Generate bounded, small-step room readings and occupancy-correlated wireless/PIR readings; render contract-valid JSON without a timestamp; use exactly `sleep 5` after each successful live publish. Make `--dry-run` render and print one reading without selecting or calling a publisher.
- [ ] **Step 4:** Update README and AGENTS.md to describe `-b`/`-r` only, the five-second indefinite development generator, Ctrl-C shutdown, `--dry-run`, and the fact that broker receipt timestamping remains unchanged.
- [ ] **Step 5:** Run the focused Python suite, `bash -n utils/insert_mqtt.sh`, `go test ./...` with the workspace temporary Go cache, `git diff --check`, and `docker compose config`; expect all to pass. If Docker is available, run the generator briefly against the Compose stack and verify at least two room telemetry points and associated PIR points arrive in InfluxDB.
- [ ] **Step 6:** Commit Task 1 files with message `feat: generate fake mqtt telemetry`.
