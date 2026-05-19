# Configuration

Omni CD is configured via a YAML file. Environment variables and the **Instances** page in the web UI act as overrides on top of the file.

**Precedence (lowest → highest):** built-in defaults → config file(s) → environment variables.

## Pointing the binary at a config file

The binary takes a repeatable `--config-path` flag:

```bash
omni-cd --config-path=/etc/omni-cd/config.yaml
```

Pass it multiple times to layer files (later wins per-field; lists are replaced when present in a layer):

```bash
omni-cd \
  --config-path=/etc/omni-cd/base.yaml \
  --config-path=/etc/omni-cd/overlay.yaml
```

A minimal config file is at [`deploy/config.example.yaml`](../deploy/config.example.yaml).

---

## File schema

```yaml
omni:
  endpoint: https://your-omni-instance.example.com
  serviceAccountKey: "..."     # see Omni Service Account in installation.md

refreshInterval: 300            # seconds between git pull + drift checks
webPort: "8080"
metricsPort: "9090"             # set "" to disable the /metrics endpoint
webhookSecret: ""               # HMAC secret for GitHub/GitLab webhook signatures

adminPassword: ""               # bootstrap password for the admin account (first boot only)
authDisabled: false             # true → skip login entirely

logLevel: INFO                  # DEBUG | INFO | WARN | ERROR
logRetentionDays: 7
auditRetentionDays: 30

oidc:
  enabled: false
  issuerUrl: ""
  clientId: ""
  clientSecret: ""
  redirectUrl: ""               # auto-derived when empty
  scopes: [openid, email, profile]
  groupsClaim: groups
  adminGroups: []
  adminEmails: []
  viewerGroups: []
  viewerEmails: []
  defaultRole: viewer           # admin | viewer | none
  insecure: false               # skip TLS verification (self-signed IdP only)

# Optional initial repository list. Repos defined here cannot be edited or
# deleted from the UI — they're marked "from config" and managed via the
# file. See "Repository management" below.
repos: []
# - name: prod
#   url: https://github.com/example/prod.git
#   branch: main
#   clusters_path: clusters
#   mc_path: machine-classes
#   token: ""                   # optional; or embed in the URL
```

---

## Environment variable overrides

Every field has a corresponding env var. When set (and non-empty) it overrides the file value. Useful for per-deployment tweaks or for injecting secrets that you don't want baked into the file.

### General

| Variable | Maps to | Notes |
|---|---|---|
| `OMNI_ENDPOINT` | `omni.endpoint` | When both Omni creds are set (file *or* env), the UI **Instances** page is read-only. |
| `OMNI_SERVICE_ACCOUNT_KEY` | `omni.serviceAccountKey` | Same lock behavior. |
| `REFRESH_INTERVAL` | `refreshInterval` | |
| `WEB_PORT` | `webPort` | |
| `METRICS_PORT` | `metricsPort` | |
| `WEBHOOK_SECRET` | `webhookSecret` | |

### Authentication

| Variable | Maps to |
|---|---|
| `ADMIN_PASSWORD` | `adminPassword` |
| `AUTH_DISABLED` | `authDisabled` |

### Logging

| Variable | Maps to |
|---|---|
| `LOG_LEVEL` | `logLevel` |
| `LOG_RETENTION_DAYS` | `logRetentionDays` |
| `AUDIT_RETENTION_DAYS` | `auditRetentionDays` |

### OIDC / Single Sign-On

OIDC requires `oidc.enabled: true` AND a non-empty issuer URL and client ID. OIDC list fields (`scopes`, `adminGroups`, `adminEmails`, `viewerGroups`, `viewerEmails`) accept comma-separated values via env vars.

| Variable | Maps to |
|---|---|
| `OIDC_ENABLED` | `oidc.enabled` |
| `OIDC_ISSUER_URL` | `oidc.issuerUrl` |
| `OIDC_CLIENT_ID` | `oidc.clientId` |
| `OIDC_CLIENT_SECRET` | `oidc.clientSecret` |
| `OIDC_REDIRECT_URL` | `oidc.redirectUrl` |
| `OIDC_SCOPES` | `oidc.scopes` (CSV) |
| `OIDC_GROUPS_CLAIM` | `oidc.groupsClaim` |
| `OIDC_ADMIN_GROUPS` | `oidc.adminGroups` (CSV) |
| `OIDC_ADMIN_EMAILS` | `oidc.adminEmails` (CSV) |
| `OIDC_VIEWER_GROUPS` | `oidc.viewerGroups` (CSV) |
| `OIDC_VIEWER_EMAILS` | `oidc.viewerEmails` (CSV) |
| `OIDC_DEFAULT_ROLE` | `oidc.defaultRole` |
| `OIDC_INSECURE` | `oidc.insecure` |

---

## Repository management

Repositories can be defined in two places:

1. **Config file (`repos:`)** — Declarative. The UI shows these with a "📄 from config" badge; Edit and Delete are disabled. Manage them by editing the file and restarting.
2. **Web UI (Repos page)** — Interactive. Persisted to `/data/config/repos.json` and fully editable.

Both kinds coexist. If a name appears in both, the config file wins (the UI version is shadowed). Remove a repo from the config and restart to demote it back to UI-managed.

Each repo entry supports:

- **`name`** — display name, must be unique
- **`url`** — HTTPS or SSH clone URL
- **`branch`** — defaults to `main`
- **`token`** — optional personal access token (allowed in the config file when stored in a Kubernetes Secret or equivalent)
- **`clusters_path`** — directory containing `cluster.yaml` files (default: `clusters`)
- **`mc_path`** — directory containing MachineClass YAMLs (default: `machine-classes`)

### Expected git layout

```
your-infra-repo/
├── machine-classes/
│   ├── controlplane.yaml
│   └── worker-general.yaml
└── clusters/
    ├── production/
    │   └── cluster.yaml
    └── dev/
        └── cluster.yaml
```

- **MachineClasses** — every `.yaml` file directly in `mc_path` is applied
- **Clusters** — files named `cluster.yaml` are found recursively under `clusters_path`
- A `cluster.yaml` may contain multiple YAML documents (`---`) including multiple named `Workers` sections
