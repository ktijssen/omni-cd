# Omni CD

A GitOps tool for [Sidero Omni](https://www.siderolabs.com/omni/). It watches one or more Git repositories and continuously reconciles **MachineClasses** and **Cluster templates** to your Omni instance.

<p>
  <img src="docs/loginpage.png" width="49%" alt="Login Page" />
  <img src="docs/clusters-view.png" width="49%" alt="Clusters" />
</p>
<p>
  <img src="docs/cluster-detail-graph.png" width="49%" alt="Cluster Detail" />
  <img src="docs/machine-classes-view.png" width="49%" alt="Machine Classes" />
</p>
<p>
  <img src="docs/repos-view.png" width="49%" alt="Repos" />
  <img src="docs/users-view.png" width="49%" alt="Users" />
</p>

---

## Features

- **GitOps sync** — MachineClasses and Clusters are continuously reconciled from Git to Omni
- **Multi-repo support** — Manage resources from multiple Git repositories in one instance
- **Drift detection** — Detects out-of-sync resources without applying changes
- **Diff view** — Colour-coded diff between desired and live state per resource
- **Live cluster status** — Cluster, controlplane, Kubernetes API, etcd, and WireGuard health badges
- **Machine graph** — Visual DAG showing Git → Omni → Cluster → MachineSets → Machines
- **Multiple worker pools** — Cluster templates with multiple named worker groups are fully supported
- **Per-cluster auto-sync** — Enable or disable automatic sync per cluster from the web UI
- **Unmanaged clusters** — Clusters created outside of Git are visible and can be exported as templates
- **Machine class usage** — Each MachineClass card lists which clusters are currently using it
- **Persistent state** — State is saved to disk and restored on restart
- **Authentication** — Username/password login with session cookies, first-time setup wizard, OIDC/SSO support, and user management (or fully disabled for internal use)
- **Real-time web UI** — WebSocket-driven dashboard; no page refreshes needed
- **Log persistence** — Logs are written to daily rotating files and survive container restarts
- **Omni instance management** — Omni endpoint and service account key can be configured via the web UI

---

## Installation

### Docker (recommended)

```bash
docker run -d \
  -v omni-cd-data:/data \
  -p 8080:8080 \
  ghcr.io/ktijssen/omni-cd:latest
```

The Omni endpoint and service account key can be set via environment variables **or** configured at runtime from the **Instances** page in the web UI.

On first boot you will be redirected to `/setup` to create the initial admin account (username `admin`).

### Docker Compose

```bash
cp deploy/compose/.env.example deploy/compose/.env
# Edit .env with your values
cd deploy/compose && docker compose up -d
```

A full example with all variables is in [`deploy/compose/`](deploy/compose/).

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `OMNI_ENDPOINT` | No* | — | Omni instance URL |
| `OMNI_SERVICE_ACCOUNT_KEY` | No* | — | Omni service account key |
| `REFRESH_INTERVAL` | No | `300` | Seconds between git pull + drift checks |
| `ADMIN_PASSWORD` | No** | — | Bootstrap password for the `admin` account, applied on first boot only (ignored once a user exists) |
| `AUTH_DISABLED` | No | `false` | Set `true` to disable login entirely (hides Users page) |
| `WEB_PORT` | No | `8080` | Web UI port |
| `LOG_LEVEL` | No | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `LOG_RETENTION_DAYS` | No | `7` | Number of days to keep daily log files |

\* Can be configured via the **Instances** page in the web UI instead. If set via environment, the values are locked and cannot be changed from the UI.

\*\* If `ADMIN_PASSWORD` is not set, navigate to `/setup` on first boot to create the initial account interactively.

---

## Authentication

### First-time Setup

On first boot (no users configured), all requests redirect to `/setup`. Enter a password for the built-in `admin` account. Username is fixed as `admin`.

Alternatively, set `ADMIN_PASSWORD` to bootstrap the account from an environment variable — useful for automated deployments.

### Local Login

![Login page](docs/loginpage.png)

Log in at `/login` with username `admin` and your password. Sessions last 24 hours. Failed login attempts are rate-limited (5 failures → 15-minute lockout per IP).

### OIDC / Single Sign-On

Set the `OIDC_*` environment variables to enable SSO. When OIDC is configured, a **Sign in with SSO** button appears on the login page alongside the local login form.

OIDC users are assigned roles (`admin`, `viewer`, or `none`) based on group/email mappings. The first OIDC user to log in is automatically promoted to `admin`. Roles can be changed from the **Users** page.

| Variable | Description |
|---|---|
| `OIDC_ISSUER_URL` | OIDC provider URL (e.g. `https://accounts.example.com`) |
| `OIDC_CLIENT_ID` | OAuth2 client ID |
| `OIDC_CLIENT_SECRET` | OAuth2 client secret |
| `OIDC_REDIRECT_URL` | Callback URL (leave empty to auto-derive) |
| `OIDC_SCOPES` | Comma-separated scopes (default: `openid,email,profile`) |
| `OIDC_GROUPS_CLAIM` | JWT claim for group membership (default: `groups`) |
| `OIDC_ADMIN_GROUPS` | Groups granted `admin` role |
| `OIDC_ADMIN_EMAILS` | Emails granted `admin` role |
| `OIDC_VIEWER_GROUPS` | Groups granted `viewer` role |
| `OIDC_VIEWER_EMAILS` | Emails granted `viewer` role |
| `OIDC_DEFAULT_ROLE` | Role when no rule matches (`admin` \| `viewer` \| `none`, default: `viewer`) |
| `OIDC_INSECURE` | Set `true` for self-signed IdP certificates |

---

## Repository Structure

```
your-infra-repo/
├── machine-classes/
│   ├── controlplane.yaml
│   └── worker-general.yaml
└── clusters/
    ├── production/
    │   └── cluster.yaml       ← only this filename is processed
    └── dev/
        └── cluster.yaml
```

- **MachineClasses** — every `.yaml` file directly in `mc_path` is applied
- **Clusters** — files named `cluster.yaml` are found recursively under `clusters_path`
- A `cluster.yaml` may contain multiple YAML documents (`---`) including multiple named `Workers` sections

---

## How It Works

### Reconciliation Modes

| Mode | Trigger | What it does |
|---|---|---|
| **Refresh** | Every `REFRESH_INTERVAL` seconds or the Refresh button | Git pull + drift detection, no changes applied |
| **Sync** | Sync button or per-cluster force sync | Full reconciliation — apply, update, and delete resources |

Resources are always processed in this order:

- **Apply:** MachineClasses → Clusters
- **Delete:** Clusters → MachineClasses

### Safety Guards

- Resources **not owned** by this Omni CD instance (created externally or by another instance) are never deleted
- Auto-sync can be disabled per cluster from the UI — out-of-sync clusters are shown but not applied
- Clusters from repos that fail to pull are protected from deletion during the outage window

### State Persistence

State is saved to `/data/state/state.json` after each reconcile and restored on startup, so the UI shows current state immediately without waiting for the first cycle.

---

## Web UI

The UI is a single-page app with a sidebar. Navigating to `/` redirects to `/clusters`.

### Clusters (`/clusters`)

![Clusters view](docs/clusters-view.png)

Card grid showing one card per cluster with:
- Talos and Kubernetes versions
- Cluster, controlplane, API, etcd, and WireGuard health badges
- Controlplane and worker pool configuration
- Per-cluster Refresh, Sync, Auto-Sync toggle, and Delete actions

Click a card to open the cluster detail page with a **Graph** tab (visual DAG of the full stack), a **Live** tab (current Omni YAML), and a **Diff** tab (desired vs live).

![Cluster detail graph](docs/cluster-detail-graph.png)

Clusters created outside of Git (unmanaged) are shown with an **Export** button to download them as a YAML template.

### Machine Classes (`/machineclasses`)

![Machine Classes view](docs/machine-classes-view.png)

Card grid showing each MachineClass with its provisioning mode, resource settings, machine limit, and a live **Used by** list showing which clusters reference it. Each card has Refresh, Sync, Auto-Sync toggle, and Delete actions. Click a cluster chip to jump to that cluster.

### Instances (`/instances`)

Manage the Omni endpoint and service account key. When credentials are provided via environment variables they are shown as read-only. When not set via ENV, use the **Add Omni Instance** button to configure the connection, or **Edit** / **Delete** an existing one. Changes take effect immediately.

### Repos (`/repos`)

![Repos view](docs/repos-view.png)

Add, edit, and remove Git repositories at runtime without restarting. Supports optional per-repo authentication tokens. Default paths are `clusters/` and `machine-classes/` if not specified.

### Users (`/users`)

![Users view](docs/users-view.png)

Manage user accounts. The **Local Admin Account** section shows the built-in `admin` account with options to:

- **Edit Profile** — update the display name
- **Change Password** — change your password (requires current password; enforces strength rules)

When OIDC is enabled, an **SSO Users** section lists all users who have logged in via SSO, with their assigned role. Admins can edit each user's role (`admin`, `viewer`, or `none`) from this page.

> This page is hidden and inaccessible when `AUTH_DISABLED=true`.

### Logs (`/logs`)

Reconciler log stream with filtering and export:

- **Level filter** — toggle INFO / WARN / ERROR (and DEBUG when `LOG_LEVEL=DEBUG`)
- **Component filter** — narrow logs to a specific component
- **Text search** — filter by message content
- **Download Today's Logs** — download the current day's log file as JSONL
- **Show Logs** — browse all stored daily log files with individual download buttons

Logs are written to daily rotating files under `/data/logs/` and are re-loaded into the ring buffer on restart so history is preserved across container restarts. Files older than `LOG_RETENTION_DAYS` days are automatically deleted.

---

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/ws` | WebSocket — real-time state updates |
| `GET` | `/api/state` | Current state as JSON |
| `POST` | `/api/reconcile` | Trigger a full sync |
| `POST` | `/api/check` | Trigger a git refresh (no apply) |
| `POST` | `/api/clusters-toggle` | Toggle global cluster sync on/off |
| `POST` | `/api/refresh-cluster` | Refresh a single cluster `{"id":"name"}` |
| `POST` | `/api/force-cluster` | Force sync a single cluster `{"id":"name"}` |
| `POST` | `/api/delete-cluster` | Delete a cluster `{"id":"name"}` |
| `POST` | `/api/set-cluster-autosync` | Set per-cluster auto-sync `{"id":"name","autoSync":true}` |
| `POST` | `/api/export-cluster` | Export an unmanaged cluster as YAML `{"id":"name"}` |
| `POST` | `/api/refresh-mc` | Refresh all machine classes (no apply) |
| `POST` | `/api/refresh-single-mc` | Refresh a single machine class `{"id":"name"}` |
| `POST` | `/api/sync-machineclass` | Force sync a single machine class `{"id":"name"}` |
| `POST` | `/api/delete-machineclass` | Delete a machine class `{"id":"name"}` |
| `POST` | `/api/set-mc-autosync` | Set per-machine-class auto-sync `{"id":"name","autoSync":true}` |
| `POST` | `/api/repos` | Add a git repository |
| `PUT` | `/api/repos` | Update a git repository |
| `DELETE` | `/api/repos` | Remove a git repository |
| `GET` | `/api/omni-instance` | Get current Omni instance config |
| `POST` | `/api/omni-instance` | Save Omni instance config |
| `POST` | `/api/omni-instance/test` | Test Omni connection |
| `DELETE` | `/api/omni-instance` | Remove stored Omni instance config |
| `GET` | `/api/logs/files` | List available daily log files |
| `GET` | `/api/logs/download?date=YYYY-MM-DD` | Download a specific day's log file |
| `GET` | `/api/users` | List local users (username + display name) |
| `POST` | `/api/users/change-password` | Change password `{"currentPassword":"…","newPassword":"…"}` |
| `POST` | `/api/users/update-profile` | Update display name `{"newDisplayName":"…"}` |
| `GET` | `/api/users/oidc` | List OIDC users with roles |
| `PATCH` | `/api/users/oidc` | Update an OIDC user's role `{"email":"…","role":"admin\|viewer\|none"}` |

---

## Development

Requires Go 1.26+, [Task](https://taskfile.dev), and Docker.

```bash
# Development
task dev                     # Run locally with DEBUG logging
task build                   # Build binary (current OS/arch)
task build:linux             # Build static linux/amd64 binary
task install                 # go install

# Code quality
task fmt                     # go fmt
task vet                     # go vet
task lint                    # golangci-lint
task check                   # Run fmt + vet

# Dependencies
task deps                    # Download modules
task deps:tidy               # go mod tidy
task deps:update             # go get -u + tidy

# Docker
task docker:build            # Build Docker image
task docker:run              # Build + run container locally

# Docker Compose
task compose:up              # Start services
task compose:down            # Stop services
task compose:build           # Build and start services
task compose:rebuild         # Rebuild and restart
task compose:rebuild:nocache # Rebuild (no cache) and restart
task compose:logs            # Follow compose logs
task compose:restart         # Restart services

# Utilities
task clean                   # Remove build artifacts
task clean:docker            # Remove Docker image
task clean:all               # Clean everything
task open                    # Open web UI in browser
task version                 # Show Go and tool versions
task                         # List all available tasks
```

---

## Troubleshooting

**Cluster stuck in Out of Sync** — Click the Diff tab to see what changed, then use the Sync button or check the Logs page for the underlying error.

**MachineClass not applying** — Open the resource modal and check the Error tab for validation output.

**State lost after restart** — Ensure `/data` is backed by a persistent volume. State is stored in `/data/state/state.json`.

**Login not working** — Use username `admin` with the password set during `/setup` or via `ADMIN_PASSWORD` on first boot. Once a user exists, `ADMIN_PASSWORD` is ignored — use the **Users** page to change the password, or set `AUTH_DISABLED=true` for passwordless access.

**Cannot connect to Omni** — Check the Instances page to verify the endpoint and key are configured. If set via ENV, ensure the variables are correct. The startup log shows which credential source is active.

---

## License

Mozilla Public License Version 2.0 — see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. Please open an issue or submit a pull request.
