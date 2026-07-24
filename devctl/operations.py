#!/usr/bin/env python3
"""Security-sensitive local environment operations used by devctl."""

from __future__ import annotations

import argparse
import base64
import binascii
import hashlib
import json
import os
import shutil
import ssl
import subprocess
import sys
import tarfile
import tempfile
from dataclasses import dataclass
from datetime import UTC, datetime
from io import BytesIO
from pathlib import Path
from typing import Any

from lib.manifests import EnvironmentManifest, ManifestError, SecretFile, load_manifest

DEFAULT_VAULT_MOUNT = "kv"
DEFAULT_CADDY_IMAGE = "caddy:2.10.2-alpine"
DEFAULT_CADDY_VOLUME = "tinyidp-local-caddy-pki"
PKI_REQUIRED_SUFFIXES = (
    "pki/authorities/local/root.crt",
    "pki/authorities/local/root.key",
    "pki/authorities/local/intermediate.crt",
    "pki/authorities/local/intermediate.key",
)


class OperationError(RuntimeError):
    """An operation failed without exposing a secret value."""


def log(message: str) -> None:
    sys.stderr.write(f"[tinyidp-ops] {message}\n")
    sys.stderr.flush()


@dataclass(frozen=True)
class VaultRecord:
    path: str
    version: int
    data: dict[str, Any]


class VaultCLI:
    def __init__(self, mount: str = DEFAULT_VAULT_MOUNT) -> None:
        self.mount = mount

    def assert_authenticated(self) -> None:
        result = subprocess.run(
            ["vault", "token", "lookup", "-format=json"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
            timeout=15,
        )
        if result.returncode != 0:
            raise OperationError("Vault authentication is unavailable; run the documented OIDC login")

    def get(self, path: str, version: int | None = None) -> VaultRecord:
        argv = ["vault", "kv", "get", "-format=json", f"-mount={self.mount}"]
        if version is not None:
            argv.append(f"-version={version}")
        argv.append(path)
        result = subprocess.run(argv, capture_output=True, timeout=30)
        if result.returncode != 0:
            raise OperationError(f"Vault read failed for logical path {path}")
        try:
            payload = json.loads(result.stdout)
            metadata = payload["data"]["metadata"]
            data = payload["data"]["data"]
            record_version = int(metadata["version"])
        except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
            raise OperationError(f"Vault returned an invalid KV v2 response for {path}") from exc
        if not isinstance(data, dict):
            raise OperationError(f"Vault data at {path} is not an object")
        return VaultRecord(path=path, version=record_version, data=data)

    def put_json_cas(self, path: str, value: dict[str, Any], cas: int) -> int:
        private_root = Path(tempfile.mkdtemp(prefix="tinyidp-vault-write-"))
        try:
            os.chmod(private_root, 0o700)
            input_path = private_root / "record.json"
            input_path.write_text(json.dumps(value, separators=(",", ":")), encoding="utf-8")
            os.chmod(input_path, 0o600)
            result = subprocess.run(
                [
                    "vault", "kv", "put", "-format=json", f"-mount={self.mount}",
                    f"-cas={cas}", path, f"@{input_path}",
                ],
                capture_output=True,
                timeout=60,
            )
            if result.returncode != 0:
                raise OperationError(f"Vault CAS write failed for logical path {path}")
            try:
                payload = json.loads(result.stdout)
                return int(payload["data"]["version"])
            except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
                raise OperationError(f"Vault returned invalid write metadata for {path}") from exc
        finally:
            shutil.rmtree(private_root, ignore_errors=True)


def _decode_secret(spec: SecretFile, value: Any) -> bytes:
    if not isinstance(value, str):
        raise OperationError(f"Vault field {spec.field} must be a string")
    if spec.encoding == "base64":
        try:
            decoded = base64.b64decode(value, validate=True)
        except (binascii.Error, ValueError) as exc:
            raise OperationError(f"Vault field {spec.field} is not valid base64") from exc
    else:
        decoded = value.encode("utf-8")
    if spec.exact_bytes is not None and len(decoded) != spec.exact_bytes:
        raise OperationError(
            f"Vault field {spec.field} decoded length is {len(decoded)}; expected {spec.exact_bytes}"
        )
    if spec.minimum_bytes is not None and len(decoded) < spec.minimum_bytes:
        raise OperationError(
            f"Vault field {spec.field} decoded length is {len(decoded)}; minimum is {spec.minimum_bytes}"
        )
    if not decoded:
        raise OperationError(f"Vault field {spec.field} must not be empty")
    return decoded


def _source_path(manifest: EnvironmentManifest, source: str) -> str:
    key = {
        "runtime": "runtime_path",
        "bootstrap": "bootstrap_path",
        "integration": "integration_path",
    }.get(source)
    if key is None:
        raise OperationError(f"unknown secret source {source}")
    path = manifest.vault.get(key)
    if not path:
        raise OperationError(f"manifest {manifest.name} does not define vault.{key}")
    return path


def materialize_secrets(repo_root: Path, manifest: EnvironmentManifest, vault: VaultCLI) -> Path:
    if not manifest.secret_target or not manifest.secret_files:
        raise OperationError(f"profile {manifest.name} does not declare secret material")
    vault.assert_authenticated()
    records: dict[str, VaultRecord] = {}
    decoded: dict[str, bytes] = {}
    for spec in manifest.secret_files:
        path = _source_path(manifest, spec.source)
        if path not in records:
            records[path] = vault.get(path)
        record = records[path]
        if record.data.get("schema_version") != 1:
            raise OperationError(f"Vault record {path} has unsupported schema_version")
        if spec.field not in record.data:
            raise OperationError(f"Vault record {path} is missing required field {spec.field}")
        decoded[spec.path] = _decode_secret(spec, record.data[spec.field])

    target = (repo_root / manifest.secret_target).resolve()
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(target.parent, 0o700)
    generations = target.parent / ".secret-generations"
    generations.mkdir(mode=0o700, exist_ok=True)
    generation = Path(tempfile.mkdtemp(prefix=f"{manifest.name}-", dir=generations))
    os.chmod(generation, 0o700)
    promoted = False
    try:
        for relative, value in decoded.items():
            output = generation / relative
            output.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            output.write_bytes(value)
            os.chmod(output, 0o600)
        metadata = {
            "schema_version": 1,
            "profile": manifest.name,
            "fetched_at": datetime.now(UTC).isoformat(),
            "vault_versions": {path: record.version for path, record in records.items()},
            "files": sorted(decoded),
        }
        metadata_path = generation / "materialization.json"
        metadata_path.write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
        os.chmod(metadata_path, 0o600)
        if target.exists() and not target.is_symlink():
            raise OperationError(
                f"secret target {manifest.secret_target} exists and is not a managed symlink"
            )
        link = target.parent / f".{target.name}.next-{os.getpid()}"
        link.symlink_to(generation.relative_to(target.parent), target_is_directory=True)
        os.replace(link, target)
        promoted = True
        return target
    finally:
        if not promoted:
            shutil.rmtree(generation, ignore_errors=True)


def _archive_members(archive: bytes) -> dict[str, bytes]:
    try:
        with tarfile.open(fileobj=BytesIO(archive), mode="r:*") as bundle:
            result: dict[str, bytes] = {}
            for member in bundle.getmembers():
                if member.isfile():
                    extracted = bundle.extractfile(member)
                    if extracted is not None:
                        result[member.name.lstrip("./")] = extracted.read()
            return result
    except (tarfile.TarError, OSError) as exc:
        raise OperationError("Caddy storage export is not a readable tar archive") from exc


def validate_caddy_archive(archive: bytes) -> tuple[str, str]:
    members = _archive_members(archive)
    selected: dict[str, bytes] = {}
    for suffix in PKI_REQUIRED_SUFFIXES:
        matches = [value for name, value in members.items() if name.endswith(suffix)]
        if len(matches) != 1:
            raise OperationError(f"Caddy storage archive must contain exactly one {suffix}")
        selected[suffix] = matches[0]
    try:
        root_pem = selected["pki/authorities/local/root.crt"].decode("ascii")
        root_der = ssl.PEM_cert_to_DER_cert(root_pem)
    except (ValueError, UnicodeError) as exc:
        raise OperationError("Caddy root certificate is not valid PEM") from exc
    return hashlib.sha256(root_der).hexdigest(), hashlib.sha256(archive).hexdigest()


def export_caddy_storage(volume: str, caddyfile: Path, image: str) -> bytes:
    result = subprocess.run(
        [
            "docker", "run", "--rm",
            "-v", f"{volume}:/data",
            "-v", f"{caddyfile.resolve()}:/etc/caddy/Caddyfile:ro",
            image, "caddy", "storage", "export",
            "-c", "/etc/caddy/Caddyfile", "-o", "-",
        ],
        capture_output=True,
        timeout=120,
    )
    if result.returncode != 0:
        raise OperationError("Caddy storage export failed")
    if not result.stdout:
        raise OperationError("Caddy storage export returned an empty archive")
    return result.stdout


def pki_backup(
    manifest: EnvironmentManifest,
    vault: VaultCLI,
    *,
    volume: str,
    caddyfile: Path,
    image: str,
    initialize: bool,
    allow_authority_change: bool,
) -> tuple[int, str, str]:
    path = manifest.vault.get("pki_path")
    if not path:
        raise OperationError(f"profile {manifest.name} does not define vault.pki_path")
    vault.assert_authenticated()
    archive = export_caddy_storage(volume, caddyfile, image)
    root_digest, archive_digest = validate_caddy_archive(archive)
    try:
        current = vault.get(path)
    except OperationError:
        current = None
    if current is None and not initialize:
        raise OperationError("PKI backup does not exist; rerun with --initialize")
    cas = 0 if current is None else current.version
    if current is not None:
        current_root = current.data.get("root_cert_sha256")
        if current_root != root_digest and not allow_authority_change:
            raise OperationError(
                "live Caddy authority differs from Vault; authority change requires explicit approval"
            )
    record = {
        "schema_version": 1,
        "format": "caddy-storage-export-tar",
        "archive_b64": base64.b64encode(archive).decode("ascii"),
        "archive_sha256": archive_digest,
        "root_cert_sha256": root_digest,
        "caddy_image": image,
        "source_volume": volume,
        "created_at": datetime.now(UTC).isoformat(),
    }
    version = vault.put_json_cas(path, record, cas)
    return version, root_digest, archive_digest


def _docker_volume_empty(volume: str, image: str) -> bool:
    inspect = subprocess.run(
        ["docker", "volume", "inspect", volume],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15,
    )
    if inspect.returncode != 0:
        create = subprocess.run(
            ["docker", "volume", "create", volume],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=30,
        )
        if create.returncode != 0:
            raise OperationError(f"could not create staging volume {volume}")
    check = subprocess.run(
        [
            "docker", "run", "--rm", "--entrypoint", "sh",
            "-v", f"{volume}:/target", image,
            "-ec", "test -z \"$(find /target -mindepth 1 -print -quit)\"",
        ],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30,
    )
    return check.returncode == 0


def pki_restore_staging(
    manifest: EnvironmentManifest,
    vault: VaultCLI,
    *,
    version: int,
    expected_root: str,
    target_volume: str,
    caddyfile: Path,
    image: str,
) -> tuple[str, str]:
    path = manifest.vault.get("pki_path")
    if not path:
        raise OperationError(f"profile {manifest.name} does not define vault.pki_path")
    if target_volume == DEFAULT_CADDY_VOLUME:
        raise OperationError("restore target must be a staging volume, not the live Caddy volume")
    vault.assert_authenticated()
    record = vault.get(path, version=version)
    data = record.data
    if data.get("schema_version") != 1 or data.get("format") != "caddy-storage-export-tar":
        raise OperationError("Vault PKI record has an unsupported schema or format")
    try:
        archive = base64.b64decode(str(data["archive_b64"]), validate=True)
    except (KeyError, binascii.Error, ValueError) as exc:
        raise OperationError("Vault PKI archive is missing or malformed") from exc
    root_digest, archive_digest = validate_caddy_archive(archive)
    if archive_digest != data.get("archive_sha256"):
        raise OperationError("Vault PKI archive digest does not match its metadata")
    if root_digest != data.get("root_cert_sha256") or root_digest != expected_root.lower():
        raise OperationError("Vault PKI root fingerprint does not match the expected authority")
    if not _docker_volume_empty(target_volume, image):
        raise OperationError(f"restore target volume {target_volume} is not empty")
    private_root = Path(tempfile.mkdtemp(prefix="tinyidp-caddy-restore-"))
    try:
        os.chmod(private_root, 0o700)
        archive_path = private_root / "caddy-storage.tar"
        archive_path.write_bytes(archive)
        os.chmod(archive_path, 0o600)
        result = subprocess.run(
            [
                "docker", "run", "--rm",
                "-v", f"{target_volume}:/data",
                "-v", f"{caddyfile.resolve()}:/etc/caddy/Caddyfile:ro",
                "-v", f"{archive_path}:/recovery/caddy-storage.tar:ro",
                image, "caddy", "storage", "import",
                "-c", "/etc/caddy/Caddyfile", "-i", "/recovery/caddy-storage.tar",
            ],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=120,
        )
        if result.returncode != 0:
            raise OperationError("Caddy storage import into staging volume failed")
    finally:
        shutil.rmtree(private_root, ignore_errors=True)
    recovered = export_caddy_storage(target_volume, caddyfile, image)
    recovered_root, recovered_archive = validate_caddy_archive(recovered)
    if recovered_root != root_digest:
        raise OperationError("restored Caddy storage has a different root fingerprint")
    return recovered_root, recovered_archive


def _manifest_caddyfile(root: Path, manifest: EnvironmentManifest) -> Path:
    explicit = manifest.vault.get("caddyfile")
    if not explicit:
        raise OperationError(f"profile {manifest.name} does not define vault.caddyfile")
    path = (root / explicit).resolve()
    try:
        path.relative_to(root)
    except ValueError as exc:
        raise OperationError("Caddyfile path escapes the repository root") from exc
    if not path.is_file():
        raise OperationError(f"Caddyfile does not exist: {explicit}")
    return path


def build_parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    root.add_argument("--repo-root", default=".")
    root.add_argument("--manifest", required=True)
    root.add_argument("--vault-mount", default=os.environ.get("TINYIDP_VAULT_MOUNT", DEFAULT_VAULT_MOUNT))
    root.add_argument("--caddy-image", default=DEFAULT_CADDY_IMAGE)
    commands = root.add_subparsers(dest="operation", required=True)
    commands.add_parser("secrets-fetch")
    backup = commands.add_parser("pki-backup")
    backup.add_argument("--volume", default=DEFAULT_CADDY_VOLUME)
    backup.add_argument("--initialize", action="store_true")
    backup.add_argument("--allow-authority-change", action="store_true")
    restore = commands.add_parser("pki-restore")
    restore.add_argument("--version", type=int, required=True)
    restore.add_argument("--expected-root-sha256", required=True)
    restore.add_argument("--target-volume", required=True)
    return root


def main() -> int:
    args = build_parser().parse_args()
    repo_root = Path(args.repo_root).resolve()
    try:
        manifest = load_manifest(repo_root, args.manifest)
        vault = VaultCLI(args.vault_mount)
        if args.operation == "secrets-fetch":
            target = materialize_secrets(repo_root, manifest, vault)
            log(f"materialized {len(manifest.secret_files)} files for {manifest.name} at {target}")
            return 0
        caddyfile = _manifest_caddyfile(repo_root, manifest)
        if args.operation == "pki-backup":
            version, root_digest, archive_digest = pki_backup(
                manifest, vault, volume=args.volume, caddyfile=caddyfile,
                image=args.caddy_image, initialize=args.initialize,
                allow_authority_change=args.allow_authority_change,
            )
            log(f"stored PKI backup version={version} root_sha256={root_digest} archive_sha256={archive_digest}")
            return 0
        if args.operation == "pki-restore":
            root_digest, archive_digest = pki_restore_staging(
                manifest, vault, version=args.version,
                expected_root=args.expected_root_sha256,
                target_volume=args.target_volume, caddyfile=caddyfile,
                image=args.caddy_image,
            )
            log(f"restored staging volume={args.target_volume} root_sha256={root_digest} archive_sha256={archive_digest}")
            return 0
        raise OperationError(f"unsupported operation {args.operation}")
    except (ManifestError, OperationError, OSError) as exc:
        log(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
