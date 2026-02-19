# Omni CD

A GitOps continuous deployment tool for [Sidero Omni](https://www.siderolabs.com/omni/), written in Go. Automatically synchronizes MachineClasses and Cluster Templates from a Git repository to your Omni instance.

![Omni CD Dashboard](docs/dashboard-screenshot.png)

## Features

- 🔄 **Automatic Synchronization**: Continuously syncs MachineClasses and Clusters from Git to Omni
- 🎯 **GitOps Workflow**: Declarative infrastructure management using Git as the source of truth
- 🌐 **Web Dashboard**: Real-time monitoring of sync status, resources, and logs
- 📊 **State Tracking**: Persistent state management with diff detection
- 🔍 **Live State Comparison**: View live vs desired state for all resources
- ⚡ **Dual Reconciliation**: Refresh mode (5min) and Sync mode (1hr) for optimized performance
- 🚨 **Error Handling**: Detailed error reporting with validation feedback
- 📦 **Single Binary**: Lightweight deployment with Docker support
- 📝 **Structured Logging**: JSON-formatted logs with configurable verbosity levels

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Omni Service Account with the role "Operator"
- Git repository with MachineClass and Cluster definitions

### Deployment

1. Clone this repository:
```bash
git clone <your-repo>
cd omni-cd
```

2. Create a `.env` file (or use docker-compose.yaml directly):
```env
OMNI_ENDPOINT=https://your-omni-instance.omni.siderolabs.io
OMNI_SERVICE_ACCOUNT_KEY=your-service-account-key-here
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

4. Access the web UI at http://localhost:8080

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `OMNI_ENDPOINT` | Your Omni instance URL | Yes | - |
| `OMNI_SERVICE_ACCOUNT_KEY` | Service account key for authentication | Yes | - |
| `GIT_REPO` | Git repository URL containing your infrastructure | Yes | - |
| `GIT_BRANCH` | Git branch to track | No | `main` |
| `GIT_TOKEN` | Git token for private repositories (PAT, etc.) | No | - |
| `CLUSTERS_ENABLED` | Enable automatic cluster syncing | No | `true` |
| `REFRESH_INTERVAL` | Refresh interval in seconds (git pull + diff only) | No | `300` (5m) |
| `SYNC_INTERVAL` | Sync interval in seconds (full reconciliation) | No | `3600` (1h) |
| `MC_PATH` | Path to machine classes in repo | No | `machine-classes` |
| `CLUSTERS_PATH` | Path to cluster templates in repo | No | `clusters` |
| `WEB_PORT` | Web UI port | No | `8080` |
| `LOG_LEVEL` | Logging level (DEBUG, INFO, WARN, ERROR) | No | `INFO` |

### Repository Structure

Your Git repository should follow this structure:

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

**Important Notes:**
- **Machine Classes**: All `.yaml` files in the `machine-classes/` directory are processed
- **Clusters**: Only files named **`cluster.yaml`** are processed from the `clusters/` directory and its subdirectories
  - The application recursively searches for `cluster.yaml` files
  - Other YAML files (like `patches/`) are ignored by the application
  - Each cluster template must be in its own subdirectory with a `cluster.yaml` file

## How It Works

### Reconciliation Modes

1. **Refresh Mode** (Soft, default: every 5 minutes):
   - Pulls latest changes from Git
   - Runs diff detection on clusters
   - Updates state without applying changes
   - Useful for monitoring drift

2. **Sync Mode** (Hard, default: every 1 hour):
   - Pulls latest changes from Git
   - Applies MachineClasses (create/update)
   - Syncs Cluster Templates
   - Deletes resources removed from Git
   - Full reconciliation

### Log Levels

Control logging verbosity with the `LOG_LEVEL` environment variable:

- **ERROR**: Only critical failures (auth failed, sync failed, validation errors)
- **WARN**: Recoverable issues (missing directories, resource conflicts, out of sync warnings)
- **INFO** (default): Normal operations (startup, reconcile events, successful syncs)
- **DEBUG**: Detailed diagnostics (version info, per-resource status, internal state)

Logs are output in JSON format to stdout and displayed in the web UI. Both stdout and the web UI respect the configured log level.

### MachineClass Management

- **Validation**: Files are validated before applying
- **Smart Apply**: Only applies when changes are detected (not just metadata updates)
- **Multi-Document Support**: YAML files can contain multiple resources separated by `---`
- **Error Tracking**: Validation errors are captured and displayed in the UI

### Cluster Template Management

- **Diff Detection**: Uses `omnictl cluster template diff` to detect changes
- **Selective Sync**: Only syncs clusters that are out of sync
- **Force Sync**: Manual sync option available in the UI for individual clusters
- **Unmanaged Clusters**: Preserves clusters not in Git (marked as "unmanaged")
- **Export**: Export unmanaged clusters as YAML templates

### Web Dashboard

The web UI provides:
- **Git Status**: Current commit, branch, sync health
- **Resource Overview**: Count of MachineClasses and Clusters
- **Reconciliation Status**: Last sync time and status
- **Resource Details**: Click any resource to view:
  - **Error Tab**: Validation or sync errors (if any)
  - **Live Tab**: Current state in Omni
  - **Diff Tab**: Changes to be applied (clusters only)
- **Logs**: Real-time reconciliation logs with collapsible view
- **Manual Actions**: Refresh, Sync, Force Sync, Export

## API Endpoints

- `GET /` - Web UI
- `GET /api/state` - Current application state (JSON)
- `POST /api/reconcile` - Trigger immediate reconciliation
- `POST /api/check-git` - Pull latest Git changes
- `POST /api/export-cluster?id=<cluster-id>` - Export cluster template
- `POST /api/force-sync?id=<cluster-id>` - Force sync specific cluster

## Development

### Building from Source

```bash
go build -o omni-cd ./cmd/omni-cd
```

### Running Locally

```bash
export OMNI_ENDPOINT=https://your-instance.omni.siderolabs.io
export OMNI_SERVICE_ACCOUNT_KEY=your-key
export GIT_REPO=https://github.com/your-org/your-repo.git
export LOG_LEVEL=DEBUG  # Optional: for verbose output

./omni-cd
```

### Project Structure

```
.
├── cmd/
│   └── omni-cd/          # Main application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── git/              # Git operations
│   ├── omni/             # Omni API wrapper (omnictl)
│   ├── reconciler/       # Reconciliation logic
│   ├── state/            # State management and persistence
│   └── web/              # Web UI and API server
├── deploy/
│   └── compose/          # Docker Compose deployment
│       └── docker-compose.yaml
├── data/                 # Persistent state storage
└── Dockerfile            # Container image definition
```

## Architecture

1. **Git Sync**: Periodically pulls from Git repository
2. **Reconciler**: Compares desired state (Git) with live state (Omni)
3. **Omni Client**: Wraps `omnictl` CLI for all Omni operations
4. **State Manager**: Tracks resource status and persists to JSON
5. **Web Server**: Serves UI and API endpoints
6. **Event Loop**: Runs reconciliation on configured intervals

## Version Compatibility

- Go 1.23+
- omnictl v1.5.0+
- Omni SaaS platform
- Omni Self Hosted

## Troubleshooting

### Version Mismatch Warning

If you see a version mismatch warning, ensure your omnictl version is compatible with your Omni instance version. The tool will disable sync operations if there's a mismatch.

### Machine Class Not Applying

- Check the Error tab for validation errors
- Ensure YAML syntax is correct
- Verify `matchlabels` format: `- key = value`

### Cluster Stuck in "Out of Sync"

- Check cluster template for errors
- Use Force Sync to retry
- Review logs for detailed error messages

## License

This project is licensed under the Mozilla Public License Version 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

