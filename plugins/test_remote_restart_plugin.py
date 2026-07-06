import io
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))

import remote_restart_plugin as plugin


class RemoteRestartPluginTests(unittest.TestCase):
    def test_extracts_host_port_bound_to_container_8080(self):
        inspect_payload = [
            {
                "NetworkSettings": {
                    "Ports": {
                        "8080/tcp": [
                            {"HostIp": "0.0.0.0", "HostPort": "9904"},
                            {"HostIp": "::", "HostPort": "9904"},
                        ],
                        "8009/tcp": [{"HostIp": "0.0.0.0", "HostPort": "9909"}],
                    }
                }
            }
        ]

        self.assertEqual(plugin.extract_8080_host_port(inspect_payload), "9904")

    def test_health_check_unknown_prints_warning_and_tail_command(self):
        output = io.StringIO()
        calls = {"health": 0, "tail": 0}

        def always_unknown(_url):
            calls["health"] += 1
            return None

        def fake_sleep(_seconds):
            return None

        def fake_tail():
            calls["tail"] += 1
            return "line 1\nline 2\n"

        result = plugin.wait_for_healthy(
            "http://localhost:9904/actuator/health",
            timeout_seconds=2,
            interval_seconds=1,
            health_getter=always_unknown,
            sleep=fake_sleep,
            tail_log=fake_tail,
            out=output,
        )

        self.assertFalse(result)
        self.assertEqual(calls["tail"], 1)
        text = output.getvalue()
        self.assertIn("\033[33m[WARNING]Startup health check is unknown.", text)
        self.assertIn(
            "Suggested command: tail -fn 300 /data/boot3/logs/sahara-social/catalina.out",
            text,
        )
        self.assertIn("line 1", text)


if __name__ == "__main__":
    unittest.main()
