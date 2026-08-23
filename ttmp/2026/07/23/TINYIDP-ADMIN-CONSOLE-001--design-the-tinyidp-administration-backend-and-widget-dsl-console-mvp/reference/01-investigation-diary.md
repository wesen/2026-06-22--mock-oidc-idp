---
Title: Investigation diary
Ticket: TINYIDP-ADMIN-CONSOLE-001
Status: review
Topics:
    - backend
    - identity
    - auth
    - architecture
    - security
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: abs:///home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/app/App.tsx
      Note: Renderer transport inspected during frontend design
    - Path: abs:///home/manuel/code/wesen/go-go-golems/upwork/verbs/upwork.js
      Note: Reference implementation inspected during research
    - Path: repo://.devctl.yaml
      Note: Profile selection and plugin registration added in control-plane interval
    - Path: repo://dev/environments/admin-console.yaml
      Note: Administration secret file contract
    - Path: repo://dev/environments/shared-two-apps.yaml
      Note: Representative production-shaped local environment manifest
    - Path: repo://devctl/lib/manifests.py
      Note: Strict environment manifest loader and invariants added in commit 655cd80
    - Path: repo://devctl/operations.py
      Note: Vault materialization and guarded Caddy recovery implementation (commit e3b0d5d)
    - Path: repo://devctl/tests/test_operations.py
      Note: Atomicity, validation, CAS, and Caddy archive evidence
    - Path: repo://devctl/tests/test_plugin.py
      Note: Protocol, dry-run, and utility-profile contract evidence
    - Path: repo://devctl/tinyidp.py
      Note: NDJSON protocol-v2 environment plugin added in commit 655cd80
    - Path: repo://go.mod
      Note: Pins the required rag-evaluation-system Widget DSL provider
    - Path: repo://go.sum
      Note: Checksums Widget DSL and resolved dependency versions
    - Path: repo://internal/admin/service.go
      Note: Evidence inspected during current-state research
    - Path: repo://internal/adminweb/auth.go
      Note: OIDC PKCE, encrypted auth attempt, admin session, current-grant, and CSRF boundary
    - Path: repo://internal/adminweb/auth_test.go
      Note: Login, callback, cookie, session, CSRF, return-path, and grant invalidation tests
    - Path: repo://internal/adminweb/frontend/src/ClientManagement.tsx
      Note: Application administration and one-time secret UI
    - Path: repo://internal/adminweb/frontend/src/InvitationManagement.tsx
      Note: Invitation administration and one-time code UI
    - Path: repo://internal/adminweb/frontend/tests/admin-console.spec.ts
      Note: Phase F browser acceptance suite and fixture matrix (commit 972a0a3)
    - Path: repo://internal/adminweb/handler_test.go
      Note: HTTP CSP, no-store, unknown-field, origin, CSRF, and body-bound evidence (commit 972a0a3)
    - Path: repo://internal/adminweb/widget_runtime.go
      Note: Deterministic Widget IR release validator (commit 972a0a3)
    - Path: repo://internal/cmds/admin.go
      Note: Registers the console operator command group
    - Path: repo://internal/cmds/admin_client.go
      Note: Shared guarded client CLI
    - Path: repo://internal/cmds/admin_console.go
      Note: Glazed console owner lifecycle commands
    - Path: repo://internal/cmds/admin_console_test.go
      Note: Glazed command wiring and lifecycle tests
    - Path: repo://internal/cmds/admin_invitation.go
      Note: Shared guarded invitation CLI
    - Path: repo://internal/cmds/serve_production.go
      Note: Evidence for public and internal listener boundaries
    - Path: repo://pkg/idp/audit.go
      Note: Evidence for post-commit delivery semantics
    - Path: repo://pkg/idpadmin/action_handle.go
      Note: Short-lived signed and session-bound action authority
    - Path: repo://pkg/idpadmin/contracts.go
      Note: Safe query and command application contracts and DTOs
    - Path: repo://pkg/idpadmin/contracts_test.go
      Note: Pagination and secret-exposure contract tests
    - Path: repo://pkg/idpadmin/model.go
      Note: Administration scopes, capabilities, principals, grants, and server-side authorization
    - Path: repo://pkg/idpadmin/model_test.go
      Note: Authorization and action-handle security tests
    - Path: repo://pkg/idpadminapp/client_commands.go
      Note: Guarded client lifecycle and one-time secret rotation
    - Path: repo://pkg/idpadminapp/commands.go
      Note: Closed signed-handle command dispatcher
    - Path: repo://pkg/idpadminapp/executor.go
      Note: Atomic authorized mutation, replay, CAS, idempotency, evidence, and outbox pipeline
    - Path: repo://pkg/idpadminapp/executor_test.go
      Note: Mutation atomicity, rollback, retry, conflict, and one-time-secret tests
    - Path: repo://pkg/idpadminapp/invitation_client_commands_test.go
      Note: Phase D lifecycle leakage replay and validation evidence
    - Path: repo://pkg/idpadminapp/invitation_commands.go
      Note: Guarded invitation issuance and public-ID revocation
    - Path: repo://pkg/idpadminapp/owner.go
      Note: Owner bootstrap, status, grant revocation, and session recovery orchestration
    - Path: repo://pkg/idpadminapp/owner_test.go
      Note: Owner lifecycle and fixed-client invariant tests
    - Path: repo://pkg/idpadminstore/store.go
      Note: Control-plane persistence and cross-domain transaction contracts
    - Path: repo://pkg/idpstore/interfaces.go
      Note: Evidence for point queries and atomic mutations
    - Path: repo://pkg/sqlitestore/admin_projection.go
      Note: Canonical user projection rebuild and drift checker
    - Path: repo://pkg/sqlitestore/admin_queries_benchmark_test.go
      Note: 10,000-user query benchmark (commit 972a0a3)
    - Path: repo://pkg/sqlitestore/admin_store.go
      Note: SQLite implementation of administration security state and atomic updates
    - Path: repo://pkg/sqlitestore/admin_store_test.go
      Note: Grant, rollback, replay, and CAS persistence tests
    - Path: repo://pkg/sqlitestore/migrations/016_admin_control_plane.sql
      Note: Durable administration control-plane schema
    - Path: repo://pkg/sqlitestore/migrations/017_admin_user_projection.sql
      Note: Indexed administration user query projection
    - Path: repo://pkg/sqlitestore/migrations/019_admin_invitation_lookup.sql
      Note: Public invitation ID lookup migration and backfill
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/scripts/devctl-lifecycle-plugin.py
      Note: Minimal NDJSON HTTP service plugin used by the full CLI reproduction
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/scripts/reproduce-devctl-up-lifecycle.sh
      Note: Full devctl up parent-exit lifecycle harness
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/scripts/reproduce-devctl-wrapper-lifecycle.sh
      Note: Black-box wrapper lifecycle and explicit SIGHUP harness
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/sources/local/tiny-idp-ux.md
      Note: Imported source read in full
    - Path: ws://devctl/cmd/devctl/cmds/wrap_service.go
      Note: Signal-forwarding implementation fixed in devctl commit 39ba416
    - Path: ws://devctl/cmd/devctl/cmds/wrap_service_test.go
      Note: Command-level regression for wrapper self-signaling
ExternalSources: []
Summary: Chronological research, design, validation, and delivery record for the TinyIDP administration console MVP ticket.
LastUpdated: 2026-07-23T20:14:57.931345362-04:00
WhatFor: Preserve how the administration-backend proposal was derived, including concrete evidence, failed assumptions, and review instructions.
WhenToUse: Read when reviewing the design, implementing a phase, or continuing the investigation.
---










# Investigation diary

## Goal

Capture the evidence-first design of a production TinyIDP administration backend and `widget.dsl` v3 console, from ticket creation through reMarkable delivery. The diary records what was inspected, why the final boundaries were selected, what failed during discovery, and how a new engineer should validate the conclusions.

## Step 1: Create the ticket and map the evidence

The first step created a distinct administration-console ticket rather than reusing `TINYIDP-ADMIN-001`, which already documents the operator CLI. The supplied 3,015-line UX brief was imported unchanged, then read from product decision through backend architecture, Widget runtime, future multitenancy, implementation phases, and definition of done.

The evidence pass covered TinyIDP's current admin service, protocol store interfaces, SQLite migrations and transaction envelope, audit semantics, production HTTP composition, the existing xgoja-based TinyIDP application, the RAG evaluation Widget DSL provider and React renderer, and the Upwork backend example.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket for tiny-idp to add a proper administration backend, using the rag-eval widget-dsl (in ~/code/wesen/go-go-golems/rag-evaluation-system/packages, here is an example of it being used for a backend system:  /home/manuel/code/wesen/go-go-golems/upwork) . Import the file /tmp/tiny-idp-ux.md and read it and design out the MVP ticket in detailed. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a new umbrella implementation ticket for a secure system-scoped admin control plane, ground its design in the current repository and the supplied UX specification, explain the work for a new intern, and deliver the ticket bundle to reMarkable.

**Inferred user intent:** Make the next engineer able to implement the administration backend without re-deriving TinyIDP's security boundaries or copying unsafe aspects of a lower-risk Widget DSL example.

### What I did

- Ran `docmgr status --summary-only` and confirmed the configured root is `tiny-idp/ttmp`.
- Detected the existing `TINYIDP-ADMIN-001--admin-cli-for-tinyidp-product-operations` ticket.
- Created `TINYIDP-ADMIN-CONSOLE-001`.
- Added a design document and investigation diary through docmgr.
- Imported `/tmp/tiny-idp-ux.md` into `sources/local/tiny-idp-ux.md`.
- Read all 3,015 lines of the source in bounded sections.
- Inspected:
  - `internal/admin`;
  - `internal/cmds/serve_production.go`;
  - `pkg/idpstore`, `pkg/sqlitestore`, `pkg/idpaccounts`, and `pkg/idp`;
  - `cmd/tinyidp-xapp`;
  - Upwork's `xgoja.yaml`, route host, page composition, frontend, and developer guide;
  - `rag-evaluation-system/pkg/widgetdsl`, its xgoja provider/help, and the React package.

### Why

- A web control plane touches identity, authorization, protocol state, secrets, durable activity, and filesystem operations. Recommendations had to be anchored to actual current contracts rather than inferred from screen mockups.
- The existing CLI ticket and new web control-plane work have different authentication, session, concurrency, and delivery concerns.
- The Upwork example demonstrates Widget DSL mechanics but intentionally exposes public JavaScript routes and direct SQLite access; those boundaries cannot be copied into an IdP administration plane.

### What worked

- `docmgr import file` preserved the source and updated ticket metadata/indexing.
- The UX brief already contained strong system-scope, action-handle, query/command, activity, and multitenancy requirements.
- Current code confirmed the brief's important gaps:
  - user storage is point-oriented;
  - `internal/admin.Service` has no principal, grant, scope, or version;
  - the file audit sink is durable but not queryable;
  - the production host already has a long-lived shared store and separate internal observability listener;
  - Widget DSL has a closed registry and typed builders;
  - the renderer's default action transport needs a TinyIDP-specific CSRF-aware consumer.

### What didn't work

- Running `git status --short` from the outer workspace returned exactly:

  ```text
  fatal: not a git repository (or any of the parent directories): .git
  ```

  The repository is the `tiny-idp/` child directory, so all later Git and repository commands ran there.

- An initial assumption that Widget IR types lived in one TypeScript file failed:

  ```text
  nl: /home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/widgets/ir.ts: No such file or directory
  ```

  A second guessed path also failed:

  ```text
  nl: /home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/src/ir.ts: No such file or directory
  ```

  The types are split across the `src/widgets/ir/` directory. `rg --files` and `find` resolved the actual layout.

### What I learned

- TinyIDP's `--admin-addr` is already an internal metrics/readiness listener. The browser console belongs on the public origin under `/admin`; it must not overload that listener.
- `idp.ErrAuditDelivery` is a committed-state marker, not a generic failure. A transactional action row and outbox are required for unambiguous web results.
- The current SQLite store is a single-active-node, one-open-connection implementation. Admin sessions, page queries, outbox work, and projections must use short bounded transactions.
- The Widget DSL React package exports `WidgetRenderer`, the default registry, IR types, and hooks, so TinyIDP can build a Redux/RTK Query consumer without copying renderer components.
- The existing TinyIDP xapp demonstrates a same-process OIDC provider plus a separate application session, which is a stronger local reference for admin authentication than Upwork.

### What was tricky to build

- The key design problem was transaction ownership. Splitting the admin SQLite implementation into a separate package would make it difficult to atomically mutate protocol records and admin action/outbox rows because `sqlitestore.Store.Update` hides its concrete transaction. The design keeps admin persistence interfaces separate but implements them on the existing concrete `sqlitestore.Store`.
- Widget DSL is both valuable and potentially dangerous in this context. The solution is not to avoid it, but to expose a narrow `tinyidp.admin` xgoja module that returns safe queries and opaque action descriptors while Go remains the security boundary.

### What warrants a second pair of eyes

- Confirm the recommendation to use a public PKCE admin client instead of a confidential client.
- Review whether separate key files for auth-attempt encryption and action-handle HMAC are preferred over HKDF derivation from an existing secret.
- Validate the proposed projection/version triggers against SQLite's current JSON blob persistence and `INSERT OR REPLACE` behavior.
- Review outbox readiness thresholds and retention periods.

### What should be done in the future

- Implementation phases should update this diary with exact migrations, API shapes, tests, failures, and commits.
- If Widget DSL lacks an administration interaction, extend the typed upstream DSL rather than introducing raw HTML or arbitrary browser behavior.

### Code review instructions

- Start with `sources/local/tiny-idp-ux.md`, especially sections 10 through 12 and 18 through 19.
- Compare claims with `internal/admin/service.go`, `pkg/idpstore/interfaces.go`, `pkg/idp/audit.go`, and `internal/cmds/serve_production.go`.
- Compare Widget integration with Upwork's `xgoja.yaml`, `verbs/upwork.js`, and `verbs/lib/pages.js`.
- No product code was changed in this research step.

### Technical details

Primary commands included:

```text
docmgr status --summary-only
docmgr ticket create-ticket --ticket TINYIDP-ADMIN-CONSOLE-001 ...
docmgr doc add --ticket TINYIDP-ADMIN-CONSOLE-001 ...
docmgr import file --ticket TINYIDP-ADMIN-CONSOLE-001 --file /tmp/tiny-idp-ux.md
rg -n ...
nl -ba ...
sed -n ...
```

## Step 2: Write the implementation architecture

The design translates the supplied screen and product specification into concrete Go, SQLite, xgoja, React, Redux, RTK Query, and HTTP contracts. It treats the ticket as an umbrella MVP with six reviewable phases and gives an intern explicit file ownership, pseudocode, API contracts, diagrams, migration tables, error semantics, tests, risks, and definition of done.

The most important outcome is a hard security boundary: Widget JavaScript composes reviewed presentation, while Go resolves the principal and scope, reloads grants, authorizes capabilities, validates action handles and CSRF, compares versions, and owns transactions.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Convert the evidence into a self-contained technical implementation guide rather than merely summarizing the imported UX document.

**Inferred user intent:** Give a new intern a safe sequence of implementation work with enough underlying explanation to recognize architectural mistakes.

### What I did

- Replaced the generated design template with a comprehensive architecture guide.
- Added:
  - executive summary and onboarding explanation;
  - evidence-backed current-state and gap analysis;
  - component and sequence diagrams;
  - package/file layout;
  - principal, scope, grant, query, command, and authorizer contracts;
  - OIDC PKCE login, session, CSRF, and fresh-auth design;
  - opaque action handle, CAS version, nonce, and idempotency design;
  - migrations, projections, activity, outbox, and operation records;
  - Widget DSL, xgoja safe module, React/Redux/RTK Query, Bootstrap, and asset/CSP design;
  - HTTP API and error mapping;
  - user, invitation, client, key, backup, and diagnostics flows;
  - eight decision records;
  - six implementation phases;
  - unit, transaction, HTTP, browser, and Widget contract testing;
  - risks, owner-review questions, definition of done, and file/API map.

### Why

- The imported UX source is product-complete but not a repository-specific intern implementation guide.
- Authentication and authorization details must be explicit before route or UI code begins.
- The guide must tell a new engineer which existing behavior to reuse, which package owns new behavior, and where the reference example is unsafe for TinyIDP.

### What worked

- Existing code supplied line-anchored evidence for every major current-state assertion.
- Widget DSL's typed builders map well to page, collection, table, action, form dialog, and app-shell requirements.
- The React package's exported registry/renderer permits a CSRF-aware RTK Query shell.
- The current store's named atomic operations provide a good base for security-sensitive user commands.

### What didn't work

- N/A. The design edit applied cleanly.

### What I learned

- The safest integration is not a full handwritten frontend and not an Upwork clone. It is a small TinyIDP consumer of the shared renderer plus a deliberately narrow Go-backed xgoja module.
- A generic resource-version table with update triggers can detect changes from CLI, protocol activity, or another tab without putting UI versions into the OIDC user struct.
- One-time secret operations need special idempotency behavior because the normal stored-response pattern must not persist or replay the secret.

### What was tricky to build

- Reconciling the project's React/Redux/RTK Query/Bootstrap guidance with the ready-made `RagEvaluationSiteApp` required inspecting package exports and request behavior. The selected design uses the lower-level exported renderer and registry so TinyIDP controls transport and CSRF.
- Preserving a future domain model without exposing fake tenant concepts required separating `AdminScope` from the existing `User.Tenant` OIDC claim throughout the guide.

### What warrants a second pair of eyes

- The migration design, especially projection synchronization and generic aggregate versions.
- OIDC auth-attempt encryption and return-path validation.
- The safe xgoja provider API and confirmation that no general DB, filesystem, or HTTP module is selected.
- Secret-bearing result behavior across Go JSON serialization and RTK Query cache.

### What should be done in the future

- Turn each implementation phase into a focused child ticket if the team prefers smaller review units.
- Record accepted answers to the open owner-review questions as compact decision records.

### Code review instructions

- Read the design in order through sections 1–10 for architecture, then sections 13–17 for decisions and execution.
- Verify line references with `nl -ba <file>`.
- Validate document structure and metadata with `docmgr doctor --ticket TINYIDP-ADMIN-CONSOLE-001 --stale-after 30`.

### Technical details

The design intentionally specifies:

```text
public console: /admin
public APIs:    /api/admin and /api/widget
static assets:  /static/admin
effective MVP scope: system/system
single mutation transport: POST /api/widget/actions/execute
security owner: Go pkg/idpadmin service
presentation owner: widget.dsl v3 + React WidgetRenderer
```

## Step 3: Validate and deliver the ticket bundle

The final step validated ticket metadata and document structure, ran the required reMarkable dry-run, and uploaded one table-of-contents PDF containing the architecture guide, diary, and imported UX specification. The successful upload response is the reMarkable workflow's verification signal.

The first real upload command had a mistyped design-document path and failed before rendering or cloud mutation. The corrected command then hit the restricted sandbox's DNS boundary; rerunning the exact validated upload with approved network access succeeded.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Store a validated ticket bundle on reMarkable after completing the research and design.

**Inferred user intent:** Make the full implementation package available for offline technical review.

### What I did

- Ran `docmgr doctor --ticket TINYIDP-ADMIN-CONSOLE-001 --stale-after 30`.
- Confirmed balanced fenced code blocks in the design, diary, and imported source.
- Ran the required `remarquee upload bundle ... --dry-run`.
- Uploaded the three-document bundle as `TINYIDP ADMIN CONSOLE MVP.pdf`.
- Used destination `/ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001`.

### Why

- The ticket workflow requires clean docmgr validation and a dry-run before upload.
- A bundle gives the reader one PDF with a table of contents while preserving the implementation guide, research history, and original product source.

### What worked

- Docmgr reported:

  ```text
  ## Doctor Report (1 findings)

  ### TINYIDP-ADMIN-CONSOLE-001

  - ✅ All checks passed
  ```

- The dry-run resolved all three exact source paths and the requested remote directory.
- The final upload returned:

  ```text
  OK: uploaded TINYIDP ADMIN CONSOLE MVP.pdf -> /ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001
  ```

### What didn't work

- The first real upload used a malformed first path and failed before any upload:

  ```text
  Error: path not found: /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md: stat /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md: no such file or directory
  ```

- The corrected sandboxed upload could not resolve reMarkable hosts:

  ```text
  ERROR: 2026/07/23 20:29:37 transport.go:258: http request failed with Get "https://internal.cloud.remarkable.com/sync/v4/root": dial tcp: lookup internal.cloud.remarkable.com: no such host
  ERROR: 2026/07/23 20:29:37 transport.go:258: http request failed with Post "https://webapp-prod.cloud.remarkable.engineering/token/json/2/user/new": dial tcp: lookup webapp-prod.cloud.remarkable.engineering: no such host
  ERROR: 2026/07/23 20:29:37 auth.go:53: failed to create user token from device token Post "https://webapp-prod.cloud.remarkable.engineering/token/json/2/user/new": dial tcp: lookup webapp-prod.cloud.remarkable.engineering: no such host
  ```

  The exact upload was rerun with approved network access and succeeded.

### What I learned

- The upload workflow's successful `OK: uploaded` line is sufficient verification; no routine cloud listing is needed.
- A dry-run validates inclusion and destination but not a later manually retyped path. Reuse exact copied paths for the real command.

### What was tricky to build

- The bundle is large and code-heavy: roughly 20,000 words across three documents, with hundreds of fenced blocks in the imported source. Balanced-fence checks and the renderer dry-run reduced the risk of a malformed PDF input.

### What warrants a second pair of eyes

- Review the rendered PDF table of contents and dense ASCII diagrams on the device.
- Confirm that the imported source should remain bundled with future revised design uploads; it materially increases length but preserves full context.

### What should be done in the future

- Use `--force` only if intentionally replacing this PDF; replacement can remove existing reMarkable annotations.
- Upload future accepted architecture revisions under a new name or date unless annotation loss is explicitly acceptable.

### Code review instructions

- Open the remote bundle at `/ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001/TINYIDP ADMIN CONSOLE MVP.pdf`.
- Use the PDF table of contents to review the architecture guide first, then the diary and imported source.
- Re-run `docmgr doctor --ticket TINYIDP-ADMIN-CONSOLE-001 --stale-after 30` after any ticket edits.

### Technical details

Successful delivery command:

```text
remarquee upload bundle <design.md> <diary.md> <source.md> \
  --name "TINYIDP ADMIN CONSOLE MVP" \
  --remote-dir "/ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001" \
  --toc-depth 2 \
  --non-interactive
```

## Step 4: Start implementation on an isolated feature branch

Implementation began by turning the accepted design into the repository's working plan, auditing the worktree, and isolating the work on `feat/tinyidp-admin-console`. The documentation baseline and an unrelated already-present Jitsi ticket were committed separately so later implementation commits can be reviewed and reverted independently.

Two path assumptions failed during the audit. They were corrected by asking Git and `rg` for authoritative paths rather than continuing from remembered paths. The first attempt to create the branch was also rejected by the filesystem sandbox because this linked worktree stores Git metadata outside the writable workspace; after the user explicitly authorized switching branches, the same operation succeeded with approved access.

### Prompt Context

**User prompt (verbatim):** `implement the ticket, keep a detailed diary as you work, commit at appropriate intervals.`

**Assistant interpretation:** Implement the complete phased administration-console ticket, maintain a chronological engineering diary with commands and failures, and create reviewable commits at coherent boundaries.

**Inferred user intent:** Produce working, tested production code—not another planning artifact—while preserving enough evidence for an intern or reviewer to understand every architectural and implementation decision.

### What I did

- Audited the current branch and untracked ticket directories.
- Attempted to create `feat/tinyidp-admin-console`; recorded the sandbox failure instead of bypassing it.
- Resolved the real checkout root and Git metadata location.
- Created `feat/tinyidp-admin-console` after explicit user authorization.
- Committed the administration-console ticket as `ca9146f`.
- Inspected and separately committed the pre-existing Jitsi deployment ticket as `d707119`.
- Began resolving the current `idpstore` and `sqlitestore` APIs and migration layout for Phase A.

### Why

- A dedicated branch prevents this multi-phase implementation from extending the unrelated Jitsi feature branch.
- Separate ticket commits keep two independently authored documentation packages independently reviewable.
- Phase A must be based on the actual store interfaces and migration mechanism because authorization, action replay protection, and protocol mutations must share a real SQLite transaction.

### What worked

- The user explicitly authorized the branch operation.
- The branch and both commits completed successfully.
- The pre-commit hook ran and correctly skipped code checks for documentation-only commits.
- `rg --files` found the actual source tree and showed that migrations currently end at `015_integration_transactions.sql`.

### What didn't work

- A remembered test-suite filename was wrong:

  ```text
  sed: can't read pkg/sqlitestore/testsuite.go: No such file or directory
  ```

  The suite is `pkg/idpstore/testsuite.go`.

- The first branch operation failed because linked-worktree Git metadata is outside the writable sandbox:

  ```text
  fatal: cannot lock ref 'refs/heads/feat/tinyidp-admin-console': Unable to create '/home/manuel/code/wesen/go-go-golems/tiny-idp/.git/refs/heads/feat/tinyidp-admin-console.lock': Read-only file system
  ```

- The initial resumed inspection used a duplicated `tiny-idp/tiny-idp` path. A later command ran from the workspace aggregator rather than a Git checkout and reported:

  ```text
  fatal: not a git repository (or any of the parent directories): .git
  ```

  The correct checkout root is `/home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp`.

### What I learned

- This checkout is a linked Git worktree whose metadata is at `/home/manuel/code/wesen/go-go-golems/tiny-idp/.git/worktrees/tiny-idp`.
- The workspace parent contains several repositories, so every source and Git command must name the TinyIDP checkout explicitly.
- The existing migration sequence is contiguous through version 15, making versions 16 and 17 available for the control-plane schema and read projection.

### What was tricky to build

- The boundary between a reversible Git operation and sandbox authority was operationally subtle: the requested implementation implied commits, but the environment still required explicit authority to write external linked-worktree metadata.
- The workspace and repository share the name `tiny-idp` at adjacent levels, which made a plausible but invalid doubled path easy to construct.

### What warrants a second pair of eyes

- Confirm whether the unrelated Jitsi ticket is expected on this feature branch long-term or should later be moved/cherry-picked elsewhere.
- Review the first Phase A commit particularly for transaction ownership and whether all security records share the protocol mutation transaction.

### What should be done in the future

- Use `git rev-parse --show-toplevel` from the candidate checkout before any multi-command workflow.
- Keep implementation commits scoped by phase or security invariant rather than by arbitrary elapsed time.

### Code review instructions

- Review `ca9146f` as the administration design baseline.
- Review `d707119` independently as the Jitsi ticket capture.
- Confirm `git status --short` before each implementation commit so no other workspace ticket is staged accidentally.

### Technical details

The branch boundary is:

```text
feat/tinyidp-plugin-jitsi
    |
    +-- ca9146f docs(ticket): design tinyidp administration console MVP
    |
    +-- d707119 docs(ticket): analyze Jitsi k3s deployment boundary
         HEAD -> feat/tinyidp-admin-console
```

## Step 5: Implement the control-plane security and persistence foundation

The first production-code slice establishes the security vocabulary and durable invariants on which every later HTTP, Widget DSL, and React feature depends. `pkg/idpadmin` now owns closed scopes and capabilities, principals, grants, server-side authorization, and short-lived signed action handles. `pkg/idpadminstore` defines the persistence boundary, while SQLite migrations 16 and 17 and `admin_store.go` make grants, sessions, replay prevention, resource versions, idempotency, actions, and the audit outbox durable.

This slice deliberately stops below HTTP and UI. It proves that presentation code cannot mint authority, a stale grant version cannot authorize a request, a handle cannot move between sessions, a nonce cannot be consumed twice, and protocol data plus control-plane evidence can commit or roll back in one SQLite transaction.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Begin with the Phase A security and transaction substrate, verify it independently, and commit it before adding application services or transport.

**Inferred user intent:** Make later console work safe by construction and leave an evidence-backed trail for intern implementation and review.

### What I did

- Added `pkg/idpadmin`:
  - MVP system scope validation with a reserved future domain shape;
  - a closed capability vocabulary;
  - authenticated and fresh principal assurance;
  - versioned, revocable grants;
  - a store-backed authorizer that revalidates every security input;
  - HMAC-SHA-256 action handles capped at five minutes.
- Added `pkg/idpadminstore` contracts for grants, admin sessions, action nonces, resource versions, idempotency records, action records, audit outbox rows, and cross-domain atomic updates.
- Added migrations:
  - `016_admin_control_plane.sql`;
  - `017_admin_user_projection.sql`.
- Added a concrete SQLite implementation and compile-time interface assertion.
- Added unit and SQLite transaction tests.

### Why

- Widget action metadata is untrusted presentation data unless Go signs and revalidates it.
- Grant revocation and capability changes must invalidate already-rendered action handles.
- Replay prevention, CAS, the domain mutation, action evidence, and audit outbox insertion need one transaction owner.
- A query projection is required before the console can support stable, indexed pagination at production user counts.

### What worked

- `go test ./pkg/idpadmin` passed after the initial model implementation.
- With a task-local Go cache, the complete focused verification passed:

  ```text
  ok  	github.com/go-go-golems/tiny-idp/pkg/idpadmin
  ?   	github.com/go-go-golems/tiny-idp/pkg/idpadminstore	[no test files]
  ok  	github.com/go-go-golems/tiny-idp/pkg/sqlitestore
  ```

- Migration discovery automatically recognized versions 16 and 17.
- The SQLite test proves `AdminUpdate` rolls an `idpstore.User` and an `idpadmin.Grant` back together.
- Tests prove handle tampering, expiration, subject/session binding, stale grant version, fresh-auth expiry, nonce replay, single active owner, and resource-version CAS.

### What didn't work

- The first combined test used the default global Go cache, which is read-only in this sandbox:

  ```text
  open /home/manuel/.cache/go-build/aa/aa822a343921883b031b37a186b18f774e758f4d56238ba134b1726f1a1937a9-d: read-only file system
  ```

  `pkg/sqlitestore` still passed in that run. Re-running with `GOCACHE=/tmp/tinyidp-admin-go-cache` made all focused packages pass.

- The first invitation-metadata migration draft referenced `durable_invitations(id)`, but the existing table is keyed only by `code_hash`. Inspection caught the mismatch before commit; the invalid foreign key was removed.

- The first pre-commit run exposed a flaky tamper-test technique. Replacing the final Base64URL character can change only unused encoding bits, so the decoded signature may remain identical:

  ```text
  --- FAIL: TestActionHandleTamperExpiryAndBinding (0.00s)
      model_test.go:75:
          Error: Expected error with "invalid administration action handle" in chain but got nil.
  ```

  The test now changes a character in the signed payload segment while retaining the original signature. The focused tests and the complete pre-commit test/lint hooks then passed.

- The obsolete top-level form `docmgr file-note --help` failed with:

  ```text
  Error: unknown command "file-note" for "docmgr"
  ```

  Current docmgr uses `docmgr doc relate --file-note`; that command related all eight implementation files and `docmgr doctor` passed.

### What I learned

- Existing migrations serialize durable invitations into a `data` blob and expose no stable relational invitation ID. Admin display metadata must remain independently keyed until invitation storage is deliberately remodeled in Phase D.
- The concrete SQLite store can implement `AdminUpdate` without weakening the existing public `idpstore.TxStore`: one scoped `*Store` safely satisfies both callback interfaces.
- An active-owner partial unique index enforces the single-owner bootstrap invariant at the database boundary.

### What was tricky to build

- Signed handles must reject unknown JSON fields and impossible validity windows as well as invalid signatures; otherwise future schema drift could silently weaken interpretation.
- A failed conditional revoke can mean “missing” or “stale.” The implementation performs a follow-up existence check so callers receive `ErrGrantNotFound` or `ErrVersionConflict` rather than an ambiguous result.
- SQL nullable nanosecond timestamps need careful conversion so `nil` retains lifecycle meaning.

### What warrants a second pair of eyes

- Review the capability list before it becomes part of persisted grant JSON.
- Review whether the owner uniqueness rule should ignore expired grants as well as revoked grants; SQLite partial indexes cannot use the current clock, so owner replacement currently requires explicit revocation.
- Review action/idempotency retention and cleanup indexes before Phase E workers are implemented.
- Review the user projection population strategy; this commit creates the schema but does not yet add synchronization triggers or rebuild code.

### What should be done in the future

- Add the Phase A query and command application contracts and owner grant CLI.
- Add a first-class resource-version initializer rather than seeding rows from tests or migration/rebuild code.
- Implement outbox dequeue/delivery and expiration maintenance with bounded batches.

### Code review instructions

- Start at `pkg/idpadmin/model.go`, then follow authorization inputs into `action_handle.go`.
- Inspect `pkg/idpadminstore/store.go` before `pkg/sqlitestore/admin_store.go` to understand the intended boundary.
- Read both migrations before reviewing SQL methods.
- Run:

  ```text
  GOCACHE=/tmp/tinyidp-admin-go-cache go test ./pkg/idpadmin ./pkg/idpadminstore ./pkg/sqlitestore
  ```

### Technical details

The atomic mutation boundary is:

```go
AdminUpdate(ctx, func(protocol idpstore.TxStore, admin idpadminstore.TxStore) error {
    // 1. consume action nonce
    // 2. compare and increment aggregate version
    // 3. mutate protocol/domain state
    // 4. insert admin action evidence
    // 5. enqueue audit event
    return nil // one commit, or one rollback
})
```

## Step 6: Complete the Phase A application and operator boundary

The second implementation slice completed the parts that turn the security substrate into an operable control plane. It added safe query and command contracts, established `pkg/idpadminapp` as the application orchestration layer, implemented the complete owner bootstrap and recovery lifecycle through Glazed commands, added a deterministic user-projection rebuild/check facility, and implemented the shared atomic mutation executor.

The executor is the key result of this step. It verifies a signed handle, reauthorizes against the current grant, recognizes prior idempotent results, consumes the action nonce, advances the target's compare-and-set version, calls the domain mutation, records action evidence, enqueues a sanitized audit event, and stores the idempotency result inside one SQLite transaction. A failure at any stage rolls all stages back.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Finish every Phase A task from the implementation guide and preserve test evidence and architectural corrections in the ticket.

**Inferred user intent:** Establish a secure, CLI-operable backend foundation before exposing any administration HTTP route or UI.

### What I did

- Added `pkg/idpadmin/contracts.go` with:
  - typed cursor requests and bounded page sizes;
  - safe overview, user, invitation, client, key, activity, and operation DTOs;
  - the full `QueryService` and `CommandService` interfaces;
  - request and result types, including a distinct one-time-secret result.
- Added `pkg/idpadminapp/owner.go`:
  - fixed public PKCE client bootstrap;
  - exact HTTPS-origin validation;
  - single active owner grant creation;
  - status, grant revocation, and session revocation;
  - atomic action/outbox evidence.
- Added four Glazed CLI commands beneath `tinyidp admin console`:
  - `bootstrap`;
  - `status`;
  - `revoke-grant`;
  - `revoke-session`.
- Added `admin_projection.go` with canonical rebuild and non-mutating drift comparison.
- Added `executor.go` with authorization, idempotency, replay, CAS, mutation, evidence, and outbox handling.
- Changed persisted action nonces and action evidence to store SHA-256 keys rather than raw handle nonces.
- Updated the design's package map to show the discovered application-layer boundary.

### Why

- DTOs must make it impossible for Widget DSL or React code to receive password hashes, client secret hashes, or private signing keys.
- Owner authority must be bootstrapped and recoverable without depending on the web console it authorizes.
- The public admin client is fixed and public with PKCE so no second long-lived client secret exists.
- Projection drift must be observable and repairable before production query endpoints depend on it.
- Mutation invariants must have one implementation rather than being reassembled independently in every HTTP handler.

### What worked

- Focused domain, application, SQLite, and command tests passed.
- The complete repository test suite passed with `go test ./...`, including the
  61-second Fosite adapter suite, server tests, SQLite tests, scripts, and
  production harness packages.
- `make lint` passed `golangci-lint`, standard `go vet`, Glazed CLI lint, and
  the project-specific IDP UI analyzer with zero reported issues.
- The complete `internal/cmds` suite passed when allowed to bind its existing loopback test servers:

  ```text
  ok   github.com/go-go-golems/tiny-idp/internal/cmds  1.080s
  ```

- Owner tests prove:
  - an existing login is required;
  - the client is public, PKCE-required, authorization-code-only, and secretless;
  - a second active owner is rejected by the database;
  - status finds the current owner;
  - grant and session revocation persist.
- Projection tests rebuild from user/security/session/grant sources, verify counts and version, and detect deliberate drift.
- Executor tests prove successful atomic evidence, cached idempotent replay, conflicting request rejection, nonce restoration after stale-CAS rollback, and non-replay of one-time secrets.

### What didn't work

- The first attempt to place owner orchestration in `pkg/idpadmin` would have created this import cycle:

  ```text
  idpadmin -> idpadminstore -> idpadmin
  ```

  This was caught while editing, before accepting a compiling-but-misowned workaround. The service moved to the explicit `pkg/idpadminapp` layer.

- The sandboxed full command test could not bind an existing `httptest` server:

  ```text
  panic: httptest: failed to listen on a port: listen tcp6 [::1]:0: socket: operation not permitted
  ```

  Focused console tests passed inside the sandbox. The unchanged full package test passed with approved loopback permission.

- The first CLI lifecycle assertion reused `rowVal`, a helper that assumes every Glazed value is a string. The new `configured` field is a Boolean:

  ```text
  panic: interface conversion: interface {} is bool, not string
  ```

  The console test now uses a typed `anyRowVal` helper. The next focused run passed.

- The initial projection rebuild called canonical-read helpers on the outer store while a single-connection transaction was active. Review caught that this could wait on its own connection. The implementation now uses the callback-scoped `*Store` for every rebuild query and write.

### What I learned

- Store-dependent administration services require a distinct application package because persistence contracts already depend on domain types. This is a real dependency rule, not a naming preference.
- Existing Glazed commands provide a strong local pattern: inject the inherited database pointer, decode only command settings from `values.Values`, emit typed rows, and default the output section to JSON.
- A projection consistency checker should not repair resource versions as a side effect. Rebuild initializes missing versions; check reports a missing version as drift.
- Idempotent replay must occur before nonce consumption. Otherwise a legitimate retry of a completed request would be indistinguishable from an attack.
- Secret-bearing idempotency needs a durable execution marker but must never persist or return the original secret again.

### What was tricky to build

- Bootstrap needs to validate an existing fixed client exactly without using the existing replace-oriented client write as an accidental compatibility path. An incompatible fixed client causes a hard conflict.
- Owner revocation action evidence still references the just-revoked grant. SQLite permits this because revocation updates the row rather than deleting it.
- Canonical projection counts require decoding the repository's blob-backed sessions and grants. The table name in the generic counter remains a closed internal constant; all user-derived values remain SQL parameters.
- The executor must return cached response bytes from inside a transaction callback without proceeding to nonce, CAS, or mutation steps.

### What warrants a second pair of eyes

- Confirm the public console client's five-minute access and ID-token lifetimes.
- Review whether CLI bootstrap should enqueue only the new outbox audit, emit the existing JSONL audit, or retain both during the migration period. It currently does both.
- Review the 24-hour idempotency retention before cleanup workers are implemented.
- Review the application package name and dependency diagram now recorded in the design.
- Review whether resource projection maintenance should remain named-command updates or move to database-normalized source columns in a later migration.

### What should be done in the future

- Phase B should construct the first read-only query service over `admin_user_projection`.
- The auth/session layer should register action nonces when a mutation widget handle is minted.
- Maintenance should delete expired nonces and idempotency records and deliver outbox records in bounded batches.
- Query benchmarks should validate the 10,000-user target once cursor SQL is implemented.

### Code review instructions

- Review contracts in `pkg/idpadmin/contracts.go`.
- Follow operator flow from `internal/cmds/admin_console.go` to `pkg/idpadminapp/owner.go`.
- Review projection transaction ownership in `pkg/sqlitestore/admin_projection.go`.
- Treat `pkg/idpadminapp/executor.go` and its tests as a security-critical unit.
- Run focused verification:

  ```text
  GOCACHE=/tmp/tinyidp-admin-go-cache go test \
    ./pkg/idpadmin ./pkg/idpadminstore ./pkg/idpadminapp ./pkg/sqlitestore
  GOCACHE=/tmp/tinyidp-admin-go-cache go test ./internal/cmds
  ```

### Technical details

The implemented execution order is:

```text
verify signed handle
  -> reload and authorize current grant
  -> BEGIN SQLite write transaction
     -> return matching prior idempotency result, if any
     -> consume H(action nonce)
     -> compare/increment resource version
     -> execute protocol/domain mutation
     -> insert admin action
     -> enqueue sanitized audit payload
     -> persist non-secret idempotency result
  -> COMMIT
```

## Step 7: Implement the Phase B authentication and session foundation

Phase B began with the backend security boundary rather than route or frontend work. The new `internal/adminweb.AuthManager` implements a public OIDC authorization-code flow with PKCE, encrypted server-side attempts, browser binding, nonce verification, a dedicated admin session cookie, grant-backed principal reconstruction, CSRF token rotation, logout, and immediate session invalidation when the backing grant changes.

The Widget DSL dependency was also resolved against the current repositories. The Go runtime provider is `github.com/go-go-golems/rag-evaluation-system/pkg/xgoja/providers/widgetsite` at v0.1.7, matching the Upwork backend configuration. The published React renderer is `@go-go-golems/rag-evaluation-site` v0.1.19.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Start the authenticated read-only console phase at the security boundary, verify it independently, and keep unsafe general-purpose xgoja modules out of the design.

**Inferred user intent:** Ensure the web console proves identity and current server-side authorization before any Widget DSL page or API becomes reachable.

### What I did

- Added the rag-evaluation-system Go module at the same provider version used by Upwork.
- Extended `idpadminstore` and SQLite with:
  - create/consume auth attempts;
  - browser-binding comparison;
  - one-time atomic consumption;
  - session touch;
  - CSRF hash rotation.
- Added `internal/adminweb/auth.go` with:
  - OIDC discovery and verifier adapter;
  - authorization-code + S256 PKCE start;
  - authenticated encryption of the PKCE verifier and raw nonce;
  - HMAC-domain-separated state, nonce, binding, session, and CSRF hashes;
  - fixed-client token exchange and ID-token verification;
  - active grant lookup by subject and system scope;
  - HttpOnly/Secure/SameSite=Lax cookies;
  - rotating in-memory CSRF bootstrap responses;
  - authentication plus exact-origin and synchronizer-token CSRF middleware;
  - session logout and grant-version invalidation.
- Added HTTP tests without opening real sockets.

### Why

- The browser must never be the authoritative store for PKCE verifier, expected nonce, return path, grant, or capability.
- State alone does not bind a login attempt to the browser that initiated it.
- An authenticated ID token proves a subject but does not grant administration authority.
- CSRF material should be returned into application memory and stored only as a keyed hash server-side.
- Every request must observe current grant revocation/version rather than trusting the version captured at login indefinitely.

### What worked

- `internal/adminweb` and `pkg/sqlitestore` tests pass.
- The login test proves S256 challenge and nonce generation plus HttpOnly/Secure browser-binding cookie behavior.
- The callback test proves server-side attempt consumption, token exchange, nonce validation, owner-grant lookup, safe return path, and secure session cookie issuance.
- The session endpoint returns a new CSRF token but never returns the raw session handle.
- Revoking the owner grant immediately turns an already-issued admin session into HTTP 401.
- An absolute external return URL is rejected with HTTP 400.

### What didn't work

- The originally referenced frontend path no longer exists:

  ```text
  sed: can't read /home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/widget-dsl/package.json: No such file or directory
  ```

  The current frontend package is `packages/rag-evaluation-site`; `widget.dsl`
  itself is the Go/xgoja provider under `pkg/widgetdsl` and
  `pkg/xgoja/providers/widgetsite`.

- A later inspection assumed a source-root `index.ts`, but this package exposes
  generated root entrypoints and keeps widget exports under `src/widgets`:

  ```text
  sed: can't read /home/manuel/code/wesen/go-go-golems/rag-evaluation-system/packages/rag-evaluation-site/index.ts: No such file or directory
  ```

  Package exports were verified from `package.json` and `src/widgets/index.ts`.

### What I learned

- The Upwork runtime selects `widget.dsl` from the `rag-widget-site` provider; it separately enables host filesystem and database modules. TinyIDP must select only the Widget provider and its own narrow `tinyidp.admin` module.
- Migration 16 can keep one encrypted blob column: the AEAD plaintext contains both PKCE verifier and raw nonce, while `nonce_hash` remains independently comparable evidence.
- The admin session cannot persist a recoverable CSRF token. `GET /api/admin/session` rotates the hash and returns the new raw value to frontend memory.
- `go get` upgraded transitive Goja, SQLite, Glamour, Chroma, and display-width dependencies while adding rag-evaluation-system; full repository verification is required at this commit boundary.

### What was tricky to build

- OIDC nonce validation requires the raw nonce after callback; a hash alone cannot be supplied to an ID-token verifier. Encrypting the nonce with the PKCE verifier preserves server ownership without storing plaintext.
- Return paths must be relative `/admin...` paths with no scheme, host, protocol-relative prefix, or fragment.
- The session cookie covers `/admin`, which also covers `/admin/auth` and `/admin`-scoped APIs only if routing is chosen consistently. Public API mounting will preserve this boundary.
- Authentication middleware reconstructs assurance from the persisted authentication time and separately rechecks grant subject, active state, and version.

### What warrants a second pair of eyes

- Review the eight-hour admin session TTL and five-minute fresh-auth window.
- Review the session cookie's `/` path. It is required because the console UI is
  `/admin` while its APIs are `/api/admin`; HttpOnly, Secure, SameSite, host-only
  scope, and server-side hashing still constrain the cookie.
- Review the dependency upgrades introduced by rag-evaluation-system v0.1.7.
- Review whether callback failures should receive a rendered browser error page rather than concise HTTP text.

### What should be done in the future

- Mount authentication and session endpoints on the public production listener.
- Preserve exact-origin validation for every state-changing endpoint as routes are added.
- Add bounded auth-attempt/session retention to maintenance.
- Add the safe Widget DSL runtime and read-only query endpoints.

### Code review instructions

- Review `internal/adminweb/auth.go` from `LoginHandler` through `CallbackHandler`, then `principal`.
- Inspect the SQLite compare-and-consume query for auth attempts.
- Run:

  ```text
  GOCACHE=/tmp/tinyidp-admin-go-cache go test ./internal/adminweb ./pkg/sqlitestore
  ```

### Technical details

```text
GET /admin/auth/login
  -> random state, nonce, verifier, browser binding
  -> store H(state), H(nonce), AEAD(verifier+nonce), H(binding)
  -> redirect to /authorize with PKCE S256

GET /admin/auth/callback
  -> atomically consume attempt by H(state)+H(binding)
  -> decrypt verifier+nonce
  -> exchange code and verify ID token+nonce
  -> load active current owner grant
  -> store H(session), grant ID/version, H(CSRF placeholder)
  -> set HttpOnly admin session cookie
```

## Step 8: Complete the authenticated read-only console

Phase B now reaches the production listener rather than ending at isolated
authentication primitives. The implementation adds bounded read models, a
fresh-per-render Goja runtime containing only `widget.dsl` and
`tinyidp.admin`, the public HTTP surface, and a compiled React application.
Production startup performs OIDC discovery through the provider's in-process
transport before opening the listener, so startup neither depends on a
hairpin network request nor races the server it is constructing.

### Prompt Context

**User prompt (verbatim):** `what's next?`

**Assistant interpretation:** Continue implementing the ticket through the
next coherent, verified commit boundary.

**Inferred user intent:** Deliver the actual administration console in
reviewable phases rather than stop after writing its design.

### What I did

- Added parameterized SQLite read queries for:
  - overview counters;
  - bounded user search and status filtering;
  - activity;
  - operation health;
  - invitation metadata.
- Added `idpadminapp.PageDataService`, which reloads the principal's current
  grant and authorizes the page-specific capability before executing a query.
- Added a constrained Widget DSL runtime:
  - a new Goja VM for every render;
  - only `widget.dsl` and the narrow `tinyidp.admin.pageData()` module;
  - no filesystem, database, process, or HTTP modules;
  - a 250 ms deadline;
  - 1 MiB encoded-output, 2,000-node, 64-level, and 32 KiB string limits.
- Added the public handler and mounted:
  - `GET /admin` and `/admin/...`;
  - `/admin/auth/...`;
  - `/api/admin/...`;
  - `/api/widget/...`;
  - `/static/admin/...`.
- Left the action execution endpoint explicitly closed with
  `501 mutations_not_enabled` until Phase C supplies the complete guarded
  mutation path.
- Added restrictive CSP, referrer, MIME-sniffing, frame, and permissions
  headers.
- Added a TypeScript React frontend with Redux Toolkit, RTK Query, Bootstrap,
  the rag-evaluation Widget renderer, accessible navigation, skip navigation,
  loading/error states, and the seven read-only navigation destinations.
- Added deterministic Vite output names, committed build artifacts, fixed
  `go:embed` assets, `go generate` directives, Make targets, and a CI gate that
  rebuilds the assets and rejects a diff.
- Added `--admin-auth-key-file`. It accepts an owner-only regular file
  containing exactly 32 bytes, distinct from the OIDC token secret.
- Wired production startup to:
  - derive the public origin from the canonical HTTPS issuer;
  - perform OIDC discovery through `NewInProcessIssuerTransport`;
  - reconstruct the user projection;
  - construct the authorizer, page service, Widget runtime, and admin handler;
  - mount admin routes before the provider's catch-all handler.

### Why

- The console must use the same production trust boundary as the provider,
  without creating a second public listener or trusting forwarded data twice.
- OIDC discovery cannot call the not-yet-listening public socket during
  startup. The repository's fail-closed in-process issuer transport provides
  the same discovery and key material through the actual provider handler.
- Read authorization belongs immediately before each query. A valid session
  alone is not proof that its backing grant remains active or still contains a
  page capability.
- Widget source is server-owned presentation logic, not an alternate backend.
  Supplying pre-authorized data to a two-module VM preserves that separation.
- Committing the generated assets keeps ordinary Go builds self-contained;
  rebuilding and diffing them in CI detects stale artifacts.

### What worked

- `pnpm --dir internal/adminweb/frontend run check` passed TypeScript checking
  and produced:

  ```text
  ../frontend_dist/index.html          0.54 kB
  ../frontend_dist/assets/admin.css  323.82 kB
  ../frontend_dist/assets/admin.js   435.42 kB
  ```

- Focused Go verification passed:

  ```text
  go test ./internal/adminweb ./internal/cmds ./internal/sections/production \
    ./pkg/idpadminapp ./pkg/sqlitestore
  ```

- `go test ./... -count=1` passed.
- `make lint` passed the pinned golangci-lint suite, Glazed analyzer, and
  IDP UI analyzer.
- Handler tests prove the admin prefixes win before the OIDC provider
  fallback, unknown embedded assets return 404, API requests require a
  session, and security headers cover both SPA and asset responses.
- Widget tests prove unavailable modules remain unavailable, execution is
  interrupted at the deadline, and oversized/deep output is rejected.

### What didn't work

- The first Widget program used a fluent convenience method that the installed
  DSL version does not expose:

  ```text
  TypeError: Object has no member 'compact' at <eval>:37:74(4)
  ```

  The program now uses the supported `table((table) => table)` builder form.

- The first TypeScript check assumed the package exported `WidgetPage` and
  that side-effect CSS imports needed no ambient declarations:

  ```text
  src/api.ts(2,15): error TS2724: '"@go-go-golems/rag-evaluation-site"' has no exported member named 'WidgetPage'. Did you mean 'useWidgetPage'?
  src/main.tsx(4,8): error TS2882: Cannot find module or type declarations for side-effect import of 'bootstrap/dist/css/bootstrap.min.css'.
  src/main.tsx(5,8): error TS2882: Cannot find module or type declarations for side-effect import of '@go-go-golems/rag-evaluation-site/styles.css'.
  src/main.tsx(6,8): error TS2882: Cannot find module or type declarations for side-effect import of '@go-go-golems/rag-evaluation-site/styles/theme.css'.
  src/main.tsx(7,8): error TS2882: Cannot find module or type declarations for side-effect import of './styles.css'.
  ```

  The API now uses the package's actual `WidgetPageResponse` export and
  `vite-env.d.ts` declares CSS modules.

- The first focused Go test in the restricted workspace could not read the
  default build cache:

  ```text
  open /home/manuel/.cache/go-build/...-d: read-only file system
  ```

  The same tests passed when executed with the already-approved Go test
  capability.

- The first staging attempt reached the linked worktree's Git metadata outside
  the writable workspace:

  ```text
  fatal: Unable to create '/home/manuel/code/wesen/go-go-golems/tiny-idp/.git/worktrees/tiny-idp/index.lock': Read-only file system
  ```

  Staging was repeated with the approved Git capability; no repository file
  contents were changed by the failed attempt.

- The first successful staging command added the whole new frontend directory
  before that directory had a scoped ignore file. The staged-stat review
  immediately exposed thousands of pnpm dependency files and CRLF/trailing
  whitespace warnings under `frontend/node_modules`. Those dependency files
  were removed from the index without deleting the local installation, and
  `frontend/.gitignore` now excludes `node_modules/`.

### What I learned

- `@go-go-golems/rag-evaluation-site` exports response types separately from
  its hooks; the server's JSON contract should follow `WidgetPageResponse`.
- Fixed Vite output names make a small Go embed boundary possible without a
  generated asset manifest.
- The admin session cookie needs path `/`, not `/admin`, because the UI and API
  live under different top-level prefixes. The cookie remains host-only,
  HttpOnly, Secure, SameSite=Lax, random, and stored only as a keyed hash.
- The owner bootstrap client can remain outside the reviewed production client
  catalog: owner bootstrap provisions the dedicated public client, and normal
  production bootstrap validates the configured catalog without deleting it.

### What was tricky to build

- The production mux has a provider catch-all, so every admin prefix must be
  registered on the outer mux before `/`.
- The issuer may have a path while the console remains origin-level. OIDC
  endpoints derive from the complete issuer, but the callback derives from
  only its scheme and host.
- SQLite invitation records use serialized blobs. Invitation metadata must be
  loaded before scanning dependent values to avoid nested reads on the same
  constrained connection.
- Goja interruption needs a goroutine that stops when normal evaluation ends;
  otherwise a late timeout can interrupt a reused VM. Creating a new VM per
  request both closes that race and prevents cross-request state.

### What warrants a second pair of eyes

- Review the deliberately separate token and admin-session secrets and their
  deployment provisioning.
- Review the CSP before adding any third-party asset or browser-test harness.
- Review the generic read-only page shape before Phase C introduces action
  descriptors.
- Review whether production startup should report projection drift separately
  before rebuilding it automatically.

### What should be done in the future

- Phase C must replace the closed action endpoint only after signed handle,
  fresh-auth, CSRF, typed-confirmation, expected-version, nonce, and
  idempotency checks can execute as one guarded workflow.
- Maintenance must expire admin auth attempts and sessions.
- Phase F must add real browser, responsive, keyboard, and accessibility
  evidence.

### Code review instructions

- Start at `internal/cmds/serve_production.go` where the console is assembled.
- Follow a page request through `internal/adminweb/handler.go`,
  `pkg/idpadminapp/page_data.go`, and `pkg/sqlitestore/admin_queries.go`.
- Review the module allowlist and bounds in
  `internal/adminweb/widget_runtime.go`.
- Inspect `internal/adminweb/frontend/src/api.ts` and `App.tsx`, then compare
  them with the response shape emitted by `verbs/pages.js`.
- Rebuild with `make frontend-check`, then run `git diff --exit-code --
  internal/adminweb/frontend_dist`.

### Technical details

```text
browser GET /api/widget/pages/users
  -> H(session cookie) lookup
  -> reload active owner grant and exact version
  -> authorize users.read
  -> bounded parameterized SQLite query
  -> safe generic page data
  -> fresh Goja VM
       require("tinyidp.admin").pageData()
       require("widget.dsl")
       build Widget IR
  -> validate size, depth, node count, and strings
  -> WidgetPageResponse JSON
  -> React WidgetRenderer
```

## Step 9: Implement guarded user mutations

Phase C replaces the read-only action stub with a single guarded mutation
pipeline shared by HTTP and the operator CLI. The implementation deliberately
does not add resource-specific CRUD endpoints. The server prepares a
short-lived signed action, persists its one-use nonce, and later treats the
handle—not browser fields—as the authority for command, target, capability,
scope, expected version, session binding, and freshness.

### Prompt Context

**User prompt (verbatim):** (automatic continuation of the implementation
goal)

**Assistant interpretation:** Continue through the next complete ticket phase,
preserve a detailed failure/decision record, and commit only a coherent tested
boundary.

**Inferred user intent:** Turn the Phase A security substrate into real user
operations without allowing the web layer or old CLI services to bypass it.

### What I did

- Replaced the coarse `users.write` and similar aggregate permissions with the
  command-level capability vocabulary specified by the design:
  `users.create`, `users.update`, `users.disable`, `users.password.set`,
  `users.unlock`, and `users.access.revoke`, plus the corresponding explicit
  invitation, client, key, and operation capabilities needed by later phases.
- Added `idpadminapp.ActionService`, a closed command registry that:
  - maps each command to one capability and target type;
  - reloads and authorizes the current grant;
  - obtains the authoritative current resource version;
  - generates create-user IDs on the server;
  - signs a five-minute session/subject/grant/scope/version-bound handle;
  - stores the nonce before returning the handle;
  - reports reason, confirmation, and fresh-auth requirements as presentation
    metadata.
- Added `idpadminapp.UserCommandService` for create, edit, enable, disable,
  unlock, set-password, and revoke-access.
- Added transaction-scoped store operations for:
  - user security-artifact revocation;
  - resource-version initialization;
  - one-user projection refresh;
  - safe projected-user lookup.
- Added `idpaccounts.PreparePassword`, so Argon password work finishes before
  the SQLite write transaction while the resulting credential is committed
  inside the protocol/admin transaction.
- Added migration 018. Administration actions now persist request ID, hashed
  session binding, scope, resulting version, operator reason, and assurance.
- Changed the action principal's session binding from the raw cookie value to a
  domain-separated HMAC digest. Raw admin session handles therefore enter
  neither signed actions nor nonce/action records.
- Extended executor results with:
  - `committed: true`;
  - `audit_status: "pending"`.

  This is the explicit committed/outbox-pending semantic: the domain mutation
  and durable audit evidence committed together, while Phase E owns delivery
  to the external audit sink.
- Added strict HTTP routes:
  - `POST /api/widget/actions/prepare`;
  - `POST /api/widget/actions/execute`;
  - `GET /api/admin/pages/{page}`.
- Both mutation routes require an authenticated current grant, exact Origin,
  session-bound CSRF, bounded JSON with unknown-field rejection, and no-store
  responses. Execution additionally requires an idempotency key.
- Added `/admin/auth/reauth`, which starts a browser-bound OIDC request with
  `prompt=login` and `max_age=0` and resumes only a validated `/admin` return
  path.
- Added React/RTK Query user flows:
  - create;
  - profile edit;
  - enable/disable;
  - unlock;
  - password replacement;
  - revoke access.

  Password controls are cleared in `finally` paths after success or failure.
  Fresh-auth errors redirect through the reauthentication route without
  retaining password input.
- Migrated CLI create, set-password, enable, and disable through the same
  action and command services. Mutations now require the dedicated action-key
  file; password replacement and state changes require operator reasons, and
  disable requires `DISABLE`.
- Closed the first-install dependency cycle by extending `console bootstrap`
  with an optional owner-only password file. It can atomically create the first
  owner account, credential, public PKCE client, owner grant, action record,
  and audit-outbox entry.

### Why

- A capability named `users.write` cannot prove that a handle was authorized
  specifically for password replacement or disabling. Command-level
  capabilities make the audit and authorization statement exact.
- Password hashing inside the one-connection SQLite transaction would hold the
  complete control-plane lock during expensive Argon work. Preparing the
  credential first and committing only its hash avoids that.
- Disabling and password replacement must revoke provider grants, codes,
  tokens, and browser sessions inside the same transaction as the user change,
  nonce, version, action, outbox, and idempotency record.
- A session cookie is bearer material. Even though the previous nonce binding
  was not returned to the UI, persisting the raw value violated the design's
  hashed-handle invariant.
- User creation cannot require a pre-existing owner on an empty installation
  without an atomic owner-provisioning path.

### What worked

- The lifecycle test performs all seven user operations and observes versions
  1 through 7.
- Two handles prepared at version 1 behave as two browser tabs: the first
  update commits version 2; the stale update returns
  `idpadminstore.ErrVersionConflict` and does not change data.
- Concurrent requests using the same handle and idempotency key both receive a
  successful result, while exactly one action and one version increment exist.
- Reusing the idempotency key with different input returns
  `ErrIdempotencyConflict`.
- Missing reasons and wrong typed confirmations fail before nonce consumption.
- A session older than five minutes cannot prepare a password action.
- Tests prove disabling and password replacement revoke seeded provider
  sessions; enabling does not reconstruct them.
- HTTP tests prove exact-origin/CSRF/session enforcement, hashed session
  binding, strict request decoding, and idempotency forwarding.
- The CLI integration test bootstraps an empty database, then creates,
  replaces the password for, disables, reads, and enables a user through the
  shared command layer.
- `pnpm --dir internal/adminweb/frontend run check` passed and rebuilt the
  fixed embedded assets.
- `go test ./... -count=1` and `make lint` passed.

### What didn't work

- The first action-service compile used an `err` variable scoped only to the
  target branch:

  ```text
  pkg/idpadminapp/actions.go:105:13: undefined: err
  pkg/idpadminapp/actions.go:106:6: undefined: err
  pkg/idpadminapp/actions.go:107:29: undefined: err
  ```

  The shared error variable is now declared before target/create resolution.

- Fetching a resulting version unconditionally broke an existing secret
  executor test whose target intentionally had no aggregate version:

  ```text
  TestExecutorNeverReplaysOneTimeSecret: Received unexpected error:
  administration record not found
  ```

  Missing versions remain valid for unversioned targets; create-user explicitly
  initializes and then reports version 1.

- The new test initially referenced the wrong existing sentinel:

  ```text
  undefined: idpadmin.ErrFreshAuthentication
  ```

  The package's established sentinel is `ErrFreshAuthRequired`.

- The first HTTP preparation request returned 400 because
  `PrepareActionRequest` had Go field names but strict decoding correctly
  rejected JSON `target_id`. Explicit JSON tags now define the contract.

- The CLI integration assertion expected lower-case JSON fields from a legacy
  `idpstore.User`, but the existing read command emits the Go field names:

  ```text
  does not contain "\"disabled\": true"
  ```

  The test now checks the actual legacy read shape. Mutation responses
  themselves use the typed lower-case administration contract.

- The first Phase C commit attempt was rejected by the pre-commit lint hook:

  ```text
  pkg/idpadminapp/executor.go:116:5: ineffectual assignment to err (ineffassign)
  internal/cmds/admin.go:347:6: func openAccountService is unused (unused)
  ```

  The result-version branch no longer assigns an error value that is never
  read, and the legacy direct account-service opener was removed after all user
  mutations moved to `adminUserRuntime`.

- The focused command work reached two consecutive implementation fixes. Per
  the repository debugging rule I stopped and emitted:

  ```text
  I think I'm stuck, let's TOUCH GRASS.
  ```

  After the required pause, the unchanged focused suite was rerun and passed
  before further work continued.

### What I learned

- The original migration 016 action row was a useful atomic skeleton but not
  sufficient audit evidence for real mutations. Append-only migration 018
  corrects it without altering a checksummed migration.
- The same concrete SQLite transaction can safely implement both protocol and
  administration projection interfaces; querying the root store from inside
  that callback would deadlock the single-connection topology.
- Projection writes must be upserts. An insert-only targeted refresh succeeds
  for create and fails every update on the user primary key.
- Action preparation is itself state-changing because it allocates a durable
  nonce, so it requires CSRF even though it does not change the target.
- Idempotency lookup must occur before nonce consumption. That ordering lets a
  completed same-key retry return its durable response while a different-key
  replay still fails.

### What was tricky to build

- Create-user handles need a target ID for useful audit evidence, but the
  browser must not choose it. The action service reserves a random target ID
  before minting the handle and the command forces account creation to use it.
- The executor increments existing versions before invoking the mutation,
  whereas creation initializes its version inside the mutation. It therefore
  reads the final version after either path before inserting action evidence.
- The frontend must clear password DOM values even when preparation—not
  execution—fails or triggers reauthentication.
- Offline CLI execution has no browser session. It uses an ephemeral random
  action binding, the active owner grant, a fresh local assurance, the same
  signed handle service, and the same durable nonce/idempotency transaction.

### What warrants a second pair of eyes

- Review which destructive user commands beyond password replacement should
  require fresh OIDC authentication. The implementation requires it for
  password replacement and access revocation; disable uses reason plus typed
  confirmation.
- Review the first-owner password-file UX and whether deployments should
  rotate that credential immediately after bootstrap.
- Review whether profile editing should expose email verification in the first
  frontend rather than only in the typed API contract.
- Review error classification before the browser tests assert every status/code
  pair.

### What should be done in the future

- Phase D should reuse `ActionService`, `Executor`, nonce/version/idempotency,
  and one-time-secret policy for invitations and clients.
- Phase E should deliver the audit outbox and change `audit_status` from
  pending only when delivery evidence exists.
- Phase F should exercise password clearing, reauthentication, stale tabs,
  focus, keyboard, and responsive behavior in a real browser.

### Code review instructions

- Start with `pkg/idpadminapp/actions.go`, then follow
  `UserCommandService.Execute` into `Executor.Execute`.
- Review migration 018 beside `idpadminstore.Action` and
  `sqlitestore.InsertAdminAction`.
- Inspect both transaction-only assertions:
  `RevokeUserSecurityArtifactsTx` and `RefreshAdminUserProjection`.
- Follow HTTP execution from `internal/adminweb/handler.go` to the shared
  service and compare it with `adminUserRuntime.execute` in
  `internal/cmds/admin.go`.
- Run:

  ```text
  go test ./pkg/idpadminapp ./internal/adminweb ./internal/cmds ./pkg/sqlitestore
  pnpm --dir internal/adminweb/frontend run check
  ```

### Technical details

```text
prepare action
  -> authenticate session and exact Origin/CSRF
  -> resolve closed command definition
  -> reload grant + check exact capability/freshness
  -> load target version (or reserve create ID)
  -> HMAC-sign immutable action claims
  -> persist H(nonce) bound to H(session)

execute action
  -> authenticate session and exact Origin/CSRF
  -> strict bounded input decode
  -> verify handle + reload grant
  -> validate reason and typed confirmation
  -> prepare password hash outside transaction, if needed
  -> BEGIN protocol/admin transaction
     -> same-key idempotency replay check
     -> consume nonce
     -> compare/increment version
     -> mutate user/credential/security artifacts
     -> refresh projected row
     -> insert complete action evidence
     -> enqueue sanitized audit event
     -> persist non-secret result
  -> COMMIT
  -> return {committed:true,audit_status:"pending",user:{...}}
```

## Step 10: Begin Phase D with transaction-safe invitation and client commands

Phase D began by extending the Phase C action envelope rather than creating
separate mutation paths. The first checkpoint implements the protocol/store
boundary, closed command registry, invitation/client command services,
production dispatcher wiring, resource versions, and focused lifecycle tests.
The frontend and CLI are deliberately still in progress at this point.

### Prompt Context

**User prompt (verbatim):** "implement the ticket, keep a detailed diary as you
work, commit at appropriate intervals."

**Assistant interpretation:** Continue through every implementation phase,
record both design reasoning and execution evidence, and checkpoint only
coherent reviewed increments.

**Inferred user intent:** Deliver the administration console described by the
ticket, not a reduced prototype or a documentation-only result.

### What I did

- Re-read the Phase D ticket requirements, current clean branch state,
  invitation CLI, legacy client service, durable invitation service, SQLite
  store, safe page models, and Phase C action/executor contracts.
- Added append-only migration
  `019_admin_invitation_lookup.sql`. It adds a unique public
  `invitation_id` lookup beside the existing private keyed code hash and
  backfills IDs from the durable JSON record.
- Extended the protocol store with `GetDurableInvitationByID` and
  `RevokeDurableInvitationByID`, implemented in SQLite and memory stores.
- Added `IssueInTransaction` and `RevokeByIDInTransaction` to
  `idpinvite.DurableService`. Code HMAC derivation remains inside
  `idpinvite`; neither the admin command layer nor metadata receives a code
  hash.
- Added transaction-scoped admin invitation metadata insertion with label,
  creator subject, creation time, and last-issued time.
- Generalized `ActionService` from user-only definitions to a closed registry
  covering:
  - `invitations.issue` and `invitations.revoke`;
  - `clients.create`, `clients.update`, `clients.enable`,
    `clients.disable`, and `clients.rotate_secret`.
- Distinguished generated targets, caller-supplied create targets, and
  existing versioned targets. Client creation checks nonexistence before
  minting; every existing invitation/client operation binds its current
  resource version.
- Added `InvitationCommandService`:
  - generated 288-bit one-time codes;
  - bounded 30-day validity;
  - audience client validation;
  - atomic durable invitation, metadata, version, action, outbox, and
    idempotency writes;
  - public-ID revocation with reason and typed `REVOKE`;
  - secret-bearing replay refusal.
- Added `ClientCommandService`:
  - create/update/enable/disable/rotate flows;
  - production-mode redirect, audience, grant, public-client, PKCE, and
    introspection validation;
  - bcrypt preparation outside the transaction;
  - immutable public/confidential client type;
  - immediate secret-hash replacement;
  - safe typed results that never include `SecretHash`;
  - secret-bearing create/rotation replay refusal.
- Added a signed-handle `CommandDispatcher` and changed the HTTP handler from a
  user-specific executor to the unified command executor.
- Made the invitation lookup key a production administration requirement,
  constructed the durable service unconditionally, and initialized versions
  for catalog clients and pre-existing durable invitations at startup.
- Added focused tests covering invitation issuance/revocation, code secrecy,
  replay refusal, client creation/update/disable/rotation, old-secret
  invalidation, new-secret verification, and unsafe redirect rejection.

### Why

- Revocation by a raw invitation code would force administrators to retain a
  bearer secret and would make list-to-revoke impossible. Resolving the public
  ID inside the protocol store preserves the secrecy boundary.
- Invitation issuance and confidential-client secret creation are committed
  mutations whose response cannot be replayed. They therefore need the
  executor's `SecretBearing` policy, not ordinary durable response caching.
- Client configuration changes affect OAuth security immediately. They need
  the same grant reload, signed authority, CAS, nonce, idempotency, action
  evidence, and audit outbox transaction as user mutations.
- Existing client catalog rows predate the admin version table. Startup
  initialization is required before an existing client can receive a signed
  update handle.

### What worked

- The invitation design required no raw-code or private-hash column in
  `admin_invitation_records`; the protocol table owns the internal mapping.
- The existing executor's secret sentinel behaved exactly as intended:
  first execution returns the secret in memory, while same-key retries receive
  `ErrSecretAlreadyIssued` and the idempotency row contains only a sentinel.
- Focused package tests passed after the store interfaces were implemented in
  both SQLite and memory:

  ```text
  ok github.com/go-go-golems/tiny-idp/pkg/idpadminapp
  ok github.com/go-go-golems/tiny-idp/pkg/idpinvite
  ok github.com/go-go-golems/tiny-idp/pkg/sqlitestore
  ok github.com/go-go-golems/tiny-idp/internal/adminweb
  ```

- The final focused Phase D command test passed:

  ```text
  ok github.com/go-go-golems/tiny-idp/pkg/idpadminapp 2.783s
  ```

### What didn't work

- The first test run never reached compilation because the default Go cache is
  outside the writable workspace:

  ```text
  open /home/manuel/.cache/go-build/...: read-only file system
  ```

  Subsequent commands use `GOCACHE=/tmp/tiny-idp-go-cache`. This is only a
  sandbox test setting; product code does not read an environment variable.

- Extending `DurableInvitationStore` initially broke the memory store's
  compile-time interface assertion:

  ```text
  *Store does not implement idpstore.Store
  (missing method GetDurableInvitationByID)
  ```

  The memory implementation now performs the same public-ID resolution and
  revocation semantics.

- The first admin metadata implementation referenced a nonexistent helper and
  omitted the `strings` import:

  ```text
  undefined: strings
  undefined: mapAdminWriteError
  ```

  It now maps SQLite uniqueness through the existing `isConstraint` helper.

- The initial leakage-test queries guessed `payload` and `response`; the
  schema names are `payload_json` and `response_json`:

  ```text
  no such column: payload
  no such column: response
  ```

  Correcting those test-only column names made the focused suite pass.

- The broad `internal/cmds` package run reached an unrelated network sandbox
  restriction:

  ```text
  httptest: failed to listen on a port:
  listen tcp6 [::1]:0: socket: operation not permitted
  ```

  That package must be rerun outside the restricted sandbox during the
  checkpoint verification.

### What I learned

- The durable invitation JSON uses exported Go field names, so migration 019
  must backfill with JSON path `$.ID`, not a guessed snake-case path.
- Action preparation needs three target modes: server-generated identity,
  caller-selected identity whose nonexistence is checked, and an existing
  versioned resource. A single `RequireTarget` flag cannot express all three.
- Secret hashes can be prepared before a short SQLite transaction without
  weakening CAS because the executor repeats authorization and performs the
  version comparison inside the transaction.
- `INSERT OR REPLACE` for clients makes the aggregate version table the
  authoritative concurrency guard; client timestamps alone cannot prevent
  stale-tab overwrites.

### What is next

- Phase E will add key and operational workflows, outbox delivery, and health
  reporting on top of the same executor.

### Phase D continuation and completion

After the repository-mandated debugging pause, the callback-scoped SQLite
query fix was rerun without further edits. The CLI lifecycle suite passed,
proving that the hang was caused by querying the root one-connection store
from inside `AdminUpdate`, not by invitation or client command logic:

```text
PASS TestAdminClientMutationsUseGuardedCommandLayer
PASS TestAdminInvitationIssueAndRevokeUseGuardedCommandLayer
PASS TestAdminUserMutationsUseGuardedCommandLayer
ok github.com/go-go-golems/tiny-idp/internal/cmds 2.807s
```

The remaining Phase D work then added:

- `InvitationManagement.tsx` with issue/list/revoke forms, an explicit
  one-time code warning, and a clear action;
- `ClientManagement.tsx` with multiline repeatable URI/scope/grant/audience
  inputs, create/edit/enable/disable/rotate operations, and one-time secret
  handling;
- immediate RTK Query mutation reset after extracting a one-time response, so
  the Redux cache never retains the code or secret;
- an authenticated, capability-checked safe client-detail endpoint which
  returns `ClientDetail` and never a hash;
- `ValidationError` and HTTP 422 `field_errors`, including exact
  `redirect_uris` attribution for wildcard/invalid/HTTP/fragment failures;
- shared guarded CLI implementations for invitation and client mutations;
- generated confidential-client secrets only. The old CLI secret flag/file
  path and its platform-specific readers were removed rather than retained as
  a compatibility route;
- integration tests that prove CLI mutations create action evidence and
  resource-version progress.

The frontend gate passed:

```text
pnpm --dir internal/adminweb/frontend run check
tsc --noEmit
vite build
✓ built in 527ms
```

The complete lint gate passed with zero issues. The first sandboxed invocation
could not download the pinned `glazed-lint` because DNS/socket access is
restricted, so the approved out-of-sandbox retry was used:

```text
Issues before processing: 150, after processing: 0
0 issues.
```

The first full test run found that the external two-process production harness
still launched `serve-production` without the Phase B administration key
files. It failed before readiness with:

```text
required field(s) missing:
production.admin-auth-key-file, production.admin-action-key-file
```

The harness now creates owner-only auth, action, and invitation key files and
passes them explicitly. Its affected package passed:

```text
ok .../scripts/01-two-process-harness 16.841s
```

Making the invitation lookup key an unconditional administration requirement
also invalidated a legacy schema assertion that described it as conditional.
The help text, required flag definition, and assertion now state its dual use
for durable signup and console issuance. The production command and section
tests passed after that intentional contract update.

### Phase D review instructions

- Follow `ActionService.Prepare` through `CommandDispatcher` into
  `InvitationCommandService` and `ClientCommandService`.
- Confirm migration 019 exposes only public invitation IDs and that keyed code
  hashes remain in `durable_invitations`.
- Inspect `Executor` secret-bearing response storage and the leakage assertions
  against actions, outbox, and idempotency tables.
- Compare the React one-time response handling with `execution.reset()` in
  both resource components.
- Run the frontend check, full Go suite, `make lint`, and the pre-commit hook.

The final full repository test rerun passed, including the real two-process
production harness and every package under `./...`:

```text
GOCACHE=/tmp/tiny-idp-go-cache go test ./... -count=1
exit 0
```

This establishes that Phase D's required production keys, migration 019,
expanded store interface, command routing, and removed client-secret-file CLI
surface do not leave a failing consumer elsewhere in the repository.

The first staged hook invocation inherited the sandbox's read-only default Go
cache and restricted DNS, so both hooks failed before analyzing the change.
The approved rerun set the isolated cache and executed outside that restriction:

```text
GOCACHE=/tmp/tiny-idp-go-cache lefthook run pre-commit
✔ lint
✔ test
```

The separate build gate passed with `-buildvcs=false`; that flag suppresses
only Go's unavailable linked-worktree VCS stamp and does not change compiled
application behavior.

## Step 11: Establish the Phase E durable worker substrate

Phase E began by extending the existing administration outbox and operations
tables rather than creating a second queueing subsystem. Migration 020 adds an
indexed `next_attempt_at_ns` deadline to each audit record and records the
actor, command, bounded label, result path, and start time for each operation.
The outbox also has a unique action index: one committed administration action
has exactly one durable audit envelope.

The `idpadminstore.WorkerStore` contract now separates asynchronous lifecycle
operations from the browser-facing read and transaction contracts. Its SQLite
implementation provides:

- bounded due-record selection ordered by retry time and creation time;
- compare-by-state delivery and operation transitions, so a second delivery
  acknowledgement or terminal rewrite cannot silently succeed;
- durable retry attempts, stable safe error codes, and a next-attempt time;
- aggregate pending count, oldest pending time, and last safe error for
  readiness and the Operations page;
- pending operation creation, single-winner claiming, and terminal completion
  or failure.

Focused persistence tests prove that a failed audit item disappears until its
retry deadline, reappears with its stable error, contributes to health, and
cannot be acknowledged twice. They also prove that an operation has one claim
winner and cannot move from completed to failed.

The new `idpadminapp.OutboxWorker` converts the bounded JSON envelope emitted
by `Executor` into the existing `idp.Event` contract. It never adds response
bodies or secrets. Delivery failure is recorded as
`audit_delivery_failed`; retries use exponential backoff capped at one minute.
Readiness is degraded while any audit is pending and becomes not ready when
the oldest pending item reaches five minutes. Cancellation returns normally,
allowing the production host's `errgroup` to cancel and join the worker.

The first focused persistence test failed with:

```text
FOREIGN KEY constraint failed
```

That was a fixture error, not a store error: `admin_audit_outbox.action_id`
correctly requires an `admin_actions` parent. The test now creates a real
owner grant and action before enqueueing the audit record. The complete focused
worker and SQLite suites then passed:

```text
ok github.com/go-go-golems/tiny-idp/pkg/idpadminapp
ok github.com/go-go-golems/tiny-idp/pkg/sqlitestore
```

The first checkpoint commit hook found one static-analysis issue in
`OutboxWorker.Run`: an intentionally empty branch documented that transient
drain errors do not terminate the long-lived worker. Replacing it with an
explicit ignored assignment preserves that policy and removes the empty
control-flow construct. The hook's package test suite had already passed.

### What is next

- Add guarded signing-key rotate and retire commands; purge remains absent
  from the browser action registry.
- Add managed doctor, backup, verification, and one-use sanitized diagnostics
  operations under an explicitly configured and confinement-checked root.
- Wire both workers into the production `errgroup`, combine outbox health with
  readiness, and expose only safe operation metadata to the React console.

## Step 12: Move signing-key rotation and retirement behind the control plane

The signing-key slice now uses the same signed-action path as users,
invitations, and clients. `ActionService` defines only `keys.rotate` and
`keys.retire` for browser dispatch. Both require fresh authentication, an
operator reason, and exact typed confirmation. Rotation targets the fixed
`keyring/system` aggregate, while retirement targets an existing
`signing_key/<kid>` resource. This distinction prevents the client from
choosing an invented keyring identity while retaining per-key stale-tab
protection.

`InitializeAdminResourceVersions` now seeds:

- the singleton `keyring/system` aggregate used to serialize rotations;
- one `signing_key/<kid>` version for every published verification key.

`KeyCommandService` generates the RSA private key before entering the short
SQLite transaction. Inside the executor callback it creates and activates the
new key, retires the prior active key without deleting its verification
material, creates the new key's version, and commits action evidence,
idempotency, and audit outbox state atomically. Responses use only
`idpadmin.SigningKeyRow`; private PEM bytes cannot enter JSON, audit, or the
Redux request cache.

Retirement reloads the key from the transaction-scoped verification set,
rejects an active key through the protocol store invariant, and executes under
the per-key expected version. There is deliberately no `keys.purge` definition
and no dispatcher branch for purge. `DeleteRetiredSigningKey` remains reachable
only through `admin keys purge-retired`.

The ordinary CLI `rotate` and `retire` commands now require the owner-only
action key and call the shared action and command services. The CLI integration
test verifies two resulting `keys.*` action rows. It then runs emergency purge
and verifies that the action count remains two, proving purge did not acquire
a browser/control-plane route accidentally.

The React signing-key panel:

- lists only the already-redacted page projection;
- explains verification overlap and the CLI-only emergency boundary;
- submits rotation and retirement through prepare/execute;
- asks the operator to type `ROTATE` or `RETIRE`;
- redirects to reauthentication on `fresh_auth_required`;
- resets RTK Query mutation state after every attempt.

Focused tests prove rotation keeps two verification keys, changes the active
key, returns no private PEM, records retirement metadata, rejects stale
authentication, and rejects missing reason or wrong confirmation. A source
search found no purge/delete command in `pkg/idpadminapp` or the browser
implementation; the only match is the explicit negative registry test.

Validation completed:

```text
pnpm --dir internal/adminweb/frontend run check
✓ tsc --noEmit
✓ vite build

go test ./pkg/idpadminapp -run TestSigningKey -count=1
ok

go test ./internal/cmds \
  -run TestAdminKeyRotationAndRetirementUseGuardedCommandLayer -count=1
ok

go test ./... -run '^$' -count=1
all packages compile
```

The broader sandboxed `internal/cmds` test run reached the known environment
restriction in `httptest.NewServer`:

```text
listen tcp6 [::1]:0: socket: operation not permitted
```

No product assertion failed before that point. The complete suite will be run
outside the socket sandbox at the Phase E gate.

The first lint invocation was also unable to resolve the pinned
`glazed-lint` module through the restricted DNS socket. The approved
out-of-sandbox rerun completed with `0 issues`, including both custom vet
analyzers.

### What is next

- Add durable doctor, managed backup create/verify, and sanitized one-use
  diagnostics operations with strict backup-root confinement.
- Run operation and outbox workers under the production host `errgroup`, then
  expose their safe state in readiness and the Operations page.

## Step 13: Implement managed operations, confined artifacts, and one-use downloads

The long-running operation slice now uses durable `admin_operations` rows.
The browser can prepare only four commands:

- `operations.doctor`;
- `operations.backup.create`;
- `operations.backup.verify`;
- `operations.diagnostics`.

Backup creation, verification, and diagnostics require fresh authentication,
an operator reason, and exact typed confirmation. Doctor is read-only and may
be queued under the doctor capability without step-up. Each accepted request
creates the operation row, operation resource version, action evidence,
idempotency receipt, and audit outbox record in the executor's single SQLite
transaction. Backup verification targets and version-checks the completed
source operation while receiving its own generated operation identity.

There are no action definitions for restore, migration, or key purge. A
negative registry test asks `ActionService` to prepare each break-glass
command and requires `ErrUnknownCommand`.

### Managed root and path confinement

Production now requires the explicit Glazed field `--admin-backup-root`.
Startup creates it owner-only, resolves its canonical path, and refuses an
empty value. Browser input contains only an optional 80-character label. The
server:

1. replaces all non `[A-Za-z0-9._-]` label characters;
2. combines the sanitized label with the random operation ID;
3. chooses the fixed `backups/` or `diagnostics/` child directory;
4. creates/chmods the child directory to `0700`;
5. resolves the canonical parent and verifies `filepath.Rel` remains below
   the canonical root;
6. stores only the slash-normalized relative artifact path.

Verification never accepts a browser path. It loads the relative path from a
completed managed backup operation, rejects absolute or `..` paths, resolves
symlinks, verifies confinement again, and then invokes SQLite's read-only
integrity/schema/checksum verification.

The confinement tests use both an input label containing `../../` and a
`backups` symlink pointing to an outside directory. The label is sanitized and
the backup remains inside the root; the symlink case is rejected before the
backup writer creates an outside file. A forged stored `../escape.db` source
path is also rejected.

### Diagnostics and download capability

Diagnostics reuse the existing doctor checks but serialize new safe DTOs:
redacted client rows, redacted signing-key rows, and the safe operations/outbox
view. They do not serialize `idpstore.Client`, private PEM bytes, database
paths, hashes, cookies, action keys, or raw error objects. The artifact is
created with `O_EXCL`, mode `0600`, fsynced, and recorded by relative path.

Download authorization is intentionally two-step:

```text
authenticated + CSRF POST operation/{id}/downloads
    -> random 288-bit handle returned once with no-store
    -> only SHA-256(handle) is persisted

authenticated GET downloads/{raw handle}
    -> atomic UPDATE ... WHERE consumed_at IS NULL AND expires_at > now
    -> canonical confinement check
    -> attachment response with no-store
```

The handle expires after ten minutes. A second consume returns
`idpadminstore.ErrNotFound`. The test reads the generated artifact and asserts
that neither `PRIVATE KEY` nor the Go field name `SecretHash` appears.

Migration 021 adds the hashed one-use download table with operation ownership,
relative path, bounded server-controlled media metadata, expiry, and consumed
time. No plaintext handle is stored in the database or operation result.

### Worker lifecycle and health

The operation worker claims one pending row with a conditional state update,
runs a bounded batch, and writes either a safe result or a stable error code
such as `backup_create_failed`. On startup it requeues rows left `running` by
a process interruption. On graceful cancellation it leaves the current row
running; the next startup performs that recovery rather than attempting a
database write through an already-cancelled context.

The production host now launches both the audit outbox and operation workers
in the same `errgroup` as its HTTP servers and maintenance loop. Cancellation
therefore stops and joins both workers before the store and audit sink close.
Readiness includes:

- `admin_audit_outbox`, degraded for pending delivery and failed after five
  minutes;
- `admin_operations`, degraded for queued/running work and failed when the
  oldest active operation reaches five minutes.

The Operations page polls the safe read model, exposes the pending-audit count
and last stable audit error, queues doctor/backup/diagnostics work, offers
verification only for completed managed backups, and prepares one-use
diagnostics downloads. Its copy explicitly states that restore and migration
remain CLI-only.

### Validation and failures

Focused backend, storage, HTTP, production-section, and frontend gates passed:

```text
pnpm --dir internal/adminweb/frontend run check
✓ tsc --noEmit
✓ vite build

go test ./pkg/idpadminapp ./pkg/sqlitestore \
  ./internal/adminweb ./internal/sections/production -count=1
ok

go test ./... -run '^$' -count=1
all packages compile

make lint
0 issues
```

As in earlier phases, the first sandboxed lint call could not resolve the
pinned analyzer through restricted DNS. The approved rerun passed.

The full repository suite passed every package except one pre-existing
linearizability test, which observed one active refresh token instead of zero:

```text
TestSQLiteRefreshRotationHistoryIsLinearizableAndReuseRevokesFamily
count=1, want 0
```

No Phase E package failed, and the real two-process production harness passed
with the newly required managed root. The exact failing concurrency test
passed immediately when rerun alone without any code change, identifying a
flaky interleaving rather than an administration regression. The complete
suite will be rerun again before Phase E is closed.

### What is next

- Add or refine production-facing operation/readiness tests and verify
  cancellation/join behavior at the host boundary.
- Update ticket tasks and changelog, then run the final Phase E gates and
  commit the managed-operation checkpoint.

## Step 14: Harden the console and prove the release gates

Phase F closes the implementation with deterministic Widget IR validation,
real-browser accessibility and security checks, an exercised failure-state
matrix, a stable tablet image, and a measured 10,000-user query benchmark. The
browser suite runs against the production Vite bundle because that is the
artifact embedded into the Go binary and is the correct compatibility boundary
for the published Widget renderer.

The final repository-wide acceptance run completed successfully. Generation,
formatting, every Go test package, the exact full build, lint plus both custom
analyzers, frontend typecheck/build, and all six Playwright tests passed. This
checkpoint is committed as `972a0a3`.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Finish the implementation rather than stopping at
the functional backend, preserve detailed debugging and validation evidence,
and commit a release-ready Phase F checkpoint.

**Inferred user intent:** Make the implemented ticket withstand a
requirement-by-requirement review, including browser behavior and reproducible
release commands, while leaving a new intern enough context to understand why
each hardening mechanism exists.

**Commit (code):** `972a0a3` — `test(admin): complete console hardening gates`

### What I did

- Added Playwright and Axe dependencies, a production-bundle test server, and a
  six-test browser suite.
- Exercised all seven screens across loading, empty, no-results, forbidden,
  stale, session-expired, audit-degraded, and operation-failed states: 56
  rendered combinations.
- Verified keyboard focus starts at the skip link, semantic landmarks and
  headings exist, and Axe reports no violations.
- Added tablet and emergency small-screen checks:
  - tablet navigation remains usable;
  - reduced-motion CSS is active;
  - mutation surfaces disappear below 480 pixels while read-only information
    and a warning remain visible.
- Added a committed tablet screenshot baseline.
- Added the exact administration CSP to the production preview and verified in
  Chromium that inline and external scripts are rejected.
- Tightened HTTP contract tests for every CSP directive, absence of
  `unsafe-inline`, `unsafe-eval`, and remote script origins, mutation
  `Cache-Control: no-store`, unknown JSON fields, and body-size limits.
- Added `object-src 'none'` to the production CSP to match the imported UX
  contract.
- Fixed Widget DSL row normalization: xgoja proxy row objects are now copied
  into plain objects before passing them to `DataTable`, so IDs and safe display
  values survive JSON serialization.
- Added deterministic Widget IR checks for:
  - schema version, page ID, title, and component root;
  - byte, node, depth, key, and string bounds;
  - a closed component allowlist;
  - forbidden authority, SQL, code, raw-secret/hash/private-key properties;
  - external, JavaScript, and inline-script values.
- Rendered every screen/state fixture through the same xgoja provider used in
  production and asserted that normalized rows survive.
- Added and ran `BenchmarkListAdminUsers10000`.
- Ran the complete release commands and committed the resulting frontend
  embedded assets and logcopter stubs.

### Why

- Widget IR is presentation input crossing a JavaScript-to-Go-to-browser
  boundary. Schema validation and a closed component/content policy prevent it
  from becoming a second authorization, code-execution, or secret transport
  channel.
- Declaring fixture names is weaker than rendering them. The 56-case matrix
  proves each required state reaches every screen.
- CSP string tests prove the server contract, while Chromium tests prove the
  browser actually enforces the policy.
- The 10,000-user benchmark turns the UX scale target into reproducible
  evidence rather than an assumption.

### What worked

- The final Playwright run completed:

  ```text
  Running 6 tests using 1 worker
  6 passed (14.8s)
  ```

- The benchmark completed at:

  ```text
  BenchmarkListAdminUsers10000-8  20  112868 ns/op  17505 B/op  458 allocs/op
  ```

- The exact full Go suite completed with every package passing, including the
  previously flaky Fosite linearizability package:

  ```text
  GOCACHE=/tmp/tiny-idp-go-cache go test ./... -count=1
  Process exited with code 0
  ```

- `go generate ./...`, `go fmt ./...`, `go build ./...`, `pnpm run check`, and
  `make lint` all completed successfully.
- The pre-commit hook independently reran package tests and lint and reported:

  ```text
  0 issues.
  ✔️ lint
  ✔️ test
  ```

### What didn't work

- The first browser server readiness URL used `/admin`, which Vite does not
  serve. Playwright timed out waiting for it. The readiness URL was corrected
  to `/static/admin/`, matching the configured Vite base.
- Fetching transformed development HTML still produced a blank application.
  The browser trace revealed the actual error:

  ```text
  Calling `require` for "react" in an environment that doesn't expose the `require` function.
  See https://rolldown.rs/in-depth/bundling-cjs#require-external-modules
  ```

  The published `@go-go-golems/rag-evaluation-site` dependency contains a
  CommonJS React external that Vite's development dependency optimizer does not
  expose correctly. The production Vite build already bundles it correctly, so
  Playwright now builds and serves `vite preview`.
- The first snapshot invocation accidentally passed an extra `--` through
  pnpm:

  ```text
  Error: No tests found.
  ```

  Running `pnpm exec playwright test --update-snapshots` passed the flag
  directly.
- The initial keyboard assertion expected the console-brand link to receive
  first focus. The actual and desirable first target is `Skip to content`.
- Axe found three Bootstrap color combinations below WCAG AA and a missing
  page-level heading. The heading and scoped darker primary colors fixed the
  production UI; the final Axe scan is clean.
- A request event is visible to Playwright even when Chromium blocks an
  external script under CSP. The reliable browser evidence is the rejected
  `addScriptTag` operation whose error names the Content Security Policy, not
  absence of the DevTools request event.
- The first formatting invocation used the read-only default Go cache:

  ```text
  pattern ./...: open /home/manuel/.cache/go-build/...: read-only file system
  ```

  Re-running with `GOCACHE=/tmp/tiny-idp-go-cache` completed.
- The first sandboxed build could not read linked-worktree VCS metadata:

  ```text
  error obtaining VCS status: exit status 128
  ```

  The exact `go build ./...` command passed with approved access to that
  metadata; VCS stamping was not disabled.

### What I learned

- Browser acceptance should exercise the built artifact for packages whose
  development-module shape differs from their production bundle.
- xgoja array elements can be proxy objects that look correct to JavaScript but
  become empty objects when the Widget builder serializes them. Copying a
  fixed safe field set into plain objects both fixes serialization and narrows
  the data boundary.
- Bootstrap 5's default primary color narrowly misses AA contrast on the
  tertiary and widget backgrounds at normal text size. A scoped darker primary
  is required for this console.
- CSP enforcement is best proven at two layers: exact server headers and actual
  browser rejection.

### What was tricky to build

- The blank browser page initially resembled a routing problem because the
  application is mounted at `/admin` while assets live under `/static/admin`.
  Network traces proved the HTML and every transformed module loaded with 200
  responses; the page error then isolated the CommonJS external. Switching to
  the production artifact solved the actual compatibility boundary without
  adding a backwards-compatibility adapter.
- Screenshot updates had to happen only after semantic and contrast changes
  stabilized. The final run without `--update-snapshots` proves the committed
  baseline is reproducible.
- The all-state matrix must isolate routes between iterations. Each fixture
  waits for route teardown before installing the next state, preventing a
  prior screen's mocks from weakening later assertions.

### What warrants a second pair of eyes

- Review the Widget IR forbidden-property list when adding new upstream widget
  components. New fields should be deliberately allowed, not accepted by
  default.
- Review the snapshot on a reMarkable or desktop display for hierarchy and
  density; pixel stability does not replace human design review.
- The benchmark is a bounded query microbenchmark, not a concurrent production
  load test. Re-run it when query filters, indexes, or SQLite connection policy
  change.

### What should be done in the future

- If the upstream Widget package becomes natively ESM-compatible in Vite dev
  mode, the Playwright production-bundle harness should still remain because it
  verifies the artifact actually shipped.
- Add desktop snapshots if future layout work introduces desktop-only
  branching; the current tablet baseline covers the MVP's required responsive
  breakpoint.

### Code review instructions

- Start with:
  - `internal/adminweb/widget_runtime.go`;
  - `internal/adminweb/verbs/pages.js`;
  - `internal/adminweb/frontend/tests/admin-console.spec.ts`;
  - `internal/adminweb/handler_test.go`;
  - `pkg/sqlitestore/admin_queries_benchmark_test.go`.
- Inspect the committed image at
  `internal/adminweb/frontend/tests/admin-console.spec.ts-snapshots/operations-tablet-linux.png`.
- Reproduce the focused acceptance evidence:

  ```text
  GOCACHE=/tmp/tiny-idp-go-cache go test ./internal/adminweb ./pkg/idpadminapp ./pkg/sqlitestore -count=1
  GOCACHE=/tmp/tiny-idp-go-cache go test ./pkg/sqlitestore -run '^$' -bench '^BenchmarkListAdminUsers10000$' -benchmem -benchtime=20x
  pnpm --dir internal/adminweb/frontend run check
  pnpm --dir internal/adminweb/frontend exec playwright test
  ```

- Reproduce the release gate:

  ```text
  GOCACHE=/tmp/tiny-idp-go-cache go generate ./...
  GOCACHE=/tmp/tiny-idp-go-cache go fmt ./...
  GOCACHE=/tmp/tiny-idp-go-cache go test ./... -count=1
  GOCACHE=/tmp/tiny-idp-go-cache go build ./...
  make lint
  ```

### Technical details

The browser acceptance topology is:

```text
Playwright Chromium
    |
    +-- /admin/<screen> document
    |      `-- route to production index at /static/admin/
    |
    +-- hashed/bundled assets from Vite preview
    |
    +-- mocked authenticated JSON APIs
    |
    `-- reviewed CSP response headers
            +-- inline script rejected
            `-- external script rejected
```

The Widget IR release boundary is:

```text
safe Go page DTO
    -> tinyidp.admin.pageData()
    -> reviewed pages.js field copy
    -> widget.dsl typed builders
    -> JSON normalization and deterministic validator
    -> defaultWidgetRegistry in React
```

## Step 15: Deliver the implemented ticket

The completed intern guide was opened through `md-view` after the
implementation and evidence commits, making the rendered document available
for immediate local review. The updated ticket bundle was then rendered and
uploaded to reMarkable as one PDF containing the index, implementation guide,
investigation diary, task state, and changelog.

This delivery happened only after a clean worktree check and a final
`docmgr doctor` pass. The uploaded bundle therefore corresponds to commits
`972a0a3` and `fe5fe5a`, including the Phase F implementation and its
requirement-by-requirement acceptance audit.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Make the finished implementation and its evidence
easy to inspect locally and on reMarkable, then preserve the delivery result in
the ticket diary.

**Inferred user intent:** End with durable, reviewable code and documentation,
not an implementation that exists only in an agent session.

### What I did

- Ran `docmgr doctor --ticket TINYIDP-ADMIN-CONSOLE-001 --stale-after 30`.
- Confirmed all doctor checks passed and the worktree was clean.
- Opened the implementation guide with:

  ```text
  md-view view ttmp/.../design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md
  ```

- Uploaded the index, design, diary, tasks, and changelog as a ToC-enabled PDF.

### Why

- Rendering catches structural Markdown problems that source inspection can
  miss and gives the requested local review surface.
- Bundling the guide with its diary and acceptance state preserves both the
  architecture and the evidence behind it.

### What worked

- `md-view` returned:

  ```text
  http://localhost:37221/render?file=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md
  ```

- reMarkable upload returned:

  ```text
  OK: uploaded TINYIDP ADMIN CONSOLE MVP IMPLEMENTED.pdf -> /ai/2026/07/24/TINYIDP-ADMIN-CONSOLE-001
  ```

### What didn't work

- N/A

### What I learned

- The reMarkable uploader can render the complete long-form ticket, including
  the large implementation diary, in one non-interactive bundle.

### What was tricky to build

- Delivery ordering matters: uploading before the release evidence exists
  would leave the remote guide looking complete without the commands and
  acceptance mapping that prove it. The implementation, evidence commit,
  doctor check, local render, and upload were therefore performed in that
  order.

### What warrants a second pair of eyes

- Human review should focus on the design's five non-blocking owner decisions
  and on the tablet snapshot's visual hierarchy.

### What should be done in the future

- N/A

### Code review instructions

- Review implementation commit `972a0a3`.
- Review evidence/documentation commit `fe5fe5a`.
- Open the local guide through the `md-view` URL above or read the uploaded PDF
  under `/ai/2026/07/24/TINYIDP-ADMIN-CONSOLE-001`.

### Technical details

The uploaded document is:

```text
TINYIDP ADMIN CONSOLE MVP IMPLEMENTED.pdf
remote directory: /ai/2026/07/24/TINYIDP-ADMIN-CONSOLE-001
ToC depth: 2
```

## Step 16: Establish the manifest-driven devctl control plane

This step began the repository-wide environment-platform implementation by
turning the design's profile model into executable devctl configuration. The
first slice deliberately contains no live Vault writes, Docker volume changes,
or destructive state operations. It establishes the protocol, manifest, and
planning contracts that later secret and PKI commands can depend on.

Nine profiles now describe the embedded provider, external Message Desk,
shared HTTPS applications, Jitsi, the administration console, the direct
message application, script tools, configuration fixtures, and the production
host reference. A Python protocol-v2 plugin loads and validates these manifests,
publishes stable configuration, computes launch plans, supports named prepare
steps, and exposes the closed dynamic-command vocabulary from the design.

### Prompt Context

**User prompt (verbatim):** "Ok, now implement the ticket, keep a detailed diary frequently, check off tasks as you go, commit at appropriate intervals."

**Assistant interpretation:** Implement every outstanding environment-platform
task in the current ticket, record the work and failures as it happens, check
tasks only against concrete evidence, and create focused commits at verified
milestones.

**Inferred user intent:** Replace the collection of ad hoc TinyIDP demo startup
paths with a secure, maintainable operational platform that another engineer
can understand, run, recover, and extend.

**Commit (code):** `655cd80` — "feat(dev): add manifest-driven environment control plane"

**Commit (hygiene):** `e46aef6` — "chore(dev): ignore Python bytecode caches"

### What I did

- Audited the clean `feat/tinyidp-admin-console` branch at commit `bd9aad2`.
- Read the complete ticket diary, task list, unified design, installed devctl
  help, representative devctl plugins, and the required diary workflow.
- Added `.devctl.yaml` with nine named profiles and a documented default.
- Added `devctl/tinyidp.py`, the protocol-v2 plugin.
- Added `devctl/lib/manifests.py`, a strict YAML loader with repository path
  confinement and production-shaped HTTPS/Vault invariants.
- Added one manifest per environment under `dev/environments`.
- Added plugin and manifest unit tests.
- Ignored devctl runtime state, local overrides, and direct-profile state.
- Ran:
  - `python3 -m unittest discover -s devctl/tests -v`;
  - `python3 -m py_compile devctl/tinyidp.py devctl/lib/manifests.py`;
  - `devctl profiles list`;
  - `devctl plugins list`;
  - `devctl plan --profile embedded`;
  - `devctl validate --profile embedded`.

### Why

- Secret retrieval, CA recovery, and state reset are high-impact operations.
  They require a deterministic selected-environment contract before they can be
  implemented safely.
- A profile manifest separates non-secret topology facts from operational
  behavior while keeping OIDC issuer identity explicit.
- Dry-run behavior and path confinement need unit tests before plugin commands
  are allowed to mutate host or Docker state.

### What worked

- devctl discovered all nine profiles and the single `tinyidp` plugin.
- The plugin handshake registered `config.mutate`, `validate.run`,
  `prepare.run`, `launch.plan`, and `command.run`.
- `devctl plan --profile embedded` produced the expected `go run
  ./examples/embedded` service and loopback health URL.
- `devctl validate --profile embedded` passed and emitted the intended
  development-only warning.
- The dry-run prepare test proved it did not create
  `var/devctl/message-app`.

### What didn't work

The first unit-test run failed:

```text
test_production_shaped_profile_requires_https ... FAIL
devctl.lib.manifests.ManifestError: runtime.services[0].command must be a list of non-empty strings
AssertionError: "HTTPS issuer" does not match "runtime.services[0].command must be a list of non-empty strings"
```

The test fixture used `command: [false]`. YAML interpreted the unquoted scalar
as Boolean `false`, so the command validator correctly rejected it before
reaching the HTTPS assertion. The fixture was changed to `command: ["false"]`.

The first implementation commit also included generated `__pycache__/*.pyc`
files because the repository had no Python bytecode ignore rule. Rather than
rewriting the published interval, commit `e46aef6` removed the six generated
files and added:

```text
__pycache__/
*.py[cod]
```

### What I learned

- YAML scalar typing is part of the manifest security boundary. Command argv is
  intentionally strings-only, which catches accidental booleans and numbers
  before they reach `subprocess`.
- The installed devctl registers plugin commands directly as `devctl <name>`;
  there is no `devctl command <name>` wrapper.
- Profile environment values reach the plugin process, allowing
  `TINYIDP_DEV_MANIFEST` to remain a non-secret selector.

### What was tricky to build

- `config.mutate` runs before other operations, so the manifest selector must
  work from the profile process environment initially and from merged config
  on subsequent calls.
- Paths supplied by manifests must remain inside `ctx.repo_root`. Resolving
  first and then checking `relative_to(root)` prevents `..` escapes and
  symlink-based escape paths.
- Utility profiles legitimately produce no services, whereas process and
  Compose profiles must produce at least one.

### What warrants a second pair of eyes

- Review whether PyYAML should become an explicitly pinned developer
  dependency or whether the loader should move into a Go plugin later.
- Review each manifest's classification and whether every current example has
  the correct primary mode.
- Confirm that a single supervised `docker compose up` process gives the
  desired devctl stop semantics for all Compose versions in use.

### What should be done in the future

- Add Vault/PKI command implementations behind the closed dynamic command set.
- Add the missing administration-console Compose example before that profile
  can validate.
- Run plan and validation against every profile, then start real servers in
  tmux for runtime acceptance.

### Code review instructions

- Start with `.devctl.yaml`, then read `devctl/lib/manifests.py` before
  `devctl/tinyidp.py`.
- Compare each `dev/environments/*.yaml` entry with its existing example
  README and launch command.
- Run `python3 -m unittest discover -s devctl/tests -v`.
- Run `devctl profiles list`, `devctl plugins list`, and
  `devctl plan --profile embedded`.

### Technical details

The initial stable configuration namespace is:

```text
env.TINYIDP_DEV_MANIFEST
environment.name
environment.classification
environment.runtime_kind
environment.issuer
vault.*
services.<name>.health_url
```

## Step 17: Materialize Vault secrets and prove Caddy CA recovery

This step implemented the security-sensitive half of the environment platform:
validated application-secret materialization and a guarded, versioned backup
and staging-restore path for Caddy's complete local authority. The live Caddy
volume remains authoritative during normal operation; Vault now contains the
requested disaster-recovery copy.

The implementation was exercised against the actual authenticated
`scapegoat.dev` Vault and retained `tinyidp-local-caddy-pki` volume. A live
storage export was written to KV v2 with CAS zero as version 1, restored into a
new staging volume, re-exported, and verified to contain the identical root
authority. The temporary recovery volume was deleted only after verification.

### Prompt Context

**User prompt (verbatim):** (same as Step 16)

**Assistant interpretation:** Add secure Vault-backed application secrets and
real Caddy private-CA safekeeping, prove recovery without touching live state,
and preserve detailed operational evidence.

**Inferred user intent:** Make local identity environments recoverable across
workstation or Docker-volume loss without normal startup gaining authority to
replace the trusted CA.

**Commit (code):** `e3b0d5d` — "feat(dev): add Vault secret and Caddy recovery operations"

### What I did

- Extended environment manifests with typed secret-file contracts:
  - Vault source record;
  - source field;
  - target filename;
  - text/base64 encoding;
  - exact or minimum decoded byte length.
- Added `devctl/operations.py` with:
  - Vault authentication checks;
  - KV v2 reads and version parsing;
  - owner-only materialization generations;
  - atomic `runtime/secrets` symlink promotion;
  - Caddy storage archive structural validation;
  - KV v2 CAS backup;
  - explicit-version, fingerprint-checked staging restore;
  - nonempty and live-volume restore refusal.
- Added unit tests for malformed lengths, atomic promotion, archive completeness,
  first-backup initialization, CAS zero, and Docker argv.
- Confirmed the real Vault server exposes KV v2 at `kv/`.
- Confirmed the retained Caddy volume labels:

  ```json
  {"dev.wesen.purpose":"local-caddy-pki","dev.wesen.retention":"manual-delete-only"}
  ```

- Performed the first live backup:

  ```text
  path: tiny-idp/dev/_shared/caddy-local/pki-storage
  Vault version: 1
  root_sha256: 2ecbd59744bea116724ace518779c5b88585571cac70a301d65615f90397f362
  archive_sha256: fea8c60f0ed02115bf6aac0fa431a7e131349fccf5eec9cb1a51b8bb34bac40f
  archive bytes: 24576
  ```

- Restored version 1 into
  `tinyidp-caddy-recovery-test-20260724`, verified the same root fingerprint,
  and removed that staging volume.
- Checked task `umal` only after the live recovery drill succeeded.

### Why

- Copying only `root.key` would omit the intermediate key, certificates, and
  storage metadata that define the working authority.
- KV v2 CAS prevents an unobserved writer from replacing the recovery record.
- A staging-only restore with an expected root fingerprint prevents ordinary
  startup and accidental operator commands from overwriting the live trusted
  authority.
- Atomic generation promotion ensures malformed or incomplete Vault records do
  not partially replace the last complete materialization.

### What worked

- Fourteen local unit tests passed.
- The live archive contained exactly one root certificate/key and intermediate
  certificate/key.
- `devctl pki-backup --profile shared-two-apps -- --initialize` created Vault
  version 1 without putting the archive in argv or terminal output.
- `devctl pki-restore` imported the selected version into an empty staging
  volume and reproduced the root fingerprint.
- The re-exported archive digest changed, as tar/storage serialization may
  change, while the authority fingerprint remained stable. The restore
  acceptance condition correctly uses the authority fingerprint.

### What didn't work

The first successful-backup unit test contained an incorrect assertion:

```text
AssertionError: 'archive' unexpectedly found in
'{"schema_version": 1, "format": "caddy-storage-export-tar",
"archive_b64": "YXJjaGl2ZQ==", ...}'
```

It intended to prove raw archive bytes were not exposed, but searched JSON
field names containing the word `archive`. The assertion now checks that the
payload is the expected base64 encoding.

The first live export attempt failed generically:

```text
operations.OperationError: Caddy storage export failed
```

The targeted diagnostic produced:

```text
docker: Error response from daemon: ... exec: "storage": executable file not found in $PATH
```

Inspection showed the local `caddy:2.10.2-alpine` image has no entrypoint and
uses:

```text
Entrypoint: null
Cmd: ["caddy","run","--config","/etc/caddy/Caddyfile","--adapter","caddyfile"]
```

The implementation was corrected to invoke `caddy storage export/import`
explicitly. The staging-volume emptiness check uses `--entrypoint sh`.

### What I learned

- The official image metadata, not an assumption about Docker Hub defaults,
  determines whether the binary name belongs in argv.
- A recovered archive is not required to be byte-identical after import/export;
  certificate identity and required authority members are the durable
  invariants.
- Vault 1.21.2 and its KV v2 CLI support the required `-cas` and JSON file
  input without placing secret payloads in process arguments.

### What was tricky to build

- Materialization must validate every field before promotion. Per-file atomic
  renames still permit process death between files, so the implementation
  writes an immutable generation and atomically swaps one symlink.
- The Vault backup must distinguish a missing record from a read failure. The
  current CLI wrapper intentionally presents both as an operation error; first
  initialization still requires `--initialize` and CAS zero, so it cannot
  overwrite an existing record.
- Restore must allow Docker to create a missing staging volume while refusing a
  nonempty one and categorically rejecting the live volume name.

### What warrants a second pair of eyes

- Review whether Vault policy should put the PKI archive on a dedicated mount,
  not only a separate path.
- Review whether the backup should quiesce Caddy explicitly on future storage
  backends; the current file storage exported and restored consistently.
- Review retention/garbage collection for old local secret generations.

### What should be done in the future

- Add an explicit idempotent application-secret initialization command using
  CAS zero for development records.
- Migrate Compose files to consume the managed file names.
- Add a separate compromise/rotation playbook; restoration is not valid for a
  known-compromised authority.

### Code review instructions

- Start with `devctl/operations.py`, specifically `materialize_secrets`,
  `pki_backup`, and `pki_restore_staging`.
- Review secret specifications in the three production-shaped manifests.
- Run `python3 -m unittest discover -s devctl/tests -v`.
- Do not rerun a live restore against `tinyidp-local-caddy-pki`; the command
  deliberately refuses that target.

### Technical details

The Vault record has this non-secret metadata envelope:

```text
schema_version
format=caddy-storage-export-tar
archive_sha256
root_cert_sha256
caddy_image
source_volume
created_at
```

`archive_b64` is the only secret-bearing field and is never logged.

## Step 18 — initialize application secrets, add the admin stack, and author the local-app playbook

### Prompt context

The implementation session continued under the request to implement the ticket,
keep a frequent detailed diary, check tasks as they become true, and commit at
appropriate intervals. The owner also asked that the reusable devctl skill be
updated after implementation.

### What I did

- Added the `secrets-init` dynamic devctl command. It groups a manifest's fields
  into runtime, bootstrap, and integration records; generates cryptographically
  random values; creates only missing Vault KV v2 records with CAS zero; and
  validates existing records without replacing them.
- Corrected repeat secret materialization. The managed target is now kept as a
  lexical repository-relative path while its destination is resolved. This
  preserves the ability to recognize and replace the plugin-owned symlink.
- Added `examples/tinyidp-admin-console` with:
  - a Caddy HTTPS proxy and the external `tinyidp-local-caddy-pki` volume;
  - Mailpit for private local email delivery;
  - persistent SQLite and admin-artifact storage;
  - first-run database/key initialization and owner-grant bootstrap;
  - file-mounted token, admin, invitation, email, and owner credentials;
  - CA export and certificate-verifying smoke commands;
  - a technical operations README.
- Initialized the live `dev` Vault records for the admin console and
  materialized six files through the managed generation symlink. The owner
  password was written only to the ignored `owner-password.txt` file and was
  never printed.
- Added the embedded Glazed tutorial
  `tutorial-local-development-apps`. It explains integration modes, issuer
  identity, public/confidential clients, PKCE, proxy listeners, browser versus
  backchannel addressing, the Vault hierarchy, atomic secret materialization,
  the persistent Caddy authority, recovery, relying-party pseudocode, a test
  matrix, and the PULP OS device-authorization extension.
- Added a Go test that loads the embedded help filesystem and verifies the
  tutorial's slug, type, top-level discoverability, command/flag references,
  and critical content.
- Updated the shared `devctl-plugin-authoring` skill with manifest-driven
  profiles, deterministic dynamic commands, foreground Compose supervision,
  secret-set atomicity, the managed-symlink path rule, KV v2 CAS initialization,
  network-pool guidance, and the installed `logs --tail` syntax.

### Commands and evidence

The Python operation suite passed after adding initialization and the repeat
materialization regression:

```text
python3 -m unittest discover -s devctl/tests -v
Ran 15 tests
OK
```

The live Vault records were created without emitting their values:

```text
kv/tiny-idp/dev/admin-console/bootstrap  version 1
kv/tiny-idp/dev/admin-console/runtime    version 1
```

The focused embedded-help test passed:

```text
go test ./cmd/tinyidp/doc
ok github.com/go-go-golems/tiny-idp/cmd/tinyidp/doc
```

The actual CLI rendered the page:

```text
go run ./cmd/tinyidp help tutorial-local-development-apps
# Tutorial: build local development applications with tinyidp
```

### What worked

- Secret initialization is idempotent: complete existing records are validated
  and skipped rather than rotated implicitly.
- Secret materialization succeeds on consecutive runs, which exercises the
  managed-symlink promotion path.
- The admin image built successfully from the shared production Dockerfile.
- The Glazed help loader discovered and rendered the new tutorial without
  additional command wiring.

### What didn't work

The first retained admin launch failed after the image build because the new
Compose network requested a subnet already used by another environment:

```text
failed to create network tinyidp-admin-console_idp-backend:
Pool overlaps with other one on this address space
```

The fixed subnet and container address were unnecessary, so they were removed.
The trusted-proxy development boundary was changed to Docker's private
`172.16.0.0/12` range and the container health check was changed to loopback.

The second launch exited during the cached Go build. `devctl status` reported
the service dead but had no exit code, stdout ended at:

```text
#12 [idp build 6/6] RUN CGO_ENABLED=1 go build ...
```

and `devctl logs --service admin-console-compose --stderr` was empty. Following
the repository rule limiting legitimate server-fix attempts to two, I stopped
runtime debugging and explicitly marked the smoke acceptance as blocked.

Two command-level details were also discovered:

- `devctl logs` uses `--tail`, not `--tail-lines`;
- `--tail-lines` belongs to `devctl status` and controls dead-service stderr
  context.

The first focused Go test attempt failed in the restricted sandbox because the
host Go build cache was read-only:

```text
open /home/manuel/.cache/go-build/...: read-only file system
```

It passed unchanged when run with approved host-cache access.

### What I learned

- Fixed Compose address pools make independently useful demos contend for
  machine-global Docker address space. Service DNS and loopback health probes
  are safer defaults.
- A symlink's lexical identity is security-relevant. Resolving the path before
  checking it transforms a managed symlink into the generation directory it
  names and makes an idempotent prepare look like an unsafe overwrite.
- The Glazed embedded help directory is sufficient for registration; a focused
  store lookup tests discoverability without coupling the test to terminal
  rendering.
- A supervised foreground Compose process can disappear while BuildKit output
  ends without a useful exit record. Acceptance must be based on live endpoints
  and smoke results, not on a successful image build.

### What was tricky to build

- Initialization must distinguish absent records from incomplete existing
  records. Missing records are generated with CAS zero; incomplete records fail
  validation rather than being silently repaired with a mixture of old and new
  fields.
- First-run admin bootstrap needs the owner password long enough to establish
  the initial grant while keeping it out of Compose YAML, argv, logs, Git, and
  shell environment variables.
- The playbook has to preserve one public issuer across browser and backchannel
  traffic while explaining that transport routing may differ.

### What warrants a second pair of eyes

- Review whether `172.16.0.0/12` is acceptably narrow for the isolated local
  Docker listener; production must use the actual proxy network.
- Review lifecycle handling for the bootstrap owner-password file after the
  first successful bootstrap.
- Diagnose the unexplained supervised Compose exit in a fresh session before
  checking the admin smoke and complete acceptance tasks.
- Review the shared skill change independently because it affects future
  repositories, not only tiny-idp.

### What should be done in the future

- Complete the admin endpoint smoke test and browser CA export after the launch
  failure is understood.
- Move shared-two-apps and Jitsi from their local secret generators to the same
  Vault materialization contract.
- Implement an explicit guarded state-reset command per persistent profile.
- Run the per-profile matrix and record teardown/state-preservation evidence.

### Code review instructions

- Review `devctl/operations.py` and
  `devctl/tests/test_operations.py` for initialization and repeat promotion.
- Review `examples/tinyidp-admin-console/compose.yaml` for listener, volumes,
  secret mounts, and first-run bootstrap.
- Run `python3 -m unittest discover -s devctl/tests -v`.
- Run `go test ./cmd/tinyidp/doc` and
  `go run ./cmd/tinyidp help tutorial-local-development-apps`.
- Do not mark the admin runtime smoke as passing until `/readyz`, `/admin`, and
  the unauthenticated API boundaries have been exercised through Caddy.

## Step 19 — unify the HTTPS examples and guard application-state reset

### Prompt context

The active implementation goal requires completing the ticket, maintaining a
detailed diary, checking tasks only when evidence supports them, and committing
at useful review boundaries. Runtime debugging remained paused after the
two-attempt limit in Step 18, so this interval focused on deterministic
control-plane, configuration, and test work.

### What I did

- Replaced the local random/deterministic secret generators in
  `shared-two-apps` and Jitsi with compatibility entrypoints that delegate to
  one repository-owned Vault bootstrap command.
- Added `dev/scripts/bootstrap-vault-profile.sh`. It accepts only the three
  production-shaped profiles, ensures the manually retained Caddy volume, then
  runs the profile's CAS-protected `secrets-init` and atomic `secrets-fetch`.
- Extended the shared and Jitsi manifests so `prepare.run` materializes every
  TinyIDP runtime key, bootstrap password, and integration credential.
- Added safe derived-secret templates to the manifest contract. A single
  Vault-generated PostgreSQL password now derives:
  - the PostgreSQL password file;
  - the DSN file used by the one-shot goja bootstrap;
  - the Glazed YAML config mounted into the distroless goja host.
- Restricted derived templates to simple same-record peer fields without
  formatting, conversion, attribute access, or indexing. Existing derived
  values must exactly match recomputation or materialization fails before
  promotion.
- Changed PostgreSQL to `POSTGRES_PASSWORD_FILE` and goja to
  `--config-file=/run/secrets/goja_auth_config`. The database credential is no
  longer in Compose YAML, rendered Compose environment, or process arguments.
- Changed Jitsi JWT, Jicofo, and JVB credentials to printable Vault text. These
  values are intentionally printable because the official Jitsi entrypoints
  place them in generated configuration/environment; arbitrary binary values
  were not a valid contract.
- Mounted the Vault-provided token/admin/invitation/email key files into both
  TinyIDP containers and removed their per-volume token generation.
- Removed fixed Docker subnets, fixed addresses, and `extra_hosts` entries.
  Caddy now owns public hostname aliases on each Compose network, while
  container-local health checks use loopback.
- Removed Mailpit's committed local password. Its operator endpoint remains
  loopback-only.
- Refactored both Playwright suites and the Python acceptance script to read
  materialized password files instead of committed deterministic values.
- Added `dev/scripts/compose-state.sh` with:
  - read-only `state-status`;
  - exact typed confirmation for every reset;
  - a protected reset mode that verifies the Compose declaration for the
    external Caddy volume before teardown and compares its creation/mount
    identity afterward;
  - a local reset mode for the HTTP-only external Message Desk demo.
- Migrated the external Message Desk README and manifest to devctl-first
  launch, status, logs, shutdown, and guarded reset.
- Documented the intentional one-time migration boundary: existing SQLite
  hashes created with the old fixture passwords require a guarded application
  state reset. The implementation does not silently import legacy credentials
  or add a compatibility path.

### Commands and evidence

The control-plane suite grew from 15 to 23 tests:

```text
python3 -m unittest discover -s devctl/tests -v
Ran 23 tests
OK
```

New coverage proves:

- every production-shaped profile has guarded status/reset commands;
- Compose secret declarations cover every materialized output;
- the old bootstrap filenames delegate to the shared Vault bootstrap and
  contain no `/dev/urandom` or deterministic password;
- a reset without typed confirmation returns exit 2 before Docker mutation;
- attribute traversal in a derived template is rejected;
- goja's DSN and Glazed config derive from the same Vault password;
- a stale or tampered derived value prevents any target promotion.

All production-shaped Compose files parsed successfully:

```text
docker compose -f examples/tinyidp-admin-console/compose.yaml config --quiet
docker compose -f examples/tinyidp-shared-two-apps/compose.yaml config --quiet
docker compose -f examples/tinyidp-jitsi/compose.yaml config --quiet
```

The refactored browser suites loaded and enumerated without executing servers:

```text
shared-two-apps: 28 Playwright tests in 1 file
jitsi:           8 Playwright tests in 1 file
```

The external Message Desk profile produced a valid plan and the live dynamic
command refused an unconfirmed reset:

```text
devctl --profile external-message-desk state-reset
refusing state reset; rerun with --confirm reset-external-message-desk-state
Error: command "state-reset" failed with exit_code=2
```

`docmgr doctor --ticket TINYIDP-ADMIN-CONSOLE-001` reported all checks passed.

Commits:

```text
6c0d3e5 refactor(dev): unify HTTPS demo secret workflows
0d7f39d feat(dev): add guarded external demo reset
```

### What worked

- Compose service aliases replace global subnet reservations without changing
  the canonical browser issuer.
- Glazed `--config-file` provides a file-only secret boundary even for the
  distroless generated goja host, so no shell wrapper or environment secret is
  necessary.
- A manifest-derived file can be validated against its source fields before
  publication, preserving one source of truth while supporting consumer-
  specific formats.
- The old `00-init-secrets.sh` filenames remain useful human entrypoints but no
  longer implement a second secret system.
- The reset command fails before invoking Docker when confirmation is absent.

### What didn't work

The first attempt to patch the shared script, Jitsi script, manifests, state
helper, and test in one patch failed because the expected context in
`test_manifests.py` did not match the actual file:

```text
apply_patch verification failed:
Failed to find expected lines in devctl/tests/test_manifests.py
```

No partial change was applied. I inspected the current test file and applied
the implementation in smaller patches.

An initial README patch also failed because its expected Jitsi line wrapping
did not match the current file. Again, no partial patch was applied; the shared
and Jitsi documentation changes were applied separately.

The first design considered a generated `env_file` for goja's PostgreSQL DSN.
That would cause the secret to become a process environment value and may
expose it in rendered configuration. Inspection of go-go-goja showed that the
generated host already consumes Glazed `--config-file`, so the environment-file
approach was discarded before commit.

### What I learned

- The same logical Vault field may need multiple consumer files. Derived
  templates are safer than independently generated values only when the
  template language is deliberately constrained and recomputed during reads.
- A distroless image does not imply secrets must be environment variables.
  An existing application config-file API is the correct file-delivery
  boundary.
- Docker network aliases can preserve public TLS hostnames inside a network
  without hard-coded addresses. This removes address-pool collisions while
  retaining certificate hostname validation.
- Migrating password sources without resetting password hashes creates a false
  "correct configuration, failed login" state. The state/secret coherence
  boundary must be explicit in the operator documentation.

### What was tricky to build

- The goja database has three consumers with different interfaces:
  PostgreSQL accepts a password file, the bootstrap command accepts a DSN file,
  and the long-running generated host accepts Glazed YAML. The derived template
  contract keeps all three coherent.
- A reset helper must work for both Caddy-backed and HTTP-only projects without
  weakening Caddy protection. The two explicit operation modes share typed
  confirmation but only the protected mode accepts a Caddy-backed profile.
- Browser tests need secrets only when a flow executes. TypeScript helpers read
  materialized files lazily so Playwright discovery and TypeScript loading do
  not require a running environment.

### What warrants a second pair of eyes

- Confirm that accepting `172.16.0.0/12` is appropriate for these isolated
  local Compose networks. Production must use the exact proxy network.
- Review the generated goja Glazed YAML field name against the sibling
  go-go-goja version whenever that dependency changes.
- Review whether the reset helper should also record volume labels or a state
  manifest before deletion.
- Run both full browser suites after the runtime launch blocker is resolved;
  enumeration proves loading/type transformation, not behavioral acceptance.

### What should be done in the future

- Initialize the shared/Jitsi Vault records only when the owner is ready to
  perform the documented one-time application-state resets.
- Complete runtime smoke, restart durability, negative-secret, logout, and
  scope/audience testing for every server profile.
- Add state reset for the direct message-app profile if operators need a single
  command beyond removing its declared `var/devctl/message-app` state root.
- Diagnose the Step 18 Compose supervision exit in a fresh debugging session.

### Code review instructions

- Begin with the three manifests in `dev/environments/` and compare their
  `secret_material.files` with the bottom of each Compose file.
- Review `_render_secret_template` and its parser restrictions before approving
  derived configuration.
- Review `dev/scripts/compose-state.sh`; the confirmation check and Caddy
  declaration check must remain before `docker compose down --volumes`.
- Run the 23 Python tests and all four Compose `config --quiet` commands.
- Use `pnpm exec playwright test --list` in both browser-test directories.
- Do not run a confirmed state reset unless deletion of that profile's
  application volumes is intentional.

## Step 20 — begin the profile acceptance matrix and identify supervision drift

### Prompt context

The implementation goal continued automatically. Step 19 left only owner
review and the complete profile acceptance matrix unchecked. The repository's
two-attempt debugging limit remained binding for the failed admin server path,
so acceptance began with utility profiles and lightweight direct Go profiles.

### What I did

- Ran the `fixtures`, `script-tools`, and `production-host-local` dynamic
  commands through devctl.
- Corrected the compile-only `phase0.js` example from the removed
  `signupInput` schema to the current host-owned `signupStartInput` schema.
- Re-ran the script profile and recorded all four deterministic fingerprints.
- Ignored the root-level `tinyidp.db`, WAL, and shared-memory files created by
  the embedded example. These are local runtime state, not repository input.
- Started embedded and message-app profiles under tmux, inspected devctl state
  and logs, and attempted readiness/discovery checks.
- Cleaned devctl and tmux state after each failed server acceptance.

### Commands and evidence

The fixture profile rendered the selected development configuration:

```text
devctl --profile fixtures smoke
addr: 127.0.0.1:5556
issuer: http://localhost:5556
client_id: dev-client
```

The first script-tools run failed:

```text
compile signup program: materialize Tiny-IDP program:
invalid Tiny-IDP program:
lambda.input_schema at lambdas.signup.lookup.inputSchema
```

Inspection of `pkg/idpsignup/open_signup.js` and
`pkg/idpprogram/validate.go` showed that the current host schema is
`signupStartInput`. After changing both example lambdas, the single corrected
attempt passed:

```text
status: valid
source: examples/tinyidp-script/phase0.js
profile: signup
source_fingerprint:
  efd8189d7725dea87a5aa17a44002b9d0282e8af6cd684625de1a4648ccb97b3
program_fingerprint:
  de6eb11bd1fb21f35e0f748e9f4acfa7725340a2b00f4841ebdfc12329ae9a17
```

The production-reference profile successfully rendered
`serve-production --help`, including file-only token/admin/email/invitation
secret flags and the new local-development tutorial.

The schema correction was committed as:

```text
a29a2c2 fix(dev): align script fixture with signup schema
```

### What worked

- All three utility/reference profiles execute through the advertised dynamic
  command surface.
- The script validator caught a genuine stale example rather than producing a
  false-positive smoke result.
- The corrected example passed without weakening validation or changing the
  host schema.
- The message-app `prepare.run` initialized its declared state successfully.

### What didn't work

The first embedded tmux invocation used only `devctl up` as the tmux command.
After `up` returned, tmux closed. The service logged:

```text
self-contained app listening at http://127.0.0.1:5556
```

but was gone roughly ten seconds later. I changed the test harness to retain an
interactive shell after `devctl up`.

The next embedded launch failed immediately:

```text
listen tcp 127.0.0.1:5556: bind: address already in use
```

`lsof-who -p 5556 -k` reported no listener immediately before and immediately
after the launch. After cleanup and a second verified-free launch, the same bind
error repeated. I stopped that path under the two-attempt rule.

The message-app profile initially reported:

```text
service started pid=2811750 service=message-app
up complete services=1
devctl status: alive=true
message application listening addr=127.0.0.1:8090
```

The immediately following readiness request failed:

```text
curl: (7) Failed to connect to 127.0.0.1 port 8090
```

`ps` showed no process for the recorded PID and both `ss` and `lsof-who`
reported no listener, while `devctl status` continued to report `alive=true`.
No application error was present in stdout or stderr. I did not retry or patch
around this contradictory supervision state.

### What I learned

- A successful `launch.plan`, `prepare.run`, and `up complete` is insufficient
  runtime evidence. The tracked PID, operating-system process table, listener,
  and HTTP endpoint must agree.
- Devctl's persisted liveness result can disagree with the current process
  table after the launched service disappears.
- Retaining the tmux shell does not resolve the direct-process disappearance,
  so the behavior is not only tmux session teardown.
- Utility-profile acceptance can proceed independently, but the server matrix
  cannot be claimed complete while supervision evidence is contradictory.

### What was tricky to diagnose

- The embedded sequence combines `go run`, a child binary, SQLite WAL state,
  tmux, and devctl process groups. A transient child may own the port even when
  the recorded `go run` PID is gone.
- The message-app contradiction is sharper: devctl reported the recorded PID
  alive after `ps` returned no row. That points at supervision/state
  interpretation rather than an ordinary application startup error.

### What warrants a second pair of eyes

- Inspect the installed devctl version's detached process and PID-liveness
  implementation before any more server retries.
- Determine whether the state file records a process-group leader while status
  tests an identifier that can be stale or reused.
- Confirm how `go run` child processes are placed into and terminated with the
  supervised process group.

### What should be done in the future

- Reproduce the devctl lifecycle in its own repository fixture before changing
  TinyIDP manifests.
- Resume embedded/message-app acceptance only after the supervisor behavior is
  understood.
- Run external and HTTPS Compose acceptance after confirming port ownership and
  supervision, then complete restart/logout/negative-secret assertions.

### Code review instructions

- Review commit `a29a2c2` and compare `phase0.js` with
  `pkg/idpsignup/open_signup.js`.
- Re-run the three non-server dynamic commands.
- Treat `uy3a` as incomplete. Static validation and test enumeration are not a
  substitute for live profile acceptance.

## Step 21 — isolate the devctl wrapper lifecycle

This step moves the runtime investigation into the workspace copy of devctl.
The immediate goal is to distinguish application failure, parent-shell
teardown, wrapper failure, and process-group signaling with a minimal service
that has no TinyIDP dependencies.

The initial source inspection found a potentially unsafe signal topology:
devctl launches the wrapper as a process-group leader, the wrapper places its
child in that same group, and the wrapper forwards `SIGHUP` to the entire group.
Because the wrapper belongs to the target group, forwarding can deliver the
same signal back to the forwarding process. This is a hypothesis until the
ticket-local reproduction records the observed behavior.

### Prompt Context

**User prompt (verbatim):** "We put devctl/ in the workspace for you to work on / analyze. Check the things, keep a diary as you debug so we can track your steps. Store any scripts and other files from the experiments/debugging in the scripts folder of the ticket."

**Assistant interpretation:** Diagnose devctl's detached-service lifecycle in
the workspace checkout, preserve a chronological debugging record, and keep
all reproduction assets with the current TinyIDP ticket.

**Inferred user intent:** Produce an auditable root-cause investigation that
can become a tested devctl fix without losing the evidence or repeating
experiments in later sessions.

**Commit (code):** `39ba41650074a6b203d4c703f46263fd45605533` —
`fix(supervise): isolate wrapper from service signals`

### What I did

- Confirmed that both the devctl and TinyIDP worktrees were clean before the
  investigation.
- Read `Supervisor.startService`, `newWrapServiceCmd`, `ProcessAlive`, existing
  supervisor tests, and the built-in supervision smoke test.
- Added `scripts/reproduce-devctl-wrapper-lifecycle.sh`, which launches a
  minimal Python HTTP server through the real hidden wrapper command and
  captures process identity, process-group membership, session membership,
  HTTP readiness, wrapper output, and exit metadata.
- Added an explicit `--send-hup` mode so baseline survival and signal handling
  can be tested separately.
- Added a full-CLI NDJSON fixture and harness that run `devctl up`, allow the
  parent command to exit, and sample devctl status, the process table, and HTTP
  reachability over time.
- Added a command-level Go regression test that launches the real hidden
  wrapper in a subprocess and asserts that forwarding `SIGHUP` does not
  signal-terminate the wrapper.
- Changed the service child to lead its own process group. The wrapper now
  forwards signals to the child-owned group rather than to its own group.

### Why

- The TinyIDP profiles contain `go run`, SQLite, plugins, and application
  initialization. A minimal HTTP fixture removes those variables.
- Process-group and session identifiers are necessary to prove which kernel
  signal domain is involved; PIDs alone are insufficient.
- An explicit signal experiment is deterministic and does not depend on tmux
  teardown timing.

### What worked

- Source inspection confirmed that the wrapper PID persisted by devctl is the
  wrapper process, not the service child.
- The existing implementation exposes exit-info and ready-file paths that make
  a focused black-box reproduction possible.
- The corrected idle harness kept the wrapper, child, and HTTP endpoint alive
  until deliberate cleanup.
- The explicit `SIGHUP` case deterministically reproduced the signal defect:
  `service.exit.json` recorded `"signal": "hangup"`, the post-signal process
  snapshot contained neither process, and the invoking shell reported the
  wrapper itself as `Hangup`.
- The full `devctl up` fixture survived normal parent exit. At zero, one, and
  five seconds after `up complete`, `devctl status` reported `alive=true`, the
  wrapper and child existed, and HTTP returned successfully. The wrapper had
  been reparented to PID 1.
- A command-level regression test failed against the unmodified wrapper with:

  ```text
  Error:       Should be false
  Messages:    wrapper itself must not die from the forwarded signal
  ```

  This proves the test detects the defect before the implementation change.
- After the fix, the targeted regression passed and the black-box result
  reported `wrapper_wait_status=1`: Cobra returned an ordinary error because
  the service was intentionally signal-terminated, but the wrapper was not
  itself signal-terminated.
- `go test -buildvcs=false ./... -count=1` passed across the complete devctl
  module with loopback networking enabled.
- The pre-commit hook independently reran `go test ./...` and
  `golangci-lint run -v`; both passed with zero lint issues.

### What didn't work

- The first attempt did not reach the experiment because the default Go build
  cache is read-only in this workspace:

  ```text
  open /home/manuel/.cache/go-build/31/31aad81fbb12ce35561baa2b623debcfe1bae025d9521afa7f0c723adcd75849-d: read-only file system
  ```

  The command was:

  ```text
  go build -o /tmp/devctl-wrapper-debug ./cmd/devctl
  ```

  The retry uses a dedicated `GOCACHE` below `/tmp`; it does not alter devctl
  behavior.

- The cache-adjusted build then stopped at VCS stamping:

  ```text
  error obtaining VCS status: exit status 128
          Use -buildvcs=false to disable VCS stamping.
  ```

  The experiment binary does not need repository revision metadata, so the
  next build follows the tool's explicit `-buildvcs=false` instruction.

- The binary built successfully with the isolated cache and VCS stamping
  disabled, but the restricted execution namespace prohibits creation of even
  a loopback socket:

  ```text
  PermissionError: [Errno 1] Operation not permitted
  ```

  The exception originated in `socket.socket()` while selecting a free
  `127.0.0.1` port. The reproduction therefore must run outside the network
  sandbox; it does not require external network access.

- The first network-enabled harness run raced application readiness:

  ```text
  curl: (7) Failed to connect to 127.0.0.1 port 32923 after 0 ms: Couldn't connect to server
  ```

  The wrapper ready file only proves that `child.Start()` succeeded. It does
  not prove that the child completed its own initialization. The harness now
  polls the HTTP endpoint for up to five seconds and fails with preserved logs
  if application readiness never arrives.

- After the implementation change, the targeted regression passed. The first
  broader package run passed `pkg/state` and `cmd/devctl/cmds`, but two existing
  `pkg/supervise` tests could not create their loopback listeners in the
  restricted namespace:

  ```text
  listen tcp 127.0.0.1:0: socket: operation not permitted
  ```

  Those same packages must be rerun with loopback networking enabled before
  the result is considered validated.

- The first scoped commit attempt could not create the external worktree lock:

  ```text
  fatal: Unable to create '/home/manuel/code/wesen/go-go-golems/devctl/.git/worktrees/devctl/index.lock': Read-only file system
  ```

  The workspace files are writable, but this checkout's Git administrative
  directory is stored in the source repository outside the workspace write
  boundary. Staging and committing therefore require access to that specific
  worktree metadata.

- The ticket commit encountered the equivalent external worktree lock under
  the TinyIDP source repository. It required the same narrowly scoped Git
  metadata access; no additional repository files were added to the commit.

### What I learned

- `devctl up` intentionally returns after the wrapper creates its ready file.
- `ProcessAlive` checks the persisted wrapper PID with `/proc` zombie
  detection and `kill(pid, 0)`; the earlier status discrepancy may therefore
  have been a timing race rather than a stable false-positive.
- The wrapper calls `Setpgid` even though its parent already starts it with
  `Setpgid: true`, and its child is assigned to the wrapper's process group.
- The wrapper subscribes to `SIGTERM`, `SIGINT`, and `SIGHUP`, then forwards
  each signal with `kill(-pgid, signal)`, which targets both child and wrapper.
- Normal orphan reparenting is not sufficient to kill a devctl service. The
  original disappearance requires a signal or an application-specific exit;
  it is not an inherent consequence of `devctl up` returning.
- The correct signal boundary is a child-owned process group. It contains the
  service and descendants such as the compiled binary launched by `go run`,
  while excluding the wrapper that receives and forwards control signals.

### What was tricky to build

- The harness must not leak a server if an assertion fails. Its exit trap
  targets the verified wrapper process group and waits for the wrapper.
- A free port is selected immediately before launch, but port allocation still
  has an unavoidable bind race; the server log and exit metadata preserve that
  distinction.
- Sending `SIGHUP` to only the wrapper is intentional: it tests whether the
  wrapper's forwarding code expands a single-process signal into a
  process-group signal.
- The wrapper must remain independently addressable because `devctl down`
  signals the persisted wrapper PID's process group. The child must also own a
  distinct group so its descendants receive forwarded signals. Using one group
  for both roles collapses the control plane into the workload signal domain.

### What warrants a second pair of eyes

- Review the child-owned process-group contract for programs that deliberately
  reassign their own process groups or sessions.
- Review whether `SIGHUP` should remain a supported forwarded control signal.
  The fix makes forwarding safe but does not change the existing policy.
- Consider whether wrappers should eventually use a new session as additional
  protection from terminal lifecycle signals. The current evidence proves
  normal parent exit is safe and fixes self-signaling, so `setsid` was not
  added speculatively.

### What should be done in the future

- Reinstall or otherwise select devctl commit `39ba416` before resuming the
  TinyIDP runtime acceptance matrix; the previously installed binary does not
  contain this fix.
- Re-run the TinyIDP embedded and message-app profiles to determine whether the
  original exits were caused by `SIGHUP` or whether an application-specific
  failure remains.
- If another spontaneous exit occurs, preserve the wrapper exit JSON and
  capture SID/PGID evidence before cleanup.

### Code review instructions

- Begin with
  `scripts/reproduce-devctl-wrapper-lifecycle.sh`.
- Compare its process model with `pkg/supervise/supervisor.go:startService` and
  `cmd/devctl/cmds/wrap_service.go:newWrapServiceCmd`.
- Build devctl, run both harness modes into separate output directories, and
  inspect `processes.txt` and `service.exit.json`.
- Run:

  ```text
  go test ./cmd/devctl/cmds -run TestWrapServiceForwardsSIGHUPWithoutKillingWrapper -count=1
  go test ./... -count=1
  ```

### Technical details

Expected process topology before a signal:

```text
devctl __wrap-service (PID = PGID)
└── python3 -m http.server (different PID, same PGID)
```

Current forwarding pseudocode:

```text
on SIGHUP:
    kill(-child_process_group, SIGHUP)
    # target set includes the service and descendants, not the wrapper
```

Validated process topology after the fix:

```text
devctl __wrap-service (PID = wrapper PGID)
└── service child (PID = child PGID)
    └── service descendants (inherit child PGID unless explicitly changed)
```
