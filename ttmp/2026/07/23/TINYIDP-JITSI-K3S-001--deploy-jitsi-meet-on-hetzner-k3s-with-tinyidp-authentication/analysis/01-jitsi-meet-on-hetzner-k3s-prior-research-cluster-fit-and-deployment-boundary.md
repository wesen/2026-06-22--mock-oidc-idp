---
Title: 'Jitsi Meet on Hetzner k3s: prior research, cluster fit, and deployment boundary'
Ticket: TINYIDP-JITSI-K3S-001
Status: active
Topics:
    - architecture
    - authentication
    - jitsi
    - oidc
    - production
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Successor deployment ticket: prior research proves TinyIDP can authenticate Jitsi only through an OIDC-to-Jitsi-JWT adapter; Hetzner k3s has reusable Argo, Vault, ingress, and TinyIDP patterns, but Jitsi must be a separate workload from the existing XMPP Prosody service."
LastUpdated: 2026-07-23T15:40:46.742764941-04:00
WhatFor: "Orient an implementer before designing the GitOps deployment and TinyIDP client registration."
WhenToUse: "Read before changing Hetzner-k3s manifests or selecting the Jitsi adapter/token configuration."
---

# Findings: prior research, cluster fit, and deployment boundary

## Decision carried forward

The existing [TINYIDP-JITSI-001](../../../09/TINYIDP-JITSI-001--use-tiny-idp-as-an-oidc-identity-provider-for-jitsi-meet/index.md) research ticket is the protocol predecessor. It completed its local OIDC and claim-mapping experiments, but its Jitsi deployment phases remain open. Its central conclusion remains valid:

```text
Browser → Jitsi web → OIDC adapter → TinyIDP
                                  ↓
                       HS256 Jitsi JWT
                                  ↓
                           Jitsi Prosody
```

Jitsi Meet does not consume TinyIDP's ordinary OIDC/JWKS tokens directly. The initial deployment should use `jitsi-contrib/jitsi-oidc-adapter` as the OIDC relying party and token translator. It performs an authorization-code flow with TinyIDP, obtains user claims, then mints the Jitsi-shaped HS256 JWT that Jitsi's token-mode Prosody validates.

This ticket must not add Jitsi-specific token issuance, PEM/JWKS compatibility, or Jitsi roles to TinyIDP itself. The adapter is the intentionally narrow integration seam.

## Existing platform capabilities

The target is the `wesen/2026-03-27--hetzner-k3s` GitOps repository. The infrastructure-and-release project note establishes the operating model: application code produces immutable images, GitOps changes desired state, Argo CD reconciles it, and Vault delivers runtime secrets. The new Jitsi deployment should follow that order.

The currently deployed shared TinyIDP and Message Desk application is a direct reusable example:

| Concern | Existing pattern | Jitsi implication |
| --- | --- | --- |
| Public issuer | `https://idp-message-desk.yolo.scapegoat.dev` behind Traefik TLS | Register the adapter as a new TinyIDP confidential client whose redirect URI is the Jitsi public origin. |
| GitOps | Argo `Application` → kustomize directory → dedicated namespace | Create a distinct `jitsi` Argo application and namespace; do not add Jitsi resources to `tiny-message-desk`. |
| Secrets | Vault Secrets Operator `VaultAuth`/`VaultStaticSecret` | Store the adapter client secret and the shared Prosody/adapter HS256 secret in a Jitsi-specific Vault path. |
| Ingress | Traefik and cert-manager serve HTTPS hostnames | Use an HTTPS Jitsi hostname for web and adapter callback paths. This handles browser HTTP; it does **not** by itself handle media UDP. |
| Identity client catalog | TinyIDP receives a reviewed clients JSON catalog | Add a Jitsi adapter client only with the exact production callback URI and scopes `openid profile email`. |

## Important separation: existing XMPP is not Jitsi Prosody

Hetzner k3s already contains an Argo application named `xmpp`, in namespace `xmpp`, and it is currently `Synced` and `Healthy`. It runs a standalone Prosody instance with `authentication = "internal_hashed"`, XMPP user persistence, and federation deliberately disabled.

Jitsi needs a different Prosody configuration: token authentication, a Jitsi `app_id`, the shared `app_secret`, conference MUC components, and Jicofo/JVB integration. Reusing the existing XMPP deployment would couple unrelated data and replace its authentication model. The deployment boundary must therefore be:

```text
xmpp namespace                 jitsi namespace
--------------                 ---------------
standalone Prosody              Jitsi Prosody (token mode)
internal_hashed accounts        adapter-minted Jitsi JWTs
chat rooms                      Jitsi conference rooms
```

## Initial deployment shape

The first production-shaped deployment has five application components and two infrastructure boundaries:

```text
Internet
  │ HTTPS
  ▼
Traefik / cert-manager ──► Jitsi web + adapter callback routes
                                 │ OIDC authorization code
                                 ▼
                    shared TinyIDP issuer (existing deployment)

Jitsi web ──► Prosody (token mode) ◄── Jicofo
                  ▲
                  └──────────────── JVB media control

Browser media ── UDP/10000 ─────────► JVB public reachability
Vault ──────────────────────────────► adapter + Prosody shared secrets
```

The WebRTC media path is the main k3s-specific unknown. Traefik can route HTTPS signalling but cannot replace the JVB public UDP service. The implementation must explicitly select and validate a public UDP/10000 exposure strategy compatible with the one-node Hetzner cluster before calling the deployment production-ready.

## Delivery phases

1. **Architecture validation:** choose the image/chart/manifests for Jitsi's web, Prosody, Jicofo, JVB, and adapter components; document image provenance and upgrade policy.
2. **Network and capacity:** choose the Jitsi hostname; verify DNS/TLS, public UDP/10000, advertised JVB address, and one-node resource limits.
3. **GitOps skeleton:** add the separate namespace, Argo application, NetworkPolicy, Vault auth/static secrets, HTTPS ingress, and observability/logging labels.
4. **Token-mode baseline:** configure Jitsi Prosody with a generated HS256 secret; prove a hand-minted Jitsi JWT can join and a wrong/expired JWT cannot.
5. **TinyIDP adapter integration:** deploy the upstream adapter; register its exact TinyIDP redirect URI; mount its client secret and the shared token secret; prove `/oidc/health` and authorization redirect behavior.
6. **End-to-end validation:** drive a browser from a room URL through TinyIDP login to a joined room; test media connectivity, rejected callback/room tampering, and no-secret-in-logs behavior.
7. **Policy decisions:** only after the baseline works, decide moderator claims, guest access, token lifetime, logout expectations, and room/tenant policy.

## Open decisions and risks

- **Adapter maintenance:** the prior ticket selected `jitsi-contrib/jitsi-oidc-adapter`, which introduces Deno. Revalidate its current image/release posture before deployment; do not silently write a replacement adapter.
- **Shared HS256 secret:** the adapter and Jitsi Prosody must receive the identical secret. Vault should be the sole source, and rotation needs an explicit coordinated rollout procedure.
- **Moderator mapping:** TinyIDP's existing groups/roles are not automatically Jitsi moderator claims. Leave moderator behavior at Jitsi's safe default until a policy is selected and tested.
- **Guest policy:** start with `allow_empty_token = false`; adding a guest virtual host is a separate authorization decision.
- **JVB reachability:** a deployment that renders the web UI but cannot establish peer media is incomplete. UDP exposure and advertised addresses are a mandatory acceptance criterion.

## Evidence consulted

- Prior Jitsi protocol design, experiments, and tasks in `TINYIDP-JITSI-001`.
- Current shared TinyIDP GitOps manifests, including trusted Traefik listener mode and Vault static-secret delivery.
- Current separate XMPP/Prosody Argo application and live healthy state.
- `Research/KB/Projects/infrastructure-and-release.md` for the platform delivery rules.
