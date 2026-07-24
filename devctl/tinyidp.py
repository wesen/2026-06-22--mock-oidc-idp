#!/usr/bin/env python3
"""Manifest-driven devctl plugin for TinyIDP development and demo profiles.

Protocol stdout is NDJSON only. Human-readable progress and child-process
output are written to stderr.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from lib.manifests import EnvironmentManifest, ManifestError, load_manifest

PLUGIN_NAME = "tinyidp"
MANIFEST_ENV = "TINYIDP_DEV_MANIFEST"
DEFAULT_MANIFEST = "dev/environments/embedded.yaml"
COMMAND_NAMES = (
    "secrets-init",
    "secrets-fetch",
    "pki-backup",
    "pki-restore",
    "pki-export-root",
    "seed",
    "smoke",
    "browser-test",
    "state-status",
    "state-reset",
)
BUILTIN_SECURITY_COMMANDS = {"secrets-init", "secrets-fetch", "pki-backup", "pki-restore"}


def emit(value: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(value, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def log(message: str) -> None:
    sys.stderr.write(f"[{PLUGIN_NAME}] {message}\n")
    sys.stderr.flush()


def ok(request_id: str, output: dict[str, Any]) -> None:
    emit({"type": "response", "request_id": request_id, "ok": True, "output": output})


def fail(request_id: str, code: str, message: str) -> None:
    emit({
        "type": "response",
        "request_id": request_id,
        "ok": False,
        "error": {"code": code, "message": message},
    })


def request_root(ctx: dict[str, Any]) -> Path:
    return Path(str(ctx.get("repo_root") or os.getcwd())).resolve()


def manifest_selector(input_obj: dict[str, Any]) -> str:
    config = input_obj.get("config")
    if isinstance(config, dict):
        env = config.get("env")
        if isinstance(env, dict) and env.get(MANIFEST_ENV):
            return str(env[MANIFEST_ENV])
    return os.environ.get(MANIFEST_ENV, DEFAULT_MANIFEST)


def manifest_for(ctx: dict[str, Any], input_obj: dict[str, Any]) -> EnvironmentManifest:
    return load_manifest(request_root(ctx), manifest_selector(input_obj))


def timeout_seconds(ctx: dict[str, Any]) -> float:
    value = ctx.get("deadline_ms")
    if not isinstance(value, (int, float)) or value <= 0:
        return 300.0
    return max(1.0, float(value) / 1000.0)


def run_command(
    argv: list[str],
    *,
    root: Path,
    cwd: str = ".",
    timeout: float,
    dry_run: bool,
) -> int:
    if dry_run:
        log(f"dry-run: cwd={cwd} argv={json.dumps(argv)}")
        return 0
    started = time.monotonic()
    process = subprocess.Popen(
        argv,
        cwd=root / cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    assert process.stdout is not None
    try:
        for line in process.stdout:
            sys.stderr.write(line)
            sys.stderr.flush()
            if time.monotonic() - started > timeout:
                process.kill()
                process.wait()
                log(f"command exceeded deadline after {timeout:.1f}s")
                return 124
        return process.wait(timeout=max(1.0, timeout - (time.monotonic() - started)))
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
        log(f"command exceeded deadline after {timeout:.1f}s")
        return 124


def handle_config(request_id: str, ctx: dict[str, Any], input_obj: dict[str, Any]) -> None:
    manifest = manifest_for(ctx, input_obj)
    config: dict[str, Any] = {
        f"env.{MANIFEST_ENV}": str(manifest.path.relative_to(request_root(ctx))),
        "environment.name": manifest.name,
        "environment.classification": manifest.classification,
        "environment.runtime_kind": manifest.runtime_kind,
    }
    if manifest.issuer:
        config["environment.issuer"] = manifest.issuer
    for key, value in manifest.vault.items():
        config[f"vault.{key}"] = value
    for service in manifest.services:
        if service.health and isinstance(service.health.get("url"), str):
            config[f"services.{service.name}.health_url"] = service.health["url"]
    ok(request_id, {"config_patch": {"set": config, "unset": []}})


def handle_validate(request_id: str, ctx: dict[str, Any], input_obj: dict[str, Any]) -> None:
    root = request_root(ctx)
    manifest = manifest_for(ctx, input_obj)
    errors: list[dict[str, str]] = []
    warnings: list[dict[str, str]] = []
    for executable in manifest.required_tools:
        if shutil.which(executable) is None:
            errors.append({
                "code": "E_MISSING_TOOL",
                "message": f"{manifest.name}: required executable not found: {executable}",
            })
    for relative in manifest.required_paths:
        if not (root / relative).exists():
            errors.append({
                "code": "E_MISSING_PATH",
                "message": f"{manifest.name}: required path does not exist: {relative}",
            })
    if manifest.classification == "development":
        warnings.append({
            "code": "W_DEVELOPMENT_ONLY",
            "message": f"{manifest.name} is a development profile and must not be exposed beyond loopback",
        })
    if manifest.classification == "production-reference":
        warnings.append({
            "code": "W_REFERENCE_ONLY",
            "message": f"{manifest.name} validates a production reference; it is not a deployment manifest",
        })
    ok(request_id, {"valid": not errors, "errors": errors, "warnings": warnings})


def handle_prepare(request_id: str, ctx: dict[str, Any], input_obj: dict[str, Any]) -> None:
    manifest = manifest_for(ctx, input_obj)
    requested = input_obj.get("steps")
    if not isinstance(requested, list):
        requested = []
    selected = list(manifest.prepare) if not requested else [str(item) for item in requested]
    unknown = [name for name in selected if name not in manifest.prepare]
    if unknown:
        fail(request_id, "E_UNKNOWN_STEP", f"unknown prepare steps: {', '.join(unknown)}")
        return
    results: list[dict[str, Any]] = []
    for name in selected:
        started = time.monotonic()
        exit_code = run_command(
            manifest.prepare[name],
            root=request_root(ctx),
            timeout=timeout_seconds(ctx),
            dry_run=bool(ctx.get("dry_run")),
        )
        results.append({
            "name": name,
            "ok": exit_code == 0,
            "duration_ms": int((time.monotonic() - started) * 1000),
        })
        if exit_code != 0:
            fail(request_id, "E_PREPARE_FAILED", f"prepare step {name} exited with {exit_code}")
            return
    ok(request_id, {"steps": results, "artifacts": {}})


def handle_plan(request_id: str, ctx: dict[str, Any], input_obj: dict[str, Any]) -> None:
    manifest = manifest_for(ctx, input_obj)
    services: list[dict[str, Any]] = []
    for service in manifest.services:
        planned: dict[str, Any] = {
            "name": service.name,
            "cwd": service.cwd,
            "command": service.command,
            "env": service.env,
        }
        if service.health:
            planned["health"] = service.health
        services.append(planned)
    notes = [
        f"Environment: {manifest.name}",
        f"Classification: {manifest.classification}",
        manifest.description,
    ]
    if manifest.issuer:
        notes.append(f"Issuer: {manifest.issuer}")
    if manifest.runtime_kind == "utility":
        notes.append("Utility profile: use the profile's dynamic commands; no server is launched.")
    ok(request_id, {"services": services, "notes": notes})


def handle_dynamic_command(
    request_id: str,
    ctx: dict[str, Any],
    input_obj: dict[str, Any],
) -> None:
    name = str(input_obj.get("name") or input_obj.get("command") or "")
    if name not in COMMAND_NAMES:
        fail(request_id, "E_UNSUPPORTED", f"unknown command: {name}")
        return
    manifest = manifest_for(ctx, input_obj)
    argv = input_obj.get("argv")
    if not isinstance(argv, list) or any(not isinstance(item, str) for item in argv):
        fail(request_id, "E_INVALID_ARGUMENT", "command argv must be a list of strings")
        return
    command = manifest.commands.get(name)
    if name in BUILTIN_SECURITY_COMMANDS:
        command = [
            "python3",
            "devctl/operations.py",
            "--repo-root",
            str(request_root(ctx)),
            "--manifest",
            manifest_selector(input_obj),
            name,
        ]
    elif command is None:
        fail(request_id, "E_UNAVAILABLE", f"{name} is not available for profile {manifest.name}")
        return
    exit_code = run_command(
        command + list(argv),
        root=request_root(ctx),
        timeout=timeout_seconds(ctx),
        dry_run=bool(ctx.get("dry_run")),
    )
    ok(request_id, {"exit_code": exit_code})


emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": PLUGIN_NAME,
    "capabilities": {
        "ops": ["config.mutate", "validate.run", "prepare.run", "launch.plan", "command.run"],
        "commands": [
            {"name": name, "help": name.replace("-", " ").capitalize(), "args_spec": []}
            for name in COMMAND_NAMES
        ],
    },
})

for raw_line in sys.stdin:
    line = raw_line.strip()
    if not line:
        continue
    request_id = ""
    try:
        request = json.loads(line)
        request_id = str(request.get("request_id") or "")
        operation = str(request.get("op") or "")
        context = request.get("ctx") if isinstance(request.get("ctx"), dict) else {}
        input_value = request.get("input") if isinstance(request.get("input"), dict) else {}
        if operation == "config.mutate":
            handle_config(request_id, context, input_value)
        elif operation == "validate.run":
            handle_validate(request_id, context, input_value)
        elif operation == "prepare.run":
            handle_prepare(request_id, context, input_value)
        elif operation == "launch.plan":
            handle_plan(request_id, context, input_value)
        elif operation == "command.run":
            handle_dynamic_command(request_id, context, input_value)
        else:
            fail(request_id, "E_UNSUPPORTED", f"unsupported op: {operation}")
    except (ManifestError, OSError, ValueError) as exc:
        fail(request_id, "E_CONFIG", str(exc))
    except json.JSONDecodeError as exc:
        fail(request_id, "E_PROTOCOL", f"invalid JSON request: {exc}")
    except Exception as exc:  # noqa: BLE001 - protocol boundary must return a frame.
        log(f"unexpected plugin error: {exc}")
        fail(request_id, "E_PLUGIN", str(exc))
