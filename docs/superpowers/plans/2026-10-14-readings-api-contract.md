# Readings API Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a Swagger/OpenAPI contract for requesting sensor readings over a timestamp range.

**Architecture:** Add a standalone OpenAPI description and update the README to distinguish the documented contract from the still-unimplemented HTTP route. No Go server or database behavior changes.

**Tech Stack:** OpenAPI 3.1.0, YAML, Markdown.

**Spec:** `docs/superpowers/specs/2026-10-14-readings-api-design.md`

## Global Constraints

- Method is `GET`; path is `/api/v1/readings/{start_timestamp}/{end_timestamp}`.
- Timestamps use `YYYY-MM-DD HH:mm:ss.SSS` in server local time; URL path spaces are percent-encoded as `%20`.
- `sensors` is a required nonempty query array encoded with repeated keys, such as `?sensors=PIR&sensors=CO2`.
- Supported case-sensitive sensor names are `PIR`, `CO2`, `humidity`, and `temprature`.
- Unknown sensor names return HTTP 400 with message `Unknown sensor name.`
- Responses contain `status`, `message`, and `data`; `status` matches the HTTP status code.
- HTTP 200 uses message `data retrieved successfully`, and requested sensor keys remain present with empty arrays when no readings match.
- HTTP 400 messages are `Invalid timestamp format.`, `Empty list of sensors are not allowed.`, and `Unknown sensor name.`
- The `PIR` response groups all PIR readings by timestamp in a `values` array; each entry contains integer `ID` and numeric `value` fields.
- The current HTTP server has no application route; the OpenAPI file documents a contract and does not implement it.

## Review Focus

- Encoded timestamp path segments preserve the documented timestamp format: show `%20` in the request example.
- Sensor arrays use repeated query keys and show `PIR` as the aggregate sensor name.
- Only the four supported, case-sensitive sensor names are accepted.
- The PIR example groups readings under each timestamp and preserves each integer PIR ID in an `ID` field.
- Empty query results preserve requested keys, including `PIR: []`.
- Invalid timestamp, empty sensor list, and unknown sensor name each have HTTP 400 examples with their exact messages.

---

### Task 1: Publish the initial OpenAPI contract — complete

**Files:**
- Create: `docs/api/openapi.yaml`
- Modify: `README.md`

**Interfaces:**
- Consumes: approved design in `docs/superpowers/specs/2026-10-14-readings-api-design.md`.
- Produces: OpenAPI operation and README route-status note.

- [x] Define the route, timestamp parameters, repeated sensors array, and schemas.
- [x] Add the HTTP response examples and message/status schemas.
- [x] Document URL encoding and route status.
- [x] Review YAML structure and whitespace.

### Task 2: Update supported sensors and aggregate PIR responses — complete; PIR value shape superseded by Task 3

**Files:**
- Modify: `docs/api/openapi.yaml`

**Interfaces:**
- Consumes: approved contract in `docs/superpowers/specs/2026-10-14-readings-api-design.md`.
- Produces: OpenAPI request schema listing `PIR`, `CO2`, `humidity`, and `temprature`; `PIR` responses grouped by timestamp with numeric `values`; HTTP 400 unknown-sensor response.

- [x] **Step 1: Update supported sensor names**: constrain sensor items to the four case-sensitive names and update all request examples to use `PIR` rather than per-PIR IDs.
- [x] **Step 2: Define PIR aggregate readings**: keep scalar reading objects for `CO2`, `humidity`, and `temprature`; initially defined `PIR` readings as `{timestamp, values: number[]}`; Task 3 updates each values entry to preserve its ID.
- [x] **Step 3: Add unknown sensor validation**: add the exact `Unknown sensor name.` HTTP 400 example and include it in the validation error schema.
- [x] **Step 4: Review the artifacts**: check YAML parsing, OpenAPI structure, exact examples/messages, and `git diff --check`. Do not add or change runtime code or run Go tests.


### Task 3: Preserve PIR IDs in aggregate readings

**Files:**
- Modify: `docs/api/openapi.yaml`

**Interfaces:**
- Consumes: updated approved contract in `docs/superpowers/specs/2026-10-14-readings-api-design.md`.
- Produces: `PIRReading.values` as an array of `PIRValue` objects with integer `ID` and numeric `value` properties.

- [x] **Step 1: Update the PIR value schema**: define `components/schemas/PIRValue` with required integer `ID` and numeric `value`, reject extra fields, and reference it from `PIRReading.values`.
- [x] **Step 2: Update PIR examples and descriptions**: show multiple identified PIR value objects at one timestamp and update text that currently says PIR IDs are omitted.
- [x] **Step 3: Review the artifacts**: validate YAML parsing and the PIR structure/examples, and run `git diff --check`. Do not add or change runtime code or run Go tests.
