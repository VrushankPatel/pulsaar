# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created
- rules.md created
- CONTRIBUTING.md created
- LICENSE created
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
- Production deployment planning completed
- Helm charts created for easy Kubernetes deployment with configurable TLS, RBAC, and monitoring options
- Production monitoring with Prometheus metrics exported from agent and webhook
- Audit aggregator implemented for centralized logging integration, receiving audit logs from agents and forwarding to external systems
- Comprehensive documentation created including API reference, deployment guides, and troubleshooting
- High availability deployment with multiple replicas and load balancing implemented in Helm chart and documentation
- Dependency vulnerability checks added to CI/CD pipeline
- Implemented backup and recovery procedures for configuration and audit data
- Runbooks created for deployment, upgrades, and incident response
- Security sign-off request document created
- CI/CD pipeline builds and pushes Docker images for agent, aggregator, cli, and webhook components
- Release process documented in CONTRIBUTING.md
- Implemented per-IP rate limiting for file operations to prevent abuse
- Bash completion for CLI added
- Man pages for CLI added
- Added support for custom path allowlists per namespace via ConfigMap
- Audit logs stored locally in aggregator persistent volume for backup and recovery
- Coverage report exporting added in CI
- Helm chart bugs fixed for proper deployment on Kubernetes clusters
- Local deployment testing completed with kind cluster
- Security team sign-off obtained for non-production use
- Stable release artifacts publishing configured with signed binaries, checksums, reproducible builds, Debian packages, and Homebrew formulas

### Decisions log
  - Default MVP connection: kubectl port-forward or apiserver proxy
  - mTLS production requirement via cert-manager
  - Max read size set to 1MB for MVP
  - Used exec.Command for kubectl port-forward in CLI for MVP simplicity
  - Optional audit aggregator sends structured JSON logs via HTTP POST
  - Certificate loading via env vars PULSAAR_TLS_CERT_FILE, PULSAAR_TLS_KEY_FILE, PULSAAR_TLS_CA_FILE for agent
  - Client certs via PULSAAR_CLIENT_CERT_FILE, PULSAAR_CLIENT_KEY_FILE, PULSAAR_CA_FILE for CLI
  - Docker images tagged as vrushankpatel/pulsaar-{component}:latest

### Next steps





