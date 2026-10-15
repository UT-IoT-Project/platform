#!/bin/sh
set -eu

image=${MOSQUITTO_TEST_IMAGE:-busyness-mosquitto:test}
if [ -z "${MOSQUITTO_TEST_IMAGE+x}" ]; then
  docker build -q -f Dockerfile.mosquitto -t "$image" . >/dev/null
fi
name="mosquitto-timestamp-test-$$"
output=$(mktemp)
trap 'docker rm -f "$name" >/dev/null 2>&1 || true; rm -f "$output"' EXIT

docker run -d --name "$name" \
  -v "$(pwd)/configs/mosquitto.conf:/mosquitto/config/mosquitto.conf:ro" \
  "$image" >/dev/null

for attempt in 1 2 3 4 5 6 7 8 9 10; do
  if docker exec "$name" mosquitto_sub -h 127.0.0.1 \
    -t healthcheck/test -E >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Create a persistent QoS 1 subscription, then let the message wait offline.
docker exec "$name" mosquitto_sub -h 127.0.0.1 \
  -i timestamp-test -c -q 1 -t sensors/BBB/212 -E >/dev/null
before=$(python3 -c 'import time; print(time.time_ns() // 1000000)')
docker exec "$name" mosquitto_pub -h 127.0.0.1 -q 1 \
  -t sensors/BBB/212 \
  -m '{"pressure":1013.25,"hum":48.2,"temp":22.5,"voc":0.42,"airresistance":12345.6,"mic_vol":0.18,"busy_level":2,"PIRs":[{"id":"pir-1","value":1},{"id":"pir-2","value":0}]}'
sleep 2
docker exec "$name" mosquitto_sub -h 127.0.0.1 \
  -i timestamp-test -c -q 1 -t sensors/BBB/212 -C 1 -W 5 >"$output"
after=$(python3 -c 'import time; print(time.time_ns() // 1000000)')

python3 - "$output" "$before" "$after" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as received:
    event = json.load(received)

timestamp = event["timestamp"]
assert isinstance(timestamp, int), timestamp
assert int(sys.argv[2]) <= timestamp < int(sys.argv[3]) - 1000, timestamp
assert event["pressure"] == 1013.25
assert event["PIRs"] == [
    {"id": "pir-1", "value": 1},
    {"id": "pir-2", "value": 0},
]
print("Broker timestamp was set before offline delivery; sensor data survived.")
PY

# Neither invalid JSON nor a publisher-supplied timestamp may reach a subscriber.
for payload in \
  '' \
  '{"temp":22} trailing' \
  '{"temp":22,"timestamp":1}'; do
  docker exec "$name" mosquitto_pub -h 127.0.0.1 -q 1 \
    -t sensors/BBB/212 -m "$payload" >/dev/null 2>&1 || true
  if docker exec "$name" mosquitto_sub -h 127.0.0.1 \
    -i timestamp-test -c -q 1 -t sensors/BBB/212 -C 1 -W 2 >"$output" 2>/dev/null; then
    printf 'Rejected sensor payload reached subscriber: %s\n' "$payload" >&2
    exit 1
  fi
done

docker exec "$name" mosquitto_sub -h 127.0.0.1 \
  -t healthcheck/test -E >/dev/null
printf 'Malformed JSON and publisher timestamps were rejected.\n'
