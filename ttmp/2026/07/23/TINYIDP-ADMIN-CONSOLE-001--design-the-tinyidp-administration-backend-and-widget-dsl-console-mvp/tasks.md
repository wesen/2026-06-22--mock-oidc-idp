# Tasks

## Research and design

- [x] Create a dedicated administration-console ticket.
- [x] Import and read the complete TinyIDP UX specification.
- [x] Inspect TinyIDP admin, store, audit, production-host, and xgoja boundaries.
- [x] Inspect the rag-evaluation Widget DSL and Upwork backend example.
- [x] Write the evidence-backed architecture and intern implementation guide.
- [ ] Review and accept the proposed decisions and open owner questions.

## Phase A — Control-plane foundation

- [x] Add `pkg/idpadmin` principals, scope, capabilities, authorizer, queries, commands, and action handles.
- [x] Add `pkg/idpadminstore` contracts and SQLite migrations/implementation.
- [x] Add owner grant bootstrap/status/revoke CLI commands.
- [x] Add resource versions, action nonces, idempotency, actions, and audit outbox.

## Phase B — Authenticated read-only console

- [x] Add dedicated public PKCE OIDC login and admin sessions.
- [x] Mount `/admin`, `/api/admin`, `/api/widget`, and `/static/admin` on the public listener.
- [x] Add safe `tinyidp.admin` and `widget.dsl` xgoja modules.
- [x] Add pnpm/React/Redux/RTK Query/Bootstrap frontend and read-only pages.

## Phase C — User operations

- [x] Migrate CLI user operations to `pkg/idpadmin`.
- [x] Add create/edit/enable/disable/unlock/set-password/revoke-access flows.
- [x] Add fresh-auth, typed confirmation, expected-version, replay, and stale-state tests.

## Phase D — Invitations and applications

- [x] Add invitation metadata projection and one-time issuance/revocation.
- [x] Add client create/edit/enable/disable/secret-rotation flows.
- [x] Add one-time secret leakage and replay tests.

## Phase E — Keys and operations

- [x] Add safe key rotation/retirement; keep purge CLI-only.
- [x] Add doctor, managed backup create/verify, and sanitized diagnostics.
- [x] Add outbox/operation workers and health reporting.

## Phase F — Hardening and release

- [x] Add accessibility, keyboard, responsive, failure-state, security, and visual tests.
- [x] Add Widget IR deterministic validators and 10,000-user query benchmarks.
- [x] Pass generation, formatting, tests, builds, lint, frontend checks, and browser tests.

## Documentation delivery

- [x] Pass `docmgr doctor` with no warnings.
- [x] Dry-run the reMarkable bundle.
- [x] Upload and verify the reMarkable bundle.

## Next session: Vault-backed local environment

- [ ] Implement Vault-backed secret materialization for the local admin-console stack <!-- t:j56c -->
- [ ] Wire the admin-console Compose stack to the persistent tinyidp-local-caddy-pki volume <!-- t:mosk -->
- [ ] Add idempotent bootstrap, CA export, and certificate-verifying smoke scripts <!-- t:3xkd -->
- [ ] Document and test secret rotation and failure behavior <!-- t:8z7g -->
- [ ] Implement manifest-driven devctl profiles for every runnable TinyIDP example and demo <!-- t:fkur -->
- [ ] Implement Vault KV v2 tier/deployment layout and atomic secret materialization <!-- t:1wda -->
- [ ] Implement guarded Caddy storage backup and staging restore through Vault with CAS and fingerprint verification <!-- t:umal -->
- [ ] Refactor shared-two-apps, admin-console, and Jitsi onto reusable HTTPS and Caddy PKI patterns <!-- t:c0f5 -->
- [ ] Migrate embedded, external Message Desk, message-app, script, config, and user demos into devctl profiles <!-- t:bbyx -->
- [ ] Add the Glazed tutorial for building and validating local TinyIDP applications <!-- t:ixlg -->
- [ ] Run a non-destructive CA recovery drill and the complete per-profile acceptance matrix <!-- t:uy3a -->
