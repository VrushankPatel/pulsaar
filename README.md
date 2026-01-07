# Pulsaar

[![CI/CD](https://github.com/VrushankPatel/pulsaar/actions/workflows/ci.yml/badge.svg)](https://github.com/VrushankPatel/pulsaar/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/VrushankPatel/pulsaar/graph/badge.svg?token=GDvWVNIFUD)](https://codecov.io/gh/VrushankPatel/pulsaar)
[![Go Report Card](https://goreportcard.com/badge/github.com/VrushankPatel/pulsaar)](https://goreportcard.com/report/github.com/VrushankPatel/pulsaar)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Production-safe, auditable, read-only file exploration for Kubernetes pods.

## Overview

Pulsaar solves the security challenge of inspecting container filesystems in production. It provides developers with safe, read-only access to troubleshoot issues without requiring `kubectl exec`, shell access, or elevated permissions.

### Key Features
- **Read-Only**: Strict enforcement of read-only operations (List, Read, Stat).
- **Secure**: mTLS encryption, RBAC integration, and granular path allowlists.
- **Auditable**: Comprehensive logging of all file access attempts.
- **Flexible**: Deploy via sidecar injection, ephemeral containers, or embedded agents.

## Installation

### CLI Tool
Download the latest release from the [Releases Page](https://github.com/VrushankPatel/pulsaar/releases).

**Homebrew (macOS/Linux)**
```bash
brew tap VrushankPatel/homebrew-pulsaar
brew install pulsaar-cli
```

### Cluster Components
Install the Pulsaar agent and webhook using Helm.

```bash
helm repo add pulsaar https://vrushankpatel.github.io/pulsaar
helm install pulsaar pulsaar/pulsaar --namespace pulsaar-system --create-namespace
```

## Usage

### Explore File System
List files in a specific pod directory.
```bash
pulsaar explore --pod my-pod -n default --path /var/log
```

### Read File Content
Securely read configuration or log files.
```bash
pulsaar read --pod my-pod -n default --path /app/config.json
```

### Check File Stats
Get file metadata (size, permissions, mod time).
```bash
pulsaar stat --pod my-pod -n default --path /tmp/app.lock
```

## Configuration

Control access using Kubernetes annotations on your pods.

```yaml
metadata:
  annotations:
    pulsaar.io/inject: "true"
    pulsaar.io/allowed-roots: "/var/log,/app/config"
```

## Contributing

We welcome contributions! Please read our [Contribution Guidelines](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a Pull Request.

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.