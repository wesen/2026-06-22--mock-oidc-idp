# TinyIDP Admin Console

## Detailed single-install MVP and multitenant-ready design

## 1. Core product decision

Build **one administration console and one control-plane architecture**.

The first release exposes only:

```text
One TinyIDP installation
One owner administrator
One system-wide scope
No customer domains
No delegated administrators
```

The later release adds:

```text
System-wide mega administrators
Identity domains
Domain administrators
Domain-scoped user management
Domain policies and audit views
```

The MVP should not create a fake customer domain or expose tenant concepts prematurely. Instead, every backend query, command, audit event, action handle, and widget definition should accept an abstract scope:

```text
MVP
Owner ──> System scope ──> All resources

Later
Mega admin   ──> System scope ──> All resources
Domain admin ──> Domain scope ──> Domain resources only
```

In the MVP, `current_scope` always resolves to:

```yaml
kind: system
id: system
```

When multitenancy is added, the same contracts also accept:

```yaml
kind: domain
id: dom_acme
```

This avoids two separate admin products and makes the future domain implementation an extension rather than a rewrite.

---

# 2. Repository starting point

TinyIDP already has a strong set of operational primitives. Its CLI supports initialization, migrations, health checks, users, OIDC clients, signing keys, backups, and sanitized diagnostics.

The present control surface is still CLI-shaped:

* User storage supports point lookups but not a paginated directory.
* The current admin service has a store, clock, and audit sink, but no authenticated actor, authorization scope, capability resolver, or optimistic version contract.
* The CLI opens SQLite and the audit sink for an individual command. That is appropriate for a CLI, but a web console needs long-lived dependencies and request-scoped principals.
* Audit delivery is synchronous and durable, but the sink is append-only rather than queryable. An operation may have committed before an audit delivery error is returned.
* Durable signup invitations are presently bound to an OIDC audience, policy version, and expiration—not to a recipient email or customer domain.
* Client administration already supports creation, listing, lookup, enable/disable, and immediate secret rotation.
* Signing-key generation, rotation, listing, retirement, and emergency purge already exist.
* The doctor report already checks the schema, all configured clients, the active signing key, and verification keys.

## 2.1 Capability status

| Function                       |    Current core support | MVP backend work                    |
| ------------------------------ | ----------------------: | ----------------------------------- |
| Get one user                   |                     Yes | Expose through scoped query service |
| List/search users              |                      No | New admin read model                |
| Create user                    |                     Yes | Actor/scope wrapper                 |
| Enable/disable user            |                     Yes | Actor/scope wrapper                 |
| Set password                   |                     Yes | Fresh-auth and reason wrapper       |
| Edit user profile              |   Partial store support | New command                         |
| Unlock account                 |  Store primitive exists | New admin command                   |
| Revoke all user access         |  Store primitive exists | New admin command                   |
| List individual sessions       |       No suitable query | Defer or add later                  |
| Issue signup invitation        |                     Yes | Web-safe command wrapper            |
| List invitations               |                      No | New read model                      |
| Revoke invitation by public ID |                      No | New command/index                   |
| Create/list clients            |                     Yes | Scoped wrapper                      |
| Update client configuration    | No public admin command | New command                         |
| Rotate client secret           |                     Yes | One-time reveal workflow            |
| List/rotate keys               |                     Yes | Scoped wrapper                      |
| Purge retired key              |                     Yes | Keep CLI-only initially             |
| Doctor checks                  |                     Yes | Web presentation                    |
| Backup create/verify           |                     Yes | Managed-directory wrapper           |
| Backup restore                 |                     Yes | Keep CLI-only initially             |
| Diagnostics export             |                     Yes | Download endpoint                   |
| Query admin activity           |                      No | New transactional activity store    |
| Domain management              |                      No | Future release                      |
| Delegated administrators       |                      No | Future UI; grant model starts now   |

---

# 3. Terminology and invariants

## 3.1 System scope

The entire TinyIDP installation.

System resources include:

* Users in the MVP.
* OIDC clients.
* Signing keys.
* Backups.
* Schema and readiness.
* Global administrative activity.
* Future identity domains.

## 3.2 Domain scope

A future identity-administration boundary owned by a customer.

Domain resources will include:

* Domain users.
* Domain invitations.
* Domain administrator grants.
* Domain policies.
* Domain aliases.
* Domain-scoped activity.

## 3.3 Do not reuse `User.Tenant`

`User.Tenant` currently sits beside groups, roles, locale, and profile information as an OIDC user claim.

It should remain an application-facing claim and must not become the authorization boundary for the admin console. That would couple customer administration to token claim semantics and make migration unsafe.

## 3.4 Preserve global identities

For the first multitenant release:

* User IDs remain globally unique.
* OIDC subjects remain unchanged.
* Logins remain globally unique.
* Credentials remain attached to global users.
* A user has exactly one home identity domain.
* Domain membership is stored separately from the user profile.

The current SQLite schema already treats user login as globally unique.

---

# 4. First release: single-owner MVP

## 4.1 Product name

Use a neutral name in the UI:

```text
TinyIDP Console
```

Do not label the administrator “mega admin” in this mode. Display:

```text
Role: Owner
Scope: This installation
```

The scope can remain visually subtle because it cannot be changed.

## 4.2 MVP goal

The owner should be able to operate TinyIDP without routinely using the CLI for daily administration.

The CLI remains the break-glass and recovery surface.

## 4.3 MVP navigation

```text
Overview
Users
Invitations
Applications
Signing keys
Activity
Operations
```

Not shown in the MVP:

```text
Domains
Administrators
Domain settings
Global security center
Scope switcher
Role editor
Bulk user operations
```

## 4.4 Route map

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

These routes remain valid after multitenancy. Future domain pages are added beneath:

```text
/admin/domains/{domain-id}/...
```

## 4.5 MVP owner capabilities

The owner receives a server-side grant containing:

```yaml
role: owner
scope:
  kind: system
  id: system
capabilities:
  - overview.read
  - users.read
  - users.create
  - users.update
  - users.disable
  - users.password.set
  - users.unlock
  - users.access.revoke
  - invitations.read
  - invitations.create
  - invitations.revoke
  - clients.read
  - clients.create
  - clients.update
  - clients.disable
  - clients.secret.rotate
  - keys.read
  - keys.generate
  - keys.rotate
  - keys.retire
  - activity.read
  - operations.read
  - operations.doctor
  - operations.backup.create
  - operations.backup.verify
  - operations.diagnostics
```

Even with only one owner, the UI and backend must check these capabilities. The single-owner deployment simply assigns all of them.

---

# 5. MVP authentication and administrative session

## 5.1 Dedicated OIDC client

The console should authenticate through TinyIDP as a normal relying party using a dedicated client:

```text
client_id: tinyidp-admin-console
grant: authorization_code
PKCE: required
redirect: https://id.example/admin/auth/callback
scopes: openid profile email
refresh tokens: disabled initially
```

The console should have its own server-side session and cookie. Do not reuse TinyIDP’s browser authentication cookie as the application session.

## 5.2 Session properties

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
  server_side: true
  stored_handle: keyed_hash
```

## 5.3 Fresh authentication

TinyIDP does not yet expose a completed production MFA administration flow. Therefore, the MVP should distinguish:

```text
Authenticated
Freshly authenticated
```

Sensitive commands require a fresh login within a short window:

* Set a user password.
* Rotate a client secret.
* Rotate or retire a signing key.
* Create a backup.
* Modify the owner grant through CLI.
* Export sensitive administrative metadata.

The UI can later replace `fresh_auth` with `aal2` without changing the command or screen model.

```yaml
authorization:
  required_assurance: fresh_auth
  max_auth_age: 5m
```

## 5.4 Bootstrap

The first owner must be bootstrapped through CLI, not through a public browser route.

A future bootstrap command should conceptually perform:

```text
1. Resolve an existing TinyIDP user by login.
2. Create one system-scoped owner grant for its subject.
3. Register or validate the admin-console OIDC client.
4. Print no persistent user or client secret.
```

When no owner exists, `/admin` displays instructions to use the CLI. It must not expose a “make me owner” button.

---

# 6. Widget DSL conventions

Every screen below assumes a closed widget registry.

```yaml
version: tinyidp.admin.ui/v1
screen:
  id: users.list
  scope: current
  requires: [users.read]
  body: []
```

## 6.1 Allowed runtime references

```yaml
source: users.list
command: user.disable
schema: user.create
dialog: user.disable.confirm
```

Each name resolves through a server-owned registry.

## 6.2 Forbidden DSL content

The DSL must not contain:

* SQL.
* Arbitrary HTTP methods or URLs.
* JavaScript expressions.
* Raw HTML.
* Inline scripts.
* Authorization logic.
* Browser-selected authoritative scope.
* Secrets.
* Filesystem paths.
* Unregistered query names.
* Unregistered commands.

## 6.3 Scope convention

Every source and command inherits the current server-resolved scope:

```yaml
scope: inherit
```

The value is `system` in the MVP and can later be a domain.

## 6.4 Presentation checks versus authorization

This is valid presentation behavior:

```yaml
requires: [users.disable]
```

It may hide a button, but it is not authorization. The server must repeat the capability and scope check when the command is submitted.

---

# 7. MVP screen specifications

---

## Screen 0 — Administrative access

### Purpose

Authenticate the owner and establish an admin-console session.

### States

```text
Checking session
Sign-in required
Fresh authentication required
Access granted
No owner grant
Grant revoked
Account disabled
Console unavailable
```

### ASCII

```text
┌──────────────────────────────────────────────────────────────────────┐
│                         TinyIDP Console                              │
│                                                                      │
│                    Administration access                             │
│                                                                      │
│  Manage users, applications, signing keys, and system operations.    │
│                                                                      │
│  Administrative access is restricted to the configured owner.       │
│                                                                      │
│                  [ Continue with TinyIDP ]                            │
│                                                                      │
│  ──────────────────────────────────────────────────────────────────  │
│  Current installation                                                │
│  https://id.example                                                  │
│                                                                      │
│  Administrative access and changes are recorded.                     │
└──────────────────────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: admin.access
  scope: system
  body:
    - access-gate:
        source: admin.session.status
        states:
          - sign_in_required
          - access_denied
          - granted
        actions:
          - command: admin.session.start
```

---

## Screen 1 — Overview

### Purpose

Show whether the installation needs attention and provide shortcuts to frequent tasks.

### Information

Metric cards:

* Active users.
* Disabled users.
* Locked users.
* Applications.
* Pending invitations.
* Active signing-key status.

Attention list:

* Doctor check failure.
* No active signing key.
* Expired or aging key.
* Disabled or invalid client.
* Audit delivery degraded.
* Locked users.
* Expiring invitations.
* Backup overdue, when backup policy is configured.

Quick actions:

* Create user.
* Issue invitation.
* Create application.
* Run checks.
* Create backup.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                     Owner · Fresh for 3m       ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Overview                                          │
│ Users            │                                                   │
│ Invitations      │ ┌───────────┐ ┌───────────┐ ┌───────────┐        │
│ Applications     │ │ Active    │ │ Locked    │ │ Apps      │        │
│ Signing keys     │ │ users  42 │ │ users   2 │ │       5   │        │
│ Activity         │ └───────────┘ └───────────┘ └───────────┘        │
│ Operations       │                                                   │
│                  │ Attention                                         │
│                  │ ┌───────────────────────────────────────────────┐ │
│                  │ │ Warning  2 accounts are locked               │ │
│                  │ │ Warning  Signing-key rotation due soon       │ │
│                  │ │ Healthy  Schema and configured clients       │ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Quick actions                                     │
│                  │ [Create user] [Issue invitation] [Create app]     │
│                  │                                                   │
│                  │ Recent activity                                   │
│                  │ 10:42 User disabled        alex                  │
│                  │ 09:18 Client created       internal-dashboard    │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: overview
  scope: current
  requires: [overview.read]
  body:
    - page-header
    - metric-grid:
        source: overview.metrics
    - attention-list:
        source: overview.findings
    - quick-actions:
        commands:
          - user.create
          - invitation.create
          - client.create
    - activity-list:
        source: activity.recent
```

### Backend work

New aggregate read query:

```go
GetOverview(ctx, principal, scope) (Overview, error)
```

The query should return counts and statuses, not raw records.

---

## Screen 2 — User directory

### Purpose

Locate users quickly and perform common account operations.

### Columns

| Column       | Notes                       |
| ------------ | --------------------------- |
| Name         | Primary identity label      |
| Login        | Canonical login             |
| Email        | Verification indicator      |
| Status       | Active, disabled, or locked |
| Last sign-in | From account security state |
| Updated      | Profile or security update  |
| Actions      | Context menu                |

### Filters

```text
Search: name, login, exact email
Status: active, disabled, locked
Email: verified, unverified, missing
Last sign-in: today, 7 days, 30 days, never, custom
Created: date range
```

Do not implement arbitrary query-language syntax.

### Actions

```text
Open
Disable
Enable
Unlock
Set password
Revoke all access
```

No delete action in the MVP.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Users                              [Add user ▾]   │
│ Users            │                                                   │
│ Invitations      │ [ Search name, login, or email…              ]   │
│ Applications     │ [Status ▾] [Email ▾] [Last sign-in ▾] [Clear]    │
│ Signing keys     │                                                   │
│ Activity         │ ┌───────────────────────────────────────────────┐ │
│ Operations       │ │ Name          Login      Status    Last sign │ │
│                  │ ├───────────────────────────────────────────────┤ │
│                  │ │ Alex Morgan   alex       Active    Today    ⋮│ │
│                  │ │ Sam Green     sam        Locked    Jul 22   ⋮│ │
│                  │ │ Erin Chen     erin       Active    Jul 19   ⋮│ │
│                  │ │ Dev Account   service-1  Disabled  Never    ⋮│ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ 1–25                                      Next › │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: users.list
  scope: current
  requires: [users.read]
  body:
    - page-header:
        actions:
          - command: user.create
          - command: invitation.create
    - filter-bar:
        fields: [query, status, email_state, last_sign_in]
    - data-table:
        source: users.list
        row-link: user.open
        row-actions:
          - user.disable
          - user.enable
          - user.unlock
          - user.password.set
          - user.access.revoke
    - cursor-pagination
```

### Backend work

The protocol store should not be expanded indiscriminately with UI-oriented methods. Add a separate read contract:

```go
type UserQueryStore interface {
    ListUsers(
        ctx context.Context,
        scope AdminScope,
        filter UserFilter,
        page PageRequest,
    ) (Page[UserRow], error)

    GetUserDetail(
        ctx context.Context,
        scope AdminScope,
        userID string,
    ) (UserDetail, error)
}
```

The current `UserStore` only provides point lookups, making this separation necessary.

### Tablet behavior

At tablet width:

```text
Alex Morgan
alex · alex@example.com ✓
Active · Last sign-in today                         [⋮]
```

Secondary fields move into row expansion. Filters open in a right-side sheet.

---

## Screen 3 — User detail

### Purpose

Show profile and security state together while keeping high-risk actions explicit.

### Tabs

```text
Overview
Security
Activity
```

Do not show an individual sessions tab until the backend can provide an accurate, bounded session inventory.

### Overview fields

```text
Display name
Login
Preferred username
Email
Email verified
Locale
Groups
Roles
Tenant claim
User ID
OIDC subject
Created
Updated
```

`Tenant claim` must appear under **OIDC claims**, not under administration scope.

### Security fields

```text
Account status
Credential status
Password changed
Failed login count
First and last failed login
Locked until
Last successful login
```

### Actions

```text
Edit profile
Disable / enable
Unlock account
Set password
Revoke all access
```

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Alex Morgan                         Active ●      │
│ Users            │ alex · alex@example.com                           │
│ Invitations      │                                      [Actions ▾] │
│ Applications     │                                                   │
│ Signing keys     │ Overview     Security     Activity                │
│ Activity         │ ────────────────────────────────────────────────  │
│ Operations       │                                                   │
│                  │ Identity                                          │
│                  │ Login                 alex                        │
│                  │ Email                 alex@example.com ✓          │
│                  │ Preferred username    alex                        │
│                  │ Locale                en-US                       │
│                  │                                                   │
│                  │ Security                                          │
│                  │ Last successful login Today at 09:42              │
│                  │ Password changed      42 days ago                 │
│                  │ Failed sign-ins       0                           │
│                  │                                                   │
│                  │ [Edit profile] [Set password] [Revoke all access] │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: user.detail
  scope: current
  requires: [users.read]
  body:
    - entity-header:
        source: user.summary
        actions:
          - user.update
          - user.disable
          - user.enable
          - user.unlock
          - user.password.set
          - user.access.revoke
    - tabs:
        items:
          - id: overview
            widgets: [user-profile, oidc-claims]
          - id: security
            widgets: [user-security]
          - id: activity
            widgets: [user-activity]
```

### Backend work

Small commands:

```go
UpdateUserProfile(...)
UnlockUser(...)
RevokeUserAccess(...)
```

`RevokeUserAccess` should call the existing atomic security-artifact revocation primitive rather than reimplementing token and session deletion.

Disabling a user already has an atomic store contract intended to revoke browser, domain-token, and Fosite protocol artifacts.

---

## Screen 4 — Create user

### Purpose

Create a durable local account directly.

The preferred path for ordinary human onboarding should eventually be a signup invitation. Direct creation remains useful for the installation owner, service accounts, recovery accounts, and controlled provisioning.

### Important wording

Do not call the initial password “temporary.”

TinyIDP deliberately removed `MustChangeAtLogin` because no safe forced-change flow existed.

Use:

```text
Set account password
```

and display:

```text
This password remains valid until it is changed again.
```

### Form sections

#### Identity

| Field              | Behavior                             |
| ------------------ | ------------------------------------ |
| Login              | Required, normalized preview, unique |
| Display name       | Recommended                          |
| Email              | Optional but validated               |
| Email verified     | Explicit checkbox                    |
| Preferred username | Defaults to login                    |
| Locale             | Optional select                      |

#### Password

| Field            | Behavior                |
| ---------------- | ----------------------- |
| Password         | Never prefilled         |
| Confirm password | Must match              |
| Show password    | Local presentation only |
| Policy feedback  | Server-authoritative    |

The current default establishment policy requires at least 15 Unicode code points, permits long passwords, normalizes with NFC, checks a blocklist, and bounds input before Argon2id work.

#### Advanced OIDC claims

```text
Groups
Roles
Tenant claim
```

User ID and OIDC subject should normally be generated and not editable in the web UI.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ Create user                                           [Cancel]      │
├──────────────────────────────────────────────────────────────────────┤
│ Identity                                                             │
│                                                                      │
│ Login *                  [ alex                                  ]   │
│ Display name             [ Alex Morgan                           ]   │
│ Email                    [ alex@example.com                      ]   │
│ [✓] Email is verified                                                │
│ Preferred username       [ alex                                  ]   │
│ Locale                   [ en-US                              ▾ ]     │
│                                                                      │
│ Account password                                                     │
│ Password *               [ ••••••••••••••••••••                ]   │
│ Confirm password *       [ ••••••••••••••••••••                ]   │
│ [ ] Show password                                                    │
│                                                                      │
│ This password remains valid until it is changed again.               │
│                                                                      │
│ Advanced OIDC claims                                      [Expand]   │
│                                                                      │
│                                   [Cancel] [Create user]              │
└──────────────────────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: user.create
  scope: current
  requires: [users.create]
  body:
    - page-header
    - form:
        schema: user.create
        sections:
          - identity
          - password
          - oidc-claims
        submit:
          command: user.create
```

### Secret handling

On validation failure:

* Preserve non-secret fields.
* Clear both password fields.
* Never include password values in page state, logs, audit fields, or DSL data.

---

## Shared flow — Set user password

Changing a password atomically resets lockout state and revokes browser sessions, grants, authorization codes, access tokens, refresh tokens, and corresponding Fosite state.

### ASCII

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Set password for Alex Morgan                                        │
│                                                                      │
│ This action will:                                                    │
│ • Replace the current password                                      │
│ • Clear account lockout state                                       │
│ • Sign the user out                                                  │
│ • Revoke active grants and tokens                                   │
│                                                                      │
│ New password        [ ••••••••••••••••••••                      ]  │
│ Confirm password    [ ••••••••••••••••••••                      ]  │
│ Reason              [ Owner-requested credential replacement     ]  │
│                                                                      │
│ Fresh authentication is required.                                   │
│                                                                      │
│                              [Cancel] [Verify and set password]       │
└──────────────────────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
dialog:
  id: user.password.set
  requires: [users.password.set]
  assurance: fresh_auth
  children:
    - impact-summary
    - form:
        schema: user.password.set
        fields: [password, password_confirm, reason]
        submit:
          command: user.password.set
```

---

## Screen 5 — Invitations

### Purpose

Issue and manage durable signup invitations.

### MVP semantics

The current invitation is:

* One-time.
* Opaque.
* Bound to an exact OIDC client audience.
* Bound to a reviewed policy version.
* Expiring.
* Revocable.
* Not currently recipient-email-bound.

The UI must not display a recipient field unless the underlying invitation model is extended.

### Table columns

```text
Invitation ID
Application
Policy
Status
Created
Expires
Redeemed
```

### Statuses

```text
Pending
Redeemed
Revoked
Expired
```

### Filters

```text
Application
Status
Policy version
Expiration date
Created date
```

### Create form

```text
Application / OIDC audience
Policy version
Expiration
Optional operator label
Administrative reason
```

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Invitations                   [Issue invitation]  │
│ Users            │                                                   │
│ Invitations      │ [Application ▾] [Status ▾] [Expires ▾] [Clear]   │
│ Applications     │                                                   │
│ Signing keys     │ ┌───────────────────────────────────────────────┐ │
│ Activity         │ │ ID          Application    Status    Expires │ │
│ Operations       │ ├───────────────────────────────────────────────┤ │
│                  │ │ inv_7M…     message-desk   Pending   Jul 30 ⋮│ │
│                  │ │ inv_A2…     admin-demo     Redeemed  —      ⋮│ │
│                  │ │ inv_P9…     message-desk   Expired   Jul 20 ⋮│ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Invitation codes are shown only once.             │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Create invitation drawer

```text
┌───────────────────────────────────────────────────────┐
│ Issue signup invitation                              │
│                                                       │
│ Application *        [ message-desk              ▾ ] │
│ Policy version *     [ signup-invite-v1          ▾ ] │
│ Expires after *      [ 24 hours                  ▾ ] │
│ Operator label       [ New contractor onboarding   ] │
│ Reason               [ Approved account signup      ] │
│                                                       │
│                         [Cancel] [Issue invitation]   │
└───────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: invitations.list
  scope: current
  requires: [invitations.read]
  body:
    - page-header:
        actions:
          - command: invitation.create
    - filter-bar:
        fields: [application, status, expiry, policy]
    - data-table:
        source: invitations.list
        row-actions:
          - invitation.revoke
    - cursor-pagination
```

### Backend work

The current table is keyed only by the secret-derived code hash and has no normalized public invitation ID column.

Add an administration metadata projection so that the UI can:

* List by public invitation ID.
* Filter without handling secrets.
* Revoke by public invitation ID.
* Record issuance time and actor.
* Later associate a domain and recipient.

A future-shaped record:

```text
signup_invitation_records
  id
  scope_kind
  scope_id
  durable_invitation_id
  audience
  policy_version
  label
  created_by
  created_at
  expires_at
  revoked_at
  redeemed_at
```

The raw code is never stored there.

---

## Shared flow — One-time secret reveal

Used for:

* Signup invitation code or link.
* Generated client secret.
* Rotated client secret.

### ASCII

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Invitation created                                                   │
│                                                                      │
│ This value will not be shown again.                                  │
│                                                                      │
│ ┌──────────────────────────────────────────────────────────────────┐ │
│ │ https://id.example/signup?invite=••••••••••••••••••••••••••   │ │
│ └──────────────────────────────────────────────────────────────────┘ │
│                                                                      │
│ [Reveal] [Copy]                                                       │
│                                                                      │
│ [ ] I have stored or delivered this value securely                   │
│                                                                      │
│                                                        [Done]        │
└──────────────────────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
dialog:
  id: secret.once
  secret-policy: reveal-once
  children:
    - warning-banner
    - masked-secret
    - reveal-action
    - copy-action
    - acknowledgement
    - close-action
```

The response must use `Cache-Control: no-store`, and the secret must disappear from the client state after dismissal.

---

## Screen 6 — Applications

### Purpose

Manage OIDC clients and resource servers.

### Display profiles

The UI may derive a friendly profile from explicit client primitives:

```text
Server-side web app
Browser SPA
Device client
Resource server
Custom
```

Do not persist this profile as authoritative client state. TinyIDP already treats client profiles as documentation conventions composed from explicit grant, scope, audience, PKCE, and introspection primitives.

### Columns

```text
Client ID
Derived profile
Public/confidential
Grant types
PKCE
Introspection
Status
Updated
```

### Filters

```text
Status
Public/confidential
Grant type
PKCE
Introspection
Audience
```

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Applications                       [Create app]   │
│ Users            │                                                   │
│ Invitations      │ [ Search client ID…                           ]   │
│ Applications     │ [Profile ▾] [Status ▾] [Grant ▾] [PKCE ▾]        │
│ Signing keys     │                                                   │
│ Activity         │ ┌───────────────────────────────────────────────┐ │
│ Operations       │ │ Client           Profile        Status       │ │
│                  │ ├───────────────────────────────────────────────┤ │
│                  │ │ message-desk     Web app        Active     ⋮ │ │
│                  │ │ admin-console    Web app        Active     ⋮ │ │
│                  │ │ message-cli      Device         Active     ⋮ │ │
│                  │ │ message-api      Resource srv.  Active     ⋮ │ │
│                  │ │ old-dashboard    Custom         Disabled   ⋮ │ │
│                  │ └───────────────────────────────────────────────┘ │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: clients.list
  scope: current
  requires: [clients.read]
  body:
    - page-header:
        actions:
          - command: client.create
    - filter-bar:
        fields: [query, profile, status, grant_type, pkce]
    - data-table:
        source: clients.list
        row-link: client.open
        row-actions:
          - client.disable
          - client.enable
    - cursor-pagination
```

---

## Screen 7 — Application detail and editor

### Tabs

```text
Overview
Redirects
Permissions
Credentials
Token policy
Activity
```

### Overview

```text
Client ID
Public/confidential
Status
Derived profile
Created
Updated
```

### Redirects

```text
Exact redirect URIs
Post-logout redirect URIs
```

Use repeatable URI editors. Every entry receives its own validation error.

### Permissions

```text
Allowed grant types
Allowed scopes
Allowed audiences
Can introspect
Require PKCE
```

### Token policy

```text
Access-token lifetime
ID-token lifetime
Refresh-token lifetime
```

### Credentials

For confidential clients:

```text
Secret configured: yes/no
Last rotated
Rotate secret
```

Never display the stored secret hash.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ message-desk                       Active ●       │
│ Users            │ Confidential web application       [Actions ▾]  │
│ Invitations      │                                                   │
│ Applications     │ Overview Redirects Permissions Credentials       │
│ Signing keys     │ Token policy Activity                             │
│ Activity         │ ────────────────────────────────────────────────  │
│ Operations       │                                                   │
│                  │ Redirect URIs                                     │
│                  │ ┌───────────────────────────────────────────────┐ │
│                  │ │ https://desk.example/auth/callback      [×] │ │
│                  │ │ https://desk.example/auth/secondary     [×] │ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │ [Add redirect URI]                                │
│                  │                                                   │
│                  │ Require PKCE                         Yes           │
│                  │ Grant types                          code, refresh │
│                  │                                                   │
│                  │                              [Discard] [Save]     │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Creation wizard

```text
1. Profile
2. Identity
3. Redirects and flows
4. Scopes and audiences
5. Token policy
6. Review
7. One-time secret
```

### Widget hierarchy

```yaml
screen:
  id: client.detail
  scope: current
  requires: [clients.read]
  body:
    - entity-header:
        source: client.summary
        actions:
          - client.disable
          - client.enable
          - client.secret.rotate
    - tabs:
        items:
          - overview
          - redirects
          - permissions
          - credentials
          - token-policy
          - activity
    - form:
        schema: client.update
        version-field: version
        submit:
          command: client.update
```

### Secret rotation behavior

The current service replaces the stored client secret hash when rotation succeeds; it does not implement a dual-secret overlap window.

The confirmation must therefore state:

```text
The existing secret stops working immediately.
Update the application before or immediately after completing this action.
```

---

## Screen 8 — Signing keys

### Purpose

Show signing trust state and make ordinary rotation understandable.

### Columns

```text
Key ID
State
Algorithm
Created
Not before
Not after
Current signing key
Published in JWKS
```

### States

```text
Active
Retired and published
Expired
```

### Actions

```text
Rotate signing key
Generate inactive key
Retire eligible key
```

Emergency purge remains CLI-only in the MVP because it can invalidate otherwise-valid tokens immediately.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Signing keys                        [Rotate key]  │
│ Users            │                                                   │
│ Invitations      │ Current signing key: rsa-20260701                 │
│ Applications     │                                                   │
│ Signing keys     │ ┌───────────────────────────────────────────────┐ │
│ Activity         │ │ Key ID          State      Created           │ │
│ Operations       │ ├───────────────────────────────────────────────┤ │
│                  │ │ rsa-20260701    Active     Jul 1, 2026      ⋮│ │
│                  │ │ rsa-20260401    Retired    Apr 1, 2026      ⋮│ │
│                  │ │ rsa-20260101    Retired    Jan 1, 2026      ⋮│ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Retired keys remain published during the         │
│                  │ verification overlap period.                      │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Rotate dialog

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Rotate signing key                                                   │
│                                                                      │
│ New key ID        [ rsa-20260723-143000                         ]   │
│ Reason            [ Scheduled key rotation                      ]   │
│                                                                      │
│ A new RSA key will become active. The current key will be retired    │
│ but remain available for verification.                               │
│                                                                      │
│ Fresh authentication is required.                                   │
│                                                                      │
│                              [Cancel] [Verify and rotate]             │
└──────────────────────────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: keys.list
  scope: system
  requires: [keys.read]
  body:
    - page-header:
        actions:
          - command: key.rotate
    - key-status-summary:
        source: keys.summary
    - data-table:
        source: keys.list
        row-actions:
          - key.retire
```

---

## Screen 9 — Administrative activity

### Purpose

Provide a queryable record of administrative activity.

Call this screen **Activity** in the MVP rather than implying that it contains every OIDC authentication event.

### Events

```text
User created
User updated
User disabled/enabled
Password changed
Account unlocked
Access revoked
Invitation issued/revoked
Client created/updated/disabled
Client secret rotated
Key generated/rotated/retired
Doctor run
Backup created/verified
Diagnostics exported
Admin sign-in accepted/rejected
```

### Columns

```text
Time
Actor
Action
Target
Result
Reason
Request ID
```

The actor column remains useful even though the MVP has one owner.

### Filters

```text
Date
Action category
Target type
Result
Request ID
```

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Activity                                          │
│ Users            │                                                   │
│ Invitations      │ [Date ▾] [Category ▾] [Result ▾]                 │
│ Applications     │ [ Search target or request ID…                ]   │
│ Signing keys     │                                                   │
│ Activity         │ ┌───────────────────────────────────────────────┐ │
│ Operations       │ │ Time   Action               Target    Result │ │
│                  │ ├───────────────────────────────────────────────┤ │
│                  │ │ 10:42  user.disable         alex      Done   │ │
│                  │ │ 10:38  user.access.revoke   alex      Done   │ │
│                  │ │ 09:18  client.create        dashboard Done   │ │
│                  │ │ 08:51  admin.login          owner     Denied │ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Request IDs are available in event details.       │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: activity.list
  scope: current
  requires: [activity.read]
  body:
    - page-header
    - filter-bar:
        fields: [date, category, target_type, result, query]
    - data-table:
        source: activity.list
        row-action: activity.open
    - detail-drawer:
        source: activity.selected
    - cursor-pagination
```

---

## Screen 10 — Operations

### Purpose

Show installation health and expose safe operational commands.

### Sections

```text
Readiness
Doctor checks
Audit delivery
Database and schema
Backups
Diagnostics
Build information
```

### Web-exposed actions

```text
Run doctor checks
Create backup
Verify managed backup
Download sanitized diagnostics
```

### CLI-only initially

```text
Restore backup
Run migrations
Purge signing key
Change issuer
Change token secret
Change audit path
Change backup root
```

### Managed backup design

Do not present a free-form server filesystem path field.

Configure a server-side backup root:

```yaml
admin:
  backup_root: /var/lib/tinyidp/backups
```

The UI asks only for an optional label. The backend generates and validates the final filename beneath that root.

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ This installation                                      Owner     ▾  │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Operations                         [Run checks]   │
│ Users            │                                                   │
│ Invitations      │ Overall status                         Healthy ●  │
│ Applications     │                                                   │
│ Signing keys     │ ┌───────────────────────────────────────────────┐ │
│ Activity         │ │ Schema                 Current · version 12  │ │
│ Operations       │ │ Configured clients     Valid                 │ │
│                  │ │ Active signing key     rsa-20260701          │ │
│                  │ │ Verification keys      3 published           │ │
│                  │ │ Audit delivery         Healthy               │ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Backups                          [Create backup]   │
│                  │ Jul 22  tinyidp-20260722.db      Verified         │
│                  │ Jul 15  tinyidp-20260715.db      Verified         │
│                  │                                                   │
│                  │ Diagnostics                 [Download report]     │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: operations
  scope: system
  requires: [operations.read]
  body:
    - page-header:
        actions:
          - command: operations.doctor
    - health-check-list:
        source: operations.health
    - backup-panel:
        source: backups.list
        actions:
          - backup.create
          - backup.verify
    - diagnostics-panel:
        actions:
          - diagnostics.export
    - build-information:
        source: operations.build
```

---

# 8. Shared interaction patterns

## 8.1 Destructive confirmation

Every destructive command receives:

* Target identity.
* Impact summary.
* Required reason.
* Expected resource version.
* Fresh-auth requirement where appropriate.
* Opaque action handle.

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Disable Alex Morgan?                                                 │
│                                                                      │
│ The user will be unable to authenticate. Existing security           │
│ artifacts will be revoked.                                          │
│                                                                      │
│ Reason *                                                             │
│ [ Account no longer requires access                              ]   │
│                                                                      │
│ Type DISABLE to continue                                             │
│ [                                                                ]   │
│                                                                      │
│                                  [Cancel] [Disable user]              │
└──────────────────────────────────────────────────────────────────────┘
```

```yaml
dialog:
  id: destructive.confirm
  children:
    - target-summary
    - impact-summary
    - reason-field
    - typed-confirmation
    - command-submit:
        action-handle: bound
        expected-version: bound
```

## 8.2 Fresh-auth dialog

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Verify it is you                                                     │
│                                                                      │
│ Rotating this client secret requires a recent authentication.        │
│                                                                      │
│                        [Continue to sign in]                          │
│                                                                      │
│ You will return to this operation after verification.                │
└──────────────────────────────────────────────────────────────────────┘
```

## 8.3 Stale-resource state

```text
This record changed after you opened it.

Changed by: Owner
Changed at: 14:32

[Review current version]
```

Never silently overwrite concurrent changes, even in a one-owner installation. Multiple tabs, CLI activity, and automation can still conflict.

## 8.4 Result semantics

Use these distinct results:

```text
Completed
Rejected
Could not start
State changed; review required
Completed, but audit delivery is degraded
```

That last state matters because the current audit contract explicitly permits a mutation to have committed before audit delivery fails.

---

# 9. Tables, filters, and tablet behavior

## 9.1 Tables

Every table should support:

```text
Server-side filtering
Stable sort
Cursor pagination
Column headers that remain visible
Keyboard row navigation
Explicit empty state
Explicit error state
Row action menu
URL-persisted filters
```

Do not add bulk selection in the first MVP. Bulk destructive operations introduce significantly more authorization, stale-state, review, and recovery complexity.

## 9.2 Filter behavior

Filter state should use typed URL parameters:

```text
/admin/users?status=locked&email=verified
```

Avoid storing filter state exclusively in browser memory.

## 9.3 Tablet layout

At approximately 768–1024 pixels:

* Sidebar becomes a navigation drawer.
* Page title and primary action remain visible.
* Filter bar collapses into a “Filters” button and sheet.
* Tables show two or three primary fields.
* Rows expand to show secondary fields.
* Detail drawers become full-screen sheets.
* Forms become one column.
* Tabs become horizontally scrollable.
* Destructive actions remain inside an overflow menu.
* The effective scope remains visible once domains exist.

## 9.4 Small-screen behavior

Phones are not the primary administration target, but emergency read and disable flows should remain usable. High-complexity forms such as OIDC client creation may show:

```text
For safer editing, use a larger screen.
```

They should not become inaccessible solely because of screen size.

---

# 10. MVP backend architecture

## 10.1 Package structure

```text
pkg/idpadmin/
  principal.go
  scope.go
  capability.go
  authorizer.go
  commands.go
  queries.go
  types.go

pkg/idpadminstore/
  interfaces.go
  sqlite/
    queries.go
    actions.go
    grants.go
    sessions.go
    invitations.go

pkg/adminui/
  schema/
  registry/
  compiler/
  model/
  renderer/

internal/adminweb/
  auth.go
  session.go
  csrf.go
  scope.go
  routes.go
  screen_assembler.go
  actions.go

internal/admin/
  existing low-level operator primitives
```

The CLI and web UI should both call `pkg/idpadmin` application services. The web UI must not invoke Cobra commands or parse CLI output.

## 10.2 Principal and scope

```go
type ScopeKind string

const (
    ScopeSystem ScopeKind = "system"
    ScopeDomain ScopeKind = "domain"
)

type AdminScope struct {
    Kind ScopeKind
    ID   string
}

type AdminPrincipal struct {
    Subject      string
    SessionID    string
    AuthTime     time.Time
    Assurance    string
    GrantVersion uint64
}

type Capability string
```

The MVP permits only:

```go
AdminScope{
    Kind: ScopeSystem,
    ID:   "system",
}
```

No command is defined without a scope argument.

## 10.3 Query service

```go
type QueryService interface {
    Overview(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
    ) (Overview, error)

    ListUsers(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
        filter UserFilter,
        page PageRequest,
    ) (Page[UserRow], error)

    GetUser(
        ctx context.Context,
        principal AdminPrincipal,
        scope AdminScope,
        userID string,
    ) (UserDetail, error)

    ListInvitations(...)
    ListClients(...)
    GetClient(...)
    ListKeys(...)
    ListActivity(...)
    GetOperations(...)
}
```

## 10.4 Command service

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
    CreateUser(ctx context.Context, cmd CommandContext, req CreateUser) (UserResult, error)
    UpdateUser(ctx context.Context, cmd CommandContext, req UpdateUser) (UserResult, error)
    SetUserDisabled(ctx context.Context, cmd CommandContext, userID string, disabled bool) error
    SetUserPassword(ctx context.Context, cmd CommandContext, req SetPassword) error
    UnlockUser(ctx context.Context, cmd CommandContext, userID string) error
    RevokeUserAccess(ctx context.Context, cmd CommandContext, userID string) error

    IssueInvitation(...)
    RevokeInvitation(...)

    CreateClient(...)
    UpdateClient(...)
    SetClientDisabled(...)
    RotateClientSecret(...)

    RotateSigningKey(...)
    RetireSigningKey(...)

    CreateBackup(...)
    VerifyBackup(...)
}
```

## 10.5 Admin database records

### Grants

```text
admin_grants
  id
  actor_subject
  scope_kind
  scope_id
  role
  capabilities_json
  version
  issued_at
  expires_at
  revoked_at
```

The MVP has one row:

```text
owner subject
system scope
owner role
all MVP capabilities
```

### Sessions

```text
admin_sessions
  id_hash
  actor_subject
  grant_id
  grant_version
  auth_time
  assurance
  created_at
  last_seen_at
  expires_at
  revoked_at
```

### Administrative actions

```text
admin_actions
  id
  request_id
  actor_subject
  admin_session_id
  scope_kind
  scope_id
  capability
  command
  target_type
  target_id
  expected_version
  outcome
  reason_code
  operator_reason
  assurance
  created_at
```

### Action nonces

```text
admin_action_nonces
  nonce_hash
  actor_subject
  session_id
  scope_kind
  scope_id
  command
  target_id
  expected_version
  expires_at
  consumed_at
```

## 10.6 Action handles

The browser should submit an opaque action handle rather than an authoritative command description.

The server binds the handle to:

```text
Actor subject
Admin session
Scope
Capability
Command
Target
Expected resource version
Required assurance
Expiry
Nonce
```

Browser submission contains:

```json
{
  "action_handle": "opaque-value",
  "csrf_token": "opaque-value",
  "input": {
    "reason": "Account no longer requires access"
  }
}
```

The browser cannot turn a `user.open` action into `user.disable`.

## 10.7 Read model

Do not force pagination and filtering into the protocol-oriented `idpstore.Store`.

Add an admin read model that can denormalize safe fields:

```text
User ID
Login
Name
Normalized email
Disabled state
Email verification
Locked-until timestamp
Last successful login
Created and updated timestamps
Future home-domain ID
```

The source user and security records remain authoritative. The read model exists to support bounded, indexed queries.

## 10.8 Transactional activity and audit outbox

For SQLite-backed mutations:

```text
BEGIN
  authorize again inside transaction where needed
  check expected version
  mutate resource
  insert admin action
  insert audit outbox entry
COMMIT
```

A delivery worker or synchronous post-commit dispatcher writes the external audit event. If delivery fails, the committed outbox record remains available for reconciliation.

This removes ambiguity from the web UI while preserving the existing durable audit sink.

## 10.9 Operations outside the main database

A backup action cannot be committed atomically with a filesystem copy. Use an operation record:

```text
operations
  id
  actor_subject
  command
  state
  started_at
  completed_at
  result_metadata
  error_code
```

States:

```text
requested
running
completed
failed
```

Only sanitized paths relative to the configured backup root appear in UI output.

---

# 11. Widget runtime architecture

TinyIDP’s interaction renderer already establishes a useful boundary: presentation receives a complete derived page model but not the request, cookies, password, raw OAuth parameters, or authorization state. HTML changes cannot authorize an OAuth interaction.

Use the same principle for the admin console.

```text
Static reviewed DSL
        │
        ▼
Screen compiler
        │
        ▼
Registered screen plan
        │
        ├── Query registry
        ├── Form-schema registry
        ├── Command registry
        └── Capability registry
        │
        ▼
Sanitized ScreenModel + opaque action handles
        │
        ▼
Browser widget renderer
```

## 11.1 Source DSL

```yaml
version: tinyidp.admin.ui/v1

screen:
  id: users.list
  route: /admin/users
  scope: current
  requires: [users.read]

  body:
    - type: page-header
      title: Users
      actions:
        - command: user.create
          label: Add user

    - type: filter-bar
      fields:
        - query
        - status
        - email_state
        - last_sign_in

    - type: data-table
      source: users.list
      columns:
        - name
        - login
        - email
        - status
        - last_sign_in
      row_actions:
        - user.open
        - user.disable
        - user.enable
```

## 11.2 Runtime screen model

The browser receives something closer to:

```json
{
  "screen_id": "users.list",
  "scope": {
    "kind": "system",
    "label": "This installation"
  },
  "widgets": [
    {
      "type": "data-table",
      "rows": [
        {
          "ref": "usr_8F2",
          "name": "Alex Morgan",
          "login": "alex",
          "status": "active",
          "actions": [
            {
              "label": "Disable",
              "handle": "opaque-bound-action"
            }
          ]
        }
      ]
    }
  ]
}
```

No capability or domain authority is inferred from that JSON.

## 11.3 Admin CSP

The login and consent renderer can prohibit scripts entirely. The richer administration console will likely require bundled JavaScript, so it needs a separate reviewed policy:

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

Requirements:

* No inline scripts.
* No `eval`.
* No third-party JavaScript.
* No remote fonts.
* No analytics injection.
* No CDN assets.
* No arbitrary external API requests.
* All interactive code bundled with the application.

---

# 12. RAG and evaluation-system workflow

The RAG system should assist design and verification, not runtime authorization.

## 12.1 Retrieval corpus

Index:

```text
Widget DSL schema
Widget catalog
Command registry
Capability registry
Form schemas
Threat model
Screen requirements
Accessibility standards
Responsive rules
Existing approved screens
TinyIDP storage and security contracts
Known anti-patterns
```

## 12.2 Candidate-generation workflow

```text
UX requirement
    ↓
Retrieve relevant patterns and contracts
    ↓
Generate candidate widget YAML
    ↓
Deterministic schema validation
    ↓
Security and scope evaluation
    ↓
Accessibility evaluation
    ↓
Responsive rendering
    ↓
Browser interaction tests
    ↓
Human review
    ↓
Versioned compiled artifact
```

## 12.3 Deterministic evaluators

Every DSL artifact should be rejected unless:

* Every source is registered.
* Every command is registered.
* Every command declares a capability.
* Every source inherits a scope.
* Dangerous commands declare confirmation.
* Sensitive commands declare fresh-auth or AAL requirements.
* Secret widgets use the one-time-secret type.
* No raw password or secret fields appear in table or timeline widgets.
* Every table has loading, empty, no-results, and error states.
* Every form has field and form-level error presentations.
* Every destructive form has a reason.
* Every mutable entity includes an expected version.
* No arbitrary URL, script, or HTML value appears.

## 12.4 Scenario evaluators

```text
User list with zero users
User list with 10,000 users
Long Unicode names
Missing email
Locked and disabled user
Client with 20 redirect URIs
Client with malformed proposed URI
Expired invitation
Retired signing key
Doctor failure
Audit-delivery degradation
Stale edit form
Expired admin session
Fresh-auth interruption
Tablet width
Keyboard-only use
Screen-reader navigation
```

## 12.5 Security evaluation cases

```text
Change an action handle’s target
Replay an action handle
Submit after owner grant revocation
Submit after resource version changes
Submit without CSRF token
Use an action from another session
Insert a hidden unsupported form field
Inject a client ID containing markup
Try to display a stored secret hash
Use a future domain identifier in MVP mode
Attempt self-grant changes through a fabricated command
```

---

# 13. Future multitenant extension

## 13.1 New data model

```text
identity_domains
  id
  slug
  display_name
  status
  policy_version
  version
  created_at
  updated_at

identity_domain_users
  user_id
  domain_id
  status
  version
  assigned_at
  assigned_by

domain_aliases
  id
  domain_id
  dns_name
  verification_status
  verification_evidence
  verified_at

domain_policies
  domain_id
  policy_json
  version
  updated_by
  updated_at
```

`identity_domain_users` is the authorization boundary. `User.Tenant` remains an OIDC claim.

## 13.2 Administrator grants

The existing MVP grant table is extended with domain-scoped rows:

```yaml
actor_subject: user-domain-admin
scope:
  kind: domain
  id: dom_acme
role: domain_admin
```

No schema replacement is required.

## 13.3 Initial roles

| Capability area        | Owner / mega admin |           Domain admin |      Helpdesk | Auditor |
| ---------------------- | -----------------: | ---------------------: | ------------: | ------: |
| View domain users      |                Yes |                    Yes |           Yes |     Yes |
| Create/invite users    |                Yes |                    Yes |            No |      No |
| Edit user profile      |                Yes |                    Yes |       Limited |      No |
| Disable/reactivate     |                Yes |                    Yes |       Limited |      No |
| Set password           |                Yes |       Policy-dependent | Initiate only |      No |
| Unlock                 |                Yes |                    Yes |           Yes |      No |
| Revoke access          |                Yes |                    Yes |           Yes |      No |
| Manage domain admins   |                Yes | Yes, no self-elevation |            No |      No |
| View domain activity   |                Yes |                    Yes |       Limited |     Yes |
| Manage clients         |                Yes |           No initially |            No |      No |
| Manage signing keys    |                Yes |                     No |            No |      No |
| Backups and schema     |                Yes |                     No |            No |      No |
| Create/suspend domains |                Yes |                     No |            No |      No |

## 13.4 Future navigation

### Mega administrator

```text
Overview
Domains
Users
Invitations
Administrators
Applications
Security
Activity
Operations
```

### Domain administrator

```text
Overview
Users
Invitations
Administrators
Security
Activity
Settings
```

The navigation is generated from capabilities. It is not a hard-coded “MVP menu” versus “multitenant menu.”

---

# 14. Future-only screens

## Screen M1 — Domains

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ Scope: All domains                                      Owner     ▾ │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Domains                           [Create domain]  │
│ Domains          │                                                   │
│ Users            │ [ Search domains…                             ]   │
│ Invitations      │ [Status ▾] [Alias verification ▾] [Findings ▾]  │
│ Administrators   │                                                   │
│ Applications     │ ┌───────────────────────────────────────────────┐ │
│ Security         │ │ Domain        Users  Admins Status  Findings │ │
│ Activity         │ ├───────────────────────────────────────────────┤ │
│ Operations       │ │ Acme Corp      412     4   Active      0    ⋮│ │
│                  │ │ Globex         203     2   Active      1    ⋮│ │
│                  │ │ Initech         71     1   Warning     2    ⋮│ │
│                  │ │ Umbrella         0     1   Suspended   —    ⋮│ │
│                  │ └───────────────────────────────────────────────┘ │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: domains.list
  scope: system
  requires: [domains.read]
  body:
    - page-header:
        actions:
          - command: domain.create
    - filter-bar:
        fields: [query, status, alias_state, findings]
    - data-table:
        source: domains.list
        row-link: domain.open
        row-actions:
          - domain.suspend
          - domain.reactivate
```

---

## Screen M2 — Domain workspace

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ Scope: Acme Corporation                                  Owner    ▾ │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Acme Corporation                    Active ●      │
│ Users            │ 412 users · 4 administrators         [Actions ▾] │
│ Invitations      │                                                   │
│ Administrators   │ Overview Users Administrators Invitations         │
│ Security         │ Activity Settings                                 │
│ Activity         │ ────────────────────────────────────────────────  │
│ Settings         │                                                   │
│                  │ ┌───────────┐ ┌───────────┐ ┌───────────┐        │
│                  │ │ Active    │ │ Pending   │ │ Locked    │        │
│                  │ │ users 405 │ │ invites 8 │ │ users  3  │        │
│                  │ └───────────┘ └───────────┘ └───────────┘        │
│                  │                                                   │
│                  │ Security posture                                  │
│                  │ Verified aliases       acme.example               │
│                  │ Admin assurance        4 of 4 compliant           │
│                  │ Session policy         12h maximum / 30m idle     │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: domain.workspace
  scope: current-domain
  requires: [domain.read]
  body:
    - domain-header
    - domain-navigation
    - metric-grid:
        source: domain.metrics
    - policy-summary:
        source: domain.policy.summary
    - activity-list:
        source: domain.activity.recent
```

---

## Screen M3 — Domain administrators

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ Scope: Acme Corporation                              Domain admin ▾ │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Administrators                    [Invite admin]  │
│ Users            │                                                   │
│ Invitations      │ [Role ▾] [Status ▾] [Assurance ▾] [Expires ▾]   │
│ Administrators   │                                                   │
│ Security         │ ┌───────────────────────────────────────────────┐ │
│ Activity         │ │ Administrator Role          Status  Expires  │ │
│ Settings         │ ├───────────────────────────────────────────────┤ │
│                  │ │ Dana Miles   Domain admin   Active  Never   ⋮│ │
│                  │ │ Kai Liu      Helpdesk       Active  Aug 30  ⋮│ │
│                  │ │ Sam Ortiz    Auditor        Warning Jul 28  ⋮│ │
│                  │ └───────────────────────────────────────────────┘ │
│                  │                                                   │
│                  │ Administrators cannot elevate their own grant.    │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: administrators.list
  scope: current-domain
  requires: [administrators.read]
  body:
    - page-header:
        actions:
          - command: administrator.invite
    - filter-bar:
        fields: [role, status, assurance, expiry]
    - data-table:
        source: administrators.list
        row-actions:
          - administrator.role.change
          - administrator.grant.revoke
          - administrator.sessions.revoke
```

---

## Screen M4 — Domain settings

### ASCII

```text
┌ TinyIDP Console ─────────────────────────────────────────────────────┐
│ Scope: Acme Corporation                              Domain admin ▾ │
├──────────────────┬───────────────────────────────────────────────────┤
│ Overview         │ Domain settings                     [Save]        │
│ Users            │                                                   │
│ Invitations      │ General                                           │
│ Administrators   │ Display name   [ Acme Corporation             ]  │
│ Security         │ Support email  [ identity@acme.example        ]  │
│ Activity         │                                                   │
│ Settings         │ Signup                                            │
│                  │ ( ) Open                                          │
│                  │ (●) Invitation only                               │
│                  │ ( ) Administrator created                         │
│                  │ [✓] Require verified email                        │
│                  │                                                   │
│                  │ Sessions                                          │
│                  │ Idle timeout       [30 minutes                ▾]  │
│                  │ Maximum lifetime  [12 hours                  ▾]   │
│                  │                                                   │
│                  │ Administrator security                            │
│                  │ Required assurance [AAL2                     ▾]   │
└──────────────────┴───────────────────────────────────────────────────┘
```

### Widget hierarchy

```yaml
screen:
  id: domain.settings
  scope: current-domain
  requires: [domain.settings.read]
  body:
    - page-header
    - form:
        schema: domain.settings
        version-field: version
        sections:
          - general
          - aliases
          - signup
          - sessions
          - administrator-security
        submit:
          command: domain.settings.update
```

---

# 15. How MVP screens evolve under multitenancy

| MVP screen         | Future behavior                                               |
| ------------------ | ------------------------------------------------------------- |
| Overview           | Metrics become scope-sensitive                                |
| Users              | Domain column appears only in system scope                    |
| User detail        | Shows home domain and domain assignment history               |
| Create user        | Requires domain in system scope; inferred in domain scope     |
| Invitations        | Adds domain and optional recipient binding                    |
| Applications       | Remains system-scoped initially                               |
| Application detail | Remains system-scoped initially                               |
| Signing keys       | Always system-scoped                                          |
| Activity           | Domain filter appears for mega admin                          |
| Operations         | Always system-scoped                                          |
| Top bar            | Scope switcher appears when more than one scope is accessible |
| Sidebar            | Domains and Administrators appear based on capabilities       |

The widget IDs and command names remain unchanged.

For example:

```yaml
screen:
  id: users.list
  scope: current
```

works in both:

```text
System scope: list all users
Domain scope: list only users assigned to that domain
```

---

# 16. Multitenant migration path

## Step 1 — Ship the MVP architecture

Persist from day one:

```text
System owner grant
System-scoped admin sessions
System-scoped action records
Scope on every query and command
Globally unique resource IDs
Optimistic versions
```

No identity-domain tables are necessary yet.

## Step 2 — Add dormant domain tables

Deploy:

```text
identity_domains
identity_domain_users
domain_aliases
domain_policies
domain-scoped admin grants
```

Keep the UI in single-install mode.

## Step 3 — Create the initial domain

When multitenancy is enabled:

```text
Create one default identity domain
Assign all existing users to it
Assign the owner a domain-admin grant in addition to the system-owner grant
```

The default domain receives a generated immutable ID. Its display name can be changed later.

## Step 4 — Preserve identity and protocol state

Do not change:

```text
User IDs
OIDC subjects
Logins
Password credentials
Sessions
Grants
Authorization codes
Access tokens
Refresh tokens
Client IDs
Signing keys
```

No token invalidation should be necessary merely to add domain assignment.

## Step 5 — Handle active invitations

Before enabling domain-scoped signup:

* Let old system-scoped invitations expire, or
* Explicitly assign them to the initial domain by policy.

Do not infer a domain from `User.Tenant` or the OIDC client without an explicit migration rule.

## Step 6 — Enable capability-driven navigation

Once domain data exists:

* Add the scope switcher.
* Add Domains.
* Add Administrators.
* Add domain settings.
* Allow domain-scoped user queries and commands.
* Keep system-only screens unchanged.

---

# 17. Architecture traps to avoid

| Avoid                                                        | Use instead                                   |
| ------------------------------------------------------------ | --------------------------------------------- |
| A separate single-tenant UI                                  | One scope-aware UI                            |
| A fake tenant exposed in the MVP                             | System scope                                  |
| Using `User.Tenant` as authorization                         | `identity_domain_users` mapping               |
| Trusting roles in an ID token forever                        | Current server-side admin grants              |
| Handler-level tenant checks after global lookup              | Scope-aware service and query contracts       |
| Wrapping Cobra commands in HTTP handlers                     | Shared application service                    |
| Free-form DSL HTTP requests                                  | Registered sources and commands               |
| Raw domain IDs supplied as trusted form input                | Server-resolved scope                         |
| Persisting a client-profile enum                             | Derive profile from client primitives         |
| Calling a direct-created password temporary                  | Permanent password or invitation flow         |
| Listing secrets after creation                               | One-time reveal                               |
| Silent overwrite on save                                     | Expected version                              |
| Arbitrary backup path input                                  | Configured backup root                        |
| Browser bootstrap of first owner                             | CLI bootstrap                                 |
| Full key-purge UI                                            | CLI break-glass command                       |
| A sessions tab without reliable inventory                    | Revoke-all command until inventory exists     |
| Treating the append-only audit file as a searchable database | Queryable admin action projection             |
| Returning “failed” after a committed mutation                | Reconcile and report committed/audit-degraded |

---

# 18. Suggested implementation sequence

## Phase A — Control-plane foundation

Deliver:

```text
AdminScope
AdminPrincipal
Capability registry
Authorizer
Owner grant table
Admin session table
Admin action table
Action-handle service
Dedicated admin-console OIDC client
CSRF and fresh-auth handling
Widget registry and compiler
```

No resource mutations through the web yet.

## Phase B — Read-only console

Deliver:

```text
Access screen
Application shell
Overview
User directory
User detail
Applications list/detail
Signing keys list
Doctor and readiness
Activity list
```

This phase validates authentication, read scoping, table UX, DSL rendering, and audit visibility.

## Phase C — User operations

Deliver:

```text
Create user
Edit profile
Enable/disable
Unlock
Set password
Revoke all access
Confirmations
Expected-version checks
Fresh-auth flows
```

## Phase D — Invitations and applications

Deliver:

```text
Invitation metadata projection
Issue/list/revoke invitation
One-time invitation reveal
Create client
Edit client
Enable/disable client
Rotate secret
One-time client-secret reveal
```

## Phase E — Keys and operations

Deliver:

```text
Rotate signing key
Retire eligible key
Managed backup creation
Managed backup verification
Diagnostics export
Doctor result details
```

Retain restore, migrations, and emergency key purge as CLI operations.

## Phase F — UX and evaluation hardening

Deliver:

```text
Tablet layouts
Keyboard navigation
Accessibility checks
Empty/error/stale states
Security scenario suite
Widget DSL deterministic evaluators
Browser-level regression fixtures
Visual snapshots
RAG-assisted candidate generation
Human approval workflow
```

---

# 19. MVP definition of done

The first polished single-owner release is complete when:

1. The owner authenticates through a dedicated admin-console OIDC client.
2. The owner grant is server-side and can be revoked through CLI.
3. The admin session is separate from the IdP browser session.
4. Every query and command carries `AdminScope`, even though only system scope exists.
5. Every mutation checks capability, scope, CSRF, action handle, and expected version.
6. Users can be listed, searched, viewed, created, edited, disabled, enabled, unlocked, assigned a new password, and signed out everywhere.
7. Signup invitations can be issued, listed, revoked by public ID, and revealed only once.
8. OIDC clients can be created, inspected, edited, enabled, disabled, and have secrets rotated.
9. Signing keys can be inspected and rotated safely.
10. Doctor checks, readiness, audit health, backups, and diagnostics are visible.
11. Administrative actions are queryable and include actor, command, target, reason, result, scope, assurance, and request ID.
12. Backup restore, migrations, and emergency key purge remain outside the browser.
13. The widget DSL contains no arbitrary code, URL, SQL, authorization, or secret values.
14. Every screen has loading, empty, no-results, permission, stale, expired-session, and safe-failure states.
15. The interface is usable at desktop and tablet widths.
16. Multitenancy can be introduced by adding domain data, grants, and navigation without replacing the MVP queries, commands, routes, widgets, or admin sessions.

The central design rule is:

> **The MVP is system-scoped, not tenant-unaware.**

That distinction provides a polished console for the current installation while preserving a clean path to delegated customer administration.

