#!/usr/bin/env python3
import json
import sys


def emit(value):
    sys.stdout.write(json.dumps(value) + "\n")
    sys.stdout.flush()


if len(sys.argv) != 2:
    raise SystemExit("usage: devctl-lifecycle-plugin.py PORT")

port = int(sys.argv[1])
url = f"http://127.0.0.1:{port}/"

emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "devctl-lifecycle-fixture",
        "capabilities": {
            "ops": ["config.mutate", "validate.run", "launch.plan"]
        },
    }
)

for line in sys.stdin:
    request = json.loads(line)
    request_id = request.get("request_id", "")
    operation = request.get("op", "")

    if operation == "config.mutate":
        output = {
            "config_patch": {
                "set": {
                    "services.lifecycle.port": port,
                    "services.lifecycle.url": url,
                },
                "unset": [],
            }
        }
    elif operation == "validate.run":
        output = {"valid": True, "errors": [], "warnings": []}
    elif operation == "launch.plan":
        output = {
            "services": [
                {
                    "name": "lifecycle",
                    "command": [
                        "python3",
                        "-m",
                        "http.server",
                        str(port),
                        "--bind",
                        "127.0.0.1",
                    ],
                    "health": {
                        "type": "http",
                        "url": url,
                        "timeout_ms": 5000,
                    },
                }
            ]
        }
    else:
        emit(
            {
                "type": "response",
                "request_id": request_id,
                "ok": False,
                "error": {
                    "code": "E_UNSUPPORTED",
                    "message": f"unsupported operation: {operation}",
                },
            }
        )
        continue

    emit(
        {
            "type": "response",
            "request_id": request_id,
            "ok": True,
            "output": output,
        }
    )
