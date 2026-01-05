# Deployment Guide

## Prerequisites

- Kubernetes cluster (EKS, GKE, AKS tested)
- kubectl configured
- Go 1.19+ for building from source
- Docker for building images
- Helm 3+ for Helm deployments

## Building Components

### Build Binaries Locally

```bash
# Generate protobuf stubs
protoc --go_out=. --go-grpc_out=. api/pulsaar.proto

# Build all components
go build -o agent ./cmd/agent
go build -o cli ./cmd/cli
go build -o webhook ./cmd/webhook
go build -o aggregator ./cmd/aggregator
```

### Build Docker Images

```bash
# Build images
docker build -f Dockerfile.agent -t vrushankpatel/pulsaar-agent:latest .
docker build -f Dockerfile.cli -t vrushankpatel/pulsaar-cli:latest .
docker build -f Dockerfile.webhook -t vrushankpatel/pulsaar-webhook:latest .
docker build -f Dockerfile.aggregator -t vrushankpatel/pulsaar-aggregator:latest .

# Push to registry
docker push vrushankpatel/pulsaar-agent:latest
docker push vrushankpatel/pulsaar-cli:latest
docker push vrushankpatel/pulsaar-webhook:latest
docker push vrushankpatel/pulsaar-aggregator:latest
```

## Deployment Modes

### 1. Embedded Agent

For teams that can modify container images.

#### Dockerfile Example

```dockerfile
FROM your-base-image

# Copy and setup agent
COPY agent /usr/local/bin/pulsaar-agent
RUN chmod +x /usr/local/bin/pulsaar-agent

# Expose port (default 8443)
EXPOSE 8443

# Run agent alongside your app
CMD ["sh", "-c", "/usr/local/bin/pulsaar-agent & your-main-command"]
```

#### Pod Configuration

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
spec:
  containers:
  - name: app
    image: your-app-with-agent
    env:
    - name: PULSAAR_TLS_CERT_FILE
      value: "/etc/ssl/certs/server.crt"
    - name: PULSAAR_TLS_KEY_FILE
      value: "/etc/ssl/private/server.key"
    - name: PULSAAR_TLS_CA_FILE
      value: "/etc/ssl/certs/ca.crt"
    volumeMounts:
    - name: tls-certs
      mountPath: /etc/ssl/certs
      readOnly: true
    - name: tls-keys
      mountPath: /etc/ssl/private
      readOnly: true
  volumes:
  - name: tls-certs
    secret:
      secretName: pulsaar-tls
  - name: tls-keys
    secret:
      secretName: pulsaar-tls
```

### 2. Sidecar Injection

For automatic adoption without image changes.

#### Deploy Mutating Webhook

```bash
# Create namespace
kubectl create namespace pulsaar-system

# Deploy webhook
kubectl apply -f manifests/webhook.yaml

# Verify
kubectl get pods -n pulsaar-system
```

#### Annotate Pods for Injection

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    pulsaar.io/inject-agent: "true"
    pulsaar.io/agent-image: "vrushankpatel/pulsaar-agent:latest"
spec:
  containers:
  - name: app
    image: your-app
```

The webhook will automatically inject the sidecar container.

### 3. Ephemeral Container

For on-demand access in locked clusters where image changes are prohibited.

The CLI will automatically inject ephemeral containers when using `--connection-method ephemeral`.

No manual deployment needed - handled by CLI.

## Helm Deployment

For production deployments, use the provided Helm chart.

```bash
# Install
helm install pulsaar ./charts/pulsaar --namespace pulsaar-system --create-namespace

# Upgrade
helm upgrade pulsaar ./charts/pulsaar --namespace pulsaar-system

# Uninstall
helm uninstall pulsaar --namespace pulsaar-system
```

### Helm Values Configuration

Key configuration options in `values.yaml`:

```yaml
# TLS configuration
tls:
  enabled: true
  certManager:
    enabled: true
    issuerName: "letsencrypt-prod"

# RBAC
rbac:
  enabled: true
  serviceAccountName: "pulsaar"

# Monitoring
monitoring:
  enabled: true
  prometheusRule:
    enabled: true

# Audit aggregator
aggregator:
  enabled: true
  image: vrushankpatel/pulsaar-aggregator:latest
```

## TLS Configuration

### MVP (Development)

No configuration needed - agent generates self-signed certificates.

### Production (mTLS)

Use cert-manager for automatic certificate management or provide static certificates.

#### Using cert-manager

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: pulsaar-issuer
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: pulsaar-tls
spec:
  secretName: pulsaar-tls
  issuerRef:
    name: pulsaar-issuer
  dnsNames:
  - "*.your-cluster.svc.cluster.local"
```

#### Static Certificates

Create secrets with certificate data:

```bash
kubectl create secret tls pulsaar-tls \
  --cert=server.crt \
  --key=server.key \
  --namespace your-namespace
```

Agent environment variables:

- `PULSAAR_TLS_CERT_FILE`: Path to server certificate (default: /etc/ssl/certs/tls.crt)
- `PULSAAR_TLS_KEY_FILE`: Path to server key (default: /etc/ssl/private/tls.key)
- `PULSAAR_TLS_CA_FILE`: Path to CA certificate for client verification

CLI environment variables:

- `PULSAAR_CLIENT_CERT_FILE`: Client certificate
- `PULSAAR_CLIENT_KEY_FILE`: Client key
- `PULSAAR_CA_FILE`: CA certificate

## RBAC Setup

For RBAC enforcement, create the necessary ClusterRole and bindings:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pulsaar-rbac
rules:
- apiGroups: ["authentication.k8s.io"]
  resources: ["tokenreviews"]
  verbs: ["create"]
- apiGroups: ["authorization.k8s.io"]
  resources: ["subjectaccessreviews"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pulsaar-rbac
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: pulsaar-rbac
subjects:
- kind: ServiceAccount
  name: pulsaar
  namespace: pulsaar-system
```

## Monitoring Setup

Agent and webhook expose Prometheus metrics on `/metrics` endpoint.

### Enable ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: pulsaar
  namespace: pulsaar-system
spec:
  selector:
    matchLabels:
      app: pulsaar
  endpoints:
  - port: metrics
    path: /metrics
```

### Metrics Available

- `pulsaar_operations_total`: Total operations by type
- `pulsaar_errors_total`: Total errors
- `pulsaar_file_size_bytes`: File sizes read
- `pulsaar_connection_duration_seconds`: Connection durations

## Audit Aggregator Deployment

For centralized logging:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pulsaar-aggregator
  namespace: pulsaar-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pulsaar-aggregator
  template:
    metadata:
      labels:
        app: pulsaar-aggregator
    spec:
      containers:
      - name: aggregator
        image: vrushankpatel/pulsaar-aggregator:latest
        ports:
        - containerPort: 8080
        env:
        - name: PULSAAR_AGGREGATOR_PORT
          value: "8080"
---
apiVersion: v1
kind: Service
metadata:
  name: pulsaar-aggregator
  namespace: pulsaar-system
spec:
  selector:
    app: pulsaar-aggregator
  ports:
  - port: 80
    targetPort: 8080
```

Configure agents to send logs:

```bash
export PULSAAR_AUDIT_AGGREGATOR_URL=http://pulsaar-aggregator.pulsaar-system.svc.cluster.local
```

## Testing Deployment

Use the test deployment script:

```bash
bash scripts/test_deployment.sh
```

This will deploy a test setup on your cluster and verify functionality.