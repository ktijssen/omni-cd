# Configuration

All configuration is done via environment variables. The Omni endpoint and service account key can alternatively be set at runtime from the **Instances** page.

---

## General

| Variable | Default | Description |
|---|---|---|
| `OMNI_ENDPOINT` | — | Omni instance URL. If set via ENV, locked and cannot be changed from the UI. |
| `OMNI_SERVICE_ACCOUNT_KEY` | — | Omni service account key. Same locking behaviour as above. |
| `REFRESH_INTERVAL` | `300` | Seconds between git pull + drift checks. |
| `WEB_PORT` | `8080` | Web UI and API port. |
| `WEBHOOK_SECRET` | — | Optional secret to validate incoming webhook payloads. |

## Authentication

| Variable | Default | Description |
|---|---|---|
| `ADMIN_PASSWORD` | — | Bootstrap password for the `admin` account, applied on first boot only. |
| `AUTH_DISABLED` | `false` | Set `true` to disable login entirely (hides the Users page). |

## Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `INFO` | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR`. |
| `LOG_RETENTION_DAYS` | `7` | Days to keep daily log files before automatic deletion. |
| `AUDIT_RETENTION_DAYS` | `30` | Days to keep daily audit files before automatic deletion. |

## OIDC / Single Sign-On

Set `OIDC_ENABLED=true` and provide at minimum `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID`. OIDC and Auth0 are mutually exclusive — only one may be enabled at a time.

| Variable | Default | Description |
|---|---|---|
| `OIDC_ENABLED` | `false` | Enable OIDC authentication. |
| `OIDC_ISSUER_URL` | — | OIDC provider discovery URL (e.g. `https://accounts.example.com`). |
| `OIDC_CLIENT_ID` | — | OAuth2 client ID. |
| `OIDC_CLIENT_SECRET` | — | OAuth2 client secret (optional for public clients). |
| `OIDC_REDIRECT_URL` | — | Callback URL registered with your IdP. Leave empty to auto-derive. |
| `OIDC_SCOPES` | `openid,email,profile` | Comma-separated scopes. |
| `OIDC_GROUPS_CLAIM` | `groups` | JWT/userinfo claim containing group memberships. |
| `OIDC_ADMIN_GROUPS` | — | Groups granted `admin` role. |
| `OIDC_ADMIN_EMAILS` | — | Email addresses granted `admin` role. |
| `OIDC_VIEWER_GROUPS` | — | Groups granted `viewer` role. |
| `OIDC_VIEWER_EMAILS` | — | Email addresses granted `viewer` role. |
| `OIDC_DEFAULT_ROLE` | `viewer` | Role when no rule matches: `admin`, `viewer`, or `none`. |
| `OIDC_INSECURE` | `false` | Skip TLS verification — for self-signed IdP certificates only. |

---

## Repository Structure

Git repositories are managed at runtime via the **Repos** page. Each repo supports:

- **URL** — HTTPS or SSH clone URL
- **Branch** — defaults to `main`
- **Token** — optional personal access token for private repos
- **Clusters path** — directory containing `cluster.yaml` files (default: `clusters/`)
- **Machine Classes path** — directory containing MachineClass YAMLs (default: `machine-classes/`)

### Expected layout

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
