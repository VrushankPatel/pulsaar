# Incident Response Runbook

## Overview
This runbook provides structured procedures for responding to Pulsaar incidents and outages.

## Incident Severity Levels
- **P0 (Critical)**: Complete system outage, security breach, data loss
- **P1 (High)**: Major functionality broken, widespread impact
- **P2 (Medium)**: Partial functionality loss, limited impact
- **P3 (Low)**: Minor issues, cosmetic problems

## Response Process

### Phase 1: Detection & Alert
1. **Monitor Alerts**
   - Prometheus alerts for high error rates
   - Pod crash loops or restarts
   - TLS certificate expiration warnings
   - Audit log delivery failures

2. **Initial Assessment**
   ```bash
   # Check system status
   kubectl get pods -n pulsaar-system
   kubectl get mutatingwebhookconfigurations
   kubectl get servicemonitor -n pulsaar-system
   ```

### Phase 2: Triage & Diagnosis

#### Quick Health Check
```bash
# Check component health
kubectl exec -n pulsaar-system deployment/pulsaar-webhook -- wget --no-check-certificate https://localhost:8443/health

# Check metrics
kubectl port-forward -n pulsaar-system svc/pulsaar-webhook 9090:9090 &
curl localhost:9090/metrics | grep -E "(errors_total|up)"
```

#### Common Issue Diagnosis

**Agent Connection Failures**
```bash
# Check agent pods
kubectl get pods -l pulsaar.io/inject-agent=true

# Test connectivity
kubectl port-forward my-pod 8443:8443 &
curl -k https://localhost:8443/health
```

**Webhook Injection Issues**
```bash
# Check webhook status
kubectl describe mutatingwebhookconfiguration pulsaar-webhook

# Check webhook pod logs
kubectl logs -n pulsaar-system deployment/pulsaar-webhook
```

**TLS/Certificate Issues**
```bash
# Check certificate validity
kubectl get secret pulsaar-tls -n pulsaar-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -checkend 0

# Check certificate details
kubectl get secret pulsaar-tls -n pulsaar-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text
```

**RBAC Permission Issues**
```bash
# Check service account
kubectl get serviceaccount pulsaar -n pulsaar-system

# Test token review
kubectl create token pulsaar -n pulsaar-system
```

### Phase 3: Containment & Mitigation

#### Immediate Actions by Severity

**P0/P1 Incidents**
1. **Stop Impact**
   - Scale down failing components: `kubectl scale deployment pulsaar-webhook --replicas=0 -n pulsaar-system`
   - Disable webhook: `kubectl delete mutatingwebhookconfiguration pulsaar-webhook`

2. **Preserve Evidence**
   - Collect logs: `kubectl logs -n pulsaar-system --all-containers --previous > incident_logs.txt`
   - Take cluster snapshot if possible

**P2 Incidents**
1. **Isolate Issue**
   - Restart affected pods: `kubectl rollout restart deployment/pulsaar-webhook -n pulsaar-system`
   - Check resource limits: `kubectl describe pod -n pulsaar-system`

**P3 Incidents**
1. **Monitor & Document**
   - Increase log verbosity
   - Document symptoms and impact

### Phase 4: Resolution

#### Standard Fixes

**Restart Components**
```bash
# Restart all pulsaar components
kubectl rollout restart deployment -n pulsaar-system -l app.kubernetes.io/name=pulsaar

# Restart injected agents
kubectl rollout restart deployment -l pulsaar.io/inject-agent=true
```

**Certificate Renewal**
```bash
# Renew with cert-manager
kubectl delete certificate pulsaar-tls -n pulsaar-system
kubectl apply -f certificate.yaml

# Or manually update secret
kubectl create secret tls pulsaar-tls --cert=new.crt --key=new.key --dry-run=client -o yaml | kubectl apply -f -
```

**RBAC Fixes**
```bash
# Reapply RBAC
kubectl apply -f rbac.yaml

# Check cluster role binding
kubectl get clusterrolebinding pulsaar-rbac
```

**Resource Issues**
```bash
# Increase limits
kubectl patch deployment pulsaar-webhook -n pulsaar-system --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value":"512Mi"}]'
```

### Phase 5: Recovery & Testing

#### Recovery Steps
1. **Gradual Restoration**
   ```bash
   # Scale back up
   kubectl scale deployment pulsaar-webhook --replicas=3 -n pulsaar-system

   # Verify health
   kubectl exec -n pulsaar-system deployment/pulsaar-webhook -- wget --no-check-certificate https://localhost:8443/health
   ```

2. **Functional Testing**
   ```bash
   # Run test suite
   bash scripts/test_deployment.sh

   # Test CLI operations
   ./cli explore --pod test-pod --path /tmp
   ```

3. **Monitor Recovery**
   - Watch error rates return to normal
   - Verify audit logs resume
   - Check user reports

### Phase 6: Post-Incident Review

#### Documentation
1. **Incident Timeline**
   - Detection time
   - Response actions and timestamps
   - Resolution time
   - Impact assessment

2. **Root Cause Analysis**
   - What caused the incident?
   - Why wasn't it caught earlier?
   - What monitoring/alerting gaps exist?

3. **Action Items**
   - Immediate fixes implemented
   - Long-term improvements planned
   - Documentation updates needed

#### Communication
- Update stakeholders on resolution
- Share post-mortem findings
- Update runbooks if needed

## Escalation Procedures

### When to Escalate
- Incident unresolved after 30 minutes (P0/P1)
- Security-related incidents
- Data loss or corruption
- Multiple component failures

### Escalation Contacts
- Primary: Platform/SRE team lead
- Secondary: Security team
- Tertiary: Development team

## Prevention Measures

### Proactive Monitoring
- Set up alerts for key metrics
- Regular certificate expiration checks
- Automated health checks
- Log aggregation and analysis

### Regular Maintenance
- Weekly health checks
- Monthly certificate rotation
- Quarterly disaster recovery testing
- Annual security audits

## Reference Information

### Key Commands
```bash
# System status
kubectl get all -n pulsaar-system

# Recent events
kubectl get events -n pulsaar-system --sort-by=.metadata.creationTimestamp

# Resource usage
kubectl top pods -n pulsaar-system

# Debug logs
kubectl logs -n pulsaar-system -l app.kubernetes.io/name=pulsaar --tail=100
```

### Useful Files
- `/docs/TROUBLESHOOTING.md` - Detailed troubleshooting guide
- `/docs/BACKUP_RECOVERY.md` - Recovery procedures
- `/scripts/test_deployment.sh` - Health verification script

### External Resources
- Kubernetes documentation
- cert-manager troubleshooting
- Prometheus alerting rules