#!/usr/bin/env bash
# Publish one telemetry message to the MQTT broker. Telegraf then writes it to InfluxDB.
set -euo pipefail
cd "$(dirname "$0")"  # so the docker compose fallback finds docker-compose.yml

MQTT_HOST=${MQTT_HOST:-127.0.0.1}
MQTT_PORT=${MQTT_PORT:-1883}

usage() {
  cat <<'EOF'
Usage: ./mqtt-insert.sh -b BUILDING -r ROOM -t TEMP -H HUMIDITY [options]

Required (unless --raw is used):
  -b, --building NAME    topic becomes sensors/<building>/<room>
  -r, --room NAME
  -t, --temperature N    sent as "temprature" (spelled as in the contract)
  -H, --humidity N

Optional:
  -c, --co2 N
  -p, --pirs LIST        id=value pairs, e.g. pir1=0,pir2=1
      --timestamp SECS   Unix seconds (default: now)
      --raw JSON         send this payload as-is instead of building one
      --topic TOPIC      override the topic (required with --raw if no -b/-r)
      --dry-run          print topic and payload, do not publish
  -h, --help

Env: MQTT_HOST (127.0.0.1), MQTT_PORT (1883), MQTT_USER, MQTT_PASS

Example:
  ./mqtt-insert.sh -b gui-test -r 101 -t 21.5 -H 44 -c 600 -p pir1=1,pir2=0
EOF
}

die() { echo "error: $*" >&2; echo "run with --help for usage" >&2; exit 1; }

is_number() { [[ $1 =~ ^-?[0-9]+(\.[0-9]+)?$ ]]; }
is_name() { [[ $1 =~ ^[A-Za-z0-9._-]+$ ]]; }

# Picks a local mosquitto client if installed, otherwise runs it inside the mosquitto container.
build_cmd() {
  local tool=$1; shift
  local auth=()
  if [[ -n ${MQTT_USER:-} ]]; then auth=(-u "$MQTT_USER" -P "${MQTT_PASS:-}"); fi
  if command -v "$tool" >/dev/null 2>&1; then
    cmd=("$tool" -h "$MQTT_HOST" -p "$MQTT_PORT" ${auth[@]+"${auth[@]}"} "$@")
  else
    cmd=(docker compose exec -T mosquitto "$tool" -h 127.0.0.1 -p 1883 ${auth[@]+"${auth[@]}"} "$@")
  fi
}

building="" room="" temperature="" humidity="" co2="" pirs="" timestamp="" raw="" topic="" dry_run=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help) usage; exit 0 ;;
    --dry-run) dry_run=1; shift ;;
    -b|--building|-r|--room|-t|--temperature|-H|--humidity|-c|--co2|-p|--pirs|--timestamp|--raw|--topic)
      [[ $# -ge 2 ]] || die "$1 needs a value"
      case $1 in
        -b|--building) building=$2 ;;
        -r|--room) room=$2 ;;
        -t|--temperature) temperature=$2 ;;
        -H|--humidity) humidity=$2 ;;
        -c|--co2) co2=$2 ;;
        -p|--pirs) pirs=$2 ;;
        --timestamp) timestamp=$2 ;;
        --raw) raw=$2 ;;
        --topic) topic=$2 ;;
      esac
      shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done

# Topic
if [[ -z $topic ]]; then
  [[ -n $building && -n $room ]] || die "give -b and -r (or --topic)"
  is_name "$building" || die "building may only contain letters, digits, . _ -"
  is_name "$room" || die "room may only contain letters, digits, . _ -"
  topic="sensors/$building/$room"
fi
[[ $topic != *[+#]* ]] || die "publish topic must not contain + or # wildcards"

# Payload
if [[ -n $raw ]]; then
  payload=$raw
else
  [[ -n $temperature && -n $humidity ]] || die "-t and -H are required (or use --raw)"
  is_number "$temperature" || die "temperature must be a number"
  is_number "$humidity" || die "humidity must be a number"
  timestamp=${timestamp:-$(date +%s)}
  [[ $timestamp =~ ^[0-9]+$ ]] || die "timestamp must be Unix seconds"

  payload="{\"timestamp\":$timestamp,\"temprature\":$temperature,\"humidity\":$humidity"
  if [[ -n $co2 ]]; then
    is_number "$co2" || die "co2 must be a number"
    payload+=",\"co2\":$co2"
  fi

  pir_json=""
  if [[ -n $pirs ]]; then
    IFS=',' read -ra pir_items <<< "$pirs"
    for item in "${pir_items[@]}"; do
      pir_id=${item%%=*}
      pir_value=${item#*=}
      [[ $item == *=* ]] || die "PIR entries must look like id=value (got '$item')"
      is_name "$pir_id" || die "invalid PIR id '$pir_id'"
      is_number "$pir_value" || die "PIR value for '$pir_id' must be a number"
      pir_json+="${pir_json:+,}{\"id\":\"$pir_id\",\"value\":$pir_value}"
    done
  fi
  payload+=",\"PIRs\":[$pir_json]}"
fi

if [[ $dry_run -eq 1 ]]; then
  echo "topic:   $topic"
  echo "payload: $payload"
  exit 0
fi

build_cmd mosquitto_pub -q 1 -t "$topic" -m "$payload"
"${cmd[@]}"
echo "published to $topic: $payload"