---
Title: Design the TinyIDP administration backend and widget-DSL console MVP
Ticket: TINYIDP-ADMIN-CONSOLE-001
Status: active
Topics:
    - backend
    - identity
    - auth
    - architecture
    - security
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://.github/workflows/ci.yml
      Note: Frontend build and committed-asset reproducibility gate
    - Path: repo://internal/adminweb/assets.go
      Note: Fixed embedded production SPA and asset handlers
    - Path: repo://internal/adminweb/auth.go
      Note: OIDC PKCE, admin-session, grant revalidation, and CSRF boundary
    - Path: repo://internal/adminweb/frontend/src/App.tsx
      Note: React administration shell and read-only navigation
    - Path: repo://internal/adminweb/frontend/src/api.ts
      Note: RTK Query session and Widget page API contracts
    - Path: repo://internal/adminweb/handler.go
      Note: Public admin route surface and response security headers
    - Path: repo://internal/adminweb/verbs/pages.js
      Note: Server-owned Widget DSL page definitions
    - Path: repo://internal/adminweb/widget_runtime.go
      Note: Constrained widget.dsl and tinyidp.admin Goja runtime
    - Path: repo://internal/cmds/serve_production.go
      Note: Production console construction and public listener integration
    - Path: repo://internal/sections/production/section.go
      Note: Owner-only admin authentication key configuration
    - Path: repo://pkg/idpadminapp/page_data.go
      Note: Capability-authorized read-only page orchestration
    - Path: repo://pkg/sqlitestore/admin_queries.go
      Note: Bounded parameterized administration read models
ExternalSources:
    - local:tiny-idp-ux.md
Summary: Design a production system-scoped administration control plane and widget.dsl v3 console for TinyIDP.
LastUpdated: 2026-07-23T20:15:16.705417445-04:00
WhatFor: Coordinate implementation of TinyIDP's first production administration control plane and Widget DSL console.
WhenToUse: Use as the landing page for architecture review, implementation planning, and phased delivery.
---



# Design the TinyIDP administration backend and widget-DSL console MVP

## Overview

This ticket defines the first production TinyIDP Console: a single-owner, system-scoped administration backend and a React console rendered from reviewed `widget.dsl` v3 Widget IR. It covers authentication, server-side grants, capabilities, sessions, CSRF, fresh authentication, opaque action handles, optimistic versions, idempotency, query projections, transactional activity and audit delivery, managed operations, frontend integration, tests, and the future domain-scoped extension.

The current deliverable is an implementation-ready design. Product code has not yet been changed.

## Key Links

- [Architecture and implementation guide](design-doc/01-tinyidp-administration-backend-mvp-architecture-and-implementation-guide.md)
- [Investigation diary](reference/01-investigation-diary.md)
- [Imported TinyIDP UX specification](sources/local/tiny-idp-ux.md)

## Status

Current status: **active**

## Topics

- backend
- identity
- auth
- architecture
- security

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
