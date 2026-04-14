# Installation

## Prerequisites

### Omni Service Account

Omni CD requires a service account with at least the **Operator** role to manage MachineClasses and Clusters.

1. In your Omni instance, go to **Settings → Service Accounts**
2. Create a new service account and assign it the **Operator** role (or higher)
3. Copy the generated key — you will need it during setup

> The **Operator** role is the minimum required. The **Admin** role is also supported but grants broader access than necessary.

---

## Docker

```bash
docker run -d \
  -v omni-cd-data:/data \
  -p 8080:8080 \
  ghcr.io/ktijssen/omni-cd:latest
```

The Omni endpoint and service account key can be set via environment variables **or** configured at runtime from the **Instances** page in the web UI.

On first boot you will be redirected to `/setup` to create the initial admin account (username `admin`).

---

## Docker Compose

```bash
cp deploy/compose/.env.example deploy/compose/.env
# Edit .env with your values
cd deploy/compose && docker compose up -d
```

A full example with all variables is in [`deploy/compose/`](../deploy/compose/).

---

## Helm

```bash
helm install omni-cd oci://ghcr.io/ktijssen/charts/omni-cd \
  --namespace omni-cd \
  --create-namespace \
  --set config.omni.endpoint=https://your-omni-instance.example.com \
  --set config.omni.serviceAccountKey=your-service-account-key
```

See [`deploy/helm/omni-cd/README.md`](../deploy/helm/omni-cd/README.md) for the full chart documentation including values, existing secret support, and Gateway API (HTTPRoute) configuration.

---

## Prometheus Metrics

The application exposes a Prometheus-compatible `/metrics` endpoint on a dedicated port (default `9090`).

### Docker

```bash
docker run -d \
  -v omni-cd-data:/data \
  -p 8080:8080 \
  -p 9090:9090 \
  ghcr.io/ktijssen/omni-cd:latest
```

### Helm

Enable a ServiceMonitor for Prometheus Operator:

```yaml
metrics:
  enabled: true
  port: 9090
  serviceMonitor:
    enabled: true
    interval: "30s"
    labels:
      release: prometheus   # match your Prometheus Operator selector
```

Enable the Grafana dashboard ConfigMap (requires the Grafana sidecar):

```yaml
metrics:
  grafanaDashboard:
    enabled: true
    labels:
      grafana_dashboard: "1"
```

Alternatively, import `deploy/grafana/omni-cd-dashboard.json` directly into Grafana.

### Exposed metrics

| Metric | Type | Description |
|---|---|---|
| `omni_cd_clusters_total` | Gauge | Cluster count by `status` label |
| `omni_cd_cluster_machines_healthy` | Gauge | Healthy machines per cluster |
| `omni_cd_cluster_machines_total` | Gauge | Total machines per cluster |
| `omni_cd_machine_classes_total` | Gauge | Machine class count by `status` label |
| `omni_cd_reconcile_last_duration_seconds` | Gauge | Duration of last completed reconcile |
| `omni_cd_reconcile_total` | Counter | Reconcile count by `result` (success/failed) |
| `omni_cd_omni_connected` | Gauge | `1` if Omni is reachable, `0` if not |
| `omni_cd_repos_total` | Gauge | Git repo count by `status` (ok/error) |
| `omni_cd_info` | Gauge | Always `1`; carries `version` label |

---

## Data Layout

All persistent data is stored under `/data`. Mount this as a volume to preserve state across restarts.

| Path | Contents |
|---|---|
| `/data/state/state.json` | Application state (clusters, repos, settings) |
| `/data/auth/users.json` | Local user accounts |
| `/data/config/oidc-users.json` | SSO user roles |
| `/data/logs/` | Daily log files (`omni-cd-YYYY-MM-DD.jsonlog`) |
| `/data/audit/` | Daily audit files (`audit-YYYY-MM-DD.jsonlog`) |

> Ensure `/data` is backed by a persistent volume. Without it, all state is lost on container restart.
