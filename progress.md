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
- Stable release v1.0.0 tagged
- v1.0.0 tag created and pushed
- Testing procedure documented for EKS, GKE, and AKS clusters
- Dependencies locked with go.sum for reproducible builds
- Agent Health response version updated to v1.0.0
- Webhook agent image configurable via PULSAAR_AGENT_IMAGE environment variable
- Docker images built locally for agent, cli, webhook, and aggregator components
- Cross-platform release binaries built with checksums and GPG signatures
- Final deployment verification performed on local cluster (binaries built, validation passed)
- Security team sign-off obtained for non production use
- Documentation updated with release notes and final deployment verification

### Next steps
- Build and push production Docker images for agent, CLI, and webhook components to a container registry
- Create Helm charts for easy Kubernetes deployment with configurable TLS, RBAC, and monitoring options
- Implement production monitoring with Prometheus metrics exported from agent and webhook
- Set up centralized logging integration with audit logs sent to external systems
- Create comprehensive documentation including API reference, deployment guides, and troubleshooting
- Implement automated CI/CD pipeline for building, testing, and releasing components
- Add security scanning and dependency vulnerability checks
- Plan for high availability deployment with multiple replicas and load balancing
- Implement backup and recovery procedures for configuration and audit data
- Create runbooks for deployment, upgrades, and incident response

### Decisions log
  - Default MVP connection: kubectl port-forward or apiserver proxy
  - mTLS production requirement via cert-manager
  - Max read size set to 1MB for MVP
  - Used exec.Command for kubectl port-forward in CLI for MVP simplicity
  - Optional audit aggregator sends structured JSON logs via HTTP POST
  - Certificate loading via env vars PULSAAR_TLS_CERT_FILE, PULSAAR_TLS_KEY_FILE, PULSAAR_TLS_CA_FILE for agent
  - Client certs via PULSAAR_CLIENT_CERT_FILE, PULSAAR_CLIENT_KEY_FILE, PULSAAR_CA_FILE for CLI
  - Docker images tagged as vrushankpatel/pulsaar-{component}:latest





