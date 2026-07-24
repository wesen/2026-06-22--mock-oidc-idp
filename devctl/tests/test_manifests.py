from __future__ import annotations

import tempfile
import subprocess
import unittest
from pathlib import Path

from devctl.lib.manifests import ManifestError, load_manifest


REPO_ROOT = Path(__file__).resolve().parents[2]


class ManifestTests(unittest.TestCase):
    def test_all_repository_manifests_load(self) -> None:
        paths = sorted((REPO_ROOT / "dev" / "environments").glob("*.yaml"))
        self.assertGreaterEqual(len(paths), 9)
        names = {
            load_manifest(REPO_ROOT, str(path.relative_to(REPO_ROOT))).name
            for path in paths
        }
        self.assertEqual(len(names), len(paths))

    def test_production_shaped_profile_requires_https(self) -> None:
        with tempfile.TemporaryDirectory(dir=REPO_ROOT) as directory:
            path = Path(directory) / "bad.yaml"
            path.write_text(
                """
schema_version: 1
name: bad
classification: production-shaped-local
description: invalid
origins: {issuer: http://localhost:8080}
vault:
  tier: dev
  deployment: bad
  runtime_path: tiny-idp/dev/bad/runtime
  pki_path: tiny-idp/dev/_shared/caddy-local/pki-storage
runtime:
  kind: process
  services:
    - {name: bad, cwd: ., command: ["false"]}
""",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ManifestError, "HTTPS issuer"):
                load_manifest(REPO_ROOT, str(path.relative_to(REPO_ROOT)))

    def test_required_path_cannot_escape_repository(self) -> None:
        with tempfile.TemporaryDirectory(dir=REPO_ROOT) as directory:
            path = Path(directory) / "bad.yaml"
            path.write_text(
                """
schema_version: 1
name: bad
classification: utility
description: invalid
origins: {}
vault: {}
required_paths: [../outside]
runtime: {kind: utility, services: []}
""",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ManifestError, "escapes"):
                load_manifest(REPO_ROOT, str(path.relative_to(REPO_ROOT)))

    def test_production_shaped_profiles_have_guarded_state_commands(self) -> None:
        for path in sorted((REPO_ROOT / "dev" / "environments").glob("*.yaml")):
            manifest = load_manifest(REPO_ROOT, str(path.relative_to(REPO_ROOT)))
            if manifest.classification != "production-shaped-local":
                continue
            with self.subTest(profile=manifest.name):
                self.assertIn("state-status", manifest.commands)
                reset = manifest.commands.get("state-reset")
                self.assertIsNotNone(reset)
                self.assertIn("dev/scripts/compose-state.sh", reset)

    def test_external_message_desk_has_guarded_local_reset(self) -> None:
        manifest = load_manifest(
            REPO_ROOT,
            "dev/environments/external-message-desk.yaml",
        )
        self.assertEqual(
            manifest.commands["state-reset"][-1],
            "reset-local",
        )

    def test_compose_secret_contracts_reference_materialized_files(self) -> None:
        for profile in ("admin-console", "shared-two-apps", "jitsi"):
            manifest = load_manifest(
                REPO_ROOT,
                f"dev/environments/{profile}.yaml",
            )
            compose = (
                REPO_ROOT / "examples" / f"tinyidp-{profile}" / "compose.yaml"
            ).read_text(encoding="utf-8")
            with self.subTest(profile=profile):
                for secret in manifest.secret_files:
                    self.assertIn(
                        f"file: ./runtime/secrets/{secret.path}",
                        compose,
                    )

    def test_state_reset_refuses_without_typed_confirmation(self) -> None:
        completed = subprocess.run(
            [
                str(REPO_ROOT / "dev/scripts/compose-state.sh"),
                "admin-console",
                "examples/tinyidp-admin-console/compose.yaml",
                "reset",
            ],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(2, completed.returncode)
        self.assertIn(
            "--confirm reset-admin-console-state",
            completed.stderr,
        )

    def test_secret_template_rejects_attribute_traversal(self) -> None:
        with tempfile.TemporaryDirectory(dir=REPO_ROOT) as directory:
            path = Path(directory) / "bad.yaml"
            path.write_text(
                """
schema_version: 1
name: bad-template
classification: utility
description: invalid derived field
origins: {}
vault: {runtime_path: tiny-idp/dev/bad/runtime}
runtime: {kind: utility, services: []}
secret_material:
  target: runtime/secrets
  files:
    - {source: runtime, field: seed, path: seed.txt, encoding: text}
    - source: runtime
      field: derived
      path: derived.txt
      encoding: text
      template: "{seed.__class__}"
""",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ManifestError, "simple peer fields"):
                load_manifest(REPO_ROOT, str(path.relative_to(REPO_ROOT)))

    def test_legacy_bootstrap_wrappers_delegate_to_shared_vault_bootstrap(self) -> None:
        for profile in ("shared-two-apps", "jitsi"):
            script = (
                REPO_ROOT
                / "examples"
                / f"tinyidp-{profile}"
                / "scripts"
                / "00-init-secrets.sh"
            ).read_text(encoding="utf-8")
            with self.subTest(profile=profile):
                self.assertIn("dev/scripts/bootstrap-vault-profile.sh", script)
                self.assertNotIn("/dev/urandom", script)
                self.assertNotIn("password-2026", script)


if __name__ == "__main__":
    unittest.main()
