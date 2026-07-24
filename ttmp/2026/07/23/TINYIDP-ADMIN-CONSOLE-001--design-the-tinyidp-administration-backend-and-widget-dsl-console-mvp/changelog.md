# Changelog

## 2026-07-23

- Started implementation on `feat/tinyidp-admin-console`.
- Added the Phase A security model, signed action handles, persistence contracts, SQLite control-plane schema, query-projection schema, and atomic store implementation.
- Added focused tests for authorization, handle integrity/binding/expiry, grant lifecycle, replay prevention, CAS, and cross-domain rollback.
- Completed Phase A with typed safe query/command contracts, a separate application orchestration layer, Glazed owner lifecycle commands, deterministic user-projection rebuild/drift checks, and an atomic mutation executor.
- Started Phase B with the required Widget DSL dependency and a tested OIDC PKCE/admin-session authentication boundary.
- Completed Phase B with typed read models, a constrained Widget DSL runtime, public admin routing, in-process OIDC discovery, exact-key session configuration, embedded React/Redux/RTK Query/Bootstrap assets, and frontend CI reproducibility checks.
- Completed Phase C with command-specific capabilities, signed server-prepared actions, transaction-safe user lifecycle commands, reason/confirmation/fresh-auth/version/idempotency enforcement, audit-pending result semantics, shared CLI execution, first-owner atomic provisioning, and React user-operation forms.
- Completed Phase D with public-ID invitation metadata and revocation, one-time invitation/client secrets, guarded client lifecycle commands, exact field validation, unified HTTP/CLI dispatch, Bootstrap administration forms, RTK Query secret-cache eviction, migration/backfill coverage, and leakage/replay/rotation tests.
- Completed Phase E with guarded signing-key rotation/retirement, CLI-only emergency purge, durable audit and operation workers, managed doctor/backup/verification/diagnostics jobs, canonical backup-root confinement, hashed one-use downloads, production errgroup lifecycle, readiness health, and React key/operations surfaces.

## 2026-07-23

- Initial workspace created


## 2026-07-23 - Research and MVP architecture completed

Imported the complete TinyIDP Console UX specification and wrote the evidence-backed administration backend, widget.dsl v3 integration, security, persistence, phased implementation, and testing guide plus the chronological investigation diary.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md — Primary implementation-ready architecture
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Chronological evidence and design record
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/sources/local/tiny-idp-ux.md — Imported product specification


## 2026-07-23 - Documentation bundle delivered to reMarkable

Validated the ticket, completed the bundle dry-run, and uploaded TINYIDP ADMIN CONSOLE MVP.pdf to /ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001. Recorded delivery outcomes and failures in the investigation diary.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Validation and delivery evidence
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/tasks.md — Documentation delivery tasks completed

## 2026-07-24

Completed Phase F hardening: 56-case all-screen fixture coverage, Axe and keyboard accessibility, responsive read-only behavior, real-browser CSP enforcement, deterministic Widget IR safety validation, tablet snapshot, 10,000-user benchmark, and all release gates (commit 972a0a3).

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/internal/adminweb/frontend/tests/admin-console.spec.ts — Browser acceptance matrix, accessibility, responsive, visual, and CSP evidence
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/internal/adminweb/widget_runtime.go — Deterministic Widget IR schema, component, authority, secret, code, and URL validation
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/pkg/sqlitestore/admin_queries_benchmark_test.go — Measured 10,000-user query acceptance evidence

## 2026-07-24

Opened the completed implementation guide with md-view and uploaded the implemented ticket bundle to /ai/2026/07/24/TINYIDP-ADMIN-CONSOLE-001 as TINYIDP ADMIN CONSOLE MVP IMPLEMENTED.pdf.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Final local-render and reMarkable delivery evidence

## 2026-07-24

Added the Vault-backed local TLS and secret provisioning design, including trust boundaries, KV schema, materialization contract, rotation matrix, phased implementation plan, and next-session tasks.

## 2026-07-24

Added the unified TinyIDP development and demo environment platform design. It inventories existing examples and specifies manifest-driven devctl profiles, consistent dev/production Vault paths, versioned Caddy private-CA storage backup and guarded restore, shared Compose patterns, an example migration program, a Glazed local-app tutorial, and an acceptance matrix. Superseded the earlier no-CA-backup decision in design document 02.

## 2026-07-24

Step 16: Established the manifest-driven devctl control plane for nine TinyIDP profiles, added protocol and manifest tests, and corrected generated Python bytecode staging (commits 655cd80 and e46aef6).

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/.devctl.yaml — Environment profiles
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/devctl/tinyidp.py — Control-plane implementation

## 2026-07-24

Step 17: Added Vault secret materialization and guarded Caddy storage backup/restore, stored live CA backup version 1, and proved a staging recovery with the identical root fingerprint (commit e3b0d5d).

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/devctl/operations.py — Security-sensitive operations
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/devctl/tests/test_operations.py — Recovery and materialization tests

## 2026-07-24

Step 18: Added CAS-protected secret initialization, repeat-safe atomic
materialization, the production-shaped admin-console environment, persistent
Caddy PKI wiring, bootstrap/CA/smoke scripts, and the embedded Glazed local
application tutorial (commit 62dcb14). Unit, Compose-config, manifest, help, and
pre-commit test/lint gates passed. Runtime smoke remains unchecked because the
second supervised Compose launch exited during the cached build without a
diagnostic; server debugging stopped under the repository's two-attempt rule.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/examples/tinyidp-admin-console/compose.yaml — Production-shaped local admin stack
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/cmd/tinyidp/doc/pages/tutorial-local-development-apps.md — Embedded Glazed playbook
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Detailed evidence, failures, and review instructions
