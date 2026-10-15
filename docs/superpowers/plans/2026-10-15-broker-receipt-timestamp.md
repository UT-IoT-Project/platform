# Broker Receipt Timestamp Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task by task.

**Goal:** Stamp sensor JSON when Mosquitto receives it and store that time in both InfluxDB measurements.

**Architecture:** A Mosquitto incoming-message plugin adds a Unix millisecond `timestamp` to `sensors/` JSON before subscriber delivery. A small image based on the pinned official Mosquitto image carries the plugin. Telegraf's existing JSON v2 consumers use the field as point time.

**Tech Stack:** Mosquitto 2.1.2 Alpine, C with cJSON, Docker Compose, Telegraf 1.40.2.

**Spec:** The user's request in this conversation: ESP32 sends no timestamp; broker receipt time must be available in the JSON delivered to Telegraf; keep PIR data unchanged and use the simplest practical deployment.

## Tasks

### Task 1: Broker plugin and image

- Test that a QoS 1 sensor event waiting for an offline subscriber retains a broker timestamp and preserves PIR data. Observe the test fail against the stock broker, then pass with the plugin.
- Build the plugin against Mosquitto 2.1.2 headers, using cJSON already present in the official runtime image.
- Load the plugin in Mosquitto config and build the derived image from Compose.
- Reject malformed sensor JSON and preexisting `timestamp` values so every accepted sensor event uses broker time.
- Verify the image builds and the isolated MQTT test passes.

### Task 2: Telegraf and documentation

- Configure both room and PIR JSON v2 parsers to use `timestamp` with `unix_ms`.
- Update README, AGENTS.md, and OpenAPI to distinguish ESP32 payload from broker-enriched payload and describe timestamp semantics and limitations.
- Validate Compose and Telegraf configuration, and run Go tests.

### Task 3: Review and finish

- Review the whole diff for correctness, simplicity, and unintended changes.
- Run the isolated broker test and all available config and Go checks again.
- Report limitations: broker clock accuracy, network/buffering delay, QoS 1 duplicate receipt, millisecond collisions, and the API scaffold status.
