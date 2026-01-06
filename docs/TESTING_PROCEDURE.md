# Testing Procedure for Pulsaar Deployment on EKS, GKE, and AKS

This document outlines the procedure for performing final deployment verification of Pulsaar on Amazon EKS, Google GKE, and Azure AKS clusters.

## Prerequisites

- Kubernetes cluster (EKS/GKE/AKS) with kubectl access
- Helm 3.x installed
- Pulsaar CLI binary or Docker image
- Test pod deployed in the cluster

## Test Pod Deployment

Deploy a test pod for verification:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
  annotations:
    pulsaar.io/inject-agent: "true"
spec:
  containers:
  - name: test-container
    image: busybox
    command: ["sh", "-c", "echo 'Hello Pulsaar' > /tmp/test.txt && sleep 3600"]
    volumeMounts:
    - name: test-volume
      mountPath: /tmp
  volumes:
  - name: test-volume
    emptyDir: {}
```

Apply with: `kubectl apply -f test-pod.yaml`

## Helm Chart Deployment

1. Install cert-manager (required for TLS):
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   ```

2. Create TLS certificates:
   ```bash
   kubectl create secret tls pulsaar-tls --cert=tls.crt --key=tls.key
   ```

3. Install Pulsaar via Helm:
   ```bash
   helm install pulsaar ./charts/pulsaar
   ```

4. Verify webhook deployment:
   ```bash
   kubectl get pods -l app.kubernetes.io/name=pulsaar
   kubectl get mutatingwebhookconfigurations
   ```

## Sidecar Injection Verification

1. Check that the test pod has the pulsaar-agent sidecar injected:
   ```bash
   kubectl get pod test-pod -o yaml | grep -A 10 containers
   ```

2. Verify the sidecar is running:
   ```bash
   kubectl logs test-pod -c pulsaar-agent
   ```

## CLI Testing

1. Build or download the Pulsaar CLI binary.

2. Test explore command:
   ```bash
   ./pulsaar explore --pod test-pod --namespace default --path /
   ```

3. Test read command:
   ```bash
   ./pulsaar read --pod test-pod --namespace default --path /tmp/test.txt
   ```

4. Test stat command:
   ```bash
   ./pulsaar stat --pod test-pod --namespace default --path /tmp/test.txt
   ```

5. Test stream command:
   ```bash
   ./pulsaar stream --pod test-pod --namespace default --path /tmp/test.txt
   ```

## Audit Logging Verification

1. Check agent logs for audit entries:
   ```bash
   kubectl logs test-pod -c pulsaar-agent
   ```
   Should contain lines like: `Audit: ListDirectory request for path: /`

2. If audit aggregator is deployed, verify logs are forwarded.

## RBAC Testing

1. Test with insufficient permissions:
   - Create a service account with limited RBAC
   - Attempt CLI commands and verify access denied

2. Test with proper permissions:
   - Ensure commands succeed with appropriate RBAC

## Security Verification

1. Verify TLS connections:
   - Check that connections fail without proper certificates

2. Test path restrictions:
   - Attempt to access restricted paths (e.g., /etc/passwd) and verify denial

3. Test size limits:
   - Attempt to read files larger than 1MB and verify truncation

## Cluster-Specific Notes

### EKS
- Ensure IAM roles are properly configured for RBAC
- Verify VPC networking allows port-forward and proxy connections

### GKE
- Check Workload Identity configuration for RBAC
- Verify GKE-specific networking policies

### AKS
- Ensure Azure AD integration for RBAC
- Check AKS networking and security policies

## Cleanup

1. Delete test pod:
   ```bash
   kubectl delete pod test-pod
   ```

2. Uninstall Helm chart:
   ```bash
   helm uninstall pulsaar
   ```

3. Remove cert-manager if not needed:
   ```bash
   kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   ```

## Success Criteria

- All CLI commands execute successfully
- Audit logs are generated for each operation
- RBAC properly enforces access control
- TLS connections are secure
- Path allowlists prevent unauthorized access
- Size limits are enforced
- No security vulnerabilities exposed

## Reporting

Document any issues encountered during testing and update the troubleshooting guide accordingly.