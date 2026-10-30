"""Contract checks for the MQTT publish helper without a running broker."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "utils" / "insert_mqtt.sh"
READINGS = [
    "-b", "BBB", "-r", "212", "--pressure", "1013.25", "--humidity", "48.2",
    "--temperature", "22.5", "--voc", "0.42", "--iaq", "12345.6",
    "--mic-vol", "0.18", "--busy-level", "2",
    "--wifi-packets", "9007199254740993", "--ble-advertisements", "-23",
    "--pirs", "pir-1=1,pir-2=0",
]


def run_helper(*args, env=None):
    return subprocess.run(
        [shutil.which("bash"), str(SCRIPT), *args], cwd="/", env=env,
        capture_output=True, text=True, check=False,
    )


class InsertMqttContractTest(unittest.TestCase):
    def test_generated_payload_uses_current_contract_without_device_timestamp(self):
        result = run_helper(*READINGS, "--co2equivalent", "615.0", "--dry-run")
        self.assertEqual(result.returncode, 0, result.stderr)
        lines = result.stdout.splitlines()
        self.assertEqual(lines[0], "topic:   sensors/BBB/212")
        payload = json.loads(lines[1].removeprefix("payload: "))
        self.assertIs(type(payload["wifiPackets"]), int)
        self.assertIs(type(payload["bleAdvertisements"]), int)
        self.assertEqual(payload, {
            "pressure": 1013.25, "humidity": 48.2, "temperature": 22.5,
            "co2equivalent": 615.0, "voc": 0.42,
            "iaq": 12345.6, "mic_vol": 0.18,
            "busy_level": 2, "wifiPackets": 9007199254740993, "bleAdvertisements": -23,
            "PIRs": [{"ID": "pir-1", "value": 1}, {"ID": "pir-2", "value": 0}],
        })

    def test_wireless_counts_require_integer_syntax(self):
        for flag in ("--wifi-packets", "--ble-advertisements"):
            for value in ("1.5", "1.0", "1e2", "abc", "+1", "01"):
                with self.subTest(flag=flag, value=value):
                    args = READINGS.copy()
                    args[args.index(flag) + 1] = value
                    result = run_helper(*args, "--dry-run")
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("must be an integer", result.stderr)

    def test_wireless_counts_are_required(self):
        for flag in ("--wifi-packets", "--ble-advertisements"):
            with self.subTest(flag=flag):
                args = READINGS.copy()
                index = args.index(flag)
                del args[index:index + 2]
                result = run_helper(*args, "--dry-run")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("required", result.stderr)

    def test_wireless_counts_accept_zero(self):
        args = READINGS.copy()
        for flag in ("--wifi-packets", "--ble-advertisements"):
            args[args.index(flag) + 1] = "0"
        result = run_helper(*args, "--dry-run")
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads(result.stdout.splitlines()[1].removeprefix("payload: "))
        self.assertIs(type(payload["wifiPackets"]), int)
        self.assertEqual(payload["wifiPackets"], 0)
        self.assertIs(type(payload["bleAdvertisements"]), int)
        self.assertEqual(payload["bleAdvertisements"], 0)

    def test_old_sensor_flags_are_rejected(self):
        for flag in ("--temp", "--hum", "--airresistance"):
            with self.subTest(flag=flag):
                result = run_helper(*READINGS, flag, "1", "--dry-run")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(f"unknown argument: {flag}", result.stderr)

    def test_optional_co2equivalent_can_be_omitted(self):
        result = run_helper(*READINGS, "--dry-run")
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads(result.stdout.splitlines()[1].removeprefix("payload: "))
        self.assertNotIn("co2equivalent", payload)

    def test_busy_level_must_fit_int8(self):
        for value in ("128", "-129", "1.5", "18446744073709551616"):
            with self.subTest(value=value):
                args = READINGS.copy()
                args[args.index("--busy-level") + 1] = value
                result = run_helper(*args, "--dry-run")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("busy_level", result.stderr)

    def test_docker_compose_fallback_runs_from_repository_root(self):
        with tempfile.TemporaryDirectory() as temp:
            temp_path = Path(temp)
            (temp_path / "dirname").symlink_to(shutil.which("dirname"))
            docker = temp_path / "docker"
            docker.write_text('#!/bin/sh\nprintf "%s\\n" "$PWD" > "$MQTT_TEST_CWD_FILE"\n')
            docker.chmod(0o755)
            marker = temp_path / "cwd"
            env = os.environ.copy()
            env["PATH"] = temp
            env["MQTT_TEST_CWD_FILE"] = str(marker)
            result = run_helper(*READINGS, env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(marker.read_text().strip(), str(ROOT))


if __name__ == "__main__":
    unittest.main()
