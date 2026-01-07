# Pulsaar

![Pulsaar Banner](docs/img/banner.png)

[![codecov](https://codecov.io/gh/VrushankPatel/pulsaar/graph/badge.svg?token=GDvWVNIFUD)](https://codecov.io/gh/VrushankPatel/pulsaar)

Pulsaar is a production-safe, auditable, read-only file exploration tool for Kubernetes pods without using kubectl exec or granting shell access.

## Problem

Platform and security teams forbid kubectl exec and shell logins in production. Developers still need safe, auditable, read-only access to container filesystems for troubleshooting and compliance.

## Solution

Pulsaar provides a gRPC-based agent that runs inside pods, serving read-only file operations (ListDirectory, Stat, ReadFile, StreamFile) with path allowlists, size limits, and comprehensive audit logging.

## Features

- **Read-only operations**: List directories, read files, stat files, stream files
- **Security**: Path allowlists/denylists, mTLS encryption, RBAC integration, rate limiting
- **Audit logging**: All operations logged to stdout with optional HTTP aggregator
- **Deployment modes**:
  - Embedded agent in container images
  - Sidecar injection via mutating webhook
  - Ephemeral container injection for on-demand access

## Documentation

- [Release Notes](docs/RELEASE_NOTES.md)
- [API Reference](docs/API_REFERENCE.md)
- [Deployment Guide](docs/DEPLOYMENT_GUIDE.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## Installation

### From Packages

#### Homebrew (macOS/Linux)

```bash
brew tap VrushankPatel/homebrew-pulsaar
brew install pulsaar-cli
```

#### Debian/Ubuntu

Download the .deb package from [GitHub Releases](https://github.com/VrushankPatel/pulsaar/releases) and install:

```bash
sudo dpkg -i pulsaar-cli_*.deb
```

### From Source

## Quick Start

### 1. Build the binaries

```bash
go build -o agent ./cmd/agent
go build -o cli ./cmd/cli
```

### 2. Deploy the agent

#### Option A: Embedded in your application image

Add the agent binary to your container image and run it alongside your app:

```dockerfile
COPY agent /usr/local/bin/pulsaar-agent
RUN chmod +x /usr/local/bin/pulsaar-agent

# Run agent with your app
CMD ["/usr/local/bin/pulsaar-agent"]
```

Set environment variables for TLS certificates:

```bash
export PULSAAR_TLS_CERT_FILE=/path/to/server.crt
export PULSAAR_TLS_KEY_FILE=/path/to/server.key
export PULSAAR_TLS_CA_FILE=/path/to/ca.crt  # For client cert verification
```

#### Option B: Sidecar injection

Apply the mutating webhook:

```bash
kubectl apply -f manifests/webhook.yaml
```

Set the agent image for injection (optional, defaults to `pulsaar/agent:latest`):

```bash
export PULSAAR_AGENT_IMAGE=your-registry/pulsaar-agent:v1.0
```

Annotate your pods for injection:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    pulsaar.io/inject-agent: "true"
spec:
  containers:
  - name: app
    image: your-app
```

#### Option C: Ephemeral container (for locked clusters)

The CLI can inject ephemeral containers on-demand.

### 3. Use the CLI

Explore files in a pod:

```bash
./cli explore --pod my-pod --namespace default --path /
```

Read a file:

```bash
./cli read --pod my-pod --namespace default --path /app/config.yaml
```

Note: When reading binary files, the CLI will display a warning as output may be corrupted.

Stream a large file:

```bash
./cli stream --pod my-pod --namespace default --path /var/log/app.log
```

Note: When streaming binary files, the CLI will display a warning as output may be corrupted.

 Get file info:

```bash
./cli stat --pod my-pod --namespace default --path /app/data.txt
```

Check agent health:

```bash
./cli health --pod my-pod --namespace default
```

Generate shell completion:

```bash
# Bash
./cli completion bash > /etc/bash_completion.d/pulsaar

# Zsh
./cli completion zsh > "${fpath[1]}/_pulsaar"

# Fish
./cli completion fish > ~/.config/fish/completions/pulsaar.fish

 # PowerShell
 ./cli completion powershell > pulsaar.ps1
 ```

Generate man pages:

```bash
./cli man
```

This generates man pages in the `man/` directory.

## Connection Methods

### Port Forward (default)

Uses `kubectl port-forward` to connect to the agent:

```bash
./cli explore --pod my-pod --connection-method port-forward
```

### API Server Proxy

Connects via Kubernetes API server proxy (useful when port-forward is blocked):

```bash
./cli explore --pod my-pod --connection-method apiserver-proxy
```

## TLS Configuration

### For MVP (self-signed)

No environment variables needed - agent generates self-signed certificates.

### For Production (mTLS)

Set these environment variables for the agent:

- `PULSAAR_TLS_CERT_FILE`: Path to server certificate
- `PULSAAR_TLS_KEY_FILE`: Path to server private key
- `PULSAAR_TLS_CA_FILE`: Path to CA certificate for client verification

For the CLI:

- `PULSAAR_CLIENT_CERT_FILE`: Path to client certificate
- `PULSAAR_CLIENT_KEY_FILE`: Path to client private key
- `PULSAAR_CA_FILE`: Path to CA certificate

Example:

```bash
# Agent
export PULSAAR_TLS_CERT_FILE=/etc/ssl/certs/pulsaar.crt
export PULSAAR_TLS_KEY_FILE=/etc/ssl/private/pulsaar.key
export PULSAAR_TLS_CA_FILE=/etc/ssl/certs/ca.crt

# CLI
export PULSAAR_CLIENT_CERT_FILE=/etc/ssl/certs/client.crt
export PULSAAR_CLIENT_KEY_FILE=/etc/ssl/private/client.key
export PULSAAR_CA_FILE=/etc/ssl/certs/ca.crt
```

## Path Allowlist Configuration

The agent enforces path allowlists to restrict file access. Configuration is checked in this order:

1. **Pod annotations** (highest priority): Set `pulsaar.io/allowed-roots` annotation on the pod with comma-separated paths
2. **Namespace ConfigMap**: Create a ConfigMap named `pulsaar-config` in the namespace with `allowed-roots` key
3. **Environment variable**: Set `PULSAAR_ALLOWED_ROOTS` with comma-separated paths
4. **Default**: `/` (allows all paths)

Example pod annotation:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    pulsaar.io/allowed-roots: "/app,/tmp"
spec:
  containers:
  - name: app
    image: your-app
```

## Audit Logging

All file operations are logged to stdout in JSON format:

```json
{"timestamp":"2023-01-01T12:00:00Z","operation":"ReadFile","path":"/app/config.yaml"}
```

To send logs to an aggregator:

```bash
export PULSAAR_AUDIT_AGGREGATOR_URL=https://your-log-aggregator.com/logs
```

## Security Considerations

- Read-only operations only
- Path allowlists enforced
- File size limits (1MB default)
- Per-IP rate limiting to prevent abuse
- mTLS encryption required in production
- RBAC integration
- Audit logging for compliance

## Development

### Prerequisites

- Go 1.19+
- protoc (for generating protobuf stubs)
- kubectl (for testing)

### Building

```bash
# Generate protobuf
protoc --go_out=. --go-grpc_out=. api/pulsaar.proto

# Build all
go build ./cmd/...
```

### Testing

```bash
go test ./...
```

### Deployment Testing

Test Pulsaar deployment on Kubernetes clusters:

```bash
# Build binaries
go build -o agent ./cmd/agent
go build -o cli ./cmd/cli
mkdir -p pulsaar && cp cli pulsaar/cli

# Test on local cluster
./scripts/test_deployment.sh local

# Deploy full Pulsaar system on cloud clusters
# Set kubeconfig environment variables as needed
export KUBECONFIG_EKS=/path/to/eks/kubeconfig
export KUBECONFIG_GKE=/path/to/gke/kubeconfig
export KUBECONFIG_AKS=/path/to/aks/kubeconfig

# Deploy and verify on EKS
./scripts/deploy_cloud.sh eks

# Deploy and verify on GKE
./scripts/deploy_cloud.sh gke

# Deploy and verify on AKS
./scripts/deploy_cloud.sh aks
```

### Validation

```bash
bash scripts/validate_repo.sh
```

## Architecture

- **Agent**: gRPC server running in pods, serves file operations
- **CLI**: Client that connects to agent via port-forward or API proxy
- **Webhook**: Mutating admission webhook for sidecar injection
- **API**: Protocol buffer definitions for gRPC service

## Contributing

1. Follow the vision in `vision.md`
2. Obey rules in `rules.md`
3. Check progress in `progress.md`
4. Run validation before committing

## License

Licensed under Apache License 2.0