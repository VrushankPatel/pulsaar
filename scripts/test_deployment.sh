#!/usr/bin/env bash
set -euo pipefail

# Test deployment script for Pulsaar on Kubernetes clusters (EKS, GKE, AKS)
# This script deploys a test pod with the agent and test files, then tests CLI functionality

echo "Testing Pulsaar deployment on Kubernetes cluster..."

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl not found. Please install kubectl."
    exit 1
fi

if ! command -v ./pulsaar/cli >/dev/null 2>&1; then
    echo "./pulsaar/cli not found. Please build the CLI first."
    exit 1
fi

# Copy agent binary to expected location for hostPath volume
cp ./agent /tmp/agent
chmod +x /tmp/agent

# Deploy test resources
echo "Deploying test pod..."
kubectl apply -f manifests/test-deployment.yaml

# Wait for pod to be ready
echo "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/pulsaar-test-pod --timeout=60s

# Test CLI functionality
echo "Testing CLI explore..."
./pulsaar/cli explore --pod pulsaar-test-pod --namespace default --path /app

echo "Testing CLI read..."
./pulsaar/cli read --pod pulsaar-test-pod --namespace default --path /app/config.yaml

echo "Testing CLI stat..."
./pulsaar/cli stat --pod pulsaar-test-pod --namespace default --path /app/log.txt

echo "Testing CLI stream..."
./pulsaar/cli stream --pod pulsaar-test-pod --namespace default --path /app/log.txt

# Test with apiserver proxy if supported
echo "Testing CLI with apiserver proxy..."
./pulsaar/cli explore --pod pulsaar-test-pod --namespace default --path /app --connection-method apiserver-proxy

echo "Testing CLI read with apiserver proxy..."
./pulsaar/cli read --pod pulsaar-test-pod --namespace default --path /app/config.yaml --connection-method apiserver-proxy

# Clean up
echo "Cleaning up test resources..."
kubectl delete -f manifests/test-deployment.yaml
rm -f /tmp/agent

echo "Deployment test completed successfully!"