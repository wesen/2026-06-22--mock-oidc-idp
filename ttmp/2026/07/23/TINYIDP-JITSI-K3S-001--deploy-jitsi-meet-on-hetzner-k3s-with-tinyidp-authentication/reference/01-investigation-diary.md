---
Title: Investigation diary
Ticket: TINYIDP-JITSI-K3S-001
Status: active
Topics:
    - architecture
    - authentication
    - jitsi
    - oidc
    - production
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-07-23T15:40:46.966005458-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigation diary

## Goal

Create the successor ticket for deploying Jitsi Meet on Hetzner k3s using the already-researched TinyIDP integration, without confusing it with the existing standalone XMPP service.

## 2026-07-23 — Prior research and platform reconnaissance

### Read

- `TINYIDP-JITSI-001` index, implementation guide, and task list.
- `Research/KB/Projects/infrastructure-and-release.md`.
- Current Hetzner-k3s `xmpp` Argo application and Prosody configuration.
- Current shared TinyIDP/Message Desk GitOps manifests from `origin/main`, without modifying the stale local checkout.

### Findings

- The prior ticket completed protocol research and local OIDC/JWT mapping experiments. Its deployment phases 1–3 remain unimplemented.
- Jitsi has no direct TinyIDP/OIDC mode. The accepted design uses an OIDC-to-Jitsi-JWT adapter and a shared HS256 secret with Jitsi Prosody token mode.
- The existing `xmpp` application is healthy but is intentionally a separate `internal_hashed` Prosody service. It is not a Jitsi foundation to modify or reuse.
- The shared TinyIDP deployment exposes a production issuer through Traefik and already demonstrates Argo, Vault Secrets Operator, immutable images, and exact client-catalog patterns needed by the adapter.
- The media-plane question remains unresolved: JVB must receive public UDP/10000; ordinary HTTPS ingress is insufficient.

### Next

Perform the architecture-validation phase before adding manifests: select supported Jitsi/adapter images, determine the JVB UDP service strategy, and establish capacity limits and observability acceptance criteria.

## Related

- Analysis: `../analysis/01-jitsi-meet-on-hetzner-k3s-prior-research-cluster-fit-and-deployment-boundary.md`
- Protocol predecessor: `../../../09/TINYIDP-JITSI-001--use-tiny-idp-as-an-oidc-identity-provider-for-jitsi-meet/`
