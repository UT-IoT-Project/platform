# Telemetry Generator Design

## Intent

Replace the manual MQTT payload helper with a local development telemetry
generator. A developer supplies only a building and room, then receives a
continuous stream of contract-valid room and PIR readings for Grafana and API
testing.

## Behavior

- `./utils/insert_mqtt.sh -b BUILDING -r ROOM` publishes indefinitely to
  `sensors/<building>/<room>` every five seconds. Stop it with Ctrl-C.
- Retain only `-b`/`--building`, `-r`/`--room`, `--dry-run`, `--help`, and
  the existing `MQTT_HOST`, `MQTT_PORT`, `MQTT_USER`, and `MQTT_PASS`
  environment overrides. Remove raw-payload and per-sensor CLI inputs.
- `--dry-run` validates the two identifiers, prints one generated topic and
  JSON payload, does not publish, and exits. It is the non-network inspection
  path for tests.
- Each payload has every agreed room field and PIR entries with stable numeric
  uppercase IDs 1, 2, and 3. It never includes `timestamp`; Mosquitto adds
  receipt time.
- Generated values remain in plausible development ranges and change by small
  bounded increments between points. `busy_level` is 0 through 3; PIR activity
  and wireless counts rise loosely with it. All wireless counts are unsigned
  32-bit JSON integers.
- Retain the current local `mosquitto_pub` or Compose-container fallback and
  QoS 1 publisher behavior.

## Limits

This is a deterministic-shape development simulator, not a calibrated model of
physical sensor behavior. It does not add a timestamp, store data directly,
change Telegraf, or attempt to emulate ESP32 firmware timing beyond the
five-second publication interval.

## Approach

Use Bash's built-in random source and integer-scaled state to avoid new runtime
dependencies. Initialize a bounded state once, perturb scalar values on each
iteration, derive occupancy-related measurements from the current busy level,
render the existing telemetry JSON shape, publish, then sleep five seconds.
The existing Python contract test becomes a generator-contract test and adds a
controlled live-loop check with a temporary `mosquitto_pub` stub.
