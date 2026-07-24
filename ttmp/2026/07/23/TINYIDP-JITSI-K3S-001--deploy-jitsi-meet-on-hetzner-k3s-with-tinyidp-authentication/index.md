---
Title: Deploy Jitsi Meet on Hetzner k3s with TinyIDP authentication
Ticket: TINYIDP-JITSI-K3S-001
Status: active
Topics:
    - architecture
    - authentication
    - jitsi
    - oidc
    - production
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: abs:///home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/applications/xmpp.yaml
      Note: Existing separate Prosody Argo application that must not be reused for Jitsi
    - Path: abs:///home/manuel/code/wesen/2026-03-27--hetzner-k3s/gitops/kustomize/tiny-message-desk/deployment.yaml
      Note: Current shared TinyIDP issuer and Vault/GitOps deployment pattern
    - Path: abs:///home/manuel/code/wesen/go-go-golems/go-go-parc/Research/KB/Projects/infrastructure-and-release.md
      Note: Platform delivery and GitOps operating model
    - Path: repo://ttmp/2026/07/09/TINYIDP-JITSI-001--use-tiny-idp-as-an-oidc-identity-provider-for-jitsi-meet/design-doc/01-tiny-idp-as-an-oidc-identity-provider-for-jitsi-meet-analysis-design-and-implementation-guide.md
      Note: Protocol predecessor and accepted OIDC-to-Jitsi-JWT architecture
ExternalSources: []
Summary: ""
LastUpdated: 2026-07-23T15:40:18.148156876-04:00
WhatFor: ""
WhenToUse: ""
---


# Deploy Jitsi Meet on Hetzner k3s with TinyIDP authentication

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- architecture
- authentication
- jitsi
- oidc
- production

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
