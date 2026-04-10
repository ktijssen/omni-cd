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
