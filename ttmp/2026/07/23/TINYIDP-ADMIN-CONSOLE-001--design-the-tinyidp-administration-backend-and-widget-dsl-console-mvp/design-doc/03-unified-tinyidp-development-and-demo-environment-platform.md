---
Title: Unified TinyIDP Development and Demo Environment Platform
Ticket: TINYIDP-ADMIN-CONSOLE-001
Status: active
Topics:
    - backend
    - identity
    - auth
    - architecture
    - security
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/tinyidp/doc/doc.go
      Note: Existing embedded Glazed help registration
    - Path: repo://cmd/tinyidp/doc/pages/tutorial-device-authorization.md
      Note: Existing tutorial and device-authorization teaching reference
    - Path: repo://examples/embedded/app.go
      Note: Reference one-process embedded-provider integration mode
    - Path: repo://examples/tinyidp-external-message-desk/compose.yaml
      Note: Reference explicit development HTTP provider and relying-party topology
    - Path: repo://examples/tinyidp-jitsi/compose.yaml
      Note: Complex integration profile whose media and identity behavior must survive migration
    - Path: repo://examples/tinyidp-shared-two-apps/compose.yaml
      Note: Richest existing local HTTPS and shared Caddy PKI reference topology
    - Path: repo://internal/sections/production/section.go
      Note: Authoritative strict-host configuration and secret requirements
ExternalSources: []
Summary: Architecture and migration guide for a unified TinyIDP dev/demo platform using environment manifests, devctl orchestration, Vault-backed secrets and Caddy PKI recovery, reusable Compose fragments, and an embedded Glazed tutorial.
LastUpdated: 2026-07-24T11:19:07.897778172-04:00
WhatFor: Give an intern enough system context and implementation detail to replace the repository's divergent demo startup paths with one secure, inspectable, reproducible workflow.
WhenToUse: Before adding or refactoring a TinyIDP local application, demo server, Vault layout, Caddy CA backup, devctl command, Compose profile, or local-development help page.
---


# Unified TinyIDP Development and Demo Environment Platform

## Executive Summary

TinyIDP currently demonstrates several valid integration modes: a minimal
embedded provider, a standalone production host, a development HTTP
provider/relying-party pair, a production-shaped local HTTPS stack shared by
two relying parties, and a Jitsi integration. These examples were built for
different tickets and consequently use different startup commands, secret
sources, state layouts, health checks, and cleanup procedures.

This document defines a repository-wide development and demonstration platform.
It does not remove the distinct integration modes. It gives them one control
plane and one set of operational contracts:

- declarative environment manifests describe applications, origins, services,
  Vault paths, state, and verification suites;
- a repository devctl plugin validates, prepares, launches, supervises, and
  operates those environments;
- Docker Compose remains the service topology engine for multi-container
  profiles, while devctl owns selection, preparation, observation, and helper
  commands;
- Vault KV v2 stores application secrets and a versioned disaster-recovery
  export of Caddy's local storage, including the local CA private keys;
- the external `tinyidp-local-caddy-pki` volume remains the live Caddy storage;
- reusable Compose fragments and shell libraries replace duplicated
  initialization, public-root export, and smoke-test behavior;
- a Glazed help tutorial teaches application authors how to build and validate
  a local TinyIDP-backed application through supported APIs.

The first implementation should migrate, not rewrite, existing examples. Each
profile must retain its protocol semantics and tests while adopting the common
control plane.

## Problem Statement

### The repository has multiple correct but divergent workflows

The examples are not duplicates in purpose:

| Example | Integration mode | Current launch model | Important behavior |
|---|---|---|---|
| `examples/embedded` | TinyIDP embedded in one Go process with an RP | `go run ./examples/embedded` | In-process issuer transport, development HTTP, seeded Alice account |
| `examples/production-host` | Strict standalone host | manual `admin` commands plus `serve-production` | Complete production validation, direct TLS example, durable SQLite |
| `examples/tinyidp-external-message-desk` | Separate development provider and RP | Compose | Loopback HTTP, committed demo seed, independent volumes |
| `examples/tinyidp-shared-two-apps` | Strict local HTTPS provider plus Message Desk and go-go-goja | Compose and numbered scripts | Caddy internal CA, Mailpit, multiple clients/themes, browser acceptance |
| `examples/tinyidp-jitsi` | Strict local HTTPS provider plus Jitsi | Compose and numbered scripts | Shared Jitsi signing material, Prosody/Jicofo/JVB, browser conference tests |
| `examples/tinyidp-message-app` | Reusable/demo relying-party implementation | direct Go commands and frontend build | External issuer/backchannel split, application session ownership |
| `examples/tinyidp-script` | Signup-policy examples | TinyIDP script commands | Validation and explanation of bounded Goja programs |
| `examples/configs` and `examples/users` | CLI configuration fixtures | direct CLI flags/config files | Root/realm and seeded-claim scenarios |

The differences that express protocol architecture must remain. The accidental
differences should not:

- fixed demo passwords versus locally generated values;
- secrets created in a host directory versus inside an application volume;
- HTTPS versus HTTP smoke checks with inconsistent certificate validation;
- startup instructions split across README snippets and ticket-local scripts;
- implicit port ownership and state locations;
- manual Caddy volume creation and public-root export;
- no uniform way to ask what will run before starting it;
- no single command for reset, seed, Vault fetch, CA backup, CA restore, smoke,
  or browser acceptance.

### Secret recovery and live state are currently conflated

The existing `tinyidp-local-caddy-pki` Docker volume correctly preserves
Caddy's data directory. Caddy documents that this directory contains
certificates, private keys, and other important assets and must not be treated
as a cache. A single local volume, however, is not a disaster-recovery copy. If
it is deleted or its Docker storage is lost, the installed root becomes
unusable and all development leaf certificates must be recreated.

The requested design therefore stores a backup of the private CA material in
Vault. The key distinction is:

- the Docker volume is the active, writable Caddy storage;
- Vault is a versioned recovery store;
- backup is explicit and content-addressed;
- restore is an exceptional, guarded operation into an empty volume;
- ordinary `devctl up` never overwrites active CA state from Vault.

### New application authors lack one authoritative playbook

The PULP OS integration demonstrates the full set of questions an application
author must answer: embedded or external provider, issuer identity, TLS, client
registration, resource indicators, opaque-token introspection, browser
approval, credential ownership, bounded state, and real browser testing. The
repository contains the answers across code, READMEs, and ticket reports, but
not as a discoverable `tinyidp help` application tutorial.

## Proposed Solution

## 1. Architecture

```text
                             developer
                                 |
                      devctl --profile <name>
                                 |
                   +-------------+-------------+
                   | TinyIDP repository plugin |
                   | config / validate / plan  |
                   | prepare / commands        |
                   +------+--------------+-----+
                          |              |
             direct Go services         | Docker Compose profiles
        (embedded, script tools)         | (HTTPS, Jitsi, multi-app)
                          |              |
                          +------+-------+
                                 |
                     environment manifest
           origins | clients | Vault refs | state | tests
                                 |
                  +--------------+----------------+
                  |                               |
          Vault KV v2                    Docker persistent state
    app secrets + Caddy backup      SQLite, audit, artifacts, live CA
                  |                               |
                  +---------------+---------------+
                                  |
                 Caddy HTTPS -> TinyIDP -> relying applications
                                  |
                       smoke and browser suites
```

devctl is the control plane, not a replacement for Compose. The plugin computes
and validates the plan. devctl supervises direct processes and a long-running
`docker compose up` process, captures logs, tracks health, and stops process
groups. Compose continues to express service dependencies, networks, volumes,
secrets, and container health checks.

## 2. Repository layout

Add these repository-owned components:

```text
.devctl.yaml
devctl/
  tinyidp.py
  lib/
    protocol.py
    manifests.py
dev/
  environments/
    embedded.yaml
    external-message-desk.yaml
    shared-two-apps.yaml
    jitsi.yaml
    admin-console.yaml
    production-host-local.yaml
  compose/
    caddy-pki.yaml
    mailpit.yaml
  scripts/
    secret-files.sh
    caddy-storage.sh
examples/
  ... existing examples remain ...
cmd/tinyidp/doc/pages/
  tutorial-local-development-apps.md
```

The plugin and manifest loader are repository infrastructure. Example-specific
assets such as Caddyfiles, client catalogs, theme catalogs, signup programs,
and browser tests remain beside each example. This keeps examples readable
without copying generic orchestration logic into every directory.

## 3. Environment manifest

Each profile has one non-secret YAML manifest. The schema is intentionally
small and is validated before any side effect:

```yaml
schema_version: 1
name: shared-two-apps
mode: compose
classification: production-shaped-local

origins:
  issuer: https://idp.localhost:8443
  applications:
    message-desk: https://message.localhost:8443
    goja-auth: https://goja.localhost:8443

vault:
  tier: dev
  deployment: shared-two-apps
  runtime_path: tiny-idp/dev/shared-two-apps/runtime
  bootstrap_path: tiny-idp/dev/shared-two-apps/bootstrap
  pki_path: tiny-idp/dev/_shared/caddy-local/pki-storage

runtime:
  kind: compose
  project_directory: examples/tinyidp-shared-two-apps
  compose_files: [compose.yaml]
  caddy_volume: tinyidp-local-caddy-pki
  state_policy: preserve

verification:
  readiness:
    - https://idp.localhost:8443/readyz
    - https://message.localhost:8443/readyz
  smoke_command: [./scripts/02-smoke.sh]
  browser_command: [./scripts/03-browser-acceptance.py]
```

Required invariants:

- `name` is unique and matches a devctl profile.
- `classification` is one of `development`, `production-shaped-local`, or
  `production-reference`; it is printed prominently by `devctl plan`.
- HTTPS profiles declare the shared Caddy volume and CA Vault path.
- Vault paths are logical paths without embedded tokens or addresses.
- secret values never appear in the manifest.
- issuer and redirect origins are explicit; service DNS is never substituted
  for the public OIDC issuer.
- reset policy declares which volumes are disposable and which require an
  additional confirmation.

## 4. devctl plugin design

The plugin speaks NDJSON stdio protocol v2. Its first stdout line is the
handshake; every later stdout line is a protocol frame. Diagnostics go to
stderr. It uses `ctx.repo_root`, honors `ctx.dry_run`, and enforces
`ctx.deadline_ms`.

Capabilities:

```json
{
  "ops": [
    "config.mutate",
    "validate.run",
    "prepare.run",
    "launch.plan",
    "command.run"
  ],
  "commands": [
    "secrets-fetch",
    "pki-backup",
    "pki-restore",
    "pki-export-root",
    "seed",
    "smoke",
    "browser-test",
    "state-status",
    "state-reset"
  ]
}
```

### `config.mutate`

This phase reads the selected manifest without side effects and publishes
useful facts:

```text
env.profile
env.classification
env.issuer
env.runtime_kind
env.compose_project_directory
env.vault_runtime_path
env.vault_pki_path
artifacts.runtime_secret_directory
artifacts.public_ca_path
services.<name>.url
```

### `validate.run`

Validation returns actionable errors and warnings:

- required executables: `go`, `docker`, Compose, `vault`, `jq`, `base64`, and
  profile-specific `pnpm` or Python;
- manifest schema and path containment;
- port collisions;
- Vault authentication and exact-path capabilities when a Vault operation is
  requested;
- required configuration files and frontend dependencies;
- external Caddy volume presence or creatability;
- secret output directory ownership and modes;
- production-shaped profiles using HTTPS and narrow trusted-proxy CIDRs;
- installed root fingerprint compared with the live or recovered public root
  when this can be checked non-destructively.

Missing Vault authentication is an error for prepare, but `devctl plan` remains
useful without authentication because planning must not read secrets.

### `prepare.run`

Named steps make expensive or privileged work explicit:

```text
secrets-fetch -> caddy-volume-ensure -> frontend-install
              -> config-validate -> bootstrap-if-absent
```

Preparation materializes secret files atomically and creates missing,
non-destructive infrastructure. It does not reset a database, replace a CA, or
rotate a key.

### `launch.plan`

Direct profiles return native services:

```json
{
  "name": "embedded",
  "cwd": ".",
  "command": ["go", "run", "./examples/embedded"],
  "health": {
    "type": "http",
    "url": "http://127.0.0.1:5556/readyz",
    "timeout_ms": 30000
  }
}
```

Compose profiles return one supervised Compose service:

```json
{
  "name": "shared-two-apps-compose",
  "cwd": "examples/tinyidp-shared-two-apps",
  "command": ["docker", "compose", "up", "--build", "--remove-orphans"],
  "health": {
    "type": "http",
    "url": "https://idp.localhost:8443/readyz",
    "timeout_ms": 120000,
    "ca_file": "runtime/caddy-local-root.crt"
  }
}
```

If the installed devctl health schema lacks `ca_file`, the plugin should launch
a small repository-owned readiness wrapper or rely on Compose health plus a
separate `smoke` command. It must not disable TLS verification.

### `command.run`

Helper commands do the operational work that is currently scattered across
numbered scripts:

```text
devctl command secrets-fetch
devctl command pki-backup
devctl command pki-export-root
devctl command smoke
devctl command browser-test
devctl command state-status
```

Destructive commands require explicit parameters and refuse dry-run ambiguity:

```text
devctl command pki-restore --arg expected_root_sha256=<fingerprint>
devctl command state-reset --arg scope=shared-two-apps-app-state
```

The exact installed CLI syntax for dynamic command arguments must be taken from
`devctl help scripting-guide` during implementation. The protocol's
`command.run` handler dispatches only names declared in the handshake; unknown
names return `E_UNSUPPORTED`.

## 5. devctl profiles

`.devctl.yaml` exposes one profile per environment:

```yaml
profile:
  active: embedded

profiles:
  embedded:
    display_name: Embedded Provider
    description: One-process development HTTP provider and relying party.
    plugins: [tinyidp]
    env:
      TINYIDP_DEV_MANIFEST: dev/environments/embedded.yaml

  shared-two-apps:
    display_name: Shared HTTPS Applications
    description: Production-shaped TinyIDP with Message Desk and go-go-goja.
    plugins: [tinyidp]
    env:
      TINYIDP_DEV_MANIFEST: dev/environments/shared-two-apps.yaml

plugins:
  - id: tinyidp
    path: python3
    args: [./devctl/tinyidp.py]
    priority: 10
```

`TINYIDP_DEV_MANIFEST` is a non-secret selector. Reading this environment value
is acceptable because it carries configuration, not credentials; the project
must document it in `.devctl.yaml` and the help tutorial.

## 6. Vault hierarchy

Use the same logical shape in every tier:

```text
kv/tiny-idp/<tier>/<deployment>/runtime
kv/tiny-idp/<tier>/<deployment>/bootstrap
kv/tiny-idp/<tier>/<deployment>/integrations/<integration>
kv/tiny-idp/<tier>/_shared/<authority>/pki-storage
```

Examples:

```text
kv/tiny-idp/dev/admin-console/runtime
kv/tiny-idp/dev/admin-console/bootstrap
kv/tiny-idp/dev/jitsi/integrations/jitsi
kv/tiny-idp/dev/_shared/caddy-local/pki-storage
kv/tiny-idp/prod/message-desk/runtime
kv/tiny-idp/prod/message-desk/bootstrap
kv/tiny-idp/prod/_shared/caddy-edge/pki-storage
```

The hierarchy is consistent, but policy and delivery differ:

| Concern | Development | Future production |
|---|---|---|
| Human authentication | OIDC CLI login | break-glass/operator only |
| Workload authentication | usually none | Kubernetes/JWT/OIDC workload identity |
| Delivery | explicit fetch to ignored files | Vault Agent/CSI/template to tmpfs |
| Runtime policy | scoped developer read | workload read only for its deployment |
| Bootstrap policy | operator read/write | separate provisioning role |
| PKI backup policy | restricted operator backup/restore | dedicated recovery custodians |
| State | Docker volumes | backed-up persistent volume, single writer |

Do not encode secret information in path names because Vault list results are
not filtered by value-level policy. KV v2 uses `data/` paths for read/write and
`metadata/` paths for listing and version settings.

### Runtime record

```yaml
schema_version: 1
token_secret_b64: ...
admin_auth_key_b64: ...
admin_action_key_b64: ...
invitation_lookup_key_b64: ...
email_challenge_key_b64: ...
```

### Bootstrap record

```yaml
schema_version: 1
owner_login: admin@example.test
owner_password: ...
bootstrap_generation: 1
```

Bootstrap credentials are inputs to an idempotent creation operation. Updating
Vault does not silently mutate a credential already stored in SQLite.

### Integration records

Integration-specific records isolate blast radius. For example, Jitsi's shared
JWT key belongs under `integrations/jitsi`, not the general runtime record.
Relying-party client secrets belong to the relying party's deployment or
integration path and are never placed in a public client catalog.

## 7. Caddy CA private-key backup and restore

### Backup unit

Use Caddy's supported `caddy storage export` command to produce a storage
archive instead of copying only `root.key`. Caddy storage contains the root and
intermediate CA material, issued assets, and storage metadata needed for a
faithful recovery. The backup command runs against a quiesced or consistently
snapshotted volume and stores:

```yaml
schema_version: 1
format: caddy-storage-export-tar
archive_b64: ...
archive_sha256: ...
root_cert_sha256: ...
caddy_version: ...
source_volume: tinyidp-local-caddy-pki
created_at: ...
created_by: ...
```

The KV record is:

```text
kv/tiny-idp/dev/_shared/caddy-local/pki-storage
```

Require KV v2 check-and-set. A backup reads the current Vault version, computes
the live root fingerprint and archive digest, and writes a new version only
when:

- no record exists and the caller explicitly requests initialization; or
- the live root fingerprint matches the current record and the archive digest
  represents a newer backup; or
- the caller explicitly authorizes an authority-generation change.

Pseudocode:

```text
assert Caddy storage is stable for export
archive = caddy storage export using the active Caddyfile
assert archive contains expected local CA certificate and private-key entries
root_fingerprint = SHA-256(public root DER)
archive_digest = SHA-256(archive bytes)
current = Vault KV metadata/data read
assert CAS preconditions
vault kv put with cas=current.version and base64 archive
read new version metadata
print version, timestamp, root fingerprint, and archive digest only
zero/remove local temporary archive
```

The CA archive is a high-impact secret. It receives a stricter policy than
ordinary development application secrets. Normal `devctl up` needs no access.
Only `pki-backup` and `pki-restore` roles can read or write it. Vault audit
logging records access; output and shell tracing must never contain
`archive_b64`.

### Restore procedure

Restore is never an automatic prepare step:

```text
1. Stop every profile that mounts tinyidp-local-caddy-pki.
2. Inspect the target volume; refuse a nonempty target.
3. Read an explicit Vault version, preferably through a short-lived session.
4. Decode into a private temporary directory with umask 077.
5. Verify archive SHA-256 and expected root certificate fingerprint.
6. Import into a newly created staging volume with caddy storage import.
7. Start an isolated Caddy validation container against the staging volume.
8. Export root.crt and compare the fingerprint again.
9. Rename/swap only after operator confirmation; preserve the old volume.
10. Run HTTPS smoke and browser tests before deleting any rollback volume.
```

If Docker cannot atomically rename volumes, copy the verified staging contents
to a newly named recovery volume and update the external-volume mapping. Do not
delete the old volume in the restore command. Recovery output reports volume
names, Vault version, and fingerprints, never key material.

### Threat model

- Anyone who reads the archive can mint certificates trusted by developer
  machines. The backup role is therefore narrower than app-secret read access.
- A malicious or stale archive can replace the trusted authority. Explicit
  Vault versions, CAS, fingerprint comparison, empty-target restore, and
  rollback volumes reduce this risk.
- Vault loss and Docker volume loss must not be correlated. Vault storage and
  its snapshots belong to the platform backup domain.
- A compromised development CA must be rotated, removed from every trust store,
  and replaced. Restoring a known-compromised version is forbidden.
- Production should normally use a purpose-built PKI/ingress model. The common
  layout does not imply that a local development CA is promoted to production.

## 8. Secret materialization

Materialize Vault values as owner-only files:

```text
examples/<profile>/runtime/secrets/
  token.key
  admin-auth.key
  admin-action.key
  invitation-lookup.key
  email-challenge.key
  owner-password.txt
```

The process uses `umask 077`, a private `mktemp -d`, byte-length validation,
atomic renames, and a cleanup trap. It never uses `set -x`, secret environment
variables, command-line secret values, or predictable `/tmp` files.

Compose consumes files through `secrets:`. Direct Go profiles receive only file
paths. The runtime directory is ignored and reconstructible from Vault.

## 9. Shared Compose patterns

Compose fragments standardize mechanisms without hiding topology:

### `dev/compose/caddy-pki.yaml`

- Caddy `/data` mounts external `tinyidp-local-caddy-pki`.
- a one-shot CA exporter mounts `/data` read-only;
- public `root.crt` is copied to a separate trust volume;
- only Caddy can read CA private keys;
- ports bind explicitly, with operator-only endpoints on loopback.

### Application stacks

Each stack retains its services and networks but adopts:

- external issuer `https://idp.localhost:8443`;
- TinyIDP `trusted-proxy-http` listener mode;
- a CIDR restricted to the Caddy-facing network;
- required production files mounted read-only;
- SQLite, audit, and admin backup artifacts on durable application storage;
- Vault-materialized Compose secrets;
- health checks that validate TLS with the exported root;
- no `curl -k`, no committed password, and no secret in `environment:`.

Compose merge behavior can be surprising for arrays such as `command` and
ports. Every migrated profile must inspect `docker compose config` and retain a
profile-level acceptance test. Shared fragments should be limited to services
and mounts that merge predictably.

## 10. Migration of existing examples

### Wave 1: establish the platform

1. Add plugin, manifest schema, profiles, and contract tests.
2. Add Vault fetch and Caddy backup/restore commands.
3. Add the Glazed help tutorial.
4. Migrate `shared-two-apps` first because it already has the richest reusable
   HTTPS pattern.

### Wave 2: HTTPS integrations

- **Jitsi:** move generated passwords and Jitsi JWT material into its Vault
  paths; reuse the CA fragment; retain UDP media exposure and full conference
  tests.
- **Admin console:** add all administration keys, backup root, owner bootstrap,
  API 401 checks, readiness worker checks, CSP, and browser acceptance.

### Wave 3: development examples

- **External Message Desk:** keep its explicit loopback-HTTP profile for
  pedagogical contrast, but launch and reset it through devctl. Add an HTTPS
  production-shaped sibling profile rather than silently changing the existing
  development semantics.
- **Embedded:** launch directly through devctl and add a readiness route if one
  is absent. Preserve the in-process transport and single-origin model.
- **Message app frontend:** use `prepare.run` for `pnpm install` and
  `build.run` or a supervised Vite service for frontend development.
- **Script/config/user fixtures:** expose validation, explanation, and selected
  scenario runs as devctl commands; they do not need Caddy or Vault unless a
  scenario actually starts a server.

### Wave 4: production reference

Refactor `production-host` documentation to consume the same logical Vault
layout and file contract, but do not claim local Compose is a production
deployment. Add a local production-reference profile for validation. Future
Kubernetes delivery should use workload identity and Vault Agent/CSI rather
than host-side fetch scripts.

## 11. Glazed help playbook

Create:

```text
cmd/tinyidp/doc/pages/tutorial-local-development-apps.md
```

Frontmatter:

```yaml
---
Title: "Build Local Applications with TinyIDP"
Slug: "tutorial-local-development-apps"
Short: "Choose an integration mode, register a client, run a trusted local issuer, and validate a TinyIDP-backed application."
Topics:
  - development
  - oidc
  - embedding
  - tls
Commands:
  - serve-dev
  - serve-production
  - admin
Flags:
  - issuer
  - listener-mode
  - clients-file
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---
```

The body must not repeat the title as an H1 because Glazed renders it. It
teaches:

1. choose embedded, external-development, or production-shaped-local mode;
2. decide the canonical issuer before registering redirects;
3. choose public PKCE or confidential client semantics;
4. keep browser issuer identity separate from private backchannel routing;
5. use Caddy and the shared CA for local HTTPS;
6. consume secrets by file and preserve SQLite/key coherence;
7. protect APIs with opaque-token introspection rather than database access;
8. keep bearer tokens out of browser-untrusted or embedded scripting layers;
9. use devctl plan, prepare, up, status, logs, smoke, and down;
10. test login, consent, CSRF rejection, logout, audience, scopes, readiness,
    TLS trust, restart durability, and failure behavior.

The PULP OS case becomes a compact advanced example: a native device uses RFC
8628, keeps bearer tokens outside JavaScript, declares an RFC 8707 resource,
and calls REST/WebSocket resources protected by RFC 7662 introspection.

The tutorial ends with:

| Problem | Cause | Solution |
|---|---|---|
| issuer mismatch | browser issuer and service DNS were confused | keep one canonical public issuer and configure a separate backchannel |
| browser certificate warning | shared public root is not installed | export the root, verify its fingerprint, and install it deliberately |
| `authorization_pending` never resolves | device approval or poll timing is wrong | inspect the device grant and follow RFC 8628 interval behavior |
| admin readiness fails | required key, audit, backup, or signing state is absent | run devctl validate and inspect readiness details |
| restart invalidates state | secret files no longer match the database | restore the Vault version paired with that state; do not regenerate |
| Compose looks correct but behaves differently | merge changed an array or mount | inspect `docker compose config` for the selected profile |

`cmd/tinyidp/doc/doc.go` already embeds `pages`, and
`cmd/tinyidp/main.go` already calls `help_cmd.SetupCobraRootCommand`; no new
help-system bootstrap is required. Tests should load the help system, resolve
the new slug, and assert the tutorial is discoverable.

## 12. Testing strategy

### Plugin contract tests

- first stdout frame is the v2 handshake;
- stdout contains only valid one-object-per-line NDJSON;
- unknown operations and commands return `E_UNSUPPORTED`;
- dry-run performs no writes, container starts, Vault writes, or volume changes;
- deadline expiration terminates child processes;
- manifest paths cannot escape `ctx.repo_root`;
- every declared profile produces a deterministic plan.

### Secret and PKI tests

- malformed/missing/wrong-length Vault fields fail before promotion;
- materialization is atomic and modes are `0700`/`0600`;
- KV writes use CAS and preserve prior versions;
- backup archive digest and root fingerprint are stable;
- restore refuses nonempty volumes and wrong expected fingerprints;
- a recovered staging volume serves a leaf that validates against the original
  root;
- logs and `docker compose config` contain no secret values.

### Profile acceptance matrix

Every server profile must pass:

- configuration render;
- readiness with certificate verification where applicable;
- browser authentication and logout;
- restart durability;
- wrong-secret negative test;
- issuer/audience/scope negative tests;
- clean `devctl down`;
- state reset limited to the declared scope.

Jitsi retains media and policy tests. Shared-two-apps retains both relying-party
flows and Mailpit. Admin-console retains authorization, audit, backup,
one-time-secret, CSP, and frontend tests. Embedded retains in-process transport
tests.

## Design Decisions

1. **One control plane, multiple runtime modes.** devctl presents a consistent
   operator workflow without pretending a one-process embedded demo and a Jitsi
   Compose stack have the same topology.
2. **Manifests describe; plugins operate.** YAML contains non-secret facts.
   Side effects and validation live in testable plugin operations.
3. **The live CA remains in Docker and the recovery copy lives in Vault.** This
   satisfies daily performance and future safekeeping without automatic
   synchronization.
4. **Back up Caddy storage through Caddy's API.** `caddy storage export/import`
   is less brittle than relying on internal filesystem paths alone.
5. **Use KV v2 CAS and versions.** A CA backup must not be silently overwritten,
   and operators must be able to select an audited recovery version.
6. **Production shares layout, not local authentication mechanics.** Paths and
   file contracts remain familiar; delivery moves to workload identity and
   template/CSI mechanisms.
7. **Refactor examples incrementally.** Existing protocol semantics and
   acceptance tests are migration constraints.
8. **Expose dangerous work as explicit commands.** Reset and CA restore never
   run as incidental startup steps.
9. **Teach through the TinyIDP CLI.** The local-application playbook belongs in
   embedded Glazed help so it is versioned and discoverable with the binary.

## Alternatives Considered

- **One giant Compose file for every demo.** Rejected because embedded and
  script-only scenarios do not need containers, while Jitsi has materially
  different networks and lifecycle.
- **A Makefile-only interface.** Rejected because it does not provide merged
  plans, profile selection, process supervision, structured validation, or
  dynamic operational commands.
- **Replace all existing scripts immediately.** Rejected because the scripts
  encode tested profile behavior. Migrate behind devctl first, then delete only
  proven duplication.
- **Restore Caddy storage on every startup.** Rejected because Vault could
  overwrite newer healthy live state and turn ordinary startup into a
  high-impact key operation.
- **Back up only `root.key`.** Rejected because the intermediate key,
  certificates, and storage metadata participate in the active authority.
- **Use Vault PKI as the local issuer now.** Deferred. It is a valid future
  architecture but changes Caddy issuance, availability, and developer setup.
  The request is safekeeping of the existing Caddy CA.
- **Put SQLite in Vault.** Rejected because KV is not a transactional database
  or application-backup format.
- **Force every demo to HTTPS.** Rejected because an explicitly classified
  loopback development profile is useful for teaching, while
  production-shaped profiles must use HTTPS.

## Implementation Plan

### Phase 0: owner decisions

- Confirm the actual KV v2 mount name and OIDC roles on `scapegoat.dev`.
- Approve `tiny-idp/<tier>/<deployment>/<class>` as the logical hierarchy.
- Confirm whether Caddy backup access requires a separate Vault group.
- Record the current live CA root fingerprint before any backup code runs.

### Phase 1: control-plane foundation

- Add `.devctl.yaml`, plugin skeleton, manifest parser, JSON-schema-style
  validation, and protocol tests.
- Implement `config.mutate`, `validate.run`, and `launch.plan`.
- Add direct `embedded` and Compose `shared-two-apps` profiles.
- Verify with `devctl plugins list`, `plan`, `up`, `status`, logs, smoke, and
  `down`.

### Phase 2: secrets and CA recovery

- Create least-privilege Vault policies for runtime, bootstrap, and PKI roles.
- Implement atomic materialization and profile-specific field validation.
- Implement Caddy storage export, KV v2 CAS backup, empty-volume restore,
  fingerprint verification, and rollback preservation.
- Perform a non-destructive recovery drill into a separate volume.

### Phase 3: example migration

- Migrate shared-two-apps and admin-console.
- Migrate Jitsi without weakening its media/browser tests.
- Bring external-message-desk, embedded, message-app, script/config, and user
  fixtures into profiles.
- Remove duplicated scripts only after equivalence tests pass.

### Phase 4: documentation and production-shaped future

- Add and test `tutorial-local-development-apps`.
- Update each README to make devctl the primary path and direct commands the
  debugging/reference path.
- Define production workload-auth and Vault Agent/CSI follow-up work.
- Document a CA compromise/rotation playbook separately from ordinary restore.

### Definition of done

- `devctl plan` works without Vault access and clearly identifies the selected
  environment classification.
- Every runnable example has a profile or a documented reason it is only a
  fixture/library.
- All production-shaped local profiles use the same live Caddy CA volume and
  Vault path for recovery.
- A tested backup and staging restore reproduce the same root fingerprint.
- No normal startup identity can read the CA archive.
- All application secrets are file-delivered from tier/deployment-scoped Vault
  paths.
- No tracked file, plan, Compose render, process argument, or log contains a
  secret.
- Existing example acceptance tests pass through devctl.
- `tinyidp help tutorial-local-development-apps` resolves and contains runnable
  commands, troubleshooting, and cross-references.
- The repository builds and tests, the plugin contract tests pass, and
  `docmgr doctor` remains clean.

## Open Questions

- What is the production Vault mount and namespace convention? The logical
  hierarchy is independent of the physical mount, but policies need the real
  `data/` and `metadata/` API paths.
- Does the installed devctl health schema support a custom CA file? If not, the
  first implementation must choose between a readiness wrapper and Compose
  health aggregation.
- Should `prepare.run` fetch secrets automatically, or should `secrets-fetch`
  remain an explicit command? Automatic fetch is convenient; explicit fetch
  gives clearer Vault audit intent. The recommended compromise is an explicit
  named prepare step that `up` invokes and `plan` only describes.
- Can `caddy storage export` operate consistently while Caddy is running for
  this file-system storage version, or should the command stop Caddy briefly?
  The implementation must test the installed Caddy image and prefer a
  quiesced export if consistency is not guaranteed.
- Should the CA archive use a separate Vault mount with stricter retention and
  replication rather than merely a separate KV path?
- Which old ticket-local assurance scripts should become repository-owned
  commands, and which should remain historical evidence?

## References

- `design-doc/02-vault-backed-local-tls-and-secret-provisioning-design.md`
- `examples/embedded/README.md`
- `examples/production-host/README.md`
- `examples/tinyidp-external-message-desk/README.md`
- `examples/tinyidp-shared-two-apps/compose.yaml`
- `examples/tinyidp-shared-two-apps/scripts/00-init-secrets.sh`
- `examples/tinyidp-jitsi/compose.yaml`
- `cmd/tinyidp/doc/doc.go`
- `cmd/tinyidp/doc/pages/tutorial-device-authorization.md`
- `internal/sections/production/section.go`
- `internal/cmds/admin_console.go`
- Obsidian: `Research/KB/Projects/infrastructure-and-release.md`
- Obsidian: `Projects/2026/07/24/PROJ - PULP OS Device Authorization - Native OAuth and E-Ink Sensor Streaming.md`
- Caddy documentation: `https://caddyserver.com/docs/conventions`
- Caddy command reference: `https://caddyserver.com/docs/command-line`
- Caddy `tls internal`: `https://caddyserver.com/docs/caddyfile/directives/tls`
- Vault KV v2: `https://developer.hashicorp.com/vault/docs/secrets/kv/kv-v2`
- Vault policies: `https://developer.hashicorp.com/vault/docs/concepts/policies`
- Vault response wrapping: `https://developer.hashicorp.com/vault/docs/concepts/response-wrapping`
- RFC 6749, RFC 7636, RFC 7662, RFC 8252, RFC 8628, and RFC 8707
