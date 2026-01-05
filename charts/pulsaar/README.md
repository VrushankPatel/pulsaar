# Pulsaar Helm Chart

This Helm chart deploys the Pulsaar webhook for automatic sidecar injection of the Pulsaar agent into Kubernetes pods.

## Prerequisites

- Kubernetes 1.16+
- Helm 3.0+

## Installing the Chart

To install the chart with the release name `pulsaar`:

```bash
helm install pulsaar ./charts/pulsaar
```

## Uninstalling the Chart

To uninstall the `pulsaar` deployment:

```bash
helm uninstall pulsaar
```

## Configuration

The following table lists the configurable parameters of the Pulsaar chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook.image.repository` | Webhook image repository | `vrushankpatel/pulsaar-webhook` |
| `webhook.image.tag` | Webhook image tag | `latest` |
| `webhook.replicaCount` | Number of webhook replicas | `1` |
| `webhook.serviceAccount.create` | Create service account | `true` |
| `webhook.tls.secretName` | TLS secret name | `pulsaar-webhook-tls` |
| `webhook.tls.caBundle` | Base64 encoded CA bundle | `""` |
| `rbac.create` | Create RBAC resources | `true` |
| `monitoring.enabled` | Enable monitoring | `false` |
| `monitoring.serviceMonitor.enabled` | Create ServiceMonitor | `false` |

## TLS Configuration

The webhook requires TLS certificates. You can either:

1. Provide a pre-created secret with the TLS certificates
2. Use cert-manager for automatic certificate management

### Using cert-manager

Set `webhook.tls.certManager.enabled` to `true` and configure the issuer.

### Manual TLS

Create a secret with the TLS certificates:

```bash
kubectl create secret tls pulsaar-webhook-tls --cert=tls.crt --key=tls.key
```

## Pod Injection

To inject the Pulsaar agent into pods, add the annotation `pulsaar.io/inject-agent: "true"` to the pod spec.

Example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  annotations:
    pulsaar.io/inject-agent: "true"
spec:
  containers:
  - name: app
    image: nginx
```

## Monitoring

When monitoring is enabled, the webhook exposes metrics at `/metrics` endpoint.

If ServiceMonitor is enabled, it will create a ServiceMonitor resource for Prometheus.