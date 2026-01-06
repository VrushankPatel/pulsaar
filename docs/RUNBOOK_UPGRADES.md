# Upgrade Runbook

## Overview
This runbook provides procedures for upgrading Pulsaar components in a Kubernetes cluster.

## Prerequisites
- Existing Pulsaar deployment via Helm
- kubectl access to the cluster
- Backup of current configuration (see BACKUP_RECOVERY.md)

## Pre-Upgrade Checklist
- [ ] Backup current Helm values: `helm get values pulsaar -n pulsaar-system > backup_values.yaml`
- [ ] Backup TLS secrets: `kubectl get secrets -l app.kubernetes.io/name=pulsaar -o yaml > tls_backup.yaml`
- [ ] Verify cluster health: `kubectl get nodes`
- [ ] Check current version: `helm list -n pulsaar-system`
- [ ] Review release notes for breaking changes
- [ ] Ensure sufficient cluster resources for upgrade

## Upgrade Procedures

### Minor Version Upgrades
For patch and minor version updates:

```bash
# Update Helm chart
helm repo update  # if using remote repo

# Upgrade with existing values
helm upgrade pulsaar ./charts/pulsaar --namespace pulsaar-system

# Verify upgrade
kubectl get pods -n pulsaar-system
kubectl rollout status deployment/pulsaar-webhook -n pulsaar-system
kubectl rollout status deployment/pulsaar-aggregator -n pulsaar-system
```

### Major Version Upgrades
For major version updates:

1. **Review Breaking Changes**
   - Check release notes for API changes
   - Update protobuf definitions if needed
   - Verify compatibility with existing agents

2. **Update Images**
   ```bash
   # Pull latest images
   docker pull vrushankpatel/pulsaar-agent:latest
   docker pull vrushankpatel/pulsaar-cli:latest
   docker pull vrushankpatel/pulsaar-webhook:latest
   docker pull vrushankpatel/pulsaar-aggregator:latest
   ```

3. **Upgrade Helm Release**
   ```bash
   # Upgrade with new values if needed
   helm upgrade pulsaar ./charts/pulsaar \
     --namespace pulsaar-system \
     --values new_values.yaml
   ```

4. **Update Injected Agents**
   ```bash
   # Restart pods with injected agents
   kubectl rollout restart deployment/my-app
   ```

### Component-Specific Upgrades

#### Agent Upgrade
```bash
# Update agent image in Helm values
helm upgrade pulsaar ./charts/pulsaar \
  --namespace pulsaar-system \
  --set agent.image.tag=latest

# For embedded agents, rebuild application images
docker build -f Dockerfile.agent -t my-app:with-new-agent .
kubectl set image deployment/my-app app=my-app:with-new-agent
```

#### Webhook Upgrade
```bash
# Webhook upgrades automatically with Helm
helm upgrade pulsaar ./charts/pulsaar --namespace pulsaar-system

# Verify webhook health
kubectl get mutatingwebhookconfigurations
kubectl describe mutatingwebhookconfiguration pulsaar-webhook
```

#### Aggregator Upgrade
```bash
# Upgrade aggregator
helm upgrade pulsaar ./charts/pulsaar \
  --namespace pulsaar-system \
  --set aggregator.image.tag=latest

# Check aggregator logs
kubectl logs -l app.kubernetes.io/component=aggregator -n pulsaar-system
```

## Post-Upgrade Verification

### Health Checks
```bash
# Check all components
kubectl get pods -n pulsaar-system

# Verify webhook
kubectl get mutatingwebhookconfigurations

# Test agent connectivity
kubectl port-forward svc/pulsaar-webhook 8443:443 -n pulsaar-system &
curl -k https://localhost:8443/health
```

### Functional Testing
```bash
# Run test deployment
bash scripts/test_deployment.sh

# Test CLI functionality
./cli explore --pod my-pod --path /app
```

### Monitoring Verification
- Check Prometheus metrics are being scraped
- Verify audit logs are flowing to aggregator
- Monitor for error spikes in logs

## Rollback Procedures

### Helm Rollback
```bash
# List release history
helm history pulsaar -n pulsaar-system

# Rollback to previous version
helm rollback pulsaar <revision> -n pulsaar-system

# Verify rollback
kubectl rollout status deployment/pulsaar-webhook -n pulsaar-system
```

### Manual Rollback Steps
1. **Restore Helm Values**
   ```bash
   helm upgrade pulsaar ./charts/pulsaar \
     -f backup_values.yaml \
     --namespace pulsaar-system
   ```

2. **Restore TLS Secrets**
   ```bash
   kubectl apply -f tls_backup.yaml
   ```

3. **Restart Affected Pods**
   ```bash
   kubectl rollout restart deployment -l pulsaar.io/inject-agent=true
   ```

### Emergency Rollback
If upgrade causes critical issues:
```bash
# Immediate uninstall
helm uninstall pulsaar --namespace pulsaar-system

# Restore from backup
bash scripts/recovery_config.sh <backup_timestamp>

# Reinstall with known good version
helm install pulsaar ./charts/pulsaar \
  --namespace pulsaar-system \
  --version <previous_version>
```

## Troubleshooting Upgrades

### Common Issues
- **Image Pull Errors**: Check image registry access and tags
- **RBAC Issues**: Verify cluster roles are updated
- **TLS Failures**: Check certificate validity and DNS names
- **Webhook Injection Fails**: Verify CA bundle in webhook config

### Debug Commands
```bash
# Check upgrade status
helm status pulsaar -n pulsaar-system

# View pod events
kubectl describe pod -l app.kubernetes.io/name=pulsaar -n pulsaar-system

# Check webhook events
kubectl describe mutatingwebhookconfiguration pulsaar-webhook

# View detailed logs
kubectl logs -l app.kubernetes.io/name=pulsaar -n pulsaar-system --previous
```

## Best Practices
- Always backup before upgrading
- Test upgrades in staging environment first
- Monitor closely for 30 minutes post-upgrade
- Have rollback plan ready
- Update documentation after successful upgrade