# omni-cd Helm Chart

GitOps continuous delivery operator for [Sidero Omni](https://www.siderolabs.com/omni/) — continuously reconciles MachineClasses and Clusters from Git to your Omni instance.

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

## Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: omni-cd.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: omni-cd-tls
      hosts:
        - omni-cd.example.com
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

## OIDC / Single Sign-On

```yaml
config:
  oidc:
    enabled: true
    issuerUrl: https://accounts.example.com
    clientId: omni-cd
    clientSecret: ""        # use existingSecret in production
    adminEmails: alice@example.com
    adminGroups: platform-admins
    viewerGroups: developers
    defaultRole: viewer
```

Role resolution order: `adminEmails` → `adminGroups` → `viewerEmails` → `viewerGroups` → `defaultRole`.

## Prometheus & Grafana

Enable the ServiceMonitor and Grafana dashboard sidecar provisioning:

```yaml
metrics:
  enabled: true
  port: 9090
  serviceMonitor:
    enabled: true
    interval: "30s"
    labels:
      release: prometheus   # match your Prometheus Operator serviceMonitorSelector
  grafanaDashboard:
    enabled: true
    labels:
      grafana_dashboard: "1"   # match your Grafana sidecar label selector
```

To import the dashboard manually, use `deploy/grafana/omni-cd-dashboard.json`.

## Values

### Image

| Key | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/ktijssen/omni-cd` | Container image repository |
| `image.tag` | `""` | Image tag override (defaults to chart appVersion) |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `replicaCount` | `1` | Number of replicas |

### Service

| Key | Default | Description |
|---|---|---|
| `service.type` | `ClusterIP` | Kubernetes service type |
| `service.port` | `8080` | Service port |
| `service.annotations` | `{}` | Extra service annotations |

### Ingress

| Key | Default | Description |
|---|---|---|
| `ingress.enabled` | `false` | Enable Ingress resource |
| `ingress.className` | `""` | Ingress class name |
| `ingress.annotations` | `{}` | Ingress annotations |
| `ingress.hosts` | see values.yaml | Host and path rules |
| `ingress.tls` | `[]` | TLS configuration |

### Gateway API

| Key | Default | Description |
|---|---|---|
| `httpRoute.enabled` | `false` | Enable HTTPRoute resource |
| `httpRoute.parentRefs` | `[{name: default, namespace: default}]` | Gateway parent references |
| `httpRoute.hostnames` | `[omni-cd.example.com]` | Hostnames to match |

### Application

| Key | Default | Description |
|---|---|---|
| `config.refreshInterval` | `"300"` | Git poll interval in seconds |
| `config.webPort` | `"8080"` | Port the web UI listens on |
| `config.logLevel` | `"INFO"` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `config.logRetentionDays` | `"7"` | Days to retain daily log files |
| `config.auditRetentionDays` | `"30"` | Days to retain daily audit log files |
| `config.authDisabled` | `"false"` | Disable authentication (all requests treated as admin) |
| `config.adminPassword` | `""` | Bootstrap admin password (ignored once a user exists) |
| `config.webhookSecret` | `""` | HMAC secret for validating webhook payloads |

### Omni

| Key | Default | Description |
|---|---|---|
| `config.omni.endpoint` | `""` | Omni instance URL |
| `config.omni.serviceAccountKey` | `""` | Omni service account key |

### OIDC / SSO

| Key | Default | Description |
|---|---|---|
| `config.oidc.enabled` | `false` | Enable OIDC authentication |
| `config.oidc.issuerUrl` | `""` | OIDC provider issuer URL |
| `config.oidc.clientId` | `""` | OIDC client ID |
| `config.oidc.clientSecret` | `""` | OIDC client secret (prefer `existingSecret`) |
| `config.oidc.redirectUrl` | `""` | Callback URL (auto-derived from request when empty) |
| `config.oidc.scopes` | `""` | Space/comma-separated scopes (default: `openid email profile`) |
| `config.oidc.groupsClaim` | `""` | JWT claim for group membership (default: `groups`) |
| `config.oidc.adminGroups` | `""` | Groups granted admin role |
| `config.oidc.adminEmails` | `""` | Email addresses granted admin role |
| `config.oidc.viewerGroups` | `""` | Groups granted viewer role |
| `config.oidc.viewerEmails` | `""` | Email addresses granted viewer role |
| `config.oidc.defaultRole` | `""` | Fallback role when no rule matches (`admin`\|`viewer`\|`none`) |
| `config.oidc.insecure` | `"false"` | Skip TLS verification for the OIDC provider |

### Metrics

| Key | Default | Description |
|---|---|---|
| `metrics.enabled` | `true` | Enable the Prometheus `/metrics` endpoint |
| `metrics.port` | `9090` | Port for the metrics server |
| `metrics.serviceMonitor.enabled` | `false` | Create a `ServiceMonitor` for Prometheus Operator |
| `metrics.serviceMonitor.namespace` | `""` | Namespace for the ServiceMonitor (defaults to release namespace) |
| `metrics.serviceMonitor.interval` | `"30s"` | Scrape interval |
| `metrics.serviceMonitor.labels` | `{}` | Extra labels on the ServiceMonitor (e.g. `release: prometheus`) |
| `metrics.grafanaDashboard.enabled` | `false` | Create a ConfigMap with the bundled Grafana dashboard |
| `metrics.grafanaDashboard.namespace` | `""` | Namespace for the ConfigMap (defaults to release namespace) |
| `metrics.grafanaDashboard.labels` | `{grafana_dashboard: "1"}` | Labels for Grafana sidecar discovery |

### Storage & Secrets

| Key | Default | Description |
|---|---|---|
| `existingSecret` | `""` | Name of an existing Secret containing credentials |
| `persistence.enabled` | `true` | Enable persistent volume for `/data` |
| `persistence.storageClass` | `""` | Storage class (uses cluster default when empty) |
| `persistence.accessMode` | `ReadWriteOnce` | PVC access mode |
| `persistence.size` | `1Gi` | PVC size |

### Pod

| Key | Default | Description |
|---|---|---|
| `resources` | `{}` | Pod resource requests and limits |
| `podAnnotations` | `{}` | Extra pod annotations |
| `podSecurityContext` | `{}` | Pod-level security context |
| `securityContext` | `{}` | Container-level security context |
| `nodeSelector` | `{}` | Node selector |
| `tolerations` | `[]` | Tolerations |
| `affinity` | `{}` | Affinity rules |

## Unit Tests

The chart includes unit tests using [helm-unittest](https://github.com/helm-unittest/helm-unittest):

```bash
helm plugin install https://github.com/helm-unittest/helm-unittest --verify=false
helm unittest deploy/helm/omni-cd
```
