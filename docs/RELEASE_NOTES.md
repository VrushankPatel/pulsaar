# Release Notes

## v1.0.0 - Stable Release

**Release Date:** January 6, 2026

This is the first stable release of Pulsaar, providing production-safe, auditable, read-only file exploration for Kubernetes pods without requiring kubectl exec or shell access.

### 🚀 New Features

#### Core Functionality
- **Read-only file operations**: ListDirectory, ReadFile, Stat, and StreamFile with comprehensive path allowlists and denylists
- **Security-first design**: All operations are read-only with explicit path restrictions and file size limits (1MB default)
- **Audit logging**: Every file access is logged to stdout in structured JSON format for compliance and monitoring
- **Multiple deployment modes**:
  - Embedded agent in container images
  - Sidecar injection via mutating webhook
  - Ephemeral container injection for on-demand access

#### Security & Compliance
- **mTLS encryption**: Production-ready mutual TLS with certificate management via environment variables
- **RBAC integration**: Control plane RBAC enforcement using Kubernetes TokenReview and SubjectAccessReview APIs
- **Path security**: Configurable allowlists and denylists with defaults blocking common secret paths
- **Certificate lifecycle management**: Support for cert-manager integration and static certificate loading

#### CLI & Connectivity
- **Flexible connection methods**:
  - kubectl port-forward (default, MVP-friendly)
  - Kubernetes API server proxy (for restricted environments)
  - Ephemeral container injection (for locked clusters)
- **Comprehensive CLI**: explore, read, stat, stream commands with TLS and RBAC support

#### Production Features
- **Helm charts**: Complete Kubernetes deployment with configurable TLS, RBAC, monitoring, and high availability
- **Monitoring & Observability**:
  - Prometheus metrics exported from agent and webhook
  - ServiceMonitor for automatic metric collection
  - Structured audit logs with optional HTTP aggregator for centralized logging
- **High availability**: Multi-replica deployment with load balancing and anti-affinity rules
- **Backup & recovery**: Procedures for configuration and audit data backup

#### Infrastructure & CI/CD
- **Cross-platform binaries**: Pre-built binaries for Linux, macOS, and Windows with GPG signatures and checksums
- **Docker images**: Multi-stage builds for all components (agent, CLI, webhook, aggregator)
- **CI/CD pipeline**: Automated building, testing, and publishing with dependency vulnerability scanning
- **Release automation**: GitHub Releases with reproducible builds and security signatures

### 📋 Supported Platforms
- **Kubernetes distributions**: EKS, GKE, AKS (tested and verified)
- **Operating systems**: Linux, macOS, Windows
- **Architectures**: amd64, arm64

### 🛠️ Installation Options
- **Homebrew**: `brew install pulsaar-cli`
- **Debian/Ubuntu packages**: .deb packages from GitHub Releases
- **Docker images**: Available on Docker Hub as `vrushankpatel/pulsaar-*`
- **Helm charts**: Production deployment via `helm install pulsaar ./charts/pulsaar`
- **From source**: Go 1.19+ required

### 📚 Documentation
- **API Reference**: Complete gRPC API documentation
- **Deployment Guide**: Step-by-step deployment instructions for all modes
- **Troubleshooting**: Common issues and solutions
- **Runbooks**: Deployment, upgrades, and incident response procedures
- **Security**: Sign-off request template and security considerations

### 🔧 Configuration
- **Environment variables**: Flexible configuration via env vars for certificates, audit settings, and limits
- **Helm values**: Comprehensive configuration options for production deployments
- **Path allowlists**: Configurable allowed root directories for file access

### 🧪 Testing & Validation
- **Unit tests**: Path sanitization, allowlist enforcement, and core functionality
- **Integration tests**: End-to-end testing on local and cloud clusters
- **Validation scripts**: Repository validation and deployment verification
- **Test procedures**: Documented testing for EKS, GKE, and AKS

### 🔒 Security Highlights
- **Zero shell access**: No exec or shell functionality - read-only by design
- **Audit trail**: Every operation logged with timestamps and user context
- **mTLS required**: Production deployments mandate mutual TLS encryption
- **RBAC enforced**: Kubernetes RBAC controls access at the control plane level
- **Path restrictions**: Explicit allowlists prevent access to sensitive directories

### 📈 Performance
- **File size limits**: 1MB default limit with streaming support for large files
- **Efficient streaming**: Chunked file streaming to handle large log files
- **Lightweight agent**: Minimal resource footprint suitable for sidecar deployment

### 🤝 Community & Support
- **Open source**: Apache 2.0 licensed
- **Contributing guidelines**: Clear contribution process with validation scripts
- **Issue tracking**: GitHub Issues for bug reports and feature requests

### 🐛 Known Limitations
- Ephemeral container injection requires Kubernetes 1.23+

### ✅ Final Deployment Verification

Pulsaar v1.0.0 has been successfully deployed and verified on all supported Kubernetes platforms:

#### EKS (Amazon Elastic Kubernetes Service)
- ✅ Sidecar injection via mutating webhook
- ✅ RBAC enforcement with IAM integration
- ✅ mTLS connections using cert-manager certificates
- ✅ Audit logging to stdout and optional aggregator
- ✅ Path allowlists and size limits enforced
- ✅ CLI operations (explore, read, stat, stream) functional
- ✅ High availability deployment with multiple replicas

#### GKE (Google Kubernetes Engine)
- ✅ Sidecar injection via mutating webhook
- ✅ RBAC enforcement with Workload Identity
- ✅ mTLS connections using cert-manager certificates
- ✅ Audit logging to stdout and optional aggregator
- ✅ Path allowlists and size limits enforced
- ✅ CLI operations (explore, read, stat, stream) functional
- ✅ High availability deployment with multiple replicas

#### AKS (Azure Kubernetes Service)
- ✅ Sidecar injection via mutating webhook
- ✅ RBAC enforcement with Azure AD integration
- ✅ mTLS connections using cert-manager certificates
- ✅ Audit logging to stdout and optional aggregator
- ✅ Path allowlists and size limits enforced
- ✅ CLI operations (explore, read, stat, stream) functional
- ✅ High availability deployment with multiple replicas

All verification tests passed according to the documented [Testing Procedure](TESTING_PROCEDURE.md).

### 📞 Migration Notes
This is the initial stable release - no migration needed from previous versions.

### 🙏 Acknowledgments
Built with Go, gRPC, and Kubernetes best practices. Thanks to the Kubernetes community and security teams for guidance on safe container access patterns.

---

For installation instructions, see the [README](../README.md). For deployment details, refer to the [Deployment Guide](DEPLOYMENT_GUIDE.md).