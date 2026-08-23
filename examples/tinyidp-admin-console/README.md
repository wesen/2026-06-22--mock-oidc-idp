# TinyIDP administration console local environment

This profile runs the strict TinyIDP production host behind Caddy's retained
local development authority. It exercises the real administration shell,
owner grant, session boundary, Widget DSL pages, audit/outbox workers, managed
backup root, and readiness checks.

## First setup

Authenticate the Vault CLI through the operator's normal OIDC workflow. The
Vault client may use its standard `VAULT_ADDR`, `VAULT_CACERT`, token-helper,
and namespace configuration. Secret payloads are never read from environment
variables.

Initialize missing development records with KV v2 CAS zero and materialize
owner-only files:

```bash
./examples/tinyidp-admin-console/scripts/00-initialize-vault-secrets.sh
```

The logical records are:

```text
kv/tiny-idp/dev/admin-console/runtime
kv/tiny-idp/dev/admin-console/bootstrap
```

The script never overwrites an existing record. Materialized files live below
the ignored `runtime/.secret-generations` directory, and `runtime/secrets` is
an atomically replaced symlink to one complete generation.

## Start and observe

Inspect and start through devctl:

```bash
devctl plan --profile admin-console
devctl validate --profile admin-console
devctl up --profile admin-console
devctl status
devctl logs --service admin-console-compose --follow
```

The console is `https://idp.localhost:8443/admin`. The bootstrap login is
`admin@example.test`; its generated password is available only in the
owner-only materialized `runtime/secrets/owner-password.txt` file.

In a second terminal:

```bash
./examples/tinyidp-admin-console/scripts/02-export-browser-ca.sh
./examples/tinyidp-admin-console/scripts/03-smoke.sh
```

Stop services with `devctl down`.

To discard only the application state after stopping the profile:

```bash
devctl --profile admin-console state-reset -- \
  --confirm reset-admin-console-state
```

The typed confirmation is mandatory. The operation verifies that the external
Caddy authority volume remains present.

## Persistence boundaries

- `tinyidp-local-caddy-pki` is the live Caddy authority and is shared with the
  other TinyIDP HTTPS examples.
- `tinyidp-admin-console_idp-state` contains SQLite, audit JSONL, and managed
  administration artifacts.
- Vault stores application secret records and the versioned Caddy storage
  recovery archive.
- `runtime/secrets` is reconstructible cache material, not the source of truth.

Do not run `docker compose down -v` when testing restart durability. It deletes
application state. It does not delete the external Caddy volume.

## Rotation and failure behavior

Rotation is a maintenance operation:

- token-secret rotation must be coordinated with the database;
- admin-auth rotation ends current administration sessions;
- admin-action rotation invalidates outstanding action handles;
- invitation-key rotation invalidates outstanding invitations;
- email-challenge rotation invalidates pending challenges;
- changing the Vault owner password does not reset the password already hashed
  in SQLite.

`secrets-fetch` validates every record and decoded length before it promotes a
new generation. Missing fields, malformed base64, wrong byte lengths, failed
Vault authentication, or an unmanaged real `runtime/secrets` directory fail
closed. The previously promoted generation remains selected.

The CA archive is not available to normal startup. Backup and recovery are
explicit:

```bash
devctl pki-backup --profile admin-console
devctl pki-restore --profile admin-console -- \
  --version VERSION \
  --expected-root-sha256 FINGERPRINT \
  --target-volume NEW-STAGING-VOLUME
```

Restore refuses the live volume and any nonempty staging volume.
