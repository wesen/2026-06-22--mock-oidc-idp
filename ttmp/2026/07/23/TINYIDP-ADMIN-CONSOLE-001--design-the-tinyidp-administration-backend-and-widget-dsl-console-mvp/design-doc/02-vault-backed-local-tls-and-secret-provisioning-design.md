---
Title: Vault-Backed Local TLS and Secret Provisioning Design
Ticket: TINYIDP-ADMIN-CONSOLE-001
Status: deprecated
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
    - Path: repo://examples/tinyidp-shared-two-apps/compose.yaml
      Note: Reference topology for persistent Caddy PKI and exported public root
    - Path: repo://examples/tinyidp-shared-two-apps/scripts/00-init-secrets.sh
      Note: Existing local secret initialization flow to replace with Vault materialization
    - Path: repo://examples/tinyidp-shared-two-apps/scripts/01-export-browser-ca.sh
      Note: Existing public CA export workflow
    - Path: repo://internal/cmds/admin_console.go
      Note: Administration console runtime and readiness integration
    - Path: repo://internal/sections/production/section.go
      Note: Authoritative production configuration and key validation contract
ExternalSources: []
Summary: Design for reproducible local HTTPS environments that obtain TinyIDP application secrets from the scapegoat.dev Vault while retaining the existing persistent Caddy PKI volume.
LastUpdated: 2026-07-24T11:02:24.564044523-04:00
WhatFor: Preserve the earlier application-secret analysis; use design document 03 for the authoritative CA backup and unified environment design.
WhenToUse: Before changing the admin-console Compose example, Vault policy, bootstrap scripts, secret rotation process, or local CA handling.
---


# Vault-Backed Local TLS and Secret Provisioning Design

> Superseded on 2026-07-24 by
> `03-unified-tinyidp-development-and-demo-environment-platform.md`. The newer
> design incorporates the owner decision to keep a guarded, versioned
> disaster-recovery export of Caddy's CA private material in Vault. This
> document remains useful for its application-secret analysis, but its decision
> not to back up Caddy private keys is no longer authoritative.

## Executive Summary

The local TinyIDP administration environment needs stable HTTPS and stable
application cryptographic keys across container rebuilds. Today, the example
stacks already preserve Caddy's local certificate authority in the external
Docker volume `tinyidp-local-caddy-pki`, but their setup scripts generate
application secrets on the workstation. Recreating or losing those files can
invalidate sessions, action handles, invitations, and email challenges. It can
also make a persisted SQLite database unusable with a newly generated token
key.

This design assigns each class of state to one authoritative persistence
mechanism:

- HashiCorp Vault at `scapegoat.dev` stores reusable TinyIDP application
  secrets and the local bootstrap credential.
- The external Docker volume `tinyidp-local-caddy-pki` stores Caddy's local CA
  private key and certificate. The CA private key is not copied into Vault.
- A local, ignored `runtime/secrets` directory materializes Vault values as
  files for Docker Compose. It is a cache, not a source of truth.
- The TinyIDP state volume stores SQLite, audit records, and administration
  backup artifacts.
- A generated public CA certificate may be copied to a local trust directory
  because it is public trust material rather than a secret.

The setup is intentionally a local-development design. It uses the existing
human Vault login and existing workstation trust in the Caddy CA. It does not
introduce a permanent Vault token, export the Caddy CA key, or attempt to make
the example a production deployment.

## Problem Statement

The administration backend adds several cryptographic dependencies to the
existing token-signing secret. A runnable production-mode server requires
stable values for token encryption/signing, administrator authentication,
administrator action handles, invitation lookup, and, when email signup is
enabled, email challenges. These values have different rotation consequences,
but they share two operational properties:

1. They must not appear in Git, Compose YAML, process arguments, shell traces,
   or terminal output.
2. They must remain consistent with durable application state across ordinary
   `docker compose down`, rebuild, and workstation restart operations.

The existing script
`examples/tinyidp-shared-two-apps/scripts/00-init-secrets.sh` creates some
values locally with `/dev/urandom` and uses fixed development passwords for
others. This is convenient for an isolated demo but does not provide a shared,
recoverable secret source. The administration-console example would need even
more local key files, increasing the probability of accidental regeneration or
inconsistent state.

TLS has a related but distinct persistence requirement. Caddy's `tls internal`
mode creates a CA private key below `/data/caddy/pki/authorities/local`. The
existing external volume gives that CA a stable identity, and the public root
has already been installed on the development workstation. Regenerating that
volume would require reinstalling trust. Copying the CA private key into Vault,
however, would create two authorities for the same highly sensitive key and
would expand the compromise surface without solving a current problem.

The design must therefore preserve the distinction between application
secrets, CA signing material, public trust material, and application state.

## Proposed Solution

### Architecture and trust boundaries

```text
                         human OIDC login
Developer workstation --------------------------+
                                                 v
                                      +--------------------+
                                      | scapegoat.dev      |
                                      | Vault KV v2        |
                                      | app secrets only   |
                                      +---------+----------+
                                                |
                                   authenticated fetch
                                                |
                                                v
+---------------------------- workstation / Docker host --------------------+
| runtime/secrets/              public root export                           |
|  token.key          +-------------------------------+                      |
|  admin-auth.key     |                               |                      |
|  admin-action.key   v                               |                      |
|  ...           Docker Compose                      |                      |
|                    |                               |                      |
|        +-----------+------------+       +----------+-----------+          |
|        | TinyIDP                | HTTP  | Caddy                |          |
|        | trusted-proxy-http     +------>| :8443, tls internal |          |
|        | SQLite + audit volume  |       | /data volume         |          |
|        +------------------------+       +----------+-----------+          |
|                                                  |                       |
|                                      tinyidp-local-caddy-pki              |
|                                      CA private key stays here            |
+---------------------------------------------------------------------------+
```

Caddy terminates HTTPS and forwards plain HTTP only over the isolated Compose
network. TinyIDP runs with `--listener-mode=trusted-proxy-http`, an external
issuer such as `https://idp.localhost:8443`, and a narrowly scoped
`--trusted-proxy-cidrs` value matching that network. The browser never connects
directly to TinyIDP's HTTP listener.

### Vault data model

Use KV v2 and one record per stable example stack:

```text
kv/dev/tiny-idp/admin-console-local
kv/dev/tiny-idp/shared-two-apps
kv/dev/tiny-idp/jitsi
```

The admin-console record contains:

```yaml
token_secret_b64:              <base64 of required token secret bytes>
admin_auth_key_b64:            <base64 of exactly 32 bytes>
admin_action_key_b64:          <base64 of at least 32 bytes>
invitation_lookup_key_b64:     <base64 of exactly 32 bytes>
email_challenge_key_b64:       <base64 of exactly 32 bytes>
owner_password:                <text bootstrap password>
schema_version:                1
```

Binary values are base64-encoded because KV JSON strings cannot safely express
arbitrary bytes. `schema_version` lets the materialization script reject an
unknown record rather than silently writing an incomplete secret set. The
password remains text and is written with its final newline stripped or
preserved according to the receiving CLI's `--password-from-stdin` contract.

The Vault policy should grant only `read` on
`kv/data/dev/tiny-idp/admin-console-local` and, if the setup is allowed to
initialize a missing record, `create` and `update` on that exact path. Listing
or reading sibling production paths is unnecessary. The preferred initial
workflow is an operator-authenticated `vault login -method=oidc`; no Vault token
is committed, included in Compose, or copied into the runtime directory.

### Materialized secret contract

The fetch script creates the following ignored files:

```text
examples/tinyidp-admin-console/runtime/
  .gitignore
  secrets/
    token.key
    admin-auth.key
    admin-action.key
    invitation-lookup.key
    email-challenge.key
    owner-password.txt
```

The directory mode is `0700`, each file mode is `0600`, and the script begins
with `umask 077`. It must not enable `set -x`, echo values, place secret JSON in
a predictable temporary file, or pass secret values as command-line
arguments. A secure implementation can keep the Vault JSON in a private
temporary directory created by `mktemp -d`, register a trap, decode each value
to a temporary file, validate its byte length, and atomically rename it.

Pseudocode:

```text
require vault, jq, base64, docker
assert authenticated Vault token is currently usable
record = vault kv get -format=json kv/dev/tiny-idp/admin-console-local
assert record.schema_version == 1

for each binary field:
    decode into private temporary file
    verify exact/minimum byte length
    chmod 0600
    atomically rename to runtime/secrets/<contract name>

write owner_password without logging it
chmod 0600
verify every expected file is present and nonempty
```

Compose mounts these files through its `secrets:` declarations or read-only
bind mounts. TinyIDP receives file paths such as
`--token-secret-file=/run/secrets/token_secret` rather than values. The setup
script may use environment variables for non-secret configuration such as the
Vault address or secret path only after documenting them; secret payloads must
not be exported into the environment.

### Caddy PKI lifecycle

The volume name remains exactly `tinyidp-local-caddy-pki` so all local TinyIDP
examples use the already trusted CA. Initialization performs a read-only
inspection and creates the volume only if it is absent:

```text
docker volume inspect tinyidp-local-caddy-pki
if absent:
    docker volume create with purpose and retention labels
```

The proxy mounts that volume at `/data`. A one-shot CA-export container mounts
it read-only and copies
`/caddy-data/caddy/pki/authorities/local/root.crt` to a separate `local-ca`
volume. Application containers that call the HTTPS issuer mount only the
exported root. No container other than Caddy needs access to the CA private
key.

If the root is already installed on the workstation, no trust mutation is
needed. `02-export-browser-ca.sh` should export the public root and print its
fingerprint so an operator can compare or install it when necessary. It must
not automatically alter the system trust store.

### Bootstrap and idempotency

The proposed example layout is:

```text
examples/tinyidp-admin-console/
  compose.yaml
  Caddyfile
  clients.json
  themes.json
  open-signup.js
  scripts/
    00-fetch-vault-secrets.sh
    01-bootstrap.sh
    02-export-browser-ca.sh
    03-smoke.sh
  runtime/
    .gitignore
  README.md
```

`00-fetch-vault-secrets.sh` performs only authentication checks, retrieval,
validation, and materialization. `01-bootstrap.sh` ensures the external Caddy
volume exists, starts the required services, initializes SQLite only when
absent, and creates the owner only when an owner with the configured login is
absent. It must never overwrite a database or reset an owner password merely
because Vault contains a different password.

The lifecycle is:

```text
Vault auth -> fetch secret files -> verify/create Caddy volume
           -> start Caddy -> wait for public root
           -> initialize durable state if absent
           -> bootstrap owner if absent
           -> start TinyIDP -> wait for /readyz -> smoke tests
```

The owner password in Vault is a bootstrap input, not a continuously reconciled
credential. Changing it in Vault does not change the password hash already in
SQLite. A separate explicit password-reset operation is required.

### Server configuration contract

The Compose command must supply the production section's required inputs,
including:

- `--listener-mode=trusted-proxy-http`
- `--issuer=https://idp.localhost:8443`
- `--clients-file`, `--theme-dir`, `--theme-catalog-file`, and
  `--signup-program-file`
- `--db` and `--audit-path`
- `--token-secret-file`
- `--admin-auth-key-file`
- `--admin-action-key-file`
- `--admin-backup-root`
- `--invitation-lookup-key-file`
- `--email-challenge-key-file` and SMTP settings when email challenges are
  enabled
- `--trusted-proxy-cidrs` restricted to the proxy network

SQLite, audit logs, and `--admin-backup-root` reside on a persistent state
volume. They are operational state and belong in backups; they do not belong in
Vault KV.

### Failure behavior

Setup fails closed before starting TinyIDP when Vault authentication fails, a
field is absent, base64 is malformed, a key length is wrong, file permissions
cannot be established, or the expected external Caddy volume cannot be
inspected or created. Existing valid materialized files are not partially
overwritten: each complete fetch is staged and promoted atomically.

If Vault is temporarily unavailable after the files have been materialized,
the default command should fail and tell the operator how to opt into an
explicit offline reuse mode. Silent reuse makes it too easy to run with stale
or partially rotated material. Offline reuse, if implemented, validates all
files and prints only record metadata such as the path and last fetched
timestamp.

## Design Decisions

1. **Vault stores application secrets, not the Caddy CA key.** This keeps the
   strongest local signing key within the established Caddy persistence
   boundary and avoids two copies with different access-control systems.
2. **The Caddy volume is shared by name across examples.** This preserves the
   root already installed by the developer and prevents unnecessary trust-store
   churn.
3. **TinyIDP consumes secret files.** Files avoid values in process listings,
   Compose interpolation, shell history, and application flags.
4. **Materialized files are disposable.** Vault is authoritative; the runtime
   directory is ignored and can be deleted and reconstructed.
5. **Application state is not stored in Vault.** SQLite and administration
   artifacts have transaction, size, and backup semantics that KV does not
   provide.
6. **Vault authentication remains human and short-lived for the MVP.** A local
   example does not justify a long-lived machine credential. A Vault Agent
   sidecar becomes appropriate for unattended environments.
7. **Each stable stack receives its own KV path.** This limits blast radius and
   allows independent rotation without introducing a global shared TinyIDP
   identity.
8. **No compatibility or multi-key rotation layer is added in this phase.**
   Rotation is an explicit maintenance event with documented consequences.

### Rotation matrix

| Material | Rotation consequence | Required coordination |
|---|---|---|
| Token secret | Existing durable token-related state may become unreadable or invalid | Back up and validate SQLite; rotate in a planned maintenance window |
| Admin auth key | Existing admin authentication/session material is invalidated | Expect administrators to sign in again |
| Admin action key | Outstanding signed action handles become invalid | Finish or abandon pending actions before rotation |
| Invitation lookup key | Existing invitations can no longer be looked up | Drain/reissue invitations before rotation |
| Email challenge key | Outstanding email challenges become invalid | Allow expiry or notify testers |
| Owner password | Vault change alone has no effect on SQLite | Run an explicit TinyIDP password reset |
| TLS leaf certificates | Normal Caddy renewal has no application-state effect | None while the CA remains trusted |
| Caddy CA | All local trust and issued leaves change | Exceptional recovery procedure; reinstall trust |

## Alternatives Considered

- **Regenerate every secret on each run.** Rejected because persistent SQLite
  and pending administrative workflows depend on stable cryptographic keys.
- **Keep generated secrets only in the Docker state volume.** Better than
  regeneration, but difficult to inspect, restore, share across worktrees, and
  recover after volume loss.
- **Commit development keys to Git.** Rejected because repository access and
  secret access have different trust requirements, and copied examples tend to
  escape their original scope.
- **Store the Caddy CA private key in Vault as well as the Docker volume.**
  Rejected because duplication expands compromise and recovery paths. If a
  future system adopts Vault PKI, Vault should become the CA and issue leaves;
  it should not casually mirror Caddy's CA key.
- **Run TinyIDP's native TLS listener and give it leaf certificates.** Rejected
  for this stack because Caddy already provides local issuance, renewal,
  hostname routing, and the trusted root. Proxy termination minimizes key
  distribution.
- **Run Vault Agent immediately.** Deferred. It is the preferred unattended
  Compose pattern, but the MVP is explicitly a developer-invoked setup using an
  existing authenticated Vault CLI session.

## Implementation Plan

### Phase 1: Vault contract and operator setup

- Confirm the actual KV mount name and the developer policy on
  `scapegoat.dev`.
- Create `kv/dev/tiny-idp/admin-console-local` with versioned metadata and
  cryptographically random values.
- Document the exact Vault login command, policy boundary, and recovery owner.
- Record only fingerprints and Vault version metadata in diagnostic output.

### Phase 2: Example scaffold and secret materialization

- Add `examples/tinyidp-admin-console` with an ignored runtime tree.
- Implement `00-fetch-vault-secrets.sh` with dependency checks, private
  temporary staging, byte-length validation, atomic promotion, and safe
  permissions.
- Add a non-secret manifest that maps Vault fields to Compose secret names,
  output filenames, and length constraints.
- Add shell tests or a fake `vault` executable covering missing fields,
  malformed base64, wrong lengths, failed authentication, and partial writes.

### Phase 3: Compose and persistent PKI

- Reuse external volume `tinyidp-local-caddy-pki`.
- Add Caddy, CA-export, TinyIDP, and optional Mailpit services.
- Mount only the public root into HTTPS client containers.
- Configure all production admin flags and a dedicated persistent backup root.
- Restrict published ports to those required for browser access and local
  diagnostics.

### Phase 4: Bootstrap, verification, and documentation

- Make owner and database initialization idempotent.
- Verify `https://idp.localhost:8443/readyz` using the exported CA, never
  `curl -k`.
- Verify the `/admin` shell, an unauthenticated API response of `401`, the owner
  grant, Content Security Policy, and internal admin readiness workers.
- Document startup, shutdown, offline behavior, secret rotation, CA export,
  volume backup, and complete teardown.

### Acceptance criteria

- Removing `runtime/secrets` and rerunning fetch reproduces byte-identical
  application secret files from Vault.
- Rebuilding containers does not change the CA fingerprint, invalidate the
  installed root, or recreate the TinyIDP database.
- No secret value appears in tracked files, `docker compose config`, process
  arguments, setup output, or test logs.
- Wrong or incomplete Vault data prevents service startup without modifying the
  last complete materialization.
- TinyIDP receives every required production-mode admin key through a file.
- Caddy alone can read the CA private key; clients receive only `root.crt`.
- The full smoke suite succeeds with certificate verification enabled.
- The README explains rotation effects and distinguishes bootstrap credentials
  from continuously reconciled state.

## Open Questions

- What is the exact Vault KV mount/path and policy naming convention on
  `scapegoat.dev`? The document uses `kv/dev/...` as the proposed contract.
- Should the first implementation be read-only, requiring an operator to create
  the record beforehand, or may it create a missing development record?
  Read-only is the safer default.
- Does the token secret have an exact byte-length contract beyond the current
  production validation? The implementation must derive this from the
  production section and tests rather than assuming 32 bytes.
- Should local offline reuse be included in the MVP, or should every setup
  require a live Vault session? This design recommends live fetch by default
  and leaves offline reuse explicit.
- Should `shared-two-apps` and `jitsi` migrate in the same change or only after
  the admin-console example proves the workflow? The recommended sequence is
  admin-console first, then mechanical adoption per independent KV path.

## References

- `design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md`
- `examples/tinyidp-shared-two-apps/compose.yaml`
- `examples/tinyidp-shared-two-apps/scripts/00-init-secrets.sh`
- `examples/tinyidp-shared-two-apps/scripts/01-export-browser-ca.sh`
- `examples/tinyidp-shared-two-apps/scripts/02-smoke.sh`
- `internal/sections/production/section.go`
- `internal/cmds/admin_console.go`
- Obsidian: `Research/KB/Projects/infrastructure-and-release.md`
