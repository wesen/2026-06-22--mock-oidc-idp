"""Load and validate non-secret TinyIDP development environment manifests."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

SCHEMA_VERSION = 1
CLASSIFICATIONS = {"development", "production-shaped-local", "production-reference", "utility"}
RUNTIME_KINDS = {"process", "compose", "utility"}


class ManifestError(ValueError):
    """A manifest violates the repository development-environment contract."""


def _mapping(value: Any, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ManifestError(f"{field} must be a mapping")
    return value


def _string(value: Any, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ManifestError(f"{field} must be a non-empty string")
    return value.strip()


def _string_list(value: Any, field: str) -> list[str]:
    if value is None:
        return []
    if not isinstance(value, list) or any(not isinstance(item, str) or not item for item in value):
        raise ManifestError(f"{field} must be a list of non-empty strings")
    return list(value)


def _command(value: Any, field: str) -> list[str]:
    result = _string_list(value, field)
    if not result:
        raise ManifestError(f"{field} must not be empty")
    return result


def _resolve_inside(root: Path, relative: str, field: str) -> Path:
    candidate = (root / relative).resolve()
    try:
        candidate.relative_to(root)
    except ValueError as exc:
        raise ManifestError(f"{field} escapes the repository root") from exc
    return candidate


@dataclass(frozen=True)
class Service:
    name: str
    cwd: str
    command: list[str]
    env: dict[str, str]
    health: dict[str, Any] | None


@dataclass(frozen=True)
class EnvironmentManifest:
    path: Path
    name: str
    classification: str
    description: str
    issuer: str | None
    vault: dict[str, str]
    runtime_kind: str
    required_tools: list[str]
    required_paths: list[str]
    services: list[Service]
    prepare: dict[str, list[str]]
    commands: dict[str, list[str]]


def load_manifest(repo_root: Path, manifest_path: str) -> EnvironmentManifest:
    root = repo_root.resolve()
    path = _resolve_inside(root, manifest_path, "manifest path")
    try:
        raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ManifestError(f"manifest does not exist: {manifest_path}") from exc
    except yaml.YAMLError as exc:
        raise ManifestError(f"invalid YAML in {manifest_path}: {exc}") from exc
    doc = _mapping(raw, "manifest")
    if doc.get("schema_version") != SCHEMA_VERSION:
        raise ManifestError(f"schema_version must be {SCHEMA_VERSION}")

    name = _string(doc.get("name"), "name")
    classification = _string(doc.get("classification"), "classification")
    if classification not in CLASSIFICATIONS:
        raise ManifestError(f"classification must be one of {sorted(CLASSIFICATIONS)}")
    description = _string(doc.get("description"), "description")

    origins = _mapping(doc.get("origins", {}), "origins")
    issuer_value = origins.get("issuer")
    issuer = None if issuer_value is None else _string(issuer_value, "origins.issuer")

    vault_doc = _mapping(doc.get("vault", {}), "vault")
    vault = {str(key): _string(value, f"vault.{key}") for key, value in vault_doc.items()}

    runtime = _mapping(doc.get("runtime"), "runtime")
    runtime_kind = _string(runtime.get("kind"), "runtime.kind")
    if runtime_kind not in RUNTIME_KINDS:
        raise ManifestError(f"runtime.kind must be one of {sorted(RUNTIME_KINDS)}")

    required_tools = _string_list(doc.get("required_tools", []), "required_tools")
    required_paths = _string_list(doc.get("required_paths", []), "required_paths")
    for index, relative in enumerate(required_paths):
        _resolve_inside(root, relative, f"required_paths[{index}]")

    services: list[Service] = []
    service_docs = runtime.get("services", [])
    if not isinstance(service_docs, list):
        raise ManifestError("runtime.services must be a list")
    for index, value in enumerate(service_docs):
        service_doc = _mapping(value, f"runtime.services[{index}]")
        service_name = _string(service_doc.get("name"), f"runtime.services[{index}].name")
        cwd = _string(service_doc.get("cwd", "."), f"runtime.services[{index}].cwd")
        _resolve_inside(root, cwd, f"runtime.services[{index}].cwd")
        command = _command(service_doc.get("command"), f"runtime.services[{index}].command")
        env_doc = _mapping(service_doc.get("env", {}), f"runtime.services[{index}].env")
        env = {str(key): str(value) for key, value in env_doc.items()}
        health_doc = service_doc.get("health")
        health = None if health_doc is None else _mapping(health_doc, f"runtime.services[{index}].health")
        services.append(Service(service_name, cwd, command, env, health))

    if runtime_kind != "utility" and not services:
        raise ManifestError(f"runtime kind {runtime_kind} requires at least one service")
    if len({service.name for service in services}) != len(services):
        raise ManifestError("runtime service names must be unique")

    prepare_doc = _mapping(doc.get("prepare", {}), "prepare")
    prepare = {str(name): _command(command, f"prepare.{name}") for name, command in prepare_doc.items()}
    commands_doc = _mapping(doc.get("commands", {}), "commands")
    commands = {str(name): _command(command, f"commands.{name}") for name, command in commands_doc.items()}

    if classification == "production-shaped-local":
        if not issuer or not issuer.startswith("https://"):
            raise ManifestError("production-shaped-local manifests require an HTTPS issuer")
        for key in ("tier", "deployment", "runtime_path", "pki_path"):
            if key not in vault:
                raise ManifestError(f"production-shaped-local manifests require vault.{key}")

    return EnvironmentManifest(
        path=path,
        name=name,
        classification=classification,
        description=description,
        issuer=issuer,
        vault=vault,
        runtime_kind=runtime_kind,
        required_tools=required_tools,
        required_paths=required_paths,
        services=services,
        prepare=prepare,
        commands=commands,
    )
