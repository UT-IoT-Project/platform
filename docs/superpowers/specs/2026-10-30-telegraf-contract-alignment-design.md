# Telegraf Contract Alignment Design

## Intent

Make the firmware-facing telemetry sample, InfluxDB query fields, HTTP sensor names, and OpenAPI response keys use the current Telegraf contract. The current API and examples still use `temp`, `hum`, and `airresistance`, which do not match Telegraf's `temperature`, `humidity`, and `iaq` fields.

## Contract

- ESP32 publishes to `sensors/<buildingName>/<roomNumber>` without a timestamp. Mosquitto adds a Unix millisecond `timestamp` before Telegraf receives the event.
- Required scalar payload fields are `temperature`, `pressure`, `humidity`, `voc`, `iaq`, `mic_vol`, and `busy_level`; `co2equivalent` remains optional. `PIRs` contains stable uppercase `ID` and numeric `value` pairs.
- Telegraf writes `room_telemetry` with `building` and `room` tags, and `pir_reading` with `building`, `room`, and `pir_id` tags. Remove the redundant `temp` parser field. Preserve broker receipt time for both measurements.
- HTTP uses the same external sensor names. `PIRs` is the query and response name; entries expose uppercase `ID`. The existing time-range request, response envelope, per-sensor arrays, grouped PIR readings, Amsterdam formatted timestamps, validation, and status behavior remain.
- Old external names `temp`, `hum`, `airresistance`, and `PIR` are rejected rather than aliased. Internal domain constants can keep their current names where they are technology-independent; the air resistance constant should become an IAQ constant for clarity.

## Approaches

The chosen approach updates Telegraf's redundant `temp` entry and aligns the Go API, OpenAPI, helper, examples, and tests to Telegraf's existing field names. Keeping old API aliases would leave two public contracts and conflict with the requested exact match. Reshaping the historical API into a single MQTT event would remove its time-range semantics, so it is outside this change.

## Constraints and risks

Firmware is maintained in another repository. This backend change documents the new payload but cannot update firmware here; the firmware owner must adopt these names before deployment. Existing InfluxDB points under old columns remain historical data and are not migrated. Changes stay within the current hexagonal layers and add no database access to clients.
