"""Exercise generated telemetry and the publisher boundary without a broker."""

import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "utils" / "insert_mqtt.sh"
BASH = shutil.which("bash")
IDENTIFIERS = ["-b", "BBB", "-r", "212"]


def run_helper(*args, env=None):
    return subprocess.run(
        [BASH, str(SCRIPT), *args], cwd="/", env=env,
        capture_output=True, text=True, check=False, timeout=3,
    )


class InsertMqttContractTest(unittest.TestCase):
    def assert_payload(self, payload):
        ranges = {
            "pressure": (980, 1040), "humidity": (30, 70),
            "temperature": (18, 28), "co2Equivalent": (400, 1600),
            "voc": (0, 2), "iaq": (0, 200), "mic_vol": (0, 1),
        }
        self.assertEqual(set(payload), set(ranges) | {
            "busy_level", "wifiPackets", "bleAdverts", "pirs",
        })
        for key, (low, high) in ranges.items():
            self.assertIs(type(payload[key]), float, key)
            self.assertGreaterEqual(payload[key], low, key)
            self.assertLessEqual(payload[key], high, key)
        busy = payload["busy_level"]
        self.assertIs(type(busy), int)
        self.assertIn(busy, range(4))
        for key in ("wifiPackets", "bleAdverts"):
            self.assertIs(type(payload[key]), int)
            self.assertGreaterEqual(payload[key], 0)
            self.assertLessEqual(payload[key], 4294967295)
        # Occupancy bands allow noise but rule out unrelated random counts.
        self.assertLessEqual(abs(payload["wifiPackets"] - busy * 40), 10)
        self.assertLessEqual(abs(payload["bleAdverts"] - busy * 20), 10)
        self.assertEqual([pir["ID"] for pir in payload["pirs"]], [1, 2, 3])
        for pir in payload["pirs"]:
            self.assertEqual(set(pir), {"ID", "activity"})
            self.assertIs(type(pir["ID"]), int)
            self.assertIs(type(pir["activity"]), int)
            self.assertIn(pir["activity"], range(101))
            self.assertLessEqual(abs(pir["activity"] - busy * 25), 10)

    def test_dry_run_generates_one_complete_reading_without_publisher(self):
        with tempfile.TemporaryDirectory() as temp:
            # No MQTT or Docker client is available in this PATH.
            (Path(temp) / "dirname").symlink_to(shutil.which("dirname"))
            env = dict(os.environ, PATH=temp)
            for _ in range(12):
                result = run_helper(*IDENTIFIERS, "--dry-run", env=env)
                self.assertEqual(result.returncode, 0, result.stderr)
                lines = result.stdout.splitlines()
                self.assertEqual(len(lines), 2)
                self.assertEqual(lines[0], "topic:   sensors/BBB/212")
                self.assert_payload(json.loads(lines[1].removeprefix("payload: ")))

    def test_missing_and_invalid_identifiers_fail(self):
        for args in ([], ["-b", "BBB"], ["-r", "212"], ["-b"]):
            with self.subTest(args=args):
                self.assertNotEqual(run_helper(*args, "--dry-run").returncode, 0)
        for flag in ("-b", "-r"):
            for value in ("", "a/b", "a+b", "#", "two words", "a\nb"):
                with self.subTest(flag=flag, value=value):
                    args = IDENTIFIERS.copy()
                    args[args.index(flag) + 1] = value
                    self.assertNotEqual(run_helper(*args, "--dry-run").returncode, 0)

    def test_long_identifier_options_accept_safe_topic_names(self):
        result = run_helper("--building", "B_1.2-3", "--room", "R_1.2-3", "--dry-run")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("topic:   sensors/B_1.2-3/R_1.2-3", result.stdout)

    def test_manual_payload_and_sensor_options_are_removed(self):
        for flag in ("--raw", "--topic", "--pressure", "-H", "--humidity",
                     "-t", "--temperature", "-c", "--co2-equivalent", "--voc",
                     "--iaq", "--mic-vol", "--mic_vol", "--busy-level",
                     "--wifi-packets", "--ble-adverts", "-p", "--pirs"):
            with self.subTest(flag=flag):
                result = run_helper(*IDENTIFIERS, flag, "1", "--dry-run")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(f"unknown argument: {flag}", result.stderr)

    def test_help_needs_no_identifiers(self):
        result = run_helper("--help")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("--dry-run", result.stdout)

    def test_live_loop_publishes_five_seconds_apart_with_mqtt_overrides(self):
        records = self.capture_publications(compose=False)
        self.assertGreaterEqual(records[1]["time"] - records[0]["time"], 5)
        self.assertLess(records[1]["time"] - records[0]["time"], 7)
        payloads = []
        for record in records:
            args = record["args"]
            self.assertEqual(args[:10], ["-h", "test-host", "-p", "2883", "-u", "test-user", "-P", "test-pass", "-q", "1"])
            self.assertEqual(args[10:12], ["-t", "sensors/BBB/212"])
            self.assertEqual(args[12], "-m")
            payload = json.loads(args[13])
            self.assert_payload(payload)
            payloads.append(payload)
        for key, step in {"pressure": .2, "humidity": .5, "temperature": .2,
                          "co2Equivalent": 10, "voc": .02, "iaq": 2,
                          "mic_vol": .02, "busy_level": 1,
                          "wifiPackets": 60, "bleAdverts": 40}.items():
            self.assertLessEqual(abs(payloads[1][key] - payloads[0][key]), step + 1e-9, key)

    def test_compose_fallback_publishes_from_repository_root(self):
        record = self.capture_publications(compose=True)[0]
        self.assertEqual(record["cwd"], str(ROOT))
        self.assertEqual(record["args"][:10], ["compose", "exec", "-T", "mosquitto", "mosquitto_pub", "-h", "127.0.0.1", "-p", "1883", "-u"])
        self.assert_payload(json.loads(record["args"][-1]))

    def capture_publications(self, compose):
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            log = directory / "publications"
            (directory / "dirname").symlink_to(shutil.which("dirname"))
            (directory / "sleep").symlink_to(shutil.which("sleep"))
            publisher = directory / ("docker" if compose else "mosquitto_pub")
            publisher.write_text(
                f"#!{shutil.which('python3')}\n"
                "import json, os, sys, time\n"
                "with open(os.environ['MQTT_TEST_LOG'], 'a') as log:\n"
                "    log.write(json.dumps({'args': sys.argv[1:], 'cwd': os.getcwd(), 'time': time.monotonic()}) + '\\n')\n"
            )
            publisher.chmod(0o755)
            env = dict(os.environ, PATH=temp, MQTT_TEST_LOG=str(log), MQTT_HOST="test-host",
                       MQTT_PORT="2883", MQTT_USER="test-user", MQTT_PASS="test-pass")
            process = subprocess.Popen([BASH, str(SCRIPT), *IDENTIFIERS], env=env,
                                       cwd="/", stdout=subprocess.DEVNULL,
                                       stderr=subprocess.PIPE, text=True, start_new_session=True)
            try:
                deadline = time.monotonic() + 9
                records = []
                while time.monotonic() < deadline:
                    if log.exists():
                        records = [json.loads(line) for line in log.read_text().splitlines()]
                    if len(records) >= (1 if compose else 2) or process.poll() is not None:
                        break
                    time.sleep(.05)
                self.assertGreaterEqual(len(records), 1 if compose else 2,
                                        "generator stopped or did not keep publishing")
                self.assertIsNone(process.poll(), "generator must continue until interrupted")
                return records
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                process.communicate(timeout=3)


if __name__ == "__main__":
    unittest.main()
