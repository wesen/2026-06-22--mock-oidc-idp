# Standalone tiny-idp + Message Desk

This development demo runs a standalone tiny-idp and Message Desk as distinct
containers. The browser uses `localhost:8081` as the canonical issuer and
`localhost:8080` as the relying party. Only discovery, JWKS, and token
requests use the private Docker address `idp:8081`; the public issuer remains
unchanged for authorization responses and ID-token validation.

Use the manifest-driven profile from the repository root:

```sh
devctl --profile external-message-desk plan
devctl --profile external-message-desk up
devctl status
devctl logs --service external-message-desk-compose --follow
```

Open `http://localhost:8080`. The committed seed is intentionally public and
development-only: it supplies `amelie` and `wesen` with the documented demo
password in `demo-seed.json`. Replace the file with an operator-controlled
secret mount before using any non-demo environment. Do not reuse a seeded
demo password.

Stop with `devctl down`. The two named volumes are independent durable
boundaries. Reset only this demo with the exact confirmation phrase:

```sh
devctl --profile external-message-desk state-reset -- \
  --confirm reset-external-message-desk-state
```

The provider owns accounts, its browser session, consent, chooser state, and
logout. Message Desk owns only messages and its own opaque application session.
`Log out of Message Desk` clears the latter; `Log out everywhere` also sends
the browser to the provider end-session endpoint.

## Deployment profiles and the HTTPS boundary

This Compose project is a **development profile**, not a production deployment
manifest. Its public origins are loopback HTTP URLs, its seeded credentials are
public test fixtures, and the standalone command is started in
`embeddedidp.DevMode`. Those choices make local OIDC redirects and browser
testing reproducible; they are not safe defaults for a networked issuer.

| Concern | Development Compose profile | Required production profile |
| --- | --- | --- |
| Browser issuer | `http://localhost:8081` | canonical public `https://` issuer |
| Message Desk public URL | `http://localhost:8080` | canonical public `https://` RP origin |
| Cookies | non-secure loopback cookies are allowed | `Secure`, explicit `SameSite`, and HTTPS-only browser delivery |
| Account source | committed, public demo seed | operator-managed provisioning; no credentials in an image or repository |
| Provider mode | `embeddedidp.DevMode` | `embeddedidp.ProductionMode` with every readiness prerequisite satisfied |
| State volumes | local named Docker volumes | backed-up persistent storage with ownership, retention, and recovery procedures |
| Network route | Compose service DNS for RP backchannel | explicitly configured internal route; public issuer identity remains unchanged |
| TLS/proxy | none | terminating proxy/load balancer with canonical-origin and forwarded-header policy |

Do not expose this Compose file on a LAN or Internet-facing host. In
particular, do not change `localhost` to a public hostname while retaining
HTTP, the demo seed, or the development provider mode.

### Production deployment contract

A production host should compose tiny-idp through the public embedding API or
a production-specific host command, not by copying this demo command line and
changing URLs. Before accepting traffic, it must establish all of the following
contracts.

1. The provider's configured issuer is the exact externally visible HTTPS URL.
   Discovery metadata, `iss` claims, authorization redirects, and registered
   redirect URIs use that URL. A container DNS name is a transport destination,
   never the issuer identity.
2. The provider is constructed in `embeddedidp.ProductionMode`. That mode
   requires a persistent supported store, a durable audit reporter, a
   production-ready rate limiter and client-address resolver, a production
   password authenticator, an active usable RSA signing key, and a token secret
   of at least 32 bytes. Its startup/readiness failures must block deployment.
3. Provider and RP session cookies use `Secure`; their `SameSite` policy is
   explicit and compatible with the top-level authorization redirect. Cookie
   names and paths are part of the deployment contract and must not collide
   with a reverse proxy or another application on the same site.
4. A TLS terminator has one documented owner for forwarded-origin headers.
   The Go host must either receive the canonical HTTPS request directly or
   trust only a configured proxy boundary. It must not derive an issuer from an
   arbitrary `Host` or forwarded header supplied by a browser.
5. Signing keys, token secrets, audit destinations, account-provisioning
   credentials, and client registration material are supplied through an
   operator-controlled secret mechanism. They must not appear in image layers,
   Compose arguments, logs, browser JavaScript, or committed seed files.
6. Backups and restore procedures preserve the provider database and signing
   keys coherently. Restoring one without the other can invalidate issued
   tokens or make sessions impossible to verify.

The external relying-party configuration already enforces the central URL and
cookie relationship: an HTTPS public base URL requires a secure RP cookie, and
an optional private backchannel URL may route discovery/token/JWKS traffic
without changing the public issuer used for OIDC validation.

### Reverse-proxy topology

The normal production topology keeps the browser-visible origins outside the
container network and uses a private route only for RP backchannel calls:

```text
browser -- HTTPS --> public reverse proxy -- private HTTP/TLS --> tiny-idp host
browser -- HTTPS --> public reverse proxy -- private HTTP/TLS --> Message Desk host
Message Desk host -- private route --> tiny-idp discovery, JWKS, and token endpoints
```

The browser must still navigate to the canonical public issuer during
authorization and end-session. A proxy route changes reachability, not the
OIDC issuer. Validate this with a real browser after proxy configuration,
because cookies, CSP `form-action`, redirect chains, and forwarded-origin
handling are browser-visible behavior.

## Automated development assurance

The ticket keeps its runnable assurance tools under
`ttmp/2026/07/14/TINYIDP-EXTERNAL-DEMO-001--standalone-tiny-idp-and-message-desk-docker-oidc-demo/scripts/`.
Run them from the repository root in this order:

```sh
# Build/start the local development topology and confirm both readiness endpoints.
ttmp/2026/07/14/TINYIDP-EXTERNAL-DEMO-001--standalone-tiny-idp-and-message-desk-docker-oidc-demo/scripts/01-compose-health-smoke.sh

# Install the pinned ticket-local Playwright runner if needed, then exercise
# actual browser login, consent, scopes, CSRF rejection, message creation,
# chooser/account switch, local logout, and global logout.
ttmp/2026/07/14/TINYIDP-EXTERNAL-DEMO-001--standalone-tiny-idp-and-message-desk-docker-oidc-demo/scripts/02-run-playwright-browser-smoke.sh

# Check unprivileged PID 1, development-fixture exposure in rendered config/logs,
# and persistent key/app/message state across controlled service restarts.
ttmp/2026/07/14/TINYIDP-EXTERNAL-DEMO-001--standalone-tiny-idp-and-message-desk-docker-oidc-demo/scripts/03-compose-durability-and-secret-check.sh
```

The browser suite uses only the committed public development fixture. Its
generated `node_modules`, trace, video, screenshot, and HTML-report artifacts
are ignored by the ticket-local `.gitignore`; do not commit test output or use
the scripts with production credentials. The durability check intentionally
restarts both services and is therefore a development-environment operation.
