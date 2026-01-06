#!/usr/bin/env bash
set -euo pipefail

# Recovery script for Pulsaar audit logs

if [ $# -ne 1 ]; then
    echo "Usage: $0 <backup_timestamp>"
    echo "Example: $0 20231201_120000"
    exit 1
fi

BACKUP_TIMESTAMP="$1"
BACKUP_DIR="backups/${BACKUP_TIMESTAMP}"

if [ ! -d "${BACKUP_DIR}" ]; then
    echo "Backup directory ${BACKUP_DIR} not found"
    echo "Available backups:"
    ls -la backups/ 2>/dev/null || echo "No backups directory found"
    exit 1
fi

echo "Starting Pulsaar audit logs recovery from ${BACKUP_DIR}"

# Find aggregator pod
AGGREGATOR_POD=$(kubectl get pods -l app.kubernetes.io/name=pulsaar,app.kubernetes.io/component=aggregator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$AGGREGATOR_POD" ]; then
    echo "No aggregator pod found"
    exit 1
fi

# Restore audit log file
if [ -f "${BACKUP_DIR}/audit.log" ]; then
    echo "Restoring audit logs to pod ${AGGREGATOR_POD}..."
    kubectl cp "${BACKUP_DIR}/audit.log" "${AGGREGATOR_POD}:/var/log/pulsaar/audit.log"
else
    echo "No audit.log file in backup"
fi

echo "Recovery completed successfully from ${BACKUP_DIR}"