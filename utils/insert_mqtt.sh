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
  -H, --humidity N
  -t, --temperature N
      --voc N
      --iaq N
      --mic-vol N       sent as "mic_vol"
      --busy-level N    integer from 0 through 255, sent as "busy_level"
      --wifi-packets N  unsigned 32-bit integer sent as "wifiPackets"
      --ble-adverts N
                        unsigned 32-bit integer sent as "bleAdverts"
  -c, --co2-equivalent N

Optional:
  -p, --pirs LIST        numeric ID=activity pairs, e.g. 1=0,2=1
      --raw JSON         send this payload as-is instead of building one
      --topic TOPIC      override the topic (required with --raw if no -b/-r)
      --dry-run          print topic and payload, do not publish
  -h, --help

Env: MQTT_HOST (127.0.0.1), MQTT_PORT (1883), MQTT_USER, MQTT_PASS

Example:
  ./utils/insert_mqtt.sh -b gui-test -r 101 --pressure 1013.25 -H 44 -t 21.5 --voc 0.42 --iaq 12345.6 --mic-vol 0.18 --busy-level 2 --wifi-packets 42 --ble-adverts 17 -c 600 -p 1=1,2=0
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

building="" room="" pressure="" humidity="" temperature="" co2Equivalent="" voc="" iaq="" mic_vol="" busy_level="" wifi_packets="" ble_adverts="" pirs="" raw="" topic="" dry_run=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help) usage; exit 0 ;;
    --dry-run) dry_run=1; shift ;;
    -b|--building|-r|--room|--pressure|-t|--temperature|-H|--humidity|-c|--co2-equivalent|--voc|--iaq|--mic-vol|--mic_vol|--busy-level|--wifi-packets|--ble-adverts|-p|--pirs|--raw|--topic)
      [[ $# -ge 2 ]] || die "$1 needs a value"
      case $1 in
        -b|--building) building=$2 ;;
        -r|--room) room=$2 ;;
        --pressure) pressure=$2 ;;
        -t|--temperature) temperature=$2 ;;
        -H|--humidity) humidity=$2 ;;
        -c|--co2-equivalent) co2Equivalent=$2 ;;
        --voc) voc=$2 ;;
        --iaq) iaq=$2 ;;
        --mic-vol|--mic_vol) mic_vol=$2 ;;
        --busy-level) busy_level=$2 ;;
        --wifi-packets) wifi_packets=$2 ;;
        --ble-adverts) ble_adverts=$2 ;;
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
  [[ -n $pressure && -n $humidity && -n $temperature && -n $voc && -n $iaq && -n $mic_vol && -n $busy_level && -n $wifi_packets && -n $ble_adverts && -n $co2Equivalent ]] ||
    die "pressure, humidity, temperature, voc, iaq, mic_vol, busy_level, wifiPackets, bleAdverts, and co2Equivalent are required (or use --raw)"
  for reading in pressure humidity temperature voc iaq mic_vol co2Equivalent; do
    is_number "${!reading}" || die "$reading must be a number"
  done
  [[ $busy_level =~ ^(0|[1-9][0-9]{0,2})$ ]] && ((busy_level <= 255)) ||
    die "busy_level must be an integer from 0 through 255"

  for reading in wifi_packets ble_adverts; do
    count_value=${!reading}
    # Limit digit length before arithmetic so even oversized inputs cannot overflow.
    [[ $count_value =~ ^(0|[1-9][0-9]{0,9})$ ]] && ((count_value <= 4294967295)) ||
      die "$reading must be an integer from 0 through 4294967295"
  done

  payload="{\"pressure\":$pressure,\"humidity\":$humidity,\"temperature\":$temperature,\"voc\":$voc,\"iaq\":$iaq,\"mic_vol\":$mic_vol,\"busy_level\":$busy_level,\"wifiPackets\":$wifi_packets,\"bleAdverts\":$ble_adverts"
  payload+=",\"co2Equivalent\":$co2Equivalent"

  pir_json=""
  if [[ -n $pirs ]]; then
    IFS=',' read -ra pir_items <<< "$pirs"
    for item in "${pir_items[@]}"; do
      pir_id=${item%%=*}
      pir_value=${item#*=}
      [[ $item == *=* ]] || die "PIR entries must look like ID=activity (got '$item')"
      [[ $pir_id =~ ^(0|[1-9][0-9]{0,9})$ ]] && ((pir_id <= 4294967295)) || die "PIR ID must be an integer from 0 through 4294967295"
      [[ $pir_value =~ ^(0|[1-9][0-9]{0,2})$ ]] && ((pir_value <= 255)) || die "PIR activity for '$pir_id' must be an integer from 0 through 255"
      pir_json+="${pir_json:+,}{\"ID\":$pir_id,\"activity\":$pir_value}"
    done
  fi
  payload+=",\"pirs\":[$pir_json]}"
fi

if [[ $dry_run -eq 1 ]]; then
  echo "topic:   $topic"
  echo "payload: $payload"
  exit 0
fi

build_cmd mosquitto_pub -q 1 -t "$topic" -m "$payload"
"${cmd[@]}"
echo "published to $topic: $payload"
