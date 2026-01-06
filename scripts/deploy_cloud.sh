#!/usr/bin/env bash
set -euo pipefail

# Deploy Pulsaar on EKS, GKE, AKS clusters and verify functionality
# This script deploys the full Pulsaar system using Helm and verifies functionality

# Usage: ./deploy_cloud.sh [eks|gke|aks]
CLOUD=${1:-}

if [ -z "$CLOUD" ]; then
    echo "Usage: $0 [eks|gke|aks]"
    exit 1
fi

echo "Deploying Pulsaar on $CLOUD Kubernetes cluster..."

# Set kubeconfig based on cloud
case $CLOUD in
    eks)
        export KUBECONFIG=${KUBECONFIG_EKS:-$HOME/.kube/eks-config}
        ;;
    gke)
        export KUBECONFIG=${KUBECONFIG_GKE:-$HOME/.kube/gke-config}
        ;;
    aks)
        export KUBECONFIG=${KUBECONFIG_AKS:-$HOME/.kube/aks-config}
        ;;
    *)
        echo "Unknown cloud: $CLOUD. Supported: eks, gke, aks"
        exit 1
        ;;
esac

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl not found. Please install kubectl."
    exit 1
fi

if ! command -v helm >/dev/null 2>&1; then
    echo "helm not found. Please install helm."
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "Cannot access Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi

# Create namespace if it doesn't exist
kubectl create namespace pulsaar-system --dry-run=client -o yaml | kubectl apply -f -

# Deploy Pulsaar using Helm
echo "Deploying Pulsaar Helm chart..."
helm upgrade --install pulsaar ./charts/pulsaar \
    --namespace pulsaar-system \
    --wait \
    --timeout=10m \
    --set tls.enabled=true \
    --set rbac.enabled=true \
    --set monitoring.enabled=true \
    --set aggregator.enabled=true

# Wait for all components to be ready
echo "Waiting for Pulsaar components to be ready..."
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/instance=pulsaar --namespace pulsaar-system --timeout=300s

# Run verification tests
echo "Running verification tests..."
bash scripts/test_deployment.sh "$CLOUD"

echo "Pulsaar deployment and verification completed successfully on $CLOUD!"