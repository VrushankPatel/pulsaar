# Deployment Runbook

## Overview
This runbook provides step-by-step instructions for deploying Pulsaar in a Kubernetes cluster.

## Prerequisites
- Kubernetes cluster (EKS, GKE, AKS) with kubectl access
- Helm 3+ installed
- Docker for building images (if not using pre-built)
- Go 1.19+ (if building from source)

## Deployment Steps

### 1. Prepare Environment
```bash
# Verify cluster access
kubectl cluster-info

# Create namespace
kubectl create namespace pulsaar-system

# Verify Helm
helm version
```

### 2. Build Components (Optional - skip if using pre-built images)
```bash
# Generate protobuf stubs
protoc --go_out=. --go-grpc_out=. api/pulsaar.proto

# Build binaries
go build -o agent ./cmd/agent
go build -o cli ./cmd/cli
go build -o webhook ./cmd/webhook
go build -o aggregator ./cmd/aggregator

# Build and push Docker images
docker build -f Dockerfile.agent -t vrushankpatel/pulsaar-agent:latest .
docker build -f Dockerfile.cli -t vrushankpatel/pulsaar-cli:latest .
docker build -f Dockerfile.webhook -t vrushankpatel/pulsaar-webhook:latest .
docker build -f Dockerfile.aggregator -t vrushankpatel/pulsaar-aggregator:latest .

docker push vrushankpatel/pulsaar-agent:latest
docker push vrushankpatel/pulsaar-cli:latest
docker push vrushankpatel/pulsaar-webhook:latest
docker push vrushankpatel/pulsaar-aggregator:latest
```

### 3. Configure TLS Certificates
For production mTLS:
```bash
# Using cert-manager (recommended)
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: pulsaar-issuer
  namespace: pulsaar-system
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: pulsaar-tls
  namespace: pulsaar-system
spec:
  secretName: pulsaar-tls
  issuerRef:
    name: pulsaar-issuer
  dnsNames:
  - "*.your-cluster.svc.cluster.local"
EOF
```

### 4. Deploy via Helm
```bash
# Add Helm repo (if applicable) or use local chart
helm install pulsaar ./charts/pulsaar \
  --namespace pulsaar-system \
  --create-namespace \
  --set tls.enabled=true \
  --set rbac.enabled=true \
  --set monitoring.enabled=true
```

### 5. Configure RBAC
```bash
# Apply RBAC resources
kubectl apply -f - <<EOF
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
EOF
```

### 6. Verify Deployment
```bash
# Check pods
kubectl get pods -n pulsaar-system

# Check services
kubectl get svc -n pulsaar-system

# Check webhook
kubectl get mutatingwebhookconfigurations

# Verify health
kubectl port-forward -n pulsaar-system svc/pulsaar-webhook 8443:443 &
curl -k https://localhost:8443/health
```

### 7. Test Deployment
```bash
# Run test deployment script
bash scripts/test_deployment.sh
```

## Post-Deployment Configuration

### Enable Sidecar Injection
Annotate target pods:
```bash
kubectl annotate pod my-app pulsaar.io/inject-agent="true"
kubectl annotate pod my-app pulsaar.io/agent-image="vrushankpatel/pulsaar-agent:latest"
```

### Configure Audit Aggregator
```bash
# Deploy aggregator
kubectl apply -f - <<EOF
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
EOF
```

## Rollback Procedures
If deployment fails:
```bash
# Uninstall
helm uninstall pulsaar --namespace pulsaar-system

# Clean up
kubectl delete namespace pulsaar-system
```

## Monitoring
- Check Prometheus metrics at `/metrics`
- Monitor pod logs: `kubectl logs -n pulsaar-system -l app.kubernetes.io/name=pulsaar`
- Verify audit logs are being generated