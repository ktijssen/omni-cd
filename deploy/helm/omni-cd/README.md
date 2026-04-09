# omni-cd Helm Chart

GitOps continuous delivery tool for [Sidero Omni](https://www.siderolabs.com/omni/) cluster management.

## Installing the Chart

```bash
helm install omni-cd oci://ghcr.io/ktijssen/charts/omni-cd \
  --namespace omni-cd \
  --create-namespace \
  --set config.omni.endpoint=https://your-omni-instance.example.com \
  --set config.omni.serviceAccountKey=your-service-account-key
```

Or with a values file:

```bash
helm install omni-cd oci://ghcr.io/ktijssen/charts/omni-cd \
  --namespace omni-cd \
  --create-namespace \
  -f values.yaml
```

## Existing Secret

To avoid storing credentials in `values.yaml`, create a Kubernetes Secret yourself and reference it:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: omni-cd-credentials
type: Opaque
stringData:
  OMNI_ENDPOINT: "https://your-omni-instance.example.com"
  OMNI_SERVICE_ACCOUNT_KEY: "your-service-account-key"
  ADMIN_PASSWORD: "your-admin-password"
```

```yaml
# values.yaml
existingSecret: omni-cd-credentials
```

## Gateway API (HTTPRoute)

If you use the Kubernetes Gateway API instead of a traditional `Ingress`:

```yaml
httpRoute:
  enabled: true
  parentRefs:
    - name: my-gateway
      namespace: gateway
  hostnames:
    - omni-cd.example.com
```

## Values

| Key | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/ktijssen/omni-cd` | Container image repository |
| `image.tag` | `""` (uses appVersion) | Image tag override |
| `replicaCount` | `1` | Number of replicas |
| `service.type` | `ClusterIP` | Service type |
| `service.port` | `8080` | Service port |
| `service.annotations` | `{}` | Service annotations |
| `ingress.enabled` | `false` | Enable Ingress |
| `ingress.className` | `""` | Ingress class name |
| `httpRoute.enabled` | `false` | Enable Gateway API HTTPRoute |
| `httpRoute.parentRefs` | `[{name: default, namespace: default}]` | Gateway parent references |
| `httpRoute.hostnames` | `[omni-cd.example.com]` | Hostnames to match |
| `config.refreshInterval` | `"300"` | Git poll interval (seconds) |
| `config.webPort` | `"8080"` | Web UI port |
| `config.logLevel` | `"INFO"` | Log level |
| `config.logRetentionDays` | `"7"` | Log file retention in days |
| `config.authDisabled` | `"false"` | Disable authentication |
| `config.adminPassword` | `""` | Bootstrap admin password |
| `config.omni.endpoint` | `""` | Omni instance URL |
| `config.omni.serviceAccountKey` | `""` | Omni service account key |
| `config.oidc.enabled` | `false` | Enable OIDC/SSO |
| `config.oidc.issuerUrl` | `""` | OIDC provider URL |
| `config.oidc.clientId` | `""` | OIDC client ID |
| `config.oidc.clientSecret` | `""` | OIDC client secret |
| `existingSecret` | `""` | Name of an existing Secret with credentials |
| `persistence.enabled` | `true` | Enable persistent storage |
| `persistence.size` | `1Gi` | PVC size |
| `resources` | `{}` | Pod resource requests/limits |

## Unit Tests

The chart includes unit tests using [helm-unittest](https://github.com/helm-unittest/helm-unittest):

```bash
helm plugin install https://github.com/helm-unittest/helm-unittest --verify=false
helm unittest deploy/helm/omni-cd
```
