from __future__ import annotations

import base64
import io
import json
import os
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "devctl"))

from lib.manifests import load_manifest  # noqa: E402
from operations import (  # noqa: E402
    OperationError,
    VaultRecord,
    export_caddy_storage,
    initialize_secrets,
    materialize_secrets,
    pki_backup,
    validate_caddy_archive,
)


class FakeVault:
    def __init__(self, records: dict[str, dict[str, object]]) -> None:
        self.records = records
        self.auth_checks = 0

    def assert_authenticated(self) -> None:
        self.auth_checks += 1

    def get(self, path: str, version: int | None = None) -> VaultRecord:
        del version
        return VaultRecord(path=path, version=7, data=self.records[path])

    def put_json_cas(self, path: str, value: dict[str, object], cas: int) -> int:
        self.put = (path, value, cas)
        return 8


def fake_records(manifest: object, size: int = 32) -> dict[str, dict[str, object]]:
    binary = base64.b64encode(b"x" * size).decode()
    vault = getattr(manifest, "vault")
    return {
        vault["runtime_path"]: {
            "schema_version": 1,
            "token_secret_b64": binary,
            "admin_auth_key_b64": binary,
            "admin_action_key_b64": binary,
            "invitation_lookup_key_b64": binary,
            "email_challenge_key_b64": binary,
        },
        vault["bootstrap_path"]: {
            "schema_version": 1,
            "owner_password": "correct horse battery staple",
        },
    }


def authority_archive(include_all: bool = True) -> bytes:
    archive = io.BytesIO()
    suffixes = [
        "pki/authorities/local/root.crt",
        "pki/authorities/local/root.key",
        "pki/authorities/local/intermediate.crt",
        "pki/authorities/local/intermediate.key",
    ]
    if not include_all:
        suffixes.remove("pki/authorities/local/intermediate.key")
    with tarfile.open(fileobj=archive, mode="w") as bundle:
        for suffix in suffixes:
            value = b"certificate" if suffix.endswith("root.crt") else b"private-or-cert"
            info = tarfile.TarInfo(f"caddy/{suffix}")
            info.size = len(value)
            bundle.addfile(info, io.BytesIO(value))
    return archive.getvalue()


class OperationTests(unittest.TestCase):
    def test_materialization_promotes_managed_symlink_atomically(self) -> None:
        manifest = load_manifest(REPO_ROOT, "dev/environments/admin-console.yaml")
        vault = FakeVault(fake_records(manifest))
        with tempfile.TemporaryDirectory(dir=REPO_ROOT) as directory:
            target = Path(directory) / "runtime" / "secrets"
            adjusted = manifest.__class__(
                **{**manifest.__dict__, "secret_target": str(target.relative_to(REPO_ROOT))}
            )
            promoted = materialize_secrets(REPO_ROOT, adjusted, vault)
            self.assertTrue(promoted.is_symlink())
            self.assertEqual((promoted / "admin-auth.key").read_bytes(), b"x" * 32)
            self.assertEqual(
                (promoted / "owner-password.txt").read_text(),
                "correct horse battery staple",
            )
            self.assertEqual(os.stat(promoted / "admin-auth.key").st_mode & 0o777, 0o600)
            metadata = json.loads((promoted / "materialization.json").read_text())
            self.assertEqual(metadata["vault_versions"], {
                manifest.vault["bootstrap_path"]: 7,
                manifest.vault["runtime_path"]: 7,
            })
            self.assertEqual(vault.auth_checks, 1)
            first_generation = promoted.resolve()
            promoted_again = materialize_secrets(REPO_ROOT, adjusted, vault)
            self.assertTrue(promoted_again.is_symlink())
            self.assertNotEqual(promoted_again.resolve(), first_generation)
            self.assertEqual((promoted_again / "admin-auth.key").read_bytes(), b"x" * 32)

    def test_wrong_key_length_leaves_no_target(self) -> None:
        manifest = load_manifest(REPO_ROOT, "dev/environments/admin-console.yaml")
        vault = FakeVault(fake_records(manifest, size=31))
        with tempfile.TemporaryDirectory(dir=REPO_ROOT) as directory:
            target = Path(directory) / "runtime" / "secrets"
            adjusted = manifest.__class__(
                **{**manifest.__dict__, "secret_target": str(target.relative_to(REPO_ROOT))}
            )
            with self.assertRaisesRegex(OperationError, "minimum is 32|expected 32"):
                materialize_secrets(REPO_ROOT, adjusted, vault)
            self.assertFalse(target.exists())

    def test_initialize_secrets_creates_each_record_with_cas_zero(self) -> None:
        manifest = load_manifest(REPO_ROOT, "dev/environments/admin-console.yaml")

        class EmptyVault(FakeVault):
            def __init__(self) -> None:
                super().__init__({})
                self.puts: list[tuple[str, dict[str, object], int]] = []

            def get(self, path: str, version: int | None = None) -> VaultRecord:
                raise OperationError(f"missing {path}")

            def put_json_cas(self, path: str, value: dict[str, object], cas: int) -> int:
                self.puts.append((path, value, cas))
                return len(self.puts)

        vault = EmptyVault()
        versions = initialize_secrets(manifest, vault)
        self.assertEqual(set(versions), {
            manifest.vault["runtime_path"],
            manifest.vault["bootstrap_path"],
        })
        self.assertTrue(all(cas == 0 for _, _, cas in vault.puts))
        runtime = next(value for path, value, _ in vault.puts if path == manifest.vault["runtime_path"])
        self.assertEqual(
            len(base64.b64decode(str(runtime["admin_auth_key_b64"]), validate=True)),
            32,
        )
        bootstrap = next(value for path, value, _ in vault.puts if path == manifest.vault["bootstrap_path"])
        self.assertEqual(bootstrap["owner_login"], "admin@example.test")
        self.assertNotIn(str(bootstrap["owner_password"]), json.dumps(versions))

    @mock.patch("operations.ssl.PEM_cert_to_DER_cert", return_value=b"root-der")
    def test_caddy_archive_requires_complete_authority(self, _mock: mock.Mock) -> None:
        root_digest, archive_digest = validate_caddy_archive(authority_archive())
        self.assertEqual(len(root_digest), 64)
        self.assertEqual(len(archive_digest), 64)

    def test_caddy_archive_rejects_missing_intermediate_key(self) -> None:
        with self.assertRaisesRegex(OperationError, "intermediate.key"):
            validate_caddy_archive(authority_archive(include_all=False))

    @mock.patch("operations.subprocess.run")
    def test_caddy_export_invokes_binary_for_image_without_entrypoint(self, run: mock.Mock) -> None:
        run.return_value = mock.Mock(returncode=0, stdout=b"archive")
        result = export_caddy_storage(
            "tinyidp-local-caddy-pki",
            REPO_ROOT / "examples/tinyidp-shared-two-apps/Caddyfile",
            "caddy:test",
        )
        self.assertEqual(result, b"archive")
        argv = run.call_args.args[0]
        image_index = argv.index("caddy:test")
        self.assertEqual(argv[image_index + 1:image_index + 4], ["caddy", "storage", "export"])

    @mock.patch("operations.validate_caddy_archive", return_value=("a" * 64, "b" * 64))
    @mock.patch("operations.export_caddy_storage", return_value=b"archive")
    def test_pki_backup_requires_initialize_for_first_version(
        self,
        _export: mock.Mock,
        _validate: mock.Mock,
    ) -> None:
        manifest = load_manifest(REPO_ROOT, "dev/environments/shared-two-apps.yaml")

        class MissingVault(FakeVault):
            def get(self, path: str, version: int | None = None) -> VaultRecord:
                raise OperationError(f"missing {path}")

        vault = MissingVault({})
        with self.assertRaisesRegex(OperationError, "--initialize"):
            pki_backup(
                manifest,
                vault,
                volume="tinyidp-local-caddy-pki",
                caddyfile=REPO_ROOT / "examples/tinyidp-shared-two-apps/Caddyfile",
                image="caddy:test",
                initialize=False,
                allow_authority_change=False,
            )

    @mock.patch("operations.validate_caddy_archive", return_value=("a" * 64, "b" * 64))
    @mock.patch("operations.export_caddy_storage", return_value=b"archive")
    def test_pki_backup_initial_write_uses_cas_zero(
        self,
        _export: mock.Mock,
        _validate: mock.Mock,
    ) -> None:
        manifest = load_manifest(REPO_ROOT, "dev/environments/shared-two-apps.yaml")

        class MissingThenWriteVault(FakeVault):
            def get(self, path: str, version: int | None = None) -> VaultRecord:
                raise OperationError(f"missing {path}")

        vault = MissingThenWriteVault({})
        version, root_digest, archive_digest = pki_backup(
            manifest,
            vault,
            volume="tinyidp-local-caddy-pki",
            caddyfile=REPO_ROOT / "examples/tinyidp-shared-two-apps/Caddyfile",
            image="caddy:test",
            initialize=True,
            allow_authority_change=False,
        )
        self.assertEqual((version, root_digest, archive_digest), (8, "a" * 64, "b" * 64))
        self.assertEqual(vault.put[2], 0)
        self.assertEqual(vault.put[1]["archive_b64"], base64.b64encode(b"archive").decode())


if __name__ == "__main__":
    unittest.main()
