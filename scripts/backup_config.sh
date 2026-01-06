#!/usr/bin/env bash
set -euo pipefail

# Backup script for Pulsaar configuration data

BACKUP_DIR="backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_PATH="${BACKUP_DIR}/${TIMESTAMP}"

mkdir -p "${BACKUP_PATH}"

echo "Starting Pulsaar configuration backup to ${BACKUP_PATH}"

# Backup TLS secrets
echo "Backing up TLS secrets..."
kubectl get secrets -l app.kubernetes.io/name=pulsaar -o yaml > "${BACKUP_PATH}/tls_secrets.yaml" 2>/dev/null || echo "No TLS secrets found"

# Backup Helm release values
echo "Backing up Helm values..."
helm get values pulsaar -n pulsaar > "${BACKUP_PATH}/helm_values.yaml" 2>/dev/null || echo "Helm release not found"

# Backup RBAC
echo "Backing up RBAC..."
kubectl get clusterroles,clusterrolebindings -l app.kubernetes.io/name=pulsaar -o yaml > "${BACKUP_PATH}/rbac.yaml" 2>/dev/null || echo "No RBAC found"

# Backup webhook configurations
echo "Backing up webhook configurations..."
kubectl get mutatingwebhookconfigurations -l app.kubernetes.io/name=pulsaar -o yaml > "${BACKUP_PATH}/webhook.yaml" 2>/dev/null || echo "No webhooks found"

# Backup service accounts
echo "Backing up service accounts..."
kubectl get serviceaccounts -l app.kubernetes.io/name=pulsaar -o yaml > "${BACKUP_PATH}/serviceaccounts.yaml" 2>/dev/null || echo "No service accounts found"

# Backup configmaps
echo "Backing up configmaps..."
kubectl get configmaps -l app.kubernetes.io/name=pulsaar -o yaml > "${BACKUP_PATH}/configmaps.yaml" 2>/dev/null || echo "No configmaps found"

# Create backup manifest
cat > "${BACKUP_PATH}/backup_manifest.txt" << EOF
Pulsaar Configuration Backup
Timestamp: ${TIMESTAMP}
Files:
$(ls -la "${BACKUP_PATH}")
EOF

echo "Backup completed successfully in ${BACKUP_PATH}"
echo "To restore, run: ./scripts/recovery_config.sh ${TIMESTAMP}"