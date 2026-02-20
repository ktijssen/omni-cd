# Omni CD

A GitOps tool for [Sidero Omni](https://www.siderolabs.com/omni/), written in Go. It watches a Git repository and automatically synchronises **MachineClasses** and **Cluster** templates to your Omni instance.

![Omni CD Dashboard](docs/dashboard-screenshot.png)

## Features

- 🔄 **Automatic Synchronisation** — Continuously syncs MachineClasses and Clusters from Git to Omni
- 🎯 **GitOps Workflow** — Git is the single source of truth for your Omni infrastructure
- 🌐 **Web Dashboard** — Real-time monitoring of sync status, resources, and logs
- 🔍 **Drift Detection** — Live vs desired state comparison with diff view per resource
- ⚡ **Dual Reconciliation** — Lightweight refresh (default: 5 min) and full sync (default: 1 h)
- 🔖 **Version Safety** — Sync is blocked automatically when Omni and omnictl versions mismatch
- 📦 **Single Container** — Lightweight deployment; no external dependencies beyond omnictl
- 📝 **Structured Logging** — JSON logs with configurable verbosity, streamed to the web UI

## Installation

### Docker (recommended)

Pre-built images are published to the GitHub Container Registry on every release:

```bash
docker pull ghcr.io/ktijssen/omni-cd:latest
```

Run it:

```bash
docker run -d \
  -e OMNI_ENDPOINT=https://your-omni.omni.siderolabs.io \
  -e OMNI_SERVICE_ACCOUNT_KEY=your-service-account-key \
  -e GIT_REPO=https://github.com/your-org/your-infra-repo.git \
  -p 8080:8080 \
  ghcr.io/ktijssen/omni-cd:latest
```

### Docker Compose

1. Copy the example environment file:
```bash
cp deploy/compose/.env.example deploy/compose/.env
```

2. Fill in your values:
```env
OMNI_ENDPOINT=https://your-omni.omni.siderolabs.io
OMNI_SERVICE_ACCOUNT_KEY=your-service-account-key
GIT_REPO=https://github.com/your-org/your-infra-repo.git
GIT_BRANCH=main
CLUSTERS_ENABLED=true
LOG_LEVEL=INFO
```

3. Start the service:
```bash
cd deploy/compose
docker compose up -d
```

4. Open the web UI at http://localhost:8080

### Binary

Download the pre-built binary for your platform from the [latest release](https://github.com/ktijssen/omni-cd/releases/latest):

```bash
curl -LO https://github.com/ktijssen/omni-cd/releases/latest/download/omni-cd-linux-amd64
chmod +x omni-cd-linux-amd64
sudo mv omni-cd-linux-amd64 /usr/local/bin/omni-cd
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|---|---|---|---|
| `OMNI_ENDPOINT` | Your Omni instance URL | Yes | — |
| `OMNI_SERVICE_ACCOUNT_KEY` | Service account key | Yes | — |
| `GIT_REPO` | Git repository URL | Yes | — |
| `GIT_BRANCH` | Branch to track | No | `main` |
| `GIT_TOKEN` | Token for private repositories | No | — |
| `CLUSTERS_ENABLED` | Enable automatic cluster syncing | No | `true` |
| `REFRESH_INTERVAL` | Refresh interval in seconds (git pull + diff) | No | `300` |
| `SYNC_INTERVAL` | Sync interval in seconds (full reconciliation) | No | `3600` |
| `MC_PATH` | Path to MachineClasses in repo | No | `machine-classes` |
| `CLUSTERS_PATH` | Path to Cluster templates in repo | No | `clusters` |
| `WEB_PORT` | Web UI port | No | `8080` |
| `LOG_LEVEL` | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | No | `INFO` |

### Repository Structure

```
your-infra-repo/
├── machine-classes/
│   ├── controlplane.yaml
│   ├── worker-general.yaml
│   └── worker-gpu.yaml
└── clusters/
    ├── production/
    │   ├── cluster.yaml
    │   └── patches/
    │       ├── cni.yaml
    │       └── storage.yaml
    └── dev/
        └── cluster.yaml
```

**Notes:**
- **MachineClasses** — all `.yaml` files in `machine-classes/` are processed
- **Clusters** — only files named `cluster.yaml` are processed (searched recursively); other YAML files such as patches are ignored

## How It Works

### Reconciliation Modes

| Mode | Default interval | What it does |
|---|---|---|
| **Refresh** (soft) | 5 minutes | Git pull + drift detection, no changes applied |
| **Sync** (hard) | 1 hour | Full reconciliation — apply, update, and delete resources |

Both modes can also be triggered manually from the web UI.

### Reconciliation Order

To avoid dependency issues, resources are applied and deleted in a fixed order:

**Apply:** MachineClasses → Clusters
**Delete:** Clusters → MachineClasses

### Version Safety

On startup, Omni CD compares the version reported by the live Omni instance with the version of the bundled `omnictl`. If they differ, all sync operations are disabled and a warning is shown in the UI. Each release of Omni CD is automatically built against the latest `omnictl` release.

## Web Dashboard

| Area | What you see |
|---|---|
| **Omni Instance** | Endpoint URL, Omni version, omnictl version |
| **Git Status** | Repo, branch, latest commit, last sync time, health badge |
| **Last Reconciliation** | Type, status, start/finish time |
| **MachineClasses** | List with sync status; click to view live state and errors |
| **Clusters** | List with sync status; auto-sync toggle; force sync and export actions |
| **Logs** | Real-time reconciliation logs, collapsible |

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Web UI |
| `GET` | `/api/state` | Current application state (JSON) |
| `POST` | `/api/reconcile` | Trigger a full sync |
| `POST` | `/api/check` | Trigger a git refresh |
| `POST` | `/api/clusters-toggle` | Toggle automatic cluster sync on/off |
| `POST` | `/api/force-cluster` | Force sync a specific cluster `{"id": "cluster-name"}` |
| `POST` | `/api/export-cluster` | Export an unmanaged cluster as YAML `{"id": "cluster-name"}` |
| `GET` | `/ws` | WebSocket — real-time state updates |

## Development

### Prerequisites

- Go 1.23+
- [Task](https://taskfile.dev) (`brew install go-task` / `go install github.com/go-task/task/v3/cmd/task@latest`)
- Docker (for container builds)
- `omnictl` in your `$PATH`

### Common Tasks

```bash
task           # List all available tasks
task dev       # Run locally with live reload (requires env vars)
task build     # Build binary for current platform
task build:linux   # Build linux/amd64 binary
task check     # Run fmt + vet
task docker:build  # Build Docker image locally
task compose:up    # Start via Docker Compose
task compose:logs  # Follow Docker Compose logs
```

### Running Locally

```bash
export OMNI_ENDPOINT=https://your-instance.omni.siderolabs.io
export OMNI_SERVICE_ACCOUNT_KEY=your-key
export GIT_REPO=https://github.com/your-org/your-repo.git
export LOG_LEVEL=DEBUG

task dev
```

### Project Structure

```
.
├── cmd/omni-cd/          # Application entry point
├── internal/
│   ├── config/           # Configuration loading
│   ├── git/              # Git operations
│   ├── omni/             # omnictl wrapper
│   ├── reconciler/       # Reconciliation logic
│   ├── state/            # State management and persistence
│   └── web/              # Web UI and API server
├── deploy/compose/       # Docker Compose deployment
├── docs/                 # Documentation assets
├── Dockerfile
└── Taskfile.yaml
```

## Releases

Releases are published automatically to [GitHub Releases](https://github.com/ktijssen/omni-cd/releases) and GHCR when:

- Code is pushed to `main`
- A new `omnictl` version is detected (checked daily at 02:00 UTC)

Each release includes a `linux/amd64` binary and a container image tagged with the version (e.g. `v1.0.0`) and `latest`.

## Troubleshooting

**Version mismatch warning** — Ensure your Omni instance and the `omnictl` bundled in the container are on the same version. Pulling the latest image usually resolves this.

**MachineClass not applying** — Check the Error tab in the UI for validation errors. Verify your YAML syntax and that `matchlabels` uses the format `- key = value`.

**Cluster stuck in "Out of Sync"** — Use the Force Sync button in the UI, then review the logs for the specific error.

## Version Compatibility

- Go 1.23+
- omnictl v1.5.0+
- Omni SaaS or Self-Hosted

## License

Mozilla Public License Version 2.0 — see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
