#!/usr/bin/env bash
# Read sensor messages: "live" from the MQTT broker, "history" from InfluxDB.
set -euo pipefail
cd "$(dirname "$0")/.."  # so the docker compose fallback finds docker-compose.yml

MQTT_HOST=${MQTT_HOST:-127.0.0.1}
MQTT_PORT=${MQTT_PORT:-1883}
INFLUX_URL=${INFLUX_URL:-http://127.0.0.1:8181}
INFLUX_DB=${INFLUX_DB:-busyness_sensing}

usage() {
  cat <<'EOF'
Usage:
  ./utils/read_mqtt.sh live    [-b BUILDING] [-r ROOM] [-n COUNT]
  ./utils/read_mqtt.sh history [-b BUILDING] [-r ROOM] [-s SINCE] [-n LIMIT] [--pir] [--format FMT]

live     Streams messages from the MQTT broker until Ctrl-C (or COUNT messages).
         Topic filter is sensors/<building|+>/<room|+>. Includes messages from
         every client. Retained messages, if any, appear first.
history  Queries InfluxDB for stored readings, newest first.

Options:
  -b, --building NAME   filter by building (default: all)
  -r, --room NAME       filter by room (default: all)
  -n, --count/--limit N live: stop after N messages. history: max rows (default 20)
  -s, --since SPAN      history window: number + s/m/h/d, e.g. 30m, 6h, 2d (default 1h)
      --pir             history: read pir_reading instead of room_telemetry
      --format FMT      history: pretty (default), json, jsonl or csv
      --dry-run         print the command/query instead of running it
  -h, --help

Env: MQTT_HOST (127.0.0.1), MQTT_PORT (1883), MQTT_USER, MQTT_PASS,
     INFLUX_URL (http://127.0.0.1:8181), INFLUX_DB (busyness_sensing),
     INFLUXDB3_AUTH_TOKEN (only if InfluxDB auth is enabled)

Examples:
  ./utils/read_mqtt.sh live
  ./utils/read_mqtt.sh live -b gui-test
  ./utils/read_mqtt.sh history -b gui-test -r 101 -s 30m -n 10
  ./utils/read_mqtt.sh history --pir -s 2h --format csv
EOF
}

die() { echo "error: $*" >&2; echo "run with --help for usage" >&2; exit 1; }

is_name() { [[ $1 =~ ^[A-Za-z0-9._-]+$ ]]; }

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

case ${1:-} in
  live|history) mode=$1; shift ;;
  -h|--help|"") usage; exit 0 ;;
  *) die "first argument must be 'live' or 'history'" ;;
esac

building="" room="" since="1h" limit="" table="room_telemetry" format="pretty" dry_run=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help) usage; exit 0 ;;
    --dry-run) dry_run=1; shift ;;
    --pir) table="pir_reading"; shift ;;
    -b|--building|-r|--room|-s|--since|-n|--count|--limit|--format)
      [[ $# -ge 2 ]] || die "$1 needs a value"
      case $1 in
        -b|--building) building=$2 ;;
        -r|--room) room=$2 ;;
        -s|--since) since=$2 ;;
        -n|--count|--limit) limit=$2 ;;
        --format) format=$2 ;;
      esac
      shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ -z $building ]] || is_name "$building" || die "building may only contain letters, digits, . _ -"
[[ -z $room ]] || is_name "$room" || die "room may only contain letters, digits, . _ -"
[[ -z $limit ]] || [[ $limit =~ ^[1-9][0-9]*$ ]] || die "count/limit must be a positive integer"

if [[ $mode == live ]]; then
  topic_filter="sensors/${building:-+}/${room:-+}"
  args=(-q 1 -t "$topic_filter" -F '%I  %t  %p')
  if [[ -n $limit ]]; then args+=(-C "$limit"); fi
  build_cmd mosquitto_sub "${args[@]}"
  if [[ $dry_run -eq 1 ]]; then echo "${cmd[*]}"; exit 0; fi
  echo "listening on $topic_filter (Ctrl-C to stop)" >&2
  exec "${cmd[@]}"
fi

# history
[[ $since =~ ^([0-9]+)([smhd])$ ]] || die "--since must look like 30m, 6h, 2d"
case ${BASH_REMATCH[2]} in
  s) unit=seconds ;; m) unit=minutes ;; h) unit=hours ;; d) unit=days ;;
esac
interval="${BASH_REMATCH[1]} $unit"
case $format in pretty|json|jsonl|csv) ;; *) die "--format must be pretty, json, jsonl or csv" ;; esac

query="SELECT * FROM $table WHERE time >= now() - INTERVAL '$interval'"
if [[ -n $building ]]; then query+=" AND building = '$building'"; fi
if [[ -n $room ]]; then query+=" AND room = '$room'"; fi
query+=" ORDER BY time DESC LIMIT ${limit:-20}"

if [[ $dry_run -eq 1 ]]; then
  echo "GET $INFLUX_URL/api/v3/query_sql  db=$INFLUX_DB  format=$format"
  echo "$query"
  exit 0
fi

curl_args=(-sS --fail-with-body -G "$INFLUX_URL/api/v3/query_sql"
  --data-urlencode "db=$INFLUX_DB" --data-urlencode "q=$query" --data-urlencode "format=$format")
if [[ -n ${INFLUXDB3_AUTH_TOKEN:-} ]]; then
  curl_args+=(-H "Authorization: Bearer $INFLUXDB3_AUTH_TOKEN")
fi
curl "${curl_args[@]}"
echo
