from __future__ import annotations

import json
import os
import subprocess
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
PLUGIN = REPO_ROOT / "devctl" / "tinyidp.py"


def invoke(requests: list[dict[str, object]], manifest: str = "dev/environments/embedded.yaml") -> tuple[list[dict[str, object]], str]:
    env = os.environ.copy()
    env["TINYIDP_DEV_MANIFEST"] = manifest
    completed = subprocess.run(
        ["python3", str(PLUGIN)],
        cwd=REPO_ROOT,
        env=env,
        input="".join(json.dumps(request) + "\n" for request in requests),
        capture_output=True,
        text=True,
        check=True,
        timeout=10,
    )
    return [json.loads(line) for line in completed.stdout.splitlines()], completed.stderr


class PluginTests(unittest.TestCase):
    def request(self, operation: str, input_value: dict[str, object] | None = None) -> dict[str, object]:
        return {
            "type": "request",
            "request_id": "test-1",
            "op": operation,
            "ctx": {"repo_root": str(REPO_ROOT), "deadline_ms": 10000, "dry_run": True},
            "input": input_value or {},
        }

    def test_handshake_is_first_and_stdout_is_ndjson(self) -> None:
        frames, stderr = invoke([self.request("launch.plan")])
        self.assertEqual(frames[0]["type"], "handshake")
        self.assertEqual(frames[0]["protocol_version"], "v2")
        self.assertEqual(frames[1]["type"], "response")
        self.assertTrue(frames[1]["ok"])
        self.assertEqual(stderr, "")

    def test_unknown_operation_is_explicit(self) -> None:
        frames, _ = invoke([self.request("missing.op")])
        response = frames[1]
        self.assertFalse(response["ok"])
        self.assertEqual(response["error"]["code"], "E_UNSUPPORTED")

    def test_dry_run_prepare_has_no_side_effect(self) -> None:
        state_root = REPO_ROOT / "var" / "devctl" / "message-app"
        self.assertFalse(state_root.exists())
        frames, stderr = invoke(
            [self.request("prepare.run")],
            "dev/environments/message-app.yaml",
        )
        self.assertTrue(frames[1]["ok"])
        self.assertIn("dry-run:", stderr)
        self.assertFalse(state_root.exists())

    def test_utility_plan_has_no_services(self) -> None:
        frames, _ = invoke(
            [self.request("launch.plan")],
            "dev/environments/script-tools.yaml",
        )
        self.assertEqual(frames[1]["output"]["services"], [])


if __name__ == "__main__":
    unittest.main()
