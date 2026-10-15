#!/usr/bin/env bash
# Publish one telemetry message to the MQTT broker. Telegraf then writes it to InfluxDB.
set -euo pipefail
cd "$(dirname "$0")/.."  # so the docker compose fallback finds docker-compose.yml

MQTT_HOST=${MQTT_HOST:-127.0.0.1}
MQTT_PORT=${MQTT_PORT:-1883}

usage() {
  cat <<'EOF'
Usage: ./utils/insert_mqtt.sh -b BUILDING -r ROOM [readings] [options]

Required (unless --raw is used):
  -b, --building NAME    topic becomes sensors/<building>/<room>
  -r, --room NAME
      --pressure N
  -H, --hum N
  -t, --temp N
      --voc N
      --airresistance N
      --mic-vol N       sent as "mic_vol"
      --busy-level N    integer from -128 through 127

Optional:
  -c, --co2equivalent N
  -p, --pirs LIST        id=value pairs, e.g. pir1=0,pir2=1
      --raw JSON         send this payload as-is instead of building one
      --topic TOPIC      override the topic (required with --raw if no -b/-r)
      --dry-run          print topic and payload, do not publish
  -h, --help

Env: MQTT_HOST (127.0.0.1), MQTT_PORT (1883), MQTT_USER, MQTT_PASS

Example:
  ./utils/insert_mqtt.sh -b gui-test -r 101 --pressure 1013.25 -H 44 -t 21.5 --voc 0.42 --airresistance 12345.6 --mic-vol 0.18 --busy-level 2 -c 600 -p pir1=1,pir2=0
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

building="" room="" pressure="" hum="" temp="" co2equivalent="" voc="" airresistance="" mic_vol="" busy_level="" pirs="" raw="" topic="" dry_run=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help) usage; exit 0 ;;
    --dry-run) dry_run=1; shift ;;
    -b|--building|-r|--room|--pressure|-t|--temp|--temperature|-H|--hum|--humidity|-c|--co2equivalent|--voc|--airresistance|--mic-vol|--mic_vol|--busy-level|--busy_level|-p|--pirs|--raw|--topic)
      [[ $# -ge 2 ]] || die "$1 needs a value"
      case $1 in
        -b|--building) building=$2 ;;
        -r|--room) room=$2 ;;
        --pressure) pressure=$2 ;;
        -t|--temp|--temperature) temp=$2 ;;
        -H|--hum|--humidity) hum=$2 ;;
        -c|--co2equivalent) co2equivalent=$2 ;;
        --voc) voc=$2 ;;
        --airresistance) airresistance=$2 ;;
        --mic-vol|--mic_vol) mic_vol=$2 ;;
        --busy-level|--busy_level) busy_level=$2 ;;
        -p|--pirs) pirs=$2 ;;
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
  [[ -n $pressure && -n $hum && -n $temp && -n $voc && -n $airresistance && -n $mic_vol && -n $busy_level ]] ||
    die "pressure, hum, temp, voc, airresistance, mic_vol, and busy_level are required (or use --raw)"
  for reading in pressure hum temp voc airresistance mic_vol; do
    is_number "${!reading}" || die "$reading must be a number"
  done
  [[ $busy_level =~ ^-?(0|[1-9][0-9]{0,2})$ ]] && ((busy_level >= -128 && busy_level <= 127)) ||
    die "busy_level must be an integer from -128 through 127"

  payload="{\"pressure\":$pressure,\"hum\":$hum,\"temp\":$temp,\"voc\":$voc,\"airresistance\":$airresistance,\"mic_vol\":$mic_vol,\"busy_level\":$busy_level"
  if [[ -n $co2equivalent ]]; then
    is_number "$co2equivalent" || die "co2equivalent must be a number"
    payload+=",\"co2equivalent\":$co2equivalent"
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
