#!/usr/bin/env bash
set -euo pipefail

# Recovery script for Pulsaar configuration data

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

echo "Starting Pulsaar configuration recovery from ${BACKUP_DIR}"

# Restore service accounts first
if [ -f "${BACKUP_DIR}/serviceaccounts.yaml" ]; then
    echo "Restoring service accounts..."
    kubectl apply -f "${BACKUP_DIR}/serviceaccounts.yaml"
fi

# Restore RBAC
if [ -f "${BACKUP_DIR}/rbac.yaml" ]; then
    echo "Restoring RBAC..."
    kubectl apply -f "${BACKUP_DIR}/rbac.yaml"
fi

# Restore TLS secrets
if [ -f "${BACKUP_DIR}/tls_secrets.yaml" ]; then
    echo "Restoring TLS secrets..."
    kubectl apply -f "${BACKUP_DIR}/tls_secrets.yaml"
fi

# Restore configmaps
if [ -f "${BACKUP_DIR}/configmaps.yaml" ]; then
    echo "Restoring configmaps..."
    kubectl apply -f "${BACKUP_DIR}/configmaps.yaml"
fi

# Restore webhook configurations
if [ -f "${BACKUP_DIR}/webhook.yaml" ]; then
    echo "Restoring webhook configurations..."
    kubectl apply -f "${BACKUP_DIR}/webhook.yaml"
fi

# Restore Helm release
if [ -f "${BACKUP_DIR}/helm_values.yaml" ]; then
    echo "Restoring Helm release..."
    helm upgrade --install pulsaar charts/pulsaar -f "${BACKUP_DIR}/helm_values.yaml" -n pulsaar --create-namespace
fi

echo "Recovery completed successfully from ${BACKUP_DIR}"
echo "Please verify the deployment and test functionality"