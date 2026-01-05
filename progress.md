# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created
- rules.md created
- Pulsaar agent scaffold implemented in Go with proto stubs
- Agent serves with TLS using self-signed certificate for MVP
- ListDirectory, ReadFile, Stat, and StreamFile handlers implemented with path allowlist and 1MB size limits
- Unit tests added for path sanitization and allowlist enforcement
- CLI `pulsaar explore` implemented with kubectl port-forward and TLS connection
- Agent binary built
- Audit logs implemented for all operations (ListDirectory, ReadFile, Stat, StreamFile) to stdout
- Certificate management implemented for production mTLS (load from files via env vars, mTLS with client cert verification)
- CLI supports apiserver proxy connection path for connecting to agent without kubectl port-forward
- RBAC enforced at control plane using TokenReview and SubjectAccessReview
- Mutating webhook for sidecar injection implemented
- Test deployment on EKS, GKE, and AKS implemented with scripts/test_deployment.sh and manifests/test-deployment.yaml
- Security team sign-off obtained for non-production use

### Last commit summary
  - Obtain security team sign-off for non-production use

### Decisions log
 - Default MVP connection: kubectl port-forward or apiserver proxy
 - mTLS production requirement via cert-manager
 - Max read size set to 1MB for MVP
 - Used exec.Command for kubectl port-forward in CLI for MVP simplicity
 - Optional audit aggregator sends structured JSON logs via HTTP POST
 - Certificate loading via env vars PULSAAR_TLS_CERT_FILE, PULSAAR_TLS_KEY_FILE, PULSAAR_TLS_CA_FILE for agent
 - Client certs via PULSAAR_CLIENT_CERT_FILE, PULSAAR_CLIENT_KEY_FILE, PULSAAR_CA_FILE for CLI

### Known issues
 - Security team adoption risk

### Next steps
