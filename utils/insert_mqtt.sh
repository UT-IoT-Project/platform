#!/usr/bin/env bash
# Generate development telemetry; Mosquitto timestamps it and Telegraf stores it.
set -euo pipefail
cd "$(dirname "$0")/.."  # so the docker compose fallback finds docker-compose.yml

MQTT_HOST=${MQTT_HOST:-127.0.0.1}
MQTT_PORT=${MQTT_PORT:-1883}

usage() {
  cat <<'EOF'
Usage: ./utils/insert_mqtt.sh -b BUILDING -r ROOM [--dry-run]

Generate complete fake room readings every five seconds until Ctrl-C.
This is a development simulator, not calibrated sensor data.

  -b, --building NAME    topic becomes sensors/<building>/<room>
  -r, --room NAME        required with building
      --dry-run          print one topic and payload without publishing
  -h, --help

Env: MQTT_HOST (127.0.0.1), MQTT_PORT (1883), MQTT_USER, MQTT_PASS

Example: ./utils/insert_mqtt.sh -b BBB -r 212
EOF
}

die() { echo "error: $*" >&2; echo "run with --help for usage" >&2; exit 1; }
is_name() { [[ $1 =~ ^[A-Za-z0-9._-]+$ ]]; }

# Picks a local client if installed, otherwise uses the Compose broker container.
build_cmd() {
  local auth=()
  if [[ -n ${MQTT_USER:-} ]]; then auth=(-u "$MQTT_USER" -P "${MQTT_PASS:-}"); fi
  if command -v mosquitto_pub >/dev/null 2>&1; then
    cmd=(mosquitto_pub -h "$MQTT_HOST" -p "$MQTT_PORT" ${auth[@]+"${auth[@]}"})
  else
    cmd=(docker compose exec -T mosquitto mosquitto_pub -h 127.0.0.1 -p 1883 ${auth[@]+"${auth[@]}"})
  fi
}

building="" room="" dry_run=0
while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help) usage; exit 0 ;;
    --dry-run) dry_run=1; shift ;;
    -b|--building|-r|--room)
      [[ $# -ge 2 ]] || die "$1 needs a value"
      case $1 in
        -b|--building) building=$2 ;;
        -r|--room) room=$2 ;;
      esac
      shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[[ -n $building && -n $room ]] || die "give -b and -r"
is_name "$building" || die "building may only contain letters, digits, . _ -"
is_name "$room" || die "room may only contain letters, digits, . _ -"
topic="sensors/$building/$room"

# Scalar state is stored in hundredths, so no external math tools are needed.
# Clamp every step to keep an indefinitely running simulation in plausible ranges.
perturb() {
  local name=$1 minimum=$2 maximum=$3 step=$4 value
  value=$((${!name} + RANDOM % (2 * step + 1) - step))
  if ((value < minimum)); then value=$minimum; fi
  if ((value > maximum)); then value=$maximum; fi
  printf -v "$name" '%d' "$value"
}

render_payload() {
  local name
  for name in pressure humidity temperature co2Equivalent voc iaq mic_vol; do
    printf -v "$name" '%d.%02d' "$((${!name} / 100))" "$((${!name} % 100))"
  done
  printf '{"pressure":%s,"humidity":%s,"temperature":%s,"co2Equivalent":%s,"voc":%s,"iaq":%s,"mic_vol":%s,"busy_level":%d,"wifiPackets":%d,"bleAdverts":%d,"pirs":[{"ID":1,"activity":%d},{"ID":2,"activity":%d},{"ID":3,"activity":%d}]}' \
    "$pressure" "$humidity" "$temperature" "$co2Equivalent" "$voc" "$iaq" "$mic_vol" \
    "$busy_level" "$wifi_packets" "$ble_adverts" "$pir1" "$pir2" "$pir3"
}

pressure=$((101000 + RANDOM % 601))
humidity=$((4000 + RANDOM % 1001))
temperature=$((2000 + RANDOM % 401))
co2Equivalent=$((50000 + RANDOM % 20001))
voc=$((20 + RANDOM % 41))
iaq=$((4000 + RANDOM % 4001))
mic_vol=$((10 + RANDOM % 21))
busy_level=$((RANDOM % 4))

while :; do
  perturb pressure 98000 104000 20
  perturb humidity 3000 7000 50
  perturb temperature 1800 2800 20
  perturb co2Equivalent 40000 160000 1000
  perturb voc 0 200 2
  perturb iaq 0 20000 200
  perturb mic_vol 0 100 2
  if ((RANDOM % 5 == 0)); then perturb busy_level 0 3 1; fi
  # Occupancy bands plus small noise keep counts and PIR activities correlated.
  wifi_packets=$((busy_level * 40 + RANDOM % 11))
  ble_adverts=$((busy_level * 20 + RANDOM % 11))
  pir1=$((busy_level * 25 + RANDOM % 11))
  pir2=$((busy_level * 25 + RANDOM % 11))
  pir3=$((busy_level * 25 + RANDOM % 11))
  # Render in a subshell so formatting preserves the integer state.
  payload=$(render_payload)
  if [[ $dry_run -eq 1 ]]; then
    echo "topic:   $topic"
    echo "payload: $payload"
    exit 0
  fi
  if [[ ${cmd+x} != x ]]; then build_cmd; fi
  "${cmd[@]}" -q 1 -t "$topic" -m "$payload"
  echo "published to $topic: $payload"
  sleep 5
done
