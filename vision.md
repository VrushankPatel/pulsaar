# Pulsaar Vision v1

## Problem statement
Platform and security teams forbid kubectl exec and shell logins in production. Developers still need safe, auditable, read only access to container filesystems for troubleshooting and compliance.

## Goal
Provide a production safe, auditable, read only file exploration experience for Kubernetes pods without using kubectl exec or granting shell access.

## Non goals
- No write operations
- No general command execution
- No replacement for full interactive debugging or profiling

## High level solution
- Pulsaar Agent runs inside target pod namespace as a tiny read only gRPC server serving ListDirectory, Stat, ReadFile, and StreamFile.
- Control plane uses kubernetes apiserver proxy or port forward to reach agent when direct networking is unavailable. :contentReference[oaicite:1]{index=1}
- Optional mutating webhook will inject sidecar or ephemeral container patterns in environments that reject image changes. :contentReference[oaicite:2]{index=2}

## Security model summary
- Read only by design
- mTLS for all connections in production. Use cert-manager for cert lifecycle. :contentReference[oaicite:3]{index=3}
- RBAC enforced at control plane using TokenReview and SubjectAccessReview
- Path allowlist and denylist with explicit defaults for common secrets paths
- Auditable every file access to immutable external logs

## Deployment modes
1. Embedded agent binary in image for teams that can rebuild images
2. Sidecar injection via mutating webhook for automatic adoption. :contentReference[oaicite:4]{index=4}
3. Ephemeral container flow for on demand sessions in locked clusters. :contentReference[oaicite:5]{index=5}

## MVP scope
- CLI only
- Agent proto implemented in Go
- Port-forward connection path for MVP
- Basic path allowlist, file size and rate limits
- Structured audit logs to stdout and optional aggregator

## Success criteria
- Works on EKS GKE AKS
- Security team sign off for non production
- File reads limited, audited, and RBAC enforced
