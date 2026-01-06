#!/usr/bin/env bash
set -euo pipefail

# Backup script for Pulsaar audit logs

BACKUP_DIR="backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_PATH="${BACKUP_DIR}/${TIMESTAMP}"

mkdir -p "${BACKUP_PATH}"

echo "Starting Pulsaar audit logs backup to ${BACKUP_PATH}"

# Find aggregator pod
AGGREGATOR_POD=$(kubectl get pods -l app.kubernetes.io/name=pulsaar,app.kubernetes.io/component=aggregator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$AGGREGATOR_POD" ]; then
    echo "No aggregator pod found"
    exit 1
fi

# Backup audit log file
echo "Backing up audit logs from pod ${AGGREGATOR_POD}..."
kubectl cp "${AGGREGATOR_POD}:/var/log/pulsaar/audit.log" "${BACKUP_PATH}/audit.log" 2>/dev/null || echo "No audit log file found"

# Create backup manifest
cat > "${BACKUP_PATH}/backup_manifest.txt" << EOF
Pulsaar Audit Logs Backup
Timestamp: ${TIMESTAMP}
Pod: ${AGGREGATOR_POD}
Files:
$(ls -la "${BACKUP_PATH}")
EOF

echo "Backup completed successfully in ${BACKUP_PATH}"
echo "To restore, run: ./scripts/recovery_audit.sh ${TIMESTAMP}"