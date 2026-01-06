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

## High Availability Deployment

For production environments requiring high availability, deploy multiple replicas of the webhook and aggregator components with load balancing.

### Configuring Replicas

The Helm chart defaults to 3 replicas for both webhook and aggregator for HA. You can adjust in `values.yaml`:

```yaml
webhook:
  replicaCount: 3

aggregator:
  replicaCount: 3
```

### Load Balancing

Kubernetes Services automatically provide load balancing across replicas. The webhook and aggregator services distribute traffic evenly.

### Node Affinity and Anti-Affinity

To ensure replicas are spread across different nodes for fault tolerance, configure anti-affinity in `values.yaml`:

```yaml
webhook:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/component
            operator: In
            values:
            - webhook
        topologyKey: kubernetes.io/hostname

aggregator:
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app.kubernetes.io/component
              operator: In
              values:
              - aggregator
          topologyKey: kubernetes.io/hostname
```

This ensures webhook pods run on different nodes (required), and aggregator pods prefer different nodes.

### Monitoring HA Setup

Ensure Prometheus is scraping all replicas for comprehensive monitoring. The ServiceMonitor will automatically discover all pods.

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

### Local Testing

Use the test deployment script for local clusters (minikube, kind, etc.):

```bash
bash scripts/test_deployment.sh local
```

This will deploy a test setup on your local cluster and verify functionality.

### Cloud Testing (EKS, GKE, AKS)

For testing on managed Kubernetes services:

1. **Configure kubectl** for your target cluster:
   - EKS: `aws eks update-kubeconfig --region <region> --name <cluster-name>`
   - GKE: `gcloud container clusters get-credentials <cluster-name> --region <region>`
   - AKS: `az aks get-credentials --resource-group <rg> --name <cluster-name>`

2. **Set environment variables** for kubeconfig if needed:
   ```bash
   export KUBECONFIG_EKS=/path/to/eks/config
   export KUBECONFIG_GKE=/path/to/gke/config
   export KUBECONFIG_AKS=/path/to/aks/config
   ```

3. **Run the test script** for each cloud provider:
   ```bash
   # Test on EKS
   bash scripts/test_deployment.sh eks

   # Test on GKE
   bash scripts/test_deployment.sh gke

   # Test on AKS
   bash scripts/test_deployment.sh aks
   ```

4. **Verify functionality**:
   - The script deploys a test pod with the Pulsaar agent
   - Tests all CLI commands: explore, read, stat, stream
   - Tests both port-forward and apiserver-proxy connection methods
   - Cleans up test resources automatically

### Expected Results

- All CLI commands should execute successfully without errors
- File operations should respect path allowlists and size limits
- Audit logs should be generated for each operation
- TLS connections should be established securely
- RBAC checks should pass for authorized users

If any tests fail, check the troubleshooting guide for common issues.

## Final Deployment Verification

After deploying Pulsaar in production, perform these verification steps to ensure everything is working correctly.

### 1. Verify Agent Health

Check that the Pulsaar agent is running and healthy:

```bash
# For embedded agent
kubectl get pods -l app=your-app
kubectl logs -l app=your-app -c pulsaar-agent

# For sidecar injection
kubectl get pods -n your-namespace
kubectl logs -n your-namespace -l pulsaar.io/injected=true

# Check agent health endpoint
kubectl port-forward pod/your-pod 8443:8443
curl -k https://localhost:8443/health
```

Expected output:
```json
{"ready":true,"version":"v1.0.0","status_message":"Agent ready"}
```

### 2. Test CLI Connectivity

Verify the CLI can connect and perform operations:

```bash
# Test with port-forward (default)
./cli explore --pod your-pod --namespace your-namespace --path /

# Test with apiserver proxy
./cli explore --pod your-pod --namespace your-namespace --connection-method apiserver-proxy --path /

# Test file operations
./cli stat --pod your-pod --namespace your-namespace --path /etc/hostname
./cli read --pod your-pod --namespace your-namespace --path /etc/hostname
```

### 3. Verify Security Controls

Ensure security features are working:

```bash
# Test path allowlist enforcement (should fail)
./cli read --pod your-pod --namespace your-namespace --path /etc/shadow

# Check audit logs are generated
kubectl logs -l app=your-app -c pulsaar-agent | grep ReadFile
```

### 4. Verify TLS Configuration

Confirm mTLS is properly configured:

```bash
# Check certificates are loaded
kubectl exec your-pod -- env | grep PULSAAR_TLS

# Verify TLS handshake (CLI should connect without errors)
./cli explore --pod your-pod --namespace your-namespace --path /
```

### 5. Check Monitoring

Ensure metrics are being exported:

```bash
# Port forward metrics port
kubectl port-forward service/pulsaar-webhook 9090:9090 -n pulsaar-system

# Query metrics
curl http://localhost:9090/metrics | grep pulsaar
```

Expected metrics:
- `pulsaar_operations_total`
- `pulsaar_errors_total`
- `pulsaar_file_size_bytes`

### 6. Verify Audit Logging

Check that audit logs are being generated and forwarded:

```bash
# Check agent logs for audit entries
kubectl logs -l app=your-app -c pulsaar-agent | tail -10

# If using aggregator, check aggregator logs
kubectl logs -l app=pulsaar-aggregator -n pulsaar-system
```

### 7. Test High Availability (if deployed)

For HA deployments, verify load balancing:

```bash
# Check multiple replicas
kubectl get pods -l app=pulsaar-webhook -n pulsaar-system

# Verify anti-affinity
kubectl describe pods -l app=pulsaar-webhook -n pulsaar-system | grep "Node:"
```

### 8. RBAC Verification

Test that RBAC is enforced:

```bash
# Switch to unauthorized user context
kubectl config use-context unauthorized-user

# This should fail with RBAC error
./cli explore --pod your-pod --namespace your-namespace --path /
```

### 9. Performance Validation

Run performance checks:

```bash
# Time file operations
time ./cli read --pod your-pod --namespace your-namespace --path /large-file.log

# Check resource usage
kubectl top pods -l app=your-app
```

### 10. Backup Verification

Test backup procedures:

```bash
# Run backup script
bash scripts/backup_config.sh

# Verify backup files exist
ls -la backup-*.tar.gz
```

### Verification Checklist

- [ ] Agent pods are running and healthy
- [ ] CLI can connect via both port-forward and apiserver-proxy
- [ ] File operations work within allowlists
- [ ] Access is blocked for restricted paths
- [ ] TLS connections are secure
- [ ] Audit logs are generated for all operations
- [ ] Prometheus metrics are available
- [ ] RBAC controls access appropriately
- [ ] High availability setup distributes load
- [ ] Backup procedures work correctly

If all checks pass, your Pulsaar deployment is ready for production use.