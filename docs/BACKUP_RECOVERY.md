# Backup and Recovery Procedures

This document outlines procedures for backing up and recovering Pulsaar configuration and audit data.

## Configuration Data Backup

Configuration data includes:
- TLS certificates and keys stored in Kubernetes secrets
- Helm release values
- RBAC configurations
- Mutating webhook configurations

### Automated Backup Script

Use the provided `scripts/backup_config.sh` script to backup configuration data:

```bash
./scripts/backup_config.sh
```

This script will:
- Export all Pulsaar-related secrets (TLS certs, etc.)
- Export Helm release values
- Export RBAC roles and bindings
- Export webhook configurations
- Create timestamped backup files in `backups/` directory

### Manual Backup Steps

1. Backup TLS secrets:
   ```bash
   kubectl get secrets -l app.kubernetes.io/name=pulsaar -o yaml > tls_secrets_backup.yaml
   ```

2. Backup Helm release values:
   ```bash
   helm get values pulsaar -n pulsaar > helm_values_backup.yaml
   ```

3. Backup RBAC:
   ```bash
   kubectl get clusterroles,clusterrolebindings -l app.kubernetes.io/name=pulsaar -o yaml > rbac_backup.yaml
   ```

4. Backup webhook configurations:
   ```bash
   kubectl get mutatingwebhookconfigurations -l app.kubernetes.io/name=pulsaar -o yaml > webhook_backup.yaml
   ```

## Configuration Data Recovery

### Automated Recovery Script

Use the provided `scripts/recovery_config.sh` script to recover configuration data:

```bash
./scripts/recovery_config.sh <backup_timestamp>
```

This script will restore secrets, Helm values, RBAC, and webhook configurations from the specified backup.

### Manual Recovery Steps

1. Restore TLS secrets:
   ```bash
   kubectl apply -f tls_secrets_backup.yaml
   ```

2. Restore RBAC:
   ```bash
   kubectl apply -f rbac_backup.yaml
   ```

3. Restore webhook configurations:
   ```bash
   kubectl apply -f webhook_backup.yaml
   ```

4. Upgrade Helm release with backed up values:
   ```bash
   helm upgrade pulsaar charts/pulsaar -f helm_values_backup.yaml -n pulsaar
   ```

## Audit Data Backup

Audit data is forwarded to external logging systems configured via `PULSAAR_EXTERNAL_LOG_URL` in the aggregator.

### Backup Procedures

1. **External System Backup**: Follow the backup procedures of your external logging system (e.g., Elasticsearch snapshot, CloudWatch export, etc.)

2. **Aggregator Logs**: If the aggregator is logging to stdout, ensure Kubernetes logs are backed up:
   ```bash
   kubectl logs -l app.kubernetes.io/name=pulsaar,app.kubernetes.io/component=aggregator --since=24h > aggregator_logs_backup.txt
   ```

### Recovery Procedures

1. Restore external logging system from its backups.

2. If aggregator logs were backed up, they can be re-ingested if needed.

## Disaster Recovery

In case of complete cluster loss:

1. Re-deploy Pulsaar using the deployment guide
2. Run the recovery script with the latest backup
3. Verify TLS certificates are restored
4. Test audit logging functionality
5. Validate RBAC permissions

## Best Practices

- Run automated backups daily
- Store backups in secure, versioned storage
- Test recovery procedures regularly
- Monitor backup success/failure
- Keep multiple backup generations