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
    - Path: repo://internal/admin/service.go
      Note: Evidence inspected during current-state research
    - Path: repo://internal/cmds/admin.go
      Note: Registers the console operator command group
    - Path: repo://internal/cmds/admin_console.go
      Note: Glazed console owner lifecycle commands
    - Path: repo://internal/cmds/admin_console_test.go
      Note: Glazed command wiring and lifecycle tests
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
    - Path: repo://pkg/idpadminapp/executor.go
      Note: Atomic authorized mutation, replay, CAS, idempotency, evidence, and outbox pipeline
    - Path: repo://pkg/idpadminapp/executor_test.go
      Note: Mutation atomicity, rollback, retry, conflict, and one-time-secret tests
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
    - Path: repo://pkg/sqlitestore/admin_store.go
      Note: SQLite implementation of administration security state and atomic updates
    - Path: repo://pkg/sqlitestore/admin_store_test.go
      Note: Grant, rollback, replay, and CAS persistence tests
    - Path: repo://pkg/sqlitestore/migrations/016_admin_control_plane.sql
      Note: Durable administration control-plane schema
    - Path: repo://pkg/sqlitestore/migrations/017_admin_user_projection.sql
      Note: Indexed administration user query projection
    - Path: repo://ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/sources/local/tiny-idp-ux.md
      Note: Imported source read in full
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
