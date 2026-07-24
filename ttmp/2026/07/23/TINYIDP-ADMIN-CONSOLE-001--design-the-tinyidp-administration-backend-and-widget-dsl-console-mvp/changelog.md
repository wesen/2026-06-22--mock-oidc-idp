# Changelog

## 2026-07-23

- Started implementation on `feat/tinyidp-admin-console`.
- Added the Phase A security model, signed action handles, persistence contracts, SQLite control-plane schema, query-projection schema, and atomic store implementation.
- Added focused tests for authorization, handle integrity/binding/expiry, grant lifecycle, replay prevention, CAS, and cross-domain rollback.
- Completed Phase A with typed safe query/command contracts, a separate application orchestration layer, Glazed owner lifecycle commands, deterministic user-projection rebuild/drift checks, and an atomic mutation executor.
- Started Phase B with the required Widget DSL dependency and a tested OIDC PKCE/admin-session authentication boundary.
- Completed Phase B with typed read models, a constrained Widget DSL runtime, public admin routing, in-process OIDC discovery, exact-key session configuration, embedded React/Redux/RTK Query/Bootstrap assets, and frontend CI reproducibility checks.

## 2026-07-23

- Initial workspace created


## 2026-07-23 - Research and MVP architecture completed

Imported the complete TinyIDP Console UX specification and wrote the evidence-backed administration backend, widget.dsl v3 integration, security, persistence, phased implementation, and testing guide plus the chronological investigation diary.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md — Primary implementation-ready architecture
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Chronological evidence and design record
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/sources/local/tiny-idp-ux.md — Imported product specification


## 2026-07-23 - Documentation bundle delivered to reMarkable

Validated the ticket, completed the bundle dry-run, and uploaded TINYIDP ADMIN CONSOLE MVP.pdf to /ai/2026/07/23/TINYIDP-ADMIN-CONSOLE-001. Recorded delivery outcomes and failures in the investigation diary.

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/reference/01-investigation-diary.md — Validation and delivery evidence
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp/2026/07/23/TINYIDP-ADMIN-CONSOLE-001--design-the-tinyidp-administration-backend-and-widget-dsl-console-mvp/tasks.md — Documentation delivery tasks completed
