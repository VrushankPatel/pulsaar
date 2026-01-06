#!/usr/bin/env bash
set -euo pipefail

# Test deployment script for Pulsaar on Kubernetes clusters (EKS, GKE, AKS)
# This script deploys a test pod with the agent and test files, then tests CLI functionality

# Usage: ./test_deployment.sh [eks|gke|aks|local]
CLOUD=${1:-local}

echo "Testing Pulsaar deployment on $CLOUD Kubernetes cluster..."

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
    local)
        # Use default kubeconfig
        ;;
    *)
        echo "Unknown cloud: $CLOUD. Supported: eks, gke, aks, local"
        exit 1
        ;;
esac

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl not found. Please install kubectl."
    exit 1
fi

if ! command -v bin/cli >/dev/null 2>&1; then
    echo "bin/cli not found. Please build the CLI first."
    exit 1
fi

# Agent binary is included in the Docker image

# Deploy test resources
echo "Deploying test pod..."
kubectl apply -f manifests/test-deployment.yaml

# Wait for pod to be ready
echo "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/pulsaar-test-pod --timeout=60s

# Test CLI functionality
echo "Testing CLI explore..."
bin/cli explore --pod pulsaar-test-pod --namespace default --path /app --connection-method port-forward

echo "Testing CLI read..."
bin/cli read --pod pulsaar-test-pod --namespace default --path /app/config.yaml --connection-method port-forward

echo "Testing CLI stat..."
bin/cli stat --pod pulsaar-test-pod --namespace default --path /app/log.txt --connection-method port-forward

echo "Testing CLI stream..."
bin/cli stream --pod pulsaar-test-pod --namespace default --path /app/log.txt --connection-method port-forward

# Test with apiserver proxy if supported
echo "Testing CLI with apiserver proxy..."
bin/cli explore --pod pulsaar-test-pod --namespace default --path /app --connection-method apiserver-proxy

echo "Testing CLI read with apiserver proxy..."
bin/cli read --pod pulsaar-test-pod --namespace default --path /app/config.yaml --connection-method apiserver-proxy

# Clean up
echo "Cleaning up test resources..."
kubectl delete -f manifests/test-deployment.yaml

echo "Deployment test completed successfully!"