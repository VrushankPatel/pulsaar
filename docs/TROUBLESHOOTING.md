# Troubleshooting

## Connection Issues

### CLI cannot connect to agent

**Symptoms:** Connection refused, TLS handshake errors, timeouts

**Possible Causes & Solutions:**

1. **Agent not running:**
   - Check if agent process is running: `kubectl exec -it my-pod -- ps aux | grep pulsaar`
   - Verify pod has agent container or sidecar

2. **TLS certificate issues:**
   - Check certificate files exist: `kubectl exec -it my-pod -- ls -la /path/to/certs`
   - Verify certificate validity: `kubectl exec -it my-pod -- openssl x509 -in /path/to/cert.pem -text -noout`
   - For mTLS, ensure client certificate is signed by the correct CA

3. **Port forwarding blocked:**
   - Test manual port-forward: `kubectl port-forward my-pod 8443:8443`
   - If blocked, use `--connection-method apiserver-proxy`

4. **Network policies:**
   - Check if network policies allow traffic to agent port (default 8443)
   - Verify service mesh configuration

5. **Agent configuration:**
   - Check agent logs: `kubectl logs my-pod -c pulsaar-agent`
   - Look for TLS setup errors or binding failures

### API Server Proxy Connection Fails

**Symptoms:** Forbidden errors, unauthorized access

**Solutions:**

1. **RBAC permissions:**
   - Verify your user has permissions for `pods/portforward` or `pods/proxy`
   - Check ClusterRole bindings

2. **API server configuration:**
   - Some clusters disable API server proxy for security
   - Switch to port-forward method

3. **Authentication:**
   - Ensure kubectl context is valid
   - Check token expiration

## File Access Issues

### Path Access Denied

**Symptoms:** "Path not allowed" errors

**Checks:**

1. **Allowed roots configuration:**
   - Verify CLI is passing correct `allowed_roots`
   - Check agent default allowlist (typically `/app`, `/tmp`, `/var/log`)

2. **Path sanitization:**
   - Paths with `..` are blocked
   - Symlinks may be restricted

3. **Security denylist:**
   - Common secret paths like `/etc/ssl/private` are blocked
   - Check agent logs for blocked access attempts

### File Read Errors

**Symptoms:** "File not found", "Permission denied", "Too large"

**Solutions:**

1. **File existence:**
   - Verify file exists in container: `kubectl exec -it my-pod -- ls -la /path/to/file`
   - Check file permissions

2. **Size limits:**
   - Default 1MB limit for ReadFile
   - Use StreamFile for larger files
   - Check agent logs for size limit hits

3. **Container filesystem:**
   - Some containers use read-only filesystems
   - Ephemeral containers may have limited access

## Deployment Issues

### Sidecar Injection Not Working

**Symptoms:** Pods don't get pulsaar-agent sidecar

**Troubleshooting:**

1. **Webhook status:**
   ```bash
   kubectl get mutatingwebhookconfigurations
   kubectl describe mutatingwebhookconfiguration pulsaar-webhook
   ```

2. **Webhook pod health:**
   ```bash
   kubectl logs -n pulsaar-system deployment/pulsaar-webhook
   ```

3. **Certificate validity:**
   ```bash
   kubectl get secret -n pulsaar-system pulsaar-webhook-tls
   kubectl describe secret -n pulsaar-system pulsaar-webhook-tls
   ```

4. **Pod annotations:**
   - Ensure `pulsaar.io/inject-agent: "true"` is set
   - Check annotation format and namespace

5. **Namespace exclusion:**
   - Webhook may exclude system namespaces
   - Check webhook configuration for namespace selectors

### Ephemeral Container Injection Fails

**Symptoms:** CLI reports injection failure

**Checks:**

1. **Cluster capabilities:**
   - Verify cluster supports ephemeral containers (Kubernetes 1.23+)
   - Check if alpha feature gate is enabled

2. **Pod security:**
   - Pod Security Standards may block injection
   - Check pod security context

3. **Resource limits:**
   - Ephemeral containers need resource allocation
   - Ensure pod has sufficient resources

## TLS Configuration Issues

### Certificate Validation Errors

**Common Issues:**

1. **Certificate expired:**
   - Check expiry: `openssl x509 -in cert.pem -text | grep -A 2 Validity`
   - Renew certificates

2. **Wrong certificate authority:**
   - Verify CA matches between client and server
   - Check certificate chain

3. **DNS name mismatch:**
   - Certificates must include pod DNS names
   - For port-forward: `localhost`
   - For API proxy: pod cluster DNS

4. **System time:**
   - Certificate validity depends on system clock
   - Check `kubectl exec -it my-pod -- date`

### mTLS Handshake Failures

**Debugging:**

1. **Client certificate:**
   - Ensure `PULSAAR_CLIENT_CERT_FILE` and `PULSAAR_CLIENT_KEY_FILE` are set
   - Verify client cert is signed by agent's CA

2. **Server CA:**
   - Agent must have `PULSAAR_TLS_CA_FILE` set for client verification
   - Check CA certificate is correct

3. **Certificate formats:**
   - Must be PEM format
   - Check for proper headers/footers

## Performance Issues

### Slow File Operations

**Causes & Fixes:**

1. **Large files without streaming:**
   - Use `StreamFile` RPC instead of `ReadFile`
   - Increase `chunk_size` parameter

2. **Network latency:**
   - Port-forward may have higher latency than direct connections
   - Consider connection method

3. **Agent resource limits:**
   - Check pod CPU/memory limits
   - Agent may be throttled

4. **File system performance:**
   - Some storage backends are slower
   - Check underlying storage class

### High Memory Usage

**Symptoms:** Agent pod OOM killed

**Solutions:**

1. **Concurrent connections:**
   - Limit number of simultaneous CLI connections
   - Check agent connection pooling

2. **Large file buffering:**
   - Use streaming to reduce memory usage
   - Adjust chunk sizes

3. **Audit logging:**
   - Disable audit logging if not needed
   - Check aggregator connectivity

## Audit Logging Issues

### Logs Not Sent to Aggregator

**Symptoms:** Local logs appear but not in central system

**Checks:**

1. **Aggregator URL:**
   - Verify `PULSAAR_AUDIT_AGGREGATOR_URL` is set correctly
   - Test connectivity: `kubectl exec -it my-pod -- curl -X POST $URL`

2. **Network policies:**
   - Ensure pods can reach aggregator service
   - Check firewall rules

3. **Aggregator health:**
   - Verify aggregator pod is running
   - Check aggregator logs for incoming requests

### Malformed Log Entries

**Issues:**

1. **JSON format:**
   - Ensure logs are valid JSON
   - Check for special characters in paths

2. **Version compatibility:**
   - Aggregator may expect specific log format
   - Check agent and aggregator versions

## General Debugging

### Enable Debug Logging

Set environment variable on agent:
```bash
export PULSAAR_LOG_LEVEL=debug
```

Restart agent pod to apply.

### Health Checks

**Via port-forward:**
```bash
kubectl port-forward my-pod 8443:8443 &
curl -k https://localhost:8443/health
```

**Via API server proxy:**
```bash
kubectl get --raw /api/v1/namespaces/default/pods/my-pod:8443/proxy/health
```

### View Agent Configuration

Agent logs configuration on startup:
```bash
kubectl logs my-pod -c pulsaar-agent | head -20
```

### Useful kubectl Commands

```bash
# Pod investigation
kubectl describe pod my-pod
kubectl get pod my-pod -o yaml

# Log inspection
kubectl logs my-pod -c pulsaar-agent --previous
kubectl logs -n pulsaar-system deployment/pulsaar-webhook

# Network testing
kubectl exec -it my-pod -- nc -zv aggregator-service 80

# Certificate inspection
kubectl get secret pulsaar-tls -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text
```

### Validation Script

Run the repository validation:
```bash
bash scripts/validate_repo.sh
```

### Test Deployment

Use the test deployment for verification:
```bash
bash scripts/test_deployment.sh
```

## Getting Help

If issues persist:

1. Check agent and CLI logs for error messages
2. Verify configuration against documentation
3. Test with simple scenarios first
4. Check GitHub issues for similar problems
5. Provide detailed logs and configuration when reporting issues