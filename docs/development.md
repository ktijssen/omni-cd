# Development

## Requirements

- [Go](https://go.dev) 1.22+
- [Task](https://taskfile.dev)
- Node.js 20+
- Docker

---

## Task Commands

### Development

```bash
task dev                     # Run locally with DEBUG logging
task build                   # Build binary (current OS/arch)
task build:linux             # Build static linux/amd64 binary
task install                 # go install
```

### Code Quality

```bash
task fmt                     # go fmt
task vet                     # go vet
task lint                    # golangci-lint
task check                   # Run fmt + vet + tests
```

### Dependencies

```bash
task deps                    # Download modules
task deps:tidy               # go mod tidy
task deps:update             # Update Go + npm dependencies to latest
```

### Docker

```bash
task docker:build            # Build Docker image
task docker:run              # Build + run container locally
```

### Docker Compose

```bash
task compose:up              # Start services
task compose:down            # Stop services
task compose:build           # Build and start services
task compose:rebuild         # Rebuild and restart
task compose:rebuild:nocache # Rebuild (no cache) and restart
task compose:logs            # Follow compose logs
task compose:restart         # Restart services
```

### Utilities

```bash
task clean                   # Remove build artifacts
task clean:docker            # Remove Docker image
task clean:all               # Clean everything
task open                    # Open web UI in browser
task version                 # Show Go and tool versions
task                         # List all available tasks
```

---

## Troubleshooting

| Problem | Likely Cause | Fix |
|---|---|---|
| Cluster stuck in Out of Sync | Drift detected but not applied | Check the **Diff** tab, then click **Sync** or check **Logs** for errors |
| Cluster stuck in Missing | Defined in git but not present in Omni — it has never been created, or it was deleted outside omni-cd | Enable **Auto-Sync** or click **Sync** to create it |
| MachineClass not applying | Validation error | Open the resource modal and check the Error tab |
| State lost after restart | No persistent volume | Ensure `/data` is mounted as a persistent volume |
| Login not working | Wrong password or stale setup | Use `admin` + the password from `/setup` or `ADMIN_PASSWORD`. Once a user exists `ADMIN_PASSWORD` is ignored — use the Users page or set `AUTH_DISABLED=true` |
| Cannot connect to Omni | Wrong endpoint or key | Check the Instances page. If set via ENV, verify the variables are correct |

---

## Contributing

Contributions are welcome. Please open an issue or submit a pull request on [GitHub](https://github.com/ktijssen/omni-cd).
