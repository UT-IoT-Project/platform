# Wireless Counts Contract Design

## Intent

Add `wifiPackets` and `bleAdvertisements` to each ESP32 room telemetry event and expose their time-series readings through the existing HTTP API. Keep the established topic, broker timestamp, PIR identity, and historical query behavior.

## Contract

- Both names are case-sensitive and exact: `wifiPackets` and `bleAdvertisements`.
- Both values are required signed 32-bit JSON integers in ESP32 publications, from -2147483648 through 2147483647 (the ESP32 `int` range). Negative values are allowed. The ESP32 omits `timestamp`; Mosquitto adds Unix milliseconds at broker receipt.
- Telegraf JSON v2 parses both with `type = "int"` into `room_telemetry` fields using the existing `building` and `room` tags and broker timestamp. PIR mapping remains separate. Telegraf does not enforce the signed 32-bit range.
- The HTTP `sensors` query accepts both names. Every requested sensor has an array in a successful response, including an empty array if it has no readings. Returned `value` properties for these two sensors are signed 32-bit JSON integers. The HTTP mapper rejects stored values outside that range with the existing generic 500 response. The existing envelope, Amsterdam timestamp format, 24-hour range limit, partial-result behavior, 404-for-all-empty behavior, and generic 500 behavior remain.
- Preserve integer values across the core and InfluxDB adapter as `int64` rather than passing them through the existing `float64` scalar model. This avoids float conversion inside Go; only signed 32-bit values are supported at the external MQTT and HTTP boundaries. The helper validates generated counts before publishing. The broker parses and reprints numbers through cJSON doubles, which exactly preserve this range but cannot preserve arbitrary int64 values.
- The database is refreshed when the telemetry contract changes. No compatibility path for a preexisting table without these columns is needed; existing query errors remain errors.
- The MQTT helper, examples, OpenAPI schema, and project guide use both names. The firmware repository is external; document the required publisher change without claiming it is implemented here.

## Approach

Extend the current ingestion config and add a small integer-reading path through the existing domain, port, repository, service, and response layers. Reusing `float64` for the two new values would be shorter but can change large integers; a separate `int64` path preserves integer storage internally without changing existing float sensors. The response mapper checks the external range before converting to an `int32` DTO. The API remains a historical time-range endpoint rather than mirroring a single MQTT publication.

## Operational limits

Deploying the required new fields before firmware starts publishing them can cause Telegraf to reject legacy queued payloads. Coordinate the firmware and Telegraf rollout and drain or handle old queued publications. No historical backfill is included.
