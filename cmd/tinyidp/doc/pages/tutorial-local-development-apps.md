---
Title: "Tutorial: build local development applications with tinyidp"
Slug: tutorial-local-development-apps
Short: "Select an integration mode, run a reproducible HTTPS environment, and validate an OIDC client against tinyidp."
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
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

This tutorial defines the standard process for building a local application that uses tinyidp. It covers direct embedding, a separately supervised provider, and a production-shaped HTTPS environment. The procedure makes issuer identity, client authentication, trust roots, secret persistence, and database persistence explicit so that a successful local test represents the intended deployment architecture.

## 1. Select the integration mode

Choose one mode before assigning ports or writing application code:

- **Embedded**: the application constructs the tinyidp handler in its own process. Use this when the application and identity provider share a lifecycle and failure boundary.
- **External HTTP**: tinyidp and the application run as separate processes on loopback HTTP. Use this for protocol development when browser secure-context behavior is not under test.
- **Production-shaped HTTPS**: Caddy terminates TLS, tinyidp runs behind a trusted-proxy HTTP listener, and both processes are supervised by devctl or Compose. Use this for administration, cookies, redirects, email links, and browser acceptance.

The selected mode determines process ownership, issuer URL, callback URLs, and whether a reverse proxy is part of the security boundary. Do not begin with an HTTP issuer and later place the same state database behind a different HTTPS issuer. Tokens, discovery metadata, callbacks, cookies, and application configuration must agree on one canonical issuer.

## 2. Define the environment contract

Create one committed environment manifest under `dev/environments/`. The manifest is declarative input to the repository's devctl plugin:

```yaml
schema_version: 1
name: my-app
classification: production-shaped-local
origins:
  issuer: https://idp.localhost:8443
vault:
  tier: dev
  deployment: my-app
  runtime_path: tiny-idp/dev/my-app/runtime
  bootstrap_path: tiny-idp/dev/my-app/bootstrap
runtime:
  kind: compose
  services:
    - name: my-app-compose
      cwd: examples/my-app
      command: [docker, compose, up, --build, --remove-orphans]
```

The manifest may name Vault paths and local materialization destinations, but it must not contain secret values. Committed defaults should include only stable topology: profile names, origins, commands, file contracts, and required tools.

The control flow is:

```text
developer
   |
   v
devctl profile ----> environment manifest
   |                         |
   | validate/prepare        | topology + secret schema
   v                         v
Vault KV v2 ---------> private staging directory
                             |
                             v
                   atomic managed symlink
                             |
                             v
                 Compose or direct processes
```

Run these commands from the repository root:

```sh
devctl --profile my-app validate
devctl --profile my-app plan
devctl --profile my-app up
devctl status
devctl logs --service my-app-compose
devctl --profile my-app smoke
devctl down
```

Use `devctl plan` to inspect the effective issuer, launch commands, working directories, and preparation steps before any process starts.

## 3. Configure the issuer and listener

For direct HTTP development, the listening address and issuer can be the same origin:

```sh
go run ./cmd/tinyidp serve-dev \
  --addr=127.0.0.1:5556 \
  --issuer=http://localhost:5556
```

For proxy-terminated HTTPS, the public issuer is the Caddy origin while tinyidp listens on an internal HTTP address:

```sh
go run ./cmd/tinyidp serve-production \
  --addr=:8081 \
  --listener-mode=trusted-proxy-http \
  --issuer=https://idp.localhost:8443 \
  --trusted-proxy-cidrs=172.16.0.0/12 \
  --clients-file=/config/clients.json \
  --db=/state/tinyidp.sqlite \
  --token-secret-file=/state/.secrets/token.key
```

`trusted-proxy-http` is appropriate only when an explicitly trusted proxy supplies the public request context. Restrict `--trusted-proxy-cidrs` to the actual proxy network in production. A broad private CIDR may be acceptable in an isolated local Docker environment, but it must not be copied into a production manifest without network review.

Verify discovery before configuring the client:

```sh
curl --cacert var/devctl/caddy-local-root.crt \
  https://idp.localhost:8443/.well-known/openid-configuration |
  jq '{issuer,authorization_endpoint,token_endpoint,jwks_uri}'
```

The returned `issuer` must exactly equal the configured issuer, including scheme, host, port, and path.

## 4. Register the client

Use a public client with Authorization Code plus PKCE for native applications, browser applications, and devices that cannot protect a client secret. Use a confidential client only when the token exchange occurs in a controlled backend.

Example public client:

```json
{
  "id": "my-public-app",
  "public": true,
  "redirect_uris": ["https://app.localhost:9443/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"]
}
```

Example confidential web client:

```json
{
  "id": "my-web-backend",
  "secret_file": "/state/.secrets/my-web-client-secret.txt",
  "redirect_uris": ["https://app.localhost:9443/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"]
}
```

Do not deliver a confidential client secret to React code. React initiates authorization and may hold a PKCE verifier, but a backend must perform any exchange that depends on a client secret.

## 5. Separate browser and backchannel addressing

The browser follows public HTTPS URLs. Containers and server processes may require an internal route to the same logical issuer. Preserve the issuer value while changing only transport resolution.

For example:

```text
Browser:
  https://idp.localhost:8443/authorize

Application backend:
  issuer metadata remains https://idp.localhost:8443
  DNS or proxy routing reaches the Caddy service

Tinyidp container:
  listens internally on http://idp:8081
  advertises only https://idp.localhost:8443
```

Do not rewrite discovery metadata to internal container names. OIDC validation uses the public issuer as a stable identifier.

## 6. Persist TLS and application secrets

The shared local Caddy authority is stored in the external Docker volume `tinyidp-local-caddy-pki`. The volume preserves the CA private key and certificate across demo restarts. Its encrypted backup is stored in Vault at:

```text
kv/tiny-idp/dev/_shared/caddy-local/pki-storage
```

Application material follows the same hierarchy:

```text
kv/tiny-idp/
├── dev/
│   ├── _shared/caddy-local/pki-storage
│   └── my-app/
│       ├── runtime
│       ├── bootstrap
│       └── integrations
└── production/
    └── <deployment>/
        ├── runtime
        ├── bootstrap
        └── integrations
```

The hierarchy is consistent across tiers, but values are never shared between development and production. Runtime values contain long-lived service keys. Bootstrap values contain one-time owner or initialization material. Integration values contain credentials issued to dependent systems.

Initialize and materialize secrets through devctl:

```sh
devctl --profile my-app secrets-init
devctl --profile my-app secrets-fetch
```

Materialization writes files with restrictive permissions into a private staging directory and atomically switches a managed symlink. This avoids partially updated secret sets. Compose mounts the files; it does not receive secret values through command arguments or committed environment files.

The SQLite database and its cryptographic keys form one recoverable unit. Restoring the database without the matching keys can make encrypted or signed state unusable. Back up and rotate them under one runbook.

## 7. Trust the local authority

Export the public root certificate after Caddy starts:

```sh
devctl --profile admin-console pki-export-root
```

Install only the root certificate in workstation or browser trust stores. Never install or distribute the CA private key. The private key remains in the Docker volume and its encrypted Vault backup.

Verify the certificate before changing trust:

```sh
openssl x509 \
  -in var/devctl/caddy-local-root.crt \
  -noout -subject -issuer -fingerprint -sha256
```

The CA backup operations are deliberately distinct:

```sh
devctl --profile admin-console pki-backup
devctl --profile admin-console pki-restore --staging-volume <new-volume>
```

A restore targets a new, empty staging volume. Compare the restored root fingerprint with the recorded fingerprint before switching any environment to that volume.

## 8. Implement the relying party

The relying party performs these protocol steps:

```text
1. Fetch discovery metadata.
2. Generate state, nonce, and PKCE verifier.
3. Store them in a server-side session or protected browser storage.
4. Redirect to authorization_endpoint with code_challenge.
5. Receive code and state at the exact registered redirect URI.
6. Reject an incorrect state.
7. Exchange the code at token_endpoint.
8. Validate the ID token signature through jwks_uri.
9. Validate issuer, audience, expiry, and nonce.
10. Establish the application's own authenticated session.
```

Pseudocode:

```text
startLogin(request):
    state    = random(32)
    nonce    = random(32)
    verifier = randomPKCEVerifier()
    savePendingLogin(state, nonce, verifier)
    redirect(authorizeURL(
        response_type = "code",
        client_id     = configuredClientID,
        redirect_uri  = configuredCallback,
        scope         = "openid profile email",
        state         = state,
        nonce         = nonce,
        code_challenge = S256(verifier)))

handleCallback(request):
    pending = consumePendingLogin(request.state)
    require(pending exists)
    tokens = exchange(request.code, pending.verifier)
    claims = verifyIDToken(
        tokens.id_token,
        expectedIssuer,
        expectedClientID,
        pending.nonce)
    createApplicationSession(claims.subject)
```

Treat `/userinfo`, introspection, administration APIs, and widget APIs as separate authorization surfaces. A browser session cookie for the administrator is not a bearer token for a relying party. A token issued to one audience must not be accepted by another API without an explicit policy.

## 9. Validate the complete environment

The minimum test matrix is:

| Test | Expected result |
|---|---|
| Discovery through public HTTPS | `200`, exact issuer |
| Authorization with unknown client | protocol error |
| Authorization with wrong redirect URI | rejected before redirect |
| Public code exchange without PKCE | rejected |
| Code exchange with correct verifier | tokens issued |
| ID-token signature and claims | valid issuer, audience, expiry, nonce |
| Reuse authorization code | rejected |
| Unauthenticated administration route | `401` or login redirect |
| Unauthenticated widget API | `401` |
| Restart with existing database and secrets | prior state remains readable |
| TLS validation with exported root | succeeds |
| TLS validation without root | fails |

Run the profile smoke command after the services are healthy:

```sh
devctl --profile my-app smoke
```

The smoke command should be noninteractive, avoid printing credentials, and fail on the first violated invariant.

## 10. Extend the pattern for device authorization

The PULP OS device flow uses the same environment contract but exercises device authorization instead of a browser callback in the device process:

```text
device -> device authorization endpoint
device <- device_code, user_code, verification_uri
user   -> verification_uri in a trusted browser
device -> token endpoint at the prescribed polling interval
device <- access token after approval
```

The device remains a public client. It must respect the server-provided polling interval and handle `authorization_pending`, `slow_down`, expiry, and denial. Its API access token still requires audience and scope enforcement at the resource server.

## Troubleshooting

| Symptom | Check |
|---|---|
| `issuer` mismatch | Compare client configuration with discovery byte-for-byte. |
| Redirect rejected | Confirm the complete registered URI, including port and path. |
| TLS unknown authority | Export and install the current shared Caddy root certificate. |
| Login works but token exchange fails | Check PKCE verifier persistence and confidential-client authentication. |
| Cookies disappear | Check HTTPS, host, `Secure`, `SameSite`, and proxy headers. |
| Service starts with new empty state | Confirm the expected SQLite volume and secret materialization target. |
| Devctl prepare rejects a secret target | Remove only an obsolete target after confirming it is a managed symlink; never overwrite an unrelated directory. |
| Docker network overlaps | Do not assign fixed local subnets unless the integration requires stable addresses. |
| Caddy restore rejected | Restore only into a new empty staging volume, then compare fingerprints. |

## See also

- `tinyidp help tutorial-first-rp-login`
- `tinyidp help tutorial-device-authorization`
- `tinyidp help tutorial-seeded-users-and-claims`
- `examples/tinyidp-admin-console/README.md`
- `dev/environments/admin-console.yaml`
- `devctl/operations.py`
