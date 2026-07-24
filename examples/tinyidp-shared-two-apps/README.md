# Shared TinyIDP local HTTPS development stack

This Compose project runs the production-shaped two-application topology on a
developer workstation. One strict TinyIDP process serves Message Desk and the
go-go-goja auth-host demo through distinct OIDC client registrations and
distinct identity-page themes. Caddy terminates local HTTPS; every Go process
uses its trusted-proxy listener mode and validates the public origin.

The public endpoints are:

- `https://message.localhost:8443` — Message Desk, including open signup.
- `https://goja.localhost:8443` — the generated go-go-goja auth-host demo.
- `https://idp.localhost:8443` — TinyIDP's canonical issuer. Start login or
  signup from an application, not by opening `/authorize` without parameters.
- `http://127.0.0.1:8025` — private local Mailpit operator outbox. It is bound
  to loopback rather than Caddy/public ingress.

The local-only, email-verified operator fixtures are:

- `admin@example.test` — bootstrapped as the
  administrator of the goja demo organization.
- `invitee@example.test` — has no initial
  application membership and is used to prove existing-user invitation
  acceptance.

The passwords and TinyIDP cryptographic keys originate in
`kv/tiny-idp/dev/shared-two-apps/{runtime,bootstrap}` and are materialized into
the gitignored `runtime/secrets` symlink. Read the password files locally when
running an interactive browser test; the setup commands never print them.

An existing checkout that previously used the deterministic password files
must perform one intentional application-state reset after the Vault records
are initialized. Existing SQLite password hashes cannot authenticate a newly
generated Vault password:

```sh
devctl down
devctl --profile shared-two-apps state-reset -- \
  --confirm reset-shared-two-apps-state
```

This is a one-time migration boundary, not an ordinary startup step. It retains
the shared Caddy authority.

## Start and verify

Authenticate the Vault CLI, then run from the repository root:

```sh
devctl --profile shared-two-apps secrets-init
devctl --profile shared-two-apps secrets-fetch
devctl --profile shared-two-apps up
devctl --profile shared-two-apps pki-export-root
devctl --profile shared-two-apps smoke
devctl --profile shared-two-apps browser-test
devctl --profile shared-two-apps state-status
devctl down
```

`ca-export` is expected to show `Exited (0)`. It is a successful one-shot job,
not a crashed server. The goja image is distroless, so the project validates
its readiness from `02-smoke.sh` through the public proxy instead of adding a
shell or HTTP client to the runtime image.

`03-browser-acceptance.py` uses independent cookie jars and the exported local
CA to exercise the complete HTTPS/OIDC behavior:

- Message Desk account creation without an invitation;
- durable email-code verification for both newly created accounts, with codes
  retrieved through Mailpit's authenticated operator API;
- a pending email challenge surviving an actual TinyIDP container restart;
- rejection followed by successful retry of an incorrect email code;
- goja account creation with a one-time TinyIDP signup invitation, validated
  before any email is sent;
- preservation of an opaque application-invite continuation through OIDC;
- successful, atomic membership creation for the newly verified signup and
  the verified invitee fixture;
- rejection of both signup-invite and membership-invite replay; and
- tenant-queryable application audit plus TinyIDP issuance/redemption audit.

The script calls Docker Compose only for operator invitation issuance and
read-only database/audit assertions. It retrieves email codes from the private
Mailpit API. Bearer codes are never written to disk or emitted to its output.

Mailpit is a first-deploy delivery substitution, not a verification bypass.
TinyIDP still generates and hashes each code, persists challenge bindings and
attempt limits, verifies browser submissions, and sets `email_verified=true`
only from native evidence. Do not expose the outbox through public ingress.
Later, point the same SMTP mailer at the real submission server and remove the
catcher; the JavaScript workflow and account semantics do not change.

## Trust the local CA in a browser

Caddy creates a development-only CA in its private data volume. The
`ca-export` job copies only the public root certificate to a separate volume;
the relying parties mount that public certificate read-only. The CA private key
never enters an application container.

`01-export-browser-ca.sh` copies the public root to:

```text
runtime/caddy-local-root.crt
```

Import that certificate as a trusted authority in the browser or operating
system you use for testing. On Debian/Ubuntu system trust, the explicit command
is:

```sh
sudo cp runtime/caddy-local-root.crt /usr/local/share/ca-certificates/tinyidp-local-caddy.crt
sudo update-ca-certificates
```

Firefox may use its own certificate store. Import the same public certificate
under Settings > Privacy & Security > Certificates > Authorities if Firefox
does not honor the operating-system store. Restart the browser after changing
trust. Installing a CA changes the workstation's trust policy, so the Compose
project never performs this step automatically.

To remove the Debian/Ubuntu trust entry later:

```sh
sudo rm /usr/local/share/ca-certificates/tinyidp-local-caddy.crt
sudo update-ca-certificates --fresh
```

## Why the CA export job exists

Caddy stores `root.crt` next to the local CA private key and deliberately uses
owner-only permissions. Mounting the entire Caddy data volume into an
application would both fail for a non-root/distroless process and expose
private signing material unnecessarily. The one-shot job implements a narrow
trust distribution contract:

```text
Caddy protected volume -- read public root --> ca-export
ca-export -- copy mode 0444 --> public trust volume
public trust volume -- read only --> TinyIDP health check, Message Desk, goja
```

Compose gates TinyIDP on `service_completed_successfully`; Message Desk and
goja then wait for TinyIDP readiness. This guarantees that TLS clients never
race the creation or publication of the local root.

The proxy owns the `idp.localhost`, `message.localhost`, and `goja.localhost`
aliases on the relevant Compose networks. Container-side TLS clients therefore
resolve the public names through service DNS without reserving machine-global
Docker subnets. The broad private CIDR accepted by the local trusted-proxy
listeners is appropriate only because these networks are Docker-local; a
production deployment must name the proxy's actual network.

## Persistent protected local CA

The Caddy PKI is stored in the explicitly named external Docker volume:

```text
tinyidp-local-caddy-pki
```

`scripts/00-init-secrets.sh` creates this volume when it does not exist and
labels it `dev.wesen.retention=manual-delete-only`. Compose declares it as an
external volume, so the volume is not owned by this Compose project and is not
removed by `docker compose down -v`. This permits the same trusted development
CA to survive complete application-state resets and to issue certificates for
additional `*.localhost` sites routed through this Caddy instance.

The protection boundary is Docker daemon access. Caddy mounts the volume
read-write at `/data`; the one-shot `ca-export` service mounts it read-only and
copies only `root.crt` to the public trust volume. TinyIDP, Message Desk, goja,
PostgreSQL, and Mailpit never mount the PKI volume. Inside the volume, Caddy
keeps `root.key` and `intermediate.key` root-owned with mode `0600`. Do not mount
this volume into application or build containers and do not copy either private
key into the repository.

Inspect the volume without printing key material:

```sh
docker volume inspect tinyidp-local-caddy-pki
docker compose exec proxy \
  ls -l /data/caddy/pki/authorities/local
```

The exported browser trust file remains public-only:

```text
runtime/caddy-local-root.crt
```

## State, reset, and CA lifetime

Normal restarts preserve all named volumes and therefore preserve the same CA:

```sh
docker compose down
docker compose up -d
```

Use `devctl --profile shared-two-apps state-reset -- --confirm
reset-shared-two-apps-state` to destroy project-owned application volumes. The
typed confirmation is mandatory, and the command verifies that
`tinyidp-local-caddy-pki` remains present.

CA deletion is a separate, explicit destructive operation:

```sh
docker compose down
docker volume rm tinyidp-local-caddy-pki
```

Do this only when intentionally rotating the local CA. It removes the root and
intermediate private keys and cannot be undone without a backup. The next
`scripts/00-init-secrets.sh` creates an empty volume; Caddy then creates a new
CA whose public root must be exported and trusted again. Remove the old CA from
Firefox when rotating it.

The goja PostgreSQL password and generated Glazed configuration also come from
the deployment's Vault integration record. PostgreSQL consumes the password
through `POSTGRES_PASSWORD_FILE`; the distroless goja host consumes its DSN
through `--config-file`. No database credential is rendered into Compose,
passed in argv, or copied into a tracked file.

## Local go-go-goja iteration

This Phase 5 workspace intentionally builds `goja-auth` from the sibling
`../../../go-go-goja` checkout. Rebuild only that service after changing the
auth host or JavaScript routes:

```sh
docker compose up --build -d goja-auth
```

The generated host image is distroless. Inspect it through Compose logs and
the public readiness/acceptance scripts rather than adding debugging packages
to the runtime image.

The one-shot `goja-bootstrap` service runs the same generated image with
`operator bootstrap-admin`. It reads its local database DSN from a Compose
secret, reconciles the immutable TinyIDP issuer/subject identity and initial
organization administrator, and writes an audit record in the same database
transaction. The operation is idempotent and may be rerun with:

```sh
docker compose run --rm goja-bootstrap
```

## Logs and diagnosis

```sh
docker compose logs -f proxy idp message-desk goja-auth postgres
docker compose ps -a
./scripts/02-smoke.sh
```

The three application services should remain `Up`; Postgres, TinyIDP, and
Message Desk should become healthy. A direct visit to TinyIDP `/authorize`
without OAuth parameters is expected to return a protocol error. Use either
application's login or signup action so it supplies the registered client ID,
redirect URI, state, nonce, and PKCE challenge.
