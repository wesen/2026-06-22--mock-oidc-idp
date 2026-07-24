---
Title: TinyIDP administration backend MVP architecture and implementation guide
Ticket: TINYIDP-ADMIN-CONSOLE-001
Status: review
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
    - Path: abs:///home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/app/App.tsx
      Note: Default React page/action transport that motivates TinyIDP RTK Query adapter
    - Path: abs:///home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/widgetdsl/v3.go
      Note: Typed widget.dsl v3 page and shell API
    - Path: abs:///home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/xgoja/providers/widgetsite/provider.go
      Note: widget.dsl xgoja provider registration
    - Path: abs:///home/manuel/code/wesen/go-go-golems/upwork/verbs/lib/pages.js
      Note: Reference typed Widget DSL page composition
    - Path: abs:///home/manuel/code/wesen/go-go-golems/upwork/verbs/upwork.js
      Note: Reference Widget page and action HTTP host
    - Path: abs:///home/manuel/code/wesen/go-go-golems/upwork/web/src/main.tsx
      Note: Reference React renderer consumer
    - Path: abs:///home/manuel/code/wesen/go-go-golems/upwork/xgoja.yaml
      Note: Reference widget.dsl provider and embedded asset wiring
    - Path: repo://cmd/tinyidp-xapp/production_app.go
      Note: Existing same-process OIDC and separate application-session reference
    - Path: repo://internal/admin/clients.go
      Note: Existing client and one-time generated-secret operations
    - Path: repo://internal/admin/keys.go
      Note: Existing key lifecycle and redaction primitives
    - Path: repo://internal/admin/service.go
      Note: Current admin service dependency and post-commit audit boundary
    - Path: repo://internal/cmds/serve_production.go
      Note: Production store, listeners, handler composition, and shutdown lifecycle
    - Path: repo://pkg/idp/audit.go
      Note: Committed-mutation audit semantics and durable JSONL sink
    - Path: repo://pkg/idpstore/interfaces.go
      Note: Protocol store contracts and named atomic security invariants
    - Path: repo://pkg/idpstore/types.go
      Note: User, client, credential, invitation, and security data boundaries
    - Path: repo://pkg/sqlitestore/store.go
      Note: Concrete single-connection SQLite transaction owner
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/sources/local/tiny-idp-ux.md
      Note: Imported complete product and screen specification
ExternalSources: []
Summary: Evidence-backed design and implementation guide for a system-scoped TinyIDP administration control plane and a widget.dsl v3 React console.
LastUpdated: 2026-07-23T20:14:57.721802857-04:00
WhatFor: Implement the first production administration console without making the browser, Widget IR, or xgoja JavaScript an authorization boundary.
WhenToUse: Read before changing TinyIDP administration services, SQLite schema, production HTTP composition, the widget.dsl host, or the admin React application.
---


# TinyIDP administration backend MVP architecture and implementation guide

## 1. Executive summary

TinyIDP needs a durable administration control plane, not an HTTP wrapper around its existing Cobra commands. The first release is a single-installation, single-owner console named **TinyIDP Console**. It manages users, invitations, OIDC clients, signing keys, administrative activity, health checks, managed backups, and sanitized diagnostics. The first release has one effective scope:

```text
AdminScope{Kind: "system", ID: "system"}
```

The service contracts nevertheless carry an explicit scope on every query, command, action handle, session, and activity record. That is the central architectural decision from the supplied UX brief: the MVP is system-scoped, not tenant-unaware. Future identity domains can therefore add domain records and scoped grants without replacing routes, screens, command names, or sessions.

The implementation should reuse the RAG evaluation system's `widget.dsl` v3 authoring module and React Widget IR renderer. The Upwork tracker demonstrates the mechanics: xgoja loads `widget.dsl`, JavaScript builds typed page IR, `/api/widget/pages/{id}` returns JSON, and `@go-go-golems/rag-evaluation-site` renders it. TinyIDP must use a stricter security boundary than that local application:

```text
Widget JavaScript = reviewed presentation composition
React renderer    = untrusted client presentation
Go services       = identity, scope, authorization, validation, transactions
SQLite            = grants, sessions, versions, actions, nonce and outbox truth
```

The browser never submits an authoritative command name, target, capability, scope, or resource version. It submits an opaque, short-lived action handle plus user-entered fields. Go verifies the handle, current admin session, current grant version, CSRF token, required assurance, expected resource version, and idempotency key before changing state.

The MVP should be implemented as an umbrella ticket with six reviewable phases:

1. Control-plane storage, principals, grants, sessions, authorization, and action handles.
2. Dedicated OIDC login and a read-only Widget DSL console.
3. User commands and concurrency-safe destructive flows.
4. Invitation and OIDC client administration with one-time secret delivery.
5. Signing-key and managed operational actions.
6. security, accessibility, tablet, failure-state, and evaluation hardening.

This ticket deliberately does not add a backwards-compatibility adapter. Existing CLI entry points should be migrated to the new `pkg/idpadmin` application service as their operations move. HTTP handlers must never execute Cobra commands or parse CLI JSON.

## 2. What an intern needs to understand first

### 2.1 TinyIDP has protocol state and control-plane state

TinyIDP is an OIDC/OAuth provider. Its protocol state includes users, password credentials, browser sessions, grants, authorization codes, tokens, clients, and signing keys. Those records are consumed by authentication and token endpoints. The administration control plane is different: it decides which administrator may inspect or mutate those records, records the reason and outcome, protects mutations against replay and stale data, and presents safe data to a browser.

Do not put browser-specific pagination or Widget IR into `pkg/idpstore`. Its existing `UserStore` deliberately provides point lookup and persistence, while `AtomicStore` exposes named security invariants such as disabling a user and revoking all of that user's security artifacts. See:

- `pkg/idpstore/interfaces.go:34-42` for point-oriented user storage.
- `pkg/idpstore/interfaces.go:217-238` for transactions and named atomic security operations.
- `pkg/idpstore/types.go:46-81` for the separation between user profile data and password credentials.
- `pkg/sqlitestore/store.go:28-45` for the concrete durable single-node SQLite store.

The new control plane adds application-level contracts in `pkg/idpadmin` and admin-specific persistence contracts in `pkg/idpadminstore`. The SQLite implementation remains in `pkg/sqlitestore` so a single SQLite transaction can update protocol state, an admin action record, resource version state, and an audit outbox row.

### 2.2 Existing `internal/admin` is a low-level operator service

The present admin service has three dependencies: an `idpstore.Store`, a clock, and an audit sink (`internal/admin/service.go:14-38`). It has no authenticated actor, grant, scope, capability, request ID, idempotency contract, expected version, or fresh-auth requirement.

Its methods demonstrate useful domain behavior:

- `SetUserDisabled` delegates to the store's atomic revocation-aware operation and emits an audit event (`internal/admin/users.go:12-26`).
- client creation validates and hashes secrets, while secret rotation immediately replaces the old hash (`internal/admin/clients.go:40-86`, `internal/admin/clients.go:124-149`);
- key methods generate, rotate, list, retire, purge, and redact private key material (`internal/admin/keys.go:18-95`);
- doctor reports schema, client, active-key, and verification-key status (`internal/admin/doctor.go:23-77`);
- backup functions currently accept caller-supplied filesystem paths (`internal/admin/backup.go:13-30`).

These are not web authorization APIs. Re-home their useful domain logic behind `pkg/idpadmin` and update the CLI to call the same application service. Do not create a permanent “web-to-internal-admin” compatibility wrapper.

### 2.3 Mutations can commit before audit delivery fails

`idp.ErrAuditDelivery` explicitly means the state mutation committed but the audit event was not delivered (`pkg/idp/audit.go:14-17`). The file sink synchronously appends JSON and calls `fsync` (`pkg/idp/audit.go:92-161`). Existing admin methods mutate first and call the sink afterward (`internal/admin/service.go:41-45`).

That contract is acceptable for an operator CLI that tells the operator to reconcile. It is ambiguous for a browser that might retry. The web control plane therefore needs:

- a transactional `admin_actions` record;
- a transactional `admin_audit_outbox` record;
- durable idempotency;
- a response that distinguishes “completed, external audit pending” from “not started.”

The outbox does not replace the durable JSONL audit sink. It makes delivery retryable and makes the browser-visible outcome unambiguous.

### 2.4 The production server already has two listeners

`serve-production` requires a public listener and a distinct `--admin-addr` (`internal/cmds/serve_production.go:112-128`). The public handler mounts theme assets, plugin routes, readiness, and the OIDC provider (`internal/cmds/serve_production.go:425-447`). The second listener serves internal observability (`internal/cmds/serve_production.go:358-399`).

The new console belongs on the public origin:

```text
https://id.example/admin/...
https://id.example/api/admin/...
https://id.example/api/widget/...
https://id.example/static/admin/...
```

The internal `--admin-addr` remains readiness/metrics only. Do not expose the browser console on that listener and do not silently change the meaning of the existing flag.

### 2.5 `User.Tenant` is an OIDC claim, not an authorization scope

`idpstore.User` places `Tenant` beside groups, roles, locale, and other claims (`pkg/idpstore/types.go:50-65`). It is application-facing identity data. It must not decide which users a domain administrator can manage.

Future multitenancy must use a separate domain membership table and domain-scoped grants. The MVP persists `ScopeKind` and `ScopeID` now, but permits only `system/system`.

## 3. Requirements and scope

### 3.1 MVP outcome

The installation owner can perform ordinary daily administration without routinely opening the CLI. The CLI remains the bootstrap, recovery, migration, restore, and emergency-key-purge surface.

The MVP navigation is:

```text
Overview
Users
Invitations
Applications
Signing keys
Activity
Operations
```

The canonical route set is:

```text
/admin
/admin/users
/admin/users/{user-id}
/admin/users/new
/admin/invitations
/admin/applications
/admin/applications/{client-id}
/admin/applications/new
/admin/keys
/admin/activity
/admin/operations
```

The complete screen-level UX, ASCII wireframes, tablet behavior, and future domain screens are preserved unchanged in `sources/local/tiny-idp-ux.md`.

### 3.2 MVP capabilities

The owner grant contains explicit capabilities even though one owner receives all of them:

```text
overview.read
users.read
users.create
users.update
users.disable
users.password.set
users.unlock
users.access.revoke
invitations.read
invitations.create
invitations.revoke
clients.read
clients.create
clients.update
clients.disable
clients.secret.rotate
keys.read
keys.generate
keys.rotate
keys.retire
activity.read
operations.read
operations.doctor
operations.backup.create
operations.backup.verify
operations.diagnostics
```

Widget visibility checks improve presentation but never authorize. Every server query and command independently resolves the current session, reloads the current grant, checks the server-resolved scope, and checks the required capability.

### 3.3 Explicit non-goals

The following are not part of the first release:

- customer identity domains or a fake default tenant in the user interface;
- delegated administrators, role editing, or browser owner bootstrap;
- bulk destructive operations;
- reliable per-user session inventory;
- refresh tokens for the admin console;
- MFA enrollment or an AAL2 administration policy;
- backup restore, schema migration, issuer changes, token-secret changes, audit-path changes, or emergency key purge in the browser;
- arbitrary SQL, URLs, HTTP calls, JavaScript, HTML, filesystem paths, or authorization expressions in Widget DSL artifacts;
- RAG-generated UI at runtime.

### 3.4 Security invariants

The implementation is not complete unless all of these remain true:

1. Admin authentication uses a dedicated OIDC client and a separate admin cookie.
2. The first owner can only be granted through an authenticated CLI/bootstrap operation.
3. Scope is resolved on the server; an incoming domain or scope ID is never trusted.
4. The current grant is loaded on every request. Revocation takes effect without waiting for the admin session to expire.
5. A mutation is authorized again at execution, not only when its button was rendered.
6. Destructive actions require a reason and a one-use action nonce.
7. Sensitive actions require an authentication time no older than five minutes.
8. Mutable resources use compare-and-swap versions.
9. Passwords, invitation codes, generated client secrets, hashes, key bytes, cookie values, CSRF tokens, and action-signing keys never enter logs, audit fields, page IR, Redux devtools, or persisted browser state.
10. Static assets are local and served under `/static/admin/`.

## 4. Current-state capability and gap analysis

| Area | Existing evidence | Gap for the console |
|---|---|---|
| User lookup | `UserStore` has ID, login, and subject point lookups (`pkg/idpstore/interfaces.go:34-42`) | Bounded search, filters, stable sorting, cursor pagination, detail aggregate |
| User disable | Atomic operation revokes browser, domain-token, and Fosite state (`pkg/idpstore/interfaces.go:225-230`) | Actor/scope/capability/version/reason/idempotency wrapper |
| Password change | Account service hashes and replaces credential/security state; its documentation warns about post-commit audit errors (`docs/embedding-foundations.md:148-159`) | Fresh auth, reason, action handle, web-safe error/result contract |
| Unlock/revoke | Atomic reset and revoke primitives exist (`pkg/idpstore/interfaces.go:223-231`) | Named admin commands and activity records |
| Clients | Existing service creates, lists, reads, disables, and rotates (`internal/admin/clients.go:40-149`) | Safe update command, CAS version, actor/scope, one-time secret response |
| Keys | Generate/rotate/list/retire/purge exist (`internal/admin/keys.go:18-73`) | Fresh auth, versions, keep purge CLI-only |
| Invitations | Store uses code hash and lifecycle methods (`pkg/idpstore/interfaces.go:136-143`); record has a public ID but table is keyed by hash (`pkg/idpstore/types.go:83-96`, migration 012) | Queryable metadata, revoke by public ID, issuance actor/time, one-time reveal |
| Doctor | Schema/client/key checks exist (`internal/admin/doctor.go:23-77`) | Safe web projection and recent operation/activity |
| Backups | SQLite backup and verification exist (`internal/admin/backup.go`) | Configured root, generated filename, operation state, no arbitrary paths |
| Audit | Durable synchronous append exists (`pkg/idp/audit.go:92-180`) | Queryable actions and retryable outbox |
| Admin web | Production public mux has no `/admin` mount (`internal/cmds/serve_production.go:425-447`) | OIDC RP, sessions, API, assets, Widget runtime |
| Widget renderer | Not yet a TinyIDP dependency | Pin Go provider and npm renderer versions; add xgoja and React build |

## 5. Proposed architecture

### 5.1 Component diagram

```text
Browser
  |
  | GET /admin/...            GET /api/widget/pages/{id}
  | POST /api/widget/actions/execute
  v
+------------------------- public TinyIDP listener -------------------------+
| internal/adminweb                                                        |
|  - OIDC RP start/callback/logout/fresh-auth                              |
|  - admin session + CSRF middleware                                       |
|  - request IDs, CSP, cache and body limits                               |
|  - Widget page/action transport                                          |
|                                                                          |
|  React + Redux + RTK Query      xgoja widget.dsl v3                      |
|  renders reviewed Widget IR <--- page composition only                    |
|                                      |                                   |
|                                      v                                   |
|                              safe tinyidp.admin module                    |
|                                      |                                   |
|                                      v                                   |
|                              pkg/idpadmin.Service                         |
|                        authorize -> validate -> transact                  |
+-------------------------------------|------------------------------------+
                                      |
                                      v
                           pkg/sqlitestore.Store
               protocol state + admin control-plane tables
                                      |
                       +--------------+--------------+
                       |                             |
                       v                             v
              admin_audit_outbox            admin_operations
                       |                             |
                       v                             v
              durable JSONL sink             managed backup root
```

### 5.2 Request sequence for a destructive command

```text
Browser          adminweb        idpadmin        SQLite          audit worker
   | GET page       |               |               |                  |
   |--------------->| principal     |               |                  |
   |                 |-------------->| authorize     |                  |
   |                 |               |-------------->| read query       |
   |                 |               |<--------------| rows + versions  |
   |                 |               | mint handle   | nonce hash       |
   |<----------------| Widget IR + opaque action handle                 |
   |                 |               |               |                  |
   | POST execute    | CSRF/session  |               |                  |
   |--------------->|-------------->| verify handle |                  |
   |                 |               | BEGIN         |                  |
   |                 |               |-------------->| reload grant     |
   |                 |               |-------------->| consume nonce    |
   |                 |               |-------------->| compare version  |
   |                 |               |-------------->| mutate resource  |
   |                 |               |-------------->| insert action     |
   |                 |               |-------------->| insert outbox     |
   |                 |               |-------------->| COMMIT            |
   |<----------------| completed + new version       |                  |
   |                 |               |               |<-----------------|
   |                 |               |               | deliver + mark   |
```

### 5.3 Package and file plan

```text
pkg/idpadmin/
  principal.go          # AdminPrincipal, assurance and session facts
  scope.go              # AdminScope and MVP validation
  capability.go         # closed capability constants and registry
  authorizer.go         # grant/capability/scope decisions
  queries.go            # QueryService and safe DTOs
  commands.go           # CommandService and request DTOs
  actionhandle.go       # signed opaque handle claims and verification
  errors.go             # stable domain error codes

pkg/idpadminapp/
  owner.go              # owner bootstrap, status, and recovery orchestration
  executor.go           # atomic authorized mutation execution

pkg/idpadminstore/
  interfaces.go         # grants, sessions, query, command, action, outbox contracts
  types.go              # persistence records, cursor and operation types

pkg/sqlitestore/
  admin_store.go        # grants, sessions, action security, evidence, outbox
  admin_projection.go   # canonical user projection rebuild and comparison
  migrations/016_admin_control_plane.sql
  migrations/017_admin_user_projection.sql

internal/adminweb/
  handler.go            # public route composition
  auth.go               # OIDC start/callback/fresh/logout
  middleware.go         # principal, request ID, CSRF, headers, body bounds
  widget.go             # page/action HTTP adapters
  errors.go             # HTTP problem mapping
  assets.go             # go:embed admin frontend
  xgoja_provider.go     # safe bridge to pkg/idpadmin
  xgoja.yaml            # widget.dsl runtime declaration
  verbs/
    site.js             # route registration and page selection
    pages.js            # typed widget.dsl page builders
  frontend/
    package.json
    pnpm-lock.yaml
    src/main.tsx
    src/store.ts
    src/adminApi.ts
    src/AdminConsoleApp.tsx
    src/authSlice.ts
    vite.config.ts

internal/cmds/
  admin_console.go      # bootstrap/status/revoke-session CLI commands
  serve_production.go   # mount admin handler and close its workers
```

`pkg/idpadminapp` is a deliberate application layer. `pkg/idpadminstore`
depends on `pkg/idpadmin` for domain values, so store-dependent orchestration
cannot live in `pkg/idpadmin` without an import cycle. The dependency direction
is:

```text
idpadmin domain <- idpadminstore contracts <- idpadminapp orchestration
                                      ^
                                      |
                              sqlitestore implementation
```

Product-specific UI remains under `internal/`. The shareable Widget DSL stays in `rag-evaluation-system`; do not fork renderer components into TinyIDP. The application and persistence interfaces are public packages because the CLI and embedded hosts need a supported common control-plane boundary.

### 5.4 Why the SQLite implementation stays in `pkg/sqlitestore`

The UX brief suggested `pkg/idpadminstore/sqlite`. That split creates an awkward transaction boundary: the admin SQLite adapter must mutate `idpstore` records and admin tables in one transaction, but `sqlitestore.Store.Update` intentionally hides its concrete `*sql.Tx`.

Keep the interfaces separate and implement both on the same concrete store:

```go
var _ idpstore.Store = (*sqlitestore.Store)(nil)
var _ idpadminstore.Store = (*sqlitestore.Store)(nil)
```

This is not permission for admin queries to leak into `idpstore.Store`. It means one concrete persistence package can uphold both contracts and one transaction.

## 6. Domain contracts

### 6.1 Principal, scope, capabilities, and grants

```go
package idpadmin

type ScopeKind string

const (
    ScopeSystem ScopeKind = "system"
    ScopeDomain ScopeKind = "domain" // reserved; rejected in MVP mode
)

type AdminScope struct {
    Kind ScopeKind
    ID   string
}

type Assurance string

const (
    AssuranceAuthenticated Assurance = "authenticated"
    AssuranceFreshAuth     Assurance = "fresh_auth"
)

type AdminPrincipal struct {
    Subject      string
    SessionID    string
    AuthTime     time.Time
    Assurance    Assurance
    GrantID      string
    GrantVersion uint64
}

type Capability string
```

Validation rules:

- system scope must have the exact ID `system`;
- domain scope is a known enum value but returns `ErrScopeUnsupported` until multitenancy is enabled;
- subject and session ID are derived from the admin session, never request JSON;
- grant version comes from the current database row, not an ID-token role claim;
- capabilities are constants registered at startup; unknown capabilities fail startup and grant writes.

The server stores grants, not roles from an ID token. OIDC proves the user subject; the current grant decides administration authority.

### 6.2 Query API

```go
type QueryService interface {
    GetOverview(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
    ) (Overview, error)

    ListUsers(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
        filter UserFilter,
        page CursorPageRequest,
    ) (CursorPage[UserRow], error)

    GetUser(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
        userID string,
    ) (UserDetail, error)

    ListInvitations(ctx context.Context, principal AdminPrincipal, scope AdminScope, filter InvitationFilter, page CursorPageRequest) (CursorPage[InvitationRow], error)
    ListClients(ctx context.Context, principal AdminPrincipal, scope AdminScope, filter ClientFilter, page CursorPageRequest) (CursorPage[ClientRow], error)
    GetClient(ctx context.Context, principal AdminPrincipal, scope AdminScope, clientID string) (ClientDetail, error)
    ListSigningKeys(ctx context.Context, principal AdminPrincipal, scope AdminScope) ([]SigningKeyRow, error)
    ListActivity(ctx context.Context, principal AdminPrincipal, scope AdminScope, filter ActivityFilter, page CursorPageRequest) (CursorPage[ActivityRow], error)
    GetOperations(ctx context.Context, principal AdminPrincipal, scope AdminScope) (OperationsView, error)
}
```

Every DTO is a safe projection. For example, `ClientDetail` contains `SecretConfigured` and `LastSecretRotation`, never `SecretHash`. `SigningKeyRow` never contains `PrivateKeyPEM`. `UserDetail` never contains `PasswordHash`.

Cursor rules:

- the cursor is an authenticated opaque encoding of the sort key and stable ID;
- allowed sort fields are a closed server-side map;
- a page size defaults to 25 and is capped at 100;
- search text is bounded and normalized;
- filters are typed; no query language or raw SQL fragment is accepted;
- each sort uses an ID tie-breaker so rows do not repeat or disappear.

Example user cursor query:

```sql
SELECT user_id, login, display_name, email, disabled, locked_until,
       last_successful_login_at, created_at, updated_at, version
FROM admin_user_projection
WHERE scope_kind = 'system'
  AND scope_id = 'system'
  AND (:status = '' OR status = :status)
  AND (:query = '' OR search_text LIKE :query_prefix)
  AND (updated_at, user_id) < (:cursor_updated_at, :cursor_user_id)
ORDER BY updated_at DESC, user_id DESC
LIMIT :page_size_plus_one;
```

All SQL values are parameters. Sort expressions come from a Go whitelist, not input strings.

### 6.3 Command API

```go
type CommandContext struct {
    Principal       AdminPrincipal
    Scope           AdminScope
    RequestID       string
    IdempotencyKey  string
    ExpectedVersion uint64
    Reason          string
}

type CommandService interface {
    CreateUser(context.Context, CommandContext, CreateUserRequest) (UserResult, error)
    UpdateUser(context.Context, CommandContext, UpdateUserRequest) (UserResult, error)
    SetUserDisabled(context.Context, CommandContext, string, bool) (UserResult, error)
    SetUserPassword(context.Context, CommandContext, SetPasswordRequest) (UserResult, error)
    UnlockUser(context.Context, CommandContext, string) (UserResult, error)
    RevokeUserAccess(context.Context, CommandContext, string) (UserResult, error)

    IssueInvitation(context.Context, CommandContext, IssueInvitationRequest) (OneTimeSecretResult, error)
    RevokeInvitation(context.Context, CommandContext, string) (InvitationResult, error)

    CreateClient(context.Context, CommandContext, CreateClientRequest) (ClientResult, error)
    UpdateClient(context.Context, CommandContext, UpdateClientRequest) (ClientResult, error)
    SetClientDisabled(context.Context, CommandContext, string, bool) (ClientResult, error)
    RotateClientSecret(context.Context, CommandContext, string) (OneTimeSecretResult, error)

    RotateSigningKey(context.Context, CommandContext, RotateKeyRequest) (KeyResult, error)
    RetireSigningKey(context.Context, CommandContext, string) (KeyResult, error)

    RunDoctor(context.Context, CommandContext) (OperationResult, error)
    CreateManagedBackup(context.Context, CommandContext, CreateBackupRequest) (OperationResult, error)
    VerifyManagedBackup(context.Context, CommandContext, string) (OperationResult, error)
}
```

Interface implementations must declare compile-time assertions:

```go
var _ QueryService = (*Service)(nil)
var _ CommandService = (*Service)(nil)
```

Use `context.Context` for every I/O or cryptographic operation. Wrap errors with `github.com/pkg/errors`. Any delivery or operation workers are owned by an `errgroup`, have explicit cancellation, and are joined during graceful shutdown.

### 6.4 Authorizer pseudocode

```text
authorize(principal, scope, capability, assurance):
    validate principal fields
    validate scope shape
    if MVP and scope != system/system:
        deny scope_unsupported

    grant = store.get_active_grant(principal.grant_id)
    if grant.actor_subject != principal.subject:
        deny grant_subject_mismatch
    if grant.version != principal.grant_version:
        deny grant_changed
    if grant revoked or expired:
        deny grant_inactive
    if grant.scope != scope:
        deny scope_mismatch
    if capability not in grant.capabilities:
        deny capability_missing

    if assurance == fresh_auth:
        if now - principal.auth_time > 5 minutes:
            deny fresh_auth_required

    allow
```

The command transaction reloads the grant and repeats version, active-state, scope, and capability checks. This closes the race between page rendering and button submission.

## 7. Authentication and session design

### 7.1 Dedicated OIDC client

Bootstrap a public OIDC client:

```yaml
id: tinyidp-admin-console
public: true
allowed_grant_types: [authorization_code]
require_pkce: true
allowed_scopes: [openid, profile, email]
redirect_uris:
  - https://id.example/admin/auth/callback
refresh_tokens: disabled
```

A public PKCE client avoids storing a second long-lived client secret inside the TinyIDP process. The server still performs the callback and token exchange. The exact redirect URI is derived from the configured issuer origin and fixed admin callback path; it is not accepted from a browser parameter.

### 7.2 Bootstrap CLI

Add a Glazed-backed command:

```text
tinyidp admin console bootstrap \
  --db /var/lib/tinyidp/idp.db \
  --owner-login owner \
  --public-base-url https://id.example
```

Conceptual steps:

```text
1. Open the existing SQLite store and durable audit sink.
2. Resolve the existing user by normalized login.
3. Create or validate the fixed public PKCE admin client.
4. Create exactly one active system/system owner grant.
5. Record a bootstrap admin action and external audit event.
6. Print grant ID, owner subject, client ID, and redirect URI; print no secret.
```

Also add read-only `status`, `revoke-grant`, and `revoke-session` commands. There is no public “claim ownership” route.

### 7.3 Authorization-code callback

The admin web package should use the existing `coreos/go-oidc` and OAuth client libraries rather than invent protocol parsing. Authentication attempts are server-side records:

```text
admin_auth_attempts
  state_hash
  nonce_hash
  pkce_verifier_box
  return_path
  existing_session_id_hash nullable
  created_at
  expires_at
  consumed_at
```

`pkce_verifier_box` is authenticated encryption under a key read from an owner-only file configured through the production Glazed section. Do not read this key from an environment variable.

Callback pseudocode:

```text
callback(request):
    hash incoming state
    atomically consume unexpired auth attempt
    exchange code with PKCE verifier through same-origin/in-process OIDC transport
    verify ID token signature, issuer, audience, expiry and nonce
    require sub and auth_time
    load active system owner grant for sub

    if this is fresh-auth for an existing session:
        require returned sub == existing session subject
        update session auth_time and assurance
        redirect to validated stored return path
    else:
        generate random session handle
        store keyed hash only
        bind session to grant ID and current grant version
        set admin cookie
        redirect to /admin
```

Only relative `/admin...` return paths are allowed. Reject scheme-relative, absolute, encoded traversal, and non-admin paths.

### 7.4 Session and cookie properties

```yaml
cookie:
  name: tinyidp_admin_session
  path: /admin
  secure: true
  http_only: true
  same_site: Lax

session:
  idle_timeout: 30m
  absolute_timeout: 8h
  handle_storage: keyed_hash
  refresh_tokens: false
```

Persist:

```text
admin_sessions
  id_hash BLOB PRIMARY KEY
  actor_subject TEXT NOT NULL
  grant_id TEXT NOT NULL
  grant_version INTEGER NOT NULL
  auth_time_ns INTEGER NOT NULL
  assurance TEXT NOT NULL
  csrf_secret_box BLOB NOT NULL
  created_at_ns INTEGER NOT NULL
  last_seen_at_ns INTEGER NOT NULL
  expires_at_ns INTEGER NOT NULL
  revoked_at_ns INTEGER
```

Idle-time updates should be rate-limited, for example at most once per minute, to avoid a write for every page asset. The session middleware still checks absolute expiry, idle expiry, revoked state, user disabled state, and current grant state on every API/page request.

### 7.5 Fresh authentication

Password changes, client-secret rotation, key rotation/retirement, managed backup creation, and sensitive diagnostics require `fresh_auth` with `max_auth_age = 5m`.

The reauthentication route starts a new authorization request with `prompt=login` and `max_age=0`, binds the attempt to the current admin session, and resumes only the stored safe return path. It does not retain a pending secret or password. The user reopens or resubmits the form after fresh auth.

## 8. Action handles, CSRF, versions, and idempotency

### 8.1 Opaque action handle

An action handle is a signed, base64url-encoded envelope:

```json
{
  "v": 1,
  "nonce": "random-public-nonce",
  "session_hash": "binding-digest",
  "subject": "owner-sub",
  "grant_id": "grt_...",
  "grant_version": 3,
  "scope_kind": "system",
  "scope_id": "system",
  "capability": "users.disable",
  "command": "user.disable",
  "target_type": "user",
  "target_id": "usr_...",
  "expected_version": 8,
  "required_assurance": "authenticated",
  "expires_at": "2026-07-23T21:00:00Z"
}
```

The envelope is authenticated with HMAC-SHA-256 using a dedicated owner-only key file. The raw key never enters SQLite. Destructive and secret-bearing commands also persist `H(handle nonce)` in `admin_action_nonces`; it is consumed in the command transaction.

The browser calls one mutation endpoint:

```http
POST /api/widget/actions/execute
Content-Type: application/json
X-CSRF-Token: <session-bound token>
Idempotency-Key: <random browser-generated key>

{
  "payload": {
    "actionHandle": "<opaque>",
    "input": {
      "reason": "Account no longer requires access",
      "confirmation": "DISABLE"
    }
  }
}
```

The endpoint ignores any browser-supplied `context.row`, command, target ID, capability, scope, or version. Those are presentation data, not authority.

### 8.2 CSRF

Use a session-bound synchronizer token. The initial authenticated bootstrap response exposes the token to the React application in memory, not in a readable long-lived cookie. RTK Query adds `X-CSRF-Token` to every state-changing request. The server checks exact origin, same-site session cookie, and token.

The UI must not depend on SameSite alone. Widget actions currently POST JSON without a CSRF header in `packages/rag-evaluation-site/src/app/App.tsx:96-100`; therefore TinyIDP needs its own RTK Query action adapter rather than using that default request path unchanged.

### 8.3 Resource versions

Create a generic aggregate version table:

```text
admin_resource_versions
  resource_type TEXT
  resource_id TEXT
  version INTEGER
  updated_at_ns INTEGER
  PRIMARY KEY(resource_type, resource_id)
```

SQLite triggers bump versions when relevant source records change:

- user aggregate: `users`, `password_credentials`, `account_security_states`;
- client aggregate: `clients`;
- signing key aggregate: `signing_keys`;
- invitation aggregate: `admin_invitation_records`.

This detects changes caused by the CLI, authentication state, another browser tab, or automation. Change `PutClient` away from `INSERT OR REPLACE` (`pkg/sqlitestore/store.go:286-289`) to an `ON CONFLICT DO UPDATE` form so update triggers and row identity behave predictably.

Command transaction:

```sql
UPDATE admin_resource_versions
SET version = version + 1, updated_at_ns = :now
WHERE resource_type = :type
  AND resource_id = :id
  AND version = :expected;
```

Zero affected rows returns `version_conflict`; it does not overwrite.

### 8.4 Idempotency

Persist:

```text
admin_idempotency
  actor_subject
  idempotency_key
  method
  route
  request_hash
  response_status
  response_json
  created_at_ns
  expires_at_ns
  PRIMARY KEY(actor_subject, idempotency_key)
```

Behavior:

```text
same key + same request hash -> return stored response
same key + different request -> 409 idempotency_conflict
new key                     -> execute once and store response in transaction
```

Never persist a response containing a one-time secret. Secret-producing commands persist a result receipt without the secret; a replay reports `secret_already_delivered`.

## 9. Persistence design

### 9.1 Migration 016: control plane

Create:

- `admin_grants`;
- `admin_auth_attempts`;
- `admin_sessions`;
- `admin_actions`;
- `admin_action_nonces`;
- `admin_idempotency`;
- `admin_audit_outbox`;
- `admin_operations`;
- `admin_invitation_records`;
- `admin_resource_versions`.

Every table has explicit indexes for its query paths, nanosecond UTC timestamps, and bounded status values enforced in Go and, where practical, `CHECK` constraints.

Grant schema:

```text
admin_grants
  id TEXT PRIMARY KEY
  actor_subject TEXT NOT NULL
  scope_kind TEXT NOT NULL
  scope_id TEXT NOT NULL
  role TEXT NOT NULL
  capabilities_json BLOB NOT NULL
  version INTEGER NOT NULL
  issued_at_ns INTEGER NOT NULL
  expires_at_ns INTEGER
  revoked_at_ns INTEGER
```

Use a partial unique index to enforce one active MVP owner:

```sql
CREATE UNIQUE INDEX admin_one_active_system_owner
ON admin_grants(scope_kind, scope_id, role)
WHERE scope_kind = 'system'
  AND scope_id = 'system'
  AND role = 'owner'
  AND revoked_at_ns IS NULL;
```

### 9.2 Migration 017: query projection

Create a safe, indexed projection:

```text
admin_user_projection
  user_id TEXT PRIMARY KEY
  scope_kind TEXT NOT NULL
  scope_id TEXT NOT NULL
  login TEXT NOT NULL
  display_name TEXT
  preferred_username TEXT
  email_normalized TEXT
  email_verified INTEGER NOT NULL
  disabled INTEGER NOT NULL
  locked_until_ns INTEGER
  last_successful_login_at_ns INTEGER
  created_at_ns INTEGER NOT NULL
  updated_at_ns INTEGER NOT NULL
  search_text TEXT NOT NULL
```

The authoritative records remain `users`, `password_credentials`, and `account_security_states`. Triggers or transaction-owned projection updates keep the safe view synchronized. Add consistency tests that rebuild the projection from source rows and compare it with the live projection.

Do not use FTS5 for the first implementation unless the repository explicitly enables and tests the SQLite build tag. Prefix/substring search over bounded normalized columns with ordinary indexes is sufficient for the stated 10,000-user scenario.

### 9.3 Invitation metadata

The raw invitation code remains represented only by a keyed hash in `durable_invitations` (`pkg/sqlitestore/migrations/012_durable_invitations.sql`). Add:

```text
admin_invitation_records
  id TEXT PRIMARY KEY
  scope_kind TEXT NOT NULL
  scope_id TEXT NOT NULL
  audience TEXT NOT NULL
  policy_version TEXT NOT NULL
  label TEXT
  created_by TEXT NOT NULL
  created_at_ns INTEGER NOT NULL
  expires_at_ns INTEGER NOT NULL
  revoked_at_ns INTEGER
  redeemed_at_ns INTEGER
  version INTEGER NOT NULL
```

The issuance transaction creates the durable invitation and metadata row together. The metadata row stores no raw code and no code hash exposed to queries. Revoke by public ID resolves the corresponding private hash inside the store; the UI never accepts a raw invitation code for revocation.

### 9.4 Activity and outbox

```text
admin_actions
  id TEXT PRIMARY KEY
  request_id TEXT NOT NULL
  actor_subject TEXT NOT NULL
  admin_session_id_hash BLOB NOT NULL
  scope_kind TEXT NOT NULL
  scope_id TEXT NOT NULL
  capability TEXT NOT NULL
  command TEXT NOT NULL
  target_type TEXT
  target_id TEXT
  expected_version INTEGER
  resulting_version INTEGER
  outcome TEXT NOT NULL
  reason_code TEXT
  operator_reason TEXT
  assurance TEXT NOT NULL
  created_at_ns INTEGER NOT NULL
```

Do not put secret values or full sensitive request bodies in the action row.

```text
admin_audit_outbox
  id TEXT PRIMARY KEY
  action_id TEXT NOT NULL UNIQUE
  event_json BLOB NOT NULL
  attempt_count INTEGER NOT NULL
  next_attempt_at_ns INTEGER NOT NULL
  delivered_at_ns INTEGER
  last_error_code TEXT
```

An `errgroup`-owned worker delivers pending rows to `idp.Sink`. Use bounded exponential backoff and readiness degradation after a configured age or attempt threshold. Keep the full error in server logs only after secret-safe classification; persist a stable error code.

### 9.5 Operations outside the database

Backups are filesystem work and cannot be atomic with the database record. Persist state:

```text
admin_operations
  id TEXT PRIMARY KEY
  actor_subject TEXT NOT NULL
  command TEXT NOT NULL
  state TEXT NOT NULL  # requested, running, completed, failed
  label TEXT
  relative_result_path TEXT
  started_at_ns INTEGER
  completed_at_ns INTEGER
  error_code TEXT
  created_at_ns INTEGER NOT NULL
```

The production Glazed configuration adds an explicit `admin-backup-root` field. The UI supplies only a bounded label. The server:

1. normalizes the label to a safe filename fragment;
2. generates a unique name;
3. joins it beneath the configured root;
4. resolves and verifies that the final path remains under the root;
5. runs the backup under an operation worker;
6. reports only a relative path.

Restore remains CLI-only.

## 10. Widget DSL and frontend architecture

### 10.1 Reuse from rag-evaluation-system

The local reference versions are:

```text
Go provider: github.com/go-go-golems/rag-evaluation-system v0.1.7
npm renderer: @go-go-golems/rag-evaluation-site 0.1.19
```

These are the versions used by the Upwork example (`upwork/go.mod:9`, `upwork/web/package.json`). Pin exact versions in the first implementation and add a consumer contract test before updating either side.

The provider exports only the hard-cutover `widget.dsl` module (`pkg/xgoja/providers/widgetsite/provider.go:10-33`). Relevant authoring APIs include:

- `widget.page(...)` for a page;
- `widget.app.shell(...)` for shared navigation and content layout;
- `widget.ui.*` for typed presentation helpers;
- `widget.data.collection(...)`, schemas, tables, search, pagination, and selection;
- `widget.act.server(...)`, `navigate(...)`, and overlay actions;
- `widget.bind.context(...)` for interaction-time values;
- `widget.ui.formDialog(...)` for typed dialogs.

The Upwork host selects the provider in `xgoja.yaml:27-31,68-70`, creates pages in `verbs/lib/pages.js`, and exposes `/api/widget/pages/{id}` in `verbs/upwork.js:252-272`. Its React entry point supplies `apiBase="/api/widget"` (`web/src/main.tsx:13-16`).

### 10.2 TinyIDP-specific safe module

Register a second xgoja module named `tinyidp.admin`. It exposes a narrow API:

```ts
interface TinyIDPAdminModule {
  currentRequest(): {
    principalRef: string
    scope: { kind: "system"; id: "system"; label: string }
    csrfToken: string
  }

  query(name: RegisteredQuery, input: unknown): SafeQueryResult
  action(command: RegisteredCommand, targetRef: string): OpaqueActionDescriptor
}
```

Internally, the provider resolves an unexported Go request context installed by `adminweb`; JavaScript cannot supply a different principal. `query` accepts only closed registry names and safe typed input. `action` returns a label and opaque handle after Go authorization.

Do not expose:

- raw `*sql.DB` or a general DB module;
- filesystem modules;
- arbitrary HTTP clients;
- raw `idpstore.Store`;
- capability mutation;
- action-key or session-key material;
- direct command execution by name.

Unlike Upwork's local-first host, TinyIDP Widget action routes must not be `.public()` and must not let JavaScript update the database directly.

### 10.3 Page composition example

Illustrative `users` page:

```js
function usersPage(widget, admin, query) {
  const result = admin.query("users.list", {
    q: String(query.q || ""),
    status: String(query.status || ""),
    cursor: String(query.cursor || ""),
    pageSize: Number(query.pageSize || 25),
  });

  const fields = widget.data.fields("tinyidp-users", (f) =>
    f.key("ref", { label: "ID", elide: true })
      .primary("name", { label: "Name" })
      .short("login", { label: "Login" })
      .short("email", { label: "Email" })
      .status("status", { label: "Status" })
      .date("lastSignIn", { label: "Last sign-in" })
  );

  const collection = widget.data.collection("users", result.rows, (c) =>
    c.schema(fields.build())
      .empty("No users match this view.")
      .paginate((p) =>
        p.current(result.page)
          .size(result.pageSize)
          .total(result.totalEstimate)
          .onChange(widget.act.navigate("/admin/users"))
      )
      .table((t) =>
        t.keyboard((k) => k.mode("rows"))
          .rowSelect(widget.act.navigate("/admin/users/${row.ref}"))
          .command("disable", (cmd) =>
            cmd.label("Disable")
              .danger()
              .action(widget.act.openOverlay("disable-user"))
          )
      )
  );

  return widget.page({ id: "users", title: "Users" }, (page) =>
    page.section("Directory", (section) => section.view(collection))
  ).toPage();
}
```

This example describes presentation only. The row DTO contains a public `ref`, safe text, current version, and opaque action descriptors. It contains no raw user object, password state, capability list, or authoritative scope.

### 10.4 React, Redux, RTK Query, pnpm, and Bootstrap

The TinyIDP consumer must use:

- pnpm and a committed lockfile;
- React and TypeScript;
- Redux Toolkit for session-local UI state;
- RTK Query for page and action transport;
- Bootstrap CSS for the access shell, fallback/error views, spacing, and non-widget utilities;
- `@go-go-golems/rag-evaluation-site` for Widget IR types, registry, renderer, and component styles.

Do not use the package's ready-made `RagEvaluationSiteApp` unchanged because its current server-action fetch does not add TinyIDP's CSRF header (`packages/rag-evaluation-site/src/app/App.tsx:87-129`). Build a small `AdminConsoleApp` around the exported `WidgetRenderer`, `defaultWidgetRegistry`, and IR types (`packages/rag-evaluation-site/src/index.ts`, `src/widgets/index.ts`).

Redux state must not persist secrets:

```ts
type AuthState = {
  status: "checking" | "anonymous" | "authenticated" | "denied"
  csrfToken?: string        // memory only
  freshUntil?: string
}
```

RTK Query base configuration:

```ts
const baseQuery = fetchBaseQuery({
  baseUrl: "/api/widget",
  credentials: "same-origin",
  prepareHeaders(headers, api) {
    const csrf = selectCSRF(api.getState())
    if (csrf) headers.set("X-CSRF-Token", csrf)
    return headers
  },
})
```

One-time secrets stay in local component state only. Close/unmount clears the value. Do not put them in Redux, RTK Query cache, URLs, local storage, session storage, toast text, or browser history.

### 10.5 Asset serving and embedding

Vite builds the frontend to the xgoja asset source declared in `internal/adminweb/xgoja.yaml`. Include that source in both the generated runtime package and the `embedded-assets` artifact, following the working `cmd/tinyidp-xapp/xgoja.yaml:60-67,87-100` pattern. The generated Go host owns the `go:embed` declarations and exposes the resulting asset store to the HTTP composition layer. Do not add a second hand-maintained asset manifest.

Mount hashed assets under `/static/admin/`. The SPA handler only handles `/admin` routes and must exclude `/api`, `/static`, OIDC endpoints, plugin routes, and health routes. Never serve static files directly beneath `/admin/`.

### 10.6 Content Security Policy and headers

Admin responses use:

```text
default-src 'none';
script-src 'self';
style-src 'self';
connect-src 'self';
img-src 'self' data:;
font-src 'self';
frame-ancestors 'none';
form-action 'self';
base-uri 'none';
object-src 'none';
```

Also set:

```text
Cache-Control: no-store                    # authenticated page/API/secret responses
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
Permissions-Policy: camera=(), microphone=(), geolocation=()
Cross-Origin-Opener-Policy: same-origin
```

Static hashed assets may be immutable; `index.html` and all authenticated data are `no-store`.

## 11. HTTP API contract

### 11.1 Authentication routes

| Method | Route | Purpose |
|---|---|---|
| GET | `/admin/auth/start` | Start dedicated OIDC authorization with PKCE |
| GET | `/admin/auth/callback` | Consume attempt, verify ID token, create/update admin session |
| POST | `/admin/auth/fresh` | Start session-bound fresh authentication |
| POST | `/admin/auth/logout` | Revoke admin session and clear cookie |
| GET | `/api/admin/session` | Return safe session/bootstrap state and CSRF token |

### 11.2 Widget routes

| Method | Route | Contract |
|---|---|---|
| GET | `/api/widget/pages/{page-id}` | Authenticated, capability-filtered Widget IR |
| POST | `/api/widget/actions/execute` | CSRF-protected opaque action execution |
| GET | `/api/widget/downloads/diagnostics/{handle}` | One-use authorized sanitized download |

Page IDs are registry keys, not filesystem names. Unknown IDs return 404. Pages may parse only a typed whitelist of URL parameters.

### 11.3 Error envelope

```json
{
  "ok": false,
  "error": {
    "code": "version_conflict",
    "message": "This record changed after the page was loaded.",
    "requestId": "req_...",
    "fieldErrors": {
      "email": "Enter a valid email address."
    }
  }
}
```

Mapping:

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `invalid_request` | Malformed or bounded-input failure |
| 401 | `admin_session_required` | Missing/expired/revoked session |
| 401 | `fresh_auth_required` | Valid session but authentication is too old |
| 403 | `csrf_rejected` | CSRF or origin check failed |
| 403 | `capability_denied` | Current grant lacks capability |
| 404 | `resource_not_found` | Resource absent in effective scope |
| 409 | `version_conflict` | Expected version is stale |
| 409 | `idempotency_conflict` | Key reused with another request |
| 410 | `action_expired` | Handle expired or nonce already consumed |
| 422 | `validation_failed` | Domain validation with field errors |
| 503 | `operation_unavailable` | Safe operation could not start |

Never reveal whether a resource exists outside the effective scope.

## 12. Command flows

### 12.1 Create user

```text
validate capability users.create
validate system scope
normalize login and profile fields
validate password using idpaccounts establishment policy
BEGIN
  recheck grant
  reserve resource ID and display-name claim if requested
  create user + credential atomically
  initialize resource version
  insert admin action
  insert audit outbox
  store idempotent non-secret result
COMMIT
return safe UserResult
```

On validation failure, the frontend may retain non-secret fields but clears both password inputs. The password remains valid until explicitly changed; do not call it “temporary.”

### 12.2 Disable/enable user

Resolve user by immutable user ID, not display login. Disabling delegates to the existing atomic `SetUserDisabled` invariant, which revokes browser and protocol artifacts. The admin transaction also compares the aggregate version, consumes the action nonce, and records activity/outbox state.

The confirmation shows the target, impact, required reason, and typed word `DISABLE`. Enabling does not restore revoked sessions or tokens.

### 12.3 Set password

Require `users.password.set` and fresh auth. Use the account service's password establishment policy. Password replacement atomically resets lockout/security state and revokes existing security artifacts. Return no password material.

If the mutation commits but external audit delivery is delayed, return:

```json
{
  "ok": true,
  "outcome": "completed",
  "audit": "pending",
  "requestId": "req_..."
}
```

The UI must not prompt an automatic mutation retry.

### 12.4 Invitations

Issue:

1. authorize and validate audience, policy version, expiry, label, and reason;
2. generate random public invitation ID and high-entropy raw code;
3. derive the private lookup hash;
4. transactionally create the durable invitation, metadata, action, outbox, and idempotency receipt;
5. return the raw code exactly once with `Cache-Control: no-store`;
6. clear server buffers where practical after serialization.

List and revoke by public invitation ID. Do not add a recipient email field until the underlying model supports recipient binding.

### 12.5 Clients

Creation and update use explicit primitives: redirect URIs, post-logout URIs, grant types, scopes, audiences, PKCE, introspection, token TTLs, public/confidential state, and disabled state. “Web app,” “SPA,” “Device,” and “Resource server” are derived display profiles, not stored authority.

Secret rotation is immediate because current code replaces the hash in one operation (`internal/admin/clients.go:140-149`). The UI must state that the existing secret stops working immediately. The new secret uses one-time delivery.

### 12.6 Signing keys

List redacted keys only. Ordinary rotation creates a new active RSA key and retires the prior active key while retaining it for verification. Require fresh auth and a reason.

Keep `DeleteRetiredSigningKey`/purge out of the browser because it can invalidate otherwise valid tokens. The CLI remains the break-glass surface.

### 12.7 Backups and diagnostics

Backup creation and verification operate only below the configured backup root and use operation records. Diagnostics output uses the existing sanitized export behavior and a one-use download handle. Restore, migration, and configuration mutation remain CLI-only.

## 13. Decision records

### Decision: one scope-aware control plane

- **Context:** The MVP has one installation and one owner; a later release needs domain administrators.
- **Options considered:** Ignore tenancy until later; expose a fake tenant; make every contract carry a scope.
- **Decision:** Persist and require `AdminScope` everywhere, with only `system/system` valid in MVP mode.
- **Rationale:** This keeps the first product honest while preventing unscoped APIs that must later be replaced.
- **Consequences:** Slightly more explicit code now; domain tables and grants can be added without route or command replacement.
- **Status:** accepted

### Decision: server-side grants, not ID-token roles

- **Context:** Long-lived browser sessions and revocable administrative authority must be supported.
- **Options considered:** Trust an ID-token role until session expiry; use a current database grant.
- **Decision:** OIDC proves subject; a current versioned server-side grant authorizes each request.
- **Rationale:** Grant revocation and capability changes take effect immediately and are auditable.
- **Consequences:** One indexed grant lookup per authenticated request; cache only within a request.
- **Status:** accepted

### Decision: public PKCE admin client

- **Context:** The console needs its own OIDC client but bootstrap should not create another persistent client secret.
- **Options considered:** Confidential client secret file; public authorization-code client with PKCE.
- **Decision:** Use a fixed public authorization-code client with required PKCE and no refresh token.
- **Rationale:** PKCE and exact same-origin redirect protect the exchange without a long-lived client secret.
- **Consequences:** State, nonce, verifier, redirect, and callback validation remain mandatory.
- **Status:** proposed

### Decision: Go owns security; Widget DSL owns presentation

- **Context:** Upwork demonstrates fast Widget DSL application development, but its public JavaScript routes and direct DB module are unsuitable for an IdP admin plane.
- **Options considered:** Copy Upwork's direct JavaScript store/actions; build every React screen by hand; use Widget DSL with a safe Go module.
- **Decision:** JavaScript composes typed Widget IR; Go resolves principals, queries, actions, authorization, and transactions.
- **Rationale:** This reuses the renderer without moving the security boundary into dynamic JavaScript or the browser.
- **Consequences:** A small `tinyidp.admin` xgoja provider is required. It intentionally exposes fewer capabilities than Upwork.
- **Status:** accepted

### Decision: custom RTK Query consumer around exported WidgetRenderer

- **Context:** Project guidelines require Redux/RTK Query, and the ready-made app's action fetch lacks TinyIDP's CSRF header.
- **Options considered:** Use `RagEvaluationSiteApp` unchanged; patch global `fetch`; build a thin typed consumer.
- **Decision:** Build `AdminConsoleApp` with RTK Query and exported Widget renderer primitives.
- **Rationale:** Request authentication, CSRF, structured errors, and cache/secret policy stay explicit.
- **Consequences:** TinyIDP owns a small amount of shell code and tests it against the pinned renderer.
- **Status:** accepted

### Decision: admin persistence interfaces are separate; SQLite implementation is shared

- **Context:** Protocol and UI interfaces should not be conflated, but mutations must be atomic across both sets of tables.
- **Options considered:** Add UI methods to `idpstore.Store`; separate SQLite packages with a leaked transaction; separate interfaces implemented by `sqlitestore.Store`.
- **Decision:** Define `pkg/idpadminstore`, implement it in `pkg/sqlitestore`.
- **Rationale:** Clean contracts and a single transaction are both preserved.
- **Consequences:** `pkg/sqlitestore` gains admin-specific files but `pkg/idpstore` remains protocol-oriented.
- **Status:** accepted

### Decision: transactional action record and audit outbox

- **Context:** Existing mutations can commit before the JSONL audit sink fails.
- **Options considered:** Return a generic error; write audit before mutation; add a transactional outbox.
- **Decision:** Commit action and outbox with the mutation, deliver to the existing sink after commit.
- **Rationale:** The UI can report the real mutation outcome and delivery becomes retryable.
- **Consequences:** Worker, retention, readiness, and reconciliation code are required.
- **Status:** accepted

### Decision: no backwards-compatibility adapter

- **Context:** Existing CLI methods and new web methods need one application service.
- **Options considered:** Keep `internal/admin` indefinitely behind an adapter; migrate callers to `pkg/idpadmin`.
- **Decision:** Hard-cut over operation by operation and remove superseded low-level service code after callers move.
- **Rationale:** Two service layers would drift on validation, audit, and security semantics.
- **Consequences:** CLI tests must be updated in the same phase as each moved operation.
- **Status:** accepted

## 14. Phased implementation guide

### Phase A: control-plane foundation

Files:

- add `pkg/idpadmin/*`;
- add `pkg/idpadminstore/*`;
- add migrations 016 and 017 plus `pkg/sqlitestore/admin_*.go`;
- add `internal/cmds/admin_console.go`.

Tasks:

1. Define closed scope and capability registries.
2. Implement grant, session, auth-attempt, nonce, idempotency, action, outbox, operation, and projection storage.
3. Add owner bootstrap/status/revoke CLI commands using Glazed.
4. Implement action-handle signing and verification with an explicit key-file setting.
5. Add transaction and projection consistency tests.
6. Add compile-time interface assertions.

Exit criteria:

- one owner grant can be bootstrapped and revoked;
- unknown capabilities and non-system scopes fail closed;
- nonce consumption, version CAS, and idempotency survive concurrent tests;
- migrations are checksummed and `go test ./...` passes.

### Phase B: authenticated read-only console

Files:

- add `internal/adminweb/auth.go`, session middleware, handler composition, asset embedding;
- add xgoja provider/spec/verbs;
- add `internal/adminweb/frontend`.

Tasks:

1. Add production config fields for enabling console, public base URL derivation, action/auth keys, backup root, and timeouts. Use Glazed fields; do not read environment variables directly.
2. Bootstrap the public PKCE admin client.
3. Implement OIDC attempts, callback, session cookie, logout, and fresh-auth refresh.
4. Mount `/admin`, `/api/admin`, `/api/widget`, and `/static/admin`.
5. Build pnpm/React/Redux/RTK Query consumer.
6. Implement access, overview, users, user detail, applications, keys, activity, and operations as read-only pages.
7. Add CSP and no-store middleware.

Exit criteria:

- grant revocation invalidates an existing browser session on its next request;
- anonymous users receive no admin data;
- every page query carries system scope;
- no Widget IR includes secret hashes or private keys;
- desktop/tablet page fixtures render through the pinned registry.

### Phase C: user mutations

Tasks:

1. Migrate CLI create/set-password/enable/disable operations to `pkg/idpadmin`.
2. Implement create/edit/enable/disable/unlock/set-password/revoke-access.
3. Add expected versions, action handles, typed confirmations, required reasons, and fresh auth.
4. Cover committed/audit-pending result semantics.
5. Add two-tab stale-update and concurrent nonce/idempotency tests.

Exit criteria:

- all user operations are actor/scope/capability audited;
- password fields clear after any failure;
- disabling and password replacement revoke expected artifacts;
- replay and stale writes do not mutate.

### Phase D: invitations and clients

Tasks:

1. Implement invitation metadata projection and public-ID lookup.
2. Issue/list/revoke invitations with one-time code display.
3. Migrate CLI client operations to `pkg/idpadmin`.
4. Implement client create/update/enable/disable/secret rotation.
5. Add repeatable URI validation and exact field errors.
6. Add one-time secret cache, reload, history, and replay tests.

Exit criteria:

- raw invitation codes are never stored in metadata or logs;
- invitation recipient email is absent;
- stored client hashes never appear in APIs;
- rotated old secrets fail immediately and the new secret is returned once.

### Phase E: keys and operations

Tasks:

1. Migrate ordinary key operations to `pkg/idpadmin`.
2. Implement rotate and eligible retire with fresh auth.
3. Keep emergency purge CLI-only.
4. Add doctor operation, managed backup create/verify, diagnostics download.
5. Run outbox and operation workers in the production host's `errgroup`.
6. Surface audit/outbox and operation health in readiness and Operations page.

Exit criteria:

- backup paths cannot escape configured root;
- server shutdown cancels and joins workers;
- pending audit delivery is visible and retryable;
- restore/migrate/purge have no browser route or action registry entry.

### Phase F: hardening and release

Tasks:

1. Add keyboard, screen-reader, focus, tablet, and reduced-motion checks.
2. Add loading, empty, no-results, forbidden, stale, session-expired, audit-degraded, and operation-failed fixtures for every screen.
3. Add deterministic Widget IR validation.
4. Add browser security cases from the supplied UX source.
5. Add visual snapshots and 10,000-user query benchmarks.
6. Run `go generate ./...`, frontend typecheck/build, `go fmt ./...`, `go test ./...`, `go build ./...`, and `make lint`.

Exit criteria are the definition of done in section 17.

## 15. Testing and validation strategy

### 15.1 Unit tests

Test:

- scope validation and capability registry completeness;
- grant active/revoked/expired/version behavior;
- action-handle signature, expiry, session, subject, scope, capability, target, and version binding;
- nonce one-use behavior;
- CSRF token verification;
- OIDC return-path validation;
- cursor signing, filter bounds, sort whitelists, and stable pagination;
- password/secret redaction;
- backup filename/root containment;
- error-to-HTTP mapping.

### 15.2 SQLite and concurrency tests

Use temporary SQLite files and real migrations. Test:

- one active owner invariant;
- transaction rollback leaves no action/outbox/idempotency record;
- mutation, action, version, and outbox commit together;
- two goroutines using one nonce produce one winner;
- two expected-version mutations produce one winner and one `version_conflict`;
- identical idempotency replay returns the stored safe response;
- mismatched replay returns conflict;
- projection rebuild matches live projection;
- invitation issue/redeem/revoke races remain one-time;
- outbox retry does not duplicate a delivered event.

Use `errgroup` for concurrent test actors where appropriate.

### 15.3 HTTP tests

Use `httptest` with a real store and fake clock:

- unauthenticated access redirects for pages and returns 401 for JSON;
- callback rejects invalid state, nonce, issuer, audience, auth time, subject, and reused attempts;
- disabled owner and revoked/changed grant invalidate access;
- missing/wrong CSRF and wrong Origin fail;
- page parameters are bounded and unknown page IDs are 404;
- action endpoint ignores fabricated context command/target/scope;
- CSP, no-store, cookie flags, and body limits are present;
- one-time secret responses do not survive a replay.

### 15.4 Browser tests

Run the server in tmux and use capture-pane for logs. Build Playwright scenarios for:

- login, logout, session expiry, and fresh-auth return;
- user directory filters persisted in URL;
- create, disable, enable, unlock, password, and revoke access;
- invitation and client one-time secret flows;
- stale form in two browser contexts;
- keyboard-only row navigation and dialogs;
- tablet viewport and emergency small-screen disable;
- audit-degraded and operation-failed states;
- CSP blocks inline script and external network attempts.

When a server must be restarted, kill its port with `lsof-who -p $PORT -k` as required by the repository guidelines.

### 15.5 Widget contract tests

For every page fixture:

1. generate Widget IR through the same xgoja provider used in production;
2. validate it against the typed IR schema;
3. assert every server action name is `execute`;
4. assert every mutation has an opaque handle;
5. reject URLs outside allowed navigation/download forms;
6. reject secret-like property names and values;
7. render with `defaultWidgetRegistry`;
8. fail on `UnknownWidget`;
9. snapshot desktop and tablet layouts.

Do not use RAG output as an acceptance oracle. RAG may generate candidate DSL during design; deterministic schema, security, accessibility, browser tests, and human review decide what ships.

### 15.6 Required commands

```text
pnpm --dir internal/adminweb/frontend install
pnpm --dir internal/adminweb/frontend typecheck
pnpm --dir internal/adminweb/frontend build
go generate ./...
go fmt ./...
go test ./...
go build ./...
make lint
```

The repository has only one top-level `go.mod`; do not create another.

## 16. Risks, mitigations, and open questions

### 16.1 Risks

**xgoja presentation code becomes an authority.** Mitigate with the narrow `tinyidp.admin` provider, one generic execute action, opaque handles, and no DB/filesystem/HTTP modules.

**Projection drift.** Mitigate with one transaction owner, triggers or named update paths, rebuild/compare tests, and readiness diagnostics.

**SQLite contention.** The store supports one open connection (`pkg/sqlitestore/store.go:47-66`). Keep page queries bounded, session touch writes rate-limited, and worker transactions short. Benchmark the 10,000-user scenario before release.

**Admin and OIDC self-dependency.** The admin console authenticates against the provider it administers. Keep CLI bootstrap/recovery and break-glass operations working when the web console is unavailable.

**Secret leakage through generic UI state.** Use dedicated one-time DTOs, memory-only components, no-store responses, property/value contract scans, and redaction tests.

**Action handles mistaken for authorization.** A valid handle is only one input. Execution must reload the current session/grant and repeat authorization in the transaction.

**Operational task ambiguity.** Use `admin_operations` state rather than holding an HTTP request open for filesystem work.

### 16.2 Open questions requiring owner review

1. Should the console be enabled by default in `serve-production`, or require an explicit `--admin-console-enabled` flag? This design recommends explicit enablement for the first release.
2. Should the admin authentication/action encryption keys be separate files or derived with HKDF from the existing token secret? This design recommends separate files to isolate compromise and rotation.
3. Should owner grant bootstrap allow replacing an existing owner in one command, or require explicit revoke then bootstrap? This design recommends two explicit commands.
4. How long should admin actions, idempotency rows, sessions, and outbox delivery history be retained? Proposed starting points: actions 365 days, idempotency 24 hours, expired sessions 30 days, delivered outbox 30 days.
5. Should the action/outbox worker be required for readiness or only degrade readiness after an age threshold? Proposed: immediate health warning, readiness failure after five minutes of oldest pending delivery.

None of these questions blocks the package, API, or transaction design.

## 17. MVP definition of done

The MVP is complete only when:

1. The owner authenticates through the fixed dedicated public PKCE OIDC client.
2. Owner bootstrap and grant/session revocation are CLI-only and audited.
3. The admin cookie is separate from the IdP browser cookie.
4. Every query, command, session, action, and action handle carries system scope.
5. Every mutation verifies current session, current grant, capability, server scope, CSRF, action handle, nonce, assurance, expected version, and idempotency.
6. Users can be listed, searched, viewed, created, edited, disabled, enabled, unlocked, assigned a new password, and signed out everywhere.
7. Invitations can be issued, listed, revoked by public ID, and revealed once.
8. Clients can be created, inspected, edited, enabled, disabled, and have secrets rotated once.
9. Signing keys can be inspected and safely rotated; emergency purge remains CLI-only.
10. Doctor, readiness, audit/outbox health, managed backup create/verify, and sanitized diagnostics are available.
11. Administrative activity includes actor, command, target, scope, reason, result, assurance, versions, and request ID.
12. Mutations and action/outbox records commit atomically.
13. Widget DSL and browser state contain no arbitrary authority, code, SQL, external URL, raw secret, stored hash, or private key.
14. Every screen has loading, empty, no-results, denied, stale, expired-session, audit-degraded, and safe-failure behavior.
15. The console is keyboard-usable and works at desktop and tablet widths.
16. `go test ./...`, `go build ./...`, generation, lint, frontend typecheck/build, browser security tests, and Widget IR contract tests pass.
17. Adding identity domains later requires new data and capability-visible navigation, not replacement of MVP contracts.

## 18. File and API reference map

### TinyIDP current code

- `internal/admin/service.go:14-45` — current low-level service and post-commit audit behavior.
- `internal/admin/users.go:12-30` — existing user get/disable operations.
- `internal/admin/clients.go:17-149` — client request fields, hashing, list/read/disable/rotate behavior.
- `internal/admin/keys.go:13-95` — signing-key lifecycle and redaction.
- `internal/admin/doctor.go:12-77` — existing doctor checks.
- `internal/admin/backup.go:10-30` — current path-oriented backup API.
- `internal/cmds/admin.go:175-229` — CLI opens store and audit per command.
- `internal/cmds/admin_invitation.go:118-180` — current one-time invitation issuance and revoke flows.
- `internal/cmds/serve_production.go:112-212` — production settings, long-lived store/audit/provider bootstrap.
- `internal/cmds/serve_production.go:307-447` — public/internal listener composition and shutdown.
- `pkg/idpstore/interfaces.go:28-238` — protocol persistence and named atomic operations.
- `pkg/idpstore/types.go:22-120` — client, user, credentials, invitation, and security record shapes.
- `pkg/sqlitestore/migrations/001_schema.sql` through `015_integration_transactions.sql` — current migration history.
- `pkg/sqlitestore/store.go:28-140` — SQLite durability and single-connection envelope.
- `pkg/sqlitestore/store.go:286-360` — current client/user persistence.
- `pkg/idp/audit.go:14-180` — audit delivery semantics and durable file sink.
- `docs/embedding-foundations.md:13-20,55-65,148-159` — supported embedding boundaries and audit warning.
- `cmd/tinyidp-xapp/production_app.go:21-149` — working same-process OIDC, separate app session, and host composition example.

### Widget DSL and Upwork reference

- `/home/manuel/code/wesen/go-go-golems/upwork/xgoja.yaml:27-31,68-93` — provider, module, help, and assets declaration.
- `/home/manuel/code/wesen/go-go-golems/upwork/verbs/upwork.js:228-319` — SPA, page routes, and Widget actions.
- `/home/manuel/code/wesen/go-go-golems/upwork/verbs/lib/pages.js:1-80` — app shell and page composition style.
- `/home/manuel/code/wesen/go-go-golems/upwork/web/src/main.tsx:1-17` — React renderer consumer.
- `/home/manuel/code/wesen/go-go-golems/upwork/docs/help/upwork-tracker-developer-guide.md:28-58` — reference architecture.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/xgoja/providers/widgetsite/provider.go:10-33` — `widget.dsl` provider registration.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/widgetdsl/v3.go:66-193` — page and app-shell builders.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/xgoja/providers/widgetsite/doc/01-widget-dsl-getting-started.md:16-39` — first typed page.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/pkg/xgoja/providers/widgetsite/doc/04-widget-dsl-v3-examples.md:21-110,165-200` — actions, bindings, serving, and fallback rules.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/app/App.tsx:67-129` — current page/action fetch behavior.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/widgets/actions.ts:10-32,114-139` — action context and default server action transport.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/widgets/registry.ts:1-44` — closed Widget registry.
- `/home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/widgets/WidgetRenderer.tsx:12-103` — registry-backed rendering and unknown-widget behavior.

### Imported product source

- `sources/local/tiny-idp-ux.md` — full 3,015-line screen, interaction, backend, multitenancy, migration, evaluation, and definition-of-done source imported from `/tmp/tiny-idp-ux.md`.
