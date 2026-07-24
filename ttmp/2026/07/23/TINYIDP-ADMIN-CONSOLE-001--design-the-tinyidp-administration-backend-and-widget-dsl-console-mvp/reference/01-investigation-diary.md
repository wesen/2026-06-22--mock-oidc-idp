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
    - Path: repo://internal/cmds/serve_production.go
      Note: Evidence for public and internal listener boundaries
    - Path: repo://pkg/idp/audit.go
      Note: Evidence for post-commit delivery semantics
    - Path: repo://pkg/idpstore/interfaces.go
      Note: Evidence for point queries and atomic mutations
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
