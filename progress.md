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
- CLI binary built
- Agent binary built
- Webhook binary built
- Audit logs implemented for all operations (ListDirectory, ReadFile, Stat, StreamFile) to stdout
- Certificate management implemented for production mTLS (load from files via env vars, mTLS with client cert verification)
- CLI supports apiserver proxy connection path for connecting to agent without kubectl port-forward
- RBAC enforced at control plane using TokenReview and SubjectAccessReview
- Mutating webhook for sidecar injection implemented
- Test deployment on EKS, GKE, and AKS implemented with scripts/test_deployment.sh and manifests/test-deployment.yaml
- Security team sign-off obtained for non-production use
- Production deployment planning completed
- Helm charts created for easy Kubernetes deployment with configurable TLS, RBAC, and monitoring options
- Production monitoring with Prometheus metrics exported from agent and webhook
- Production Docker images built and pushed to docker.io/vrushankpatel/pulsaar-{component}:latest
  - Audit aggregator implemented for centralized logging integration, receiving audit logs from agents and forwarding to external systems
  - Comprehensive documentation created including API reference, deployment guides, and troubleshooting
  - High availability deployment with multiple replicas and load balancing implemented in Helm chart and documentation

- Security scanning and dependency vulnerability checks added to CI/CD pipeline

### Last commit summary
    - Implemented high availability deployment with multiple replicas and load balancing in Helm chart and documentation

### Decisions log
  - Default MVP connection: kubectl port-forward or apiserver proxy
  - mTLS production requirement via cert-manager
  - Max read size set to 1MB for MVP
  - Used exec.Command for kubectl port-forward in CLI for MVP simplicity
  - Optional audit aggregator sends structured JSON logs via HTTP POST
  - Certificate loading via env vars PULSAAR_TLS_CERT_FILE, PULSAAR_TLS_KEY_FILE, PULSAAR_TLS_CA_FILE for agent
  - Client certs via PULSAAR_CLIENT_CERT_FILE, PULSAAR_CLIENT_KEY_FILE, PULSAAR_CA_FILE for CLI
  - Docker images tagged as vrushankpatel/pulsaar-{component}:latest

- Added gosec for code security scanning, govulncheck for Go dependency vulnerability checks, and Trivy for container image vulnerability scanning in CI/CD

### Production Deployment Plan
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

### Known issues
  - Security team adoption risk

### Next steps
- Implement backup and recovery procedures for configuration and audit data
- Create runbooks for deployment, upgrades, and incident response