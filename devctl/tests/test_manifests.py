from __future__ import annotations

import tempfile
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


if __name__ == "__main__":
    unittest.main()
