# Tasks

## TODO

- [ ] Phase 1 — Revalidate the selected upstream Jitsi distribution and OIDC-to-Jitsi-JWT adapter: image provenance, maintenance posture, configuration contract, and upgrade policy. <!-- t:s3ou -->
- [ ] Phase 1 — Decide and document the JVB media-plane exposure strategy: public UDP/10000, advertised address, firewall/load-balancer requirements, and one-node capacity limits. <!-- t:rhgv -->
- [ ] Phase 2 — Add a separate Argo CD application and Jitsi namespace with Kustomize, network isolation, resource limits, and logging/observability labels; do not modify the standalone XMPP application. <!-- t:vi2h -->
- [ ] Phase 2 — Add Vault Secrets Operator wiring for a Jitsi-only shared HS256 Prosody/adapter secret and the TinyIDP adapter client secret, including a coordinated rotation runbook. <!-- t:12th -->
- [ ] Phase 3 — Deploy Jitsi in token mode and prove hand-minted valid JWT room join plus wrong-secret, expired-token, and room-mismatch rejection. <!-- t:zzq1 -->
- [ ] Phase 4 — Register the adapter as an exact-redirect TinyIDP client, deploy it, and prove OIDC discovery, callback exchange, userinfo mapping, and adapter health. <!-- t:76mb -->
- [ ] Phase 5 — Add end-to-end browser validation from a Jitsi room through TinyIDP login to a media-connected room; verify logs do not contain tokens or shared secrets. <!-- t:x6z7 -->
- [ ] Phase 6 — Decide and implement only after baseline validation: moderator claim mapping, guest policy, token lifetime, room/tenant policy, and logout behavior. <!-- t:nsin -->
