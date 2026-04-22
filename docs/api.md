# API Reference

All endpoints require an active session cookie unless `AUTH_DISABLED=true`. Viewer-role sessions have read-only access; admin-role sessions can perform mutations.

---

## State & Reconciliation

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/ws` | viewer | WebSocket — real-time state updates |
| `GET` | `/api/state` | viewer | Current application state as JSON |
| `POST` | `/api/reconcile` | admin | Trigger a full sync (apply + delete) |
| `POST` | `/api/check` | admin | Git refresh only — no changes applied |
| `POST` | `/api/clusters-toggle` | admin | Toggle global cluster auto-sync on/off |

## Clusters

| Method | Path | Role | Description |
|---|---|---|---|
| `POST` | `/api/refresh-cluster` | admin | Drift-check a single cluster `{"id":"name"}` |
| `POST` | `/api/force-cluster` | admin | Force sync a single cluster `{"id":"name"}` |
| `POST` | `/api/delete-cluster` | admin | Delete a cluster `{"id":"name"}` |
| `POST` | `/api/set-cluster-autosync` | admin | Set per-cluster auto-sync `{"id":"name","autoSync":true}` |
| `POST` | `/api/export-cluster` | admin | Export an unmanaged cluster as YAML `{"id":"name"}` |

## Machine Classes

| Method | Path | Role | Description |
|---|---|---|---|
| `POST` | `/api/refresh-mc` | admin | Drift-check all machine classes |
| `POST` | `/api/refresh-single-mc` | admin | Drift-check a single machine class `{"id":"name"}` |
| `POST` | `/api/sync-machineclass` | admin | Force sync a single machine class `{"id":"name"}` |
| `POST` | `/api/delete-machineclass` | admin | Delete a machine class `{"id":"name"}` |
| `POST` | `/api/set-mc-autosync` | admin | Set per-machine-class auto-sync `{"id":"name","autoSync":true}` |

## Repositories

| Method | Path | Role | Description |
|---|---|---|---|
| `POST` | `/api/repos` | admin | Add a git repository |
| `PUT` | `/api/repos` | admin | Update a git repository |
| `DELETE` | `/api/repos` | admin | Remove a git repository |
| `POST` | `/api/repos/test` | admin | Test Git repo connectivity `{"url":"…","branch":"…","token":"…"}` |

## Omni Instance

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/omni-instance` | admin | Get current Omni instance config |
| `POST` | `/api/omni-instance` | admin | Save Omni instance config |
| `POST` | `/api/omni-instance/test` | admin | Test Omni connection |
| `POST` | `/api/omni-instance/refresh` | admin | Re-check connectivity and refresh version |
| `DELETE` | `/api/omni-instance` | admin | Remove stored Omni instance config |

## Logs

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/logs/files` | viewer | List available daily log files |
| `GET` | `/api/logs/download?date=YYYY-MM-DD` | viewer | Download a specific day's log file |

## Audit Log

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/audit` | viewer | List audit log entries (newest first) |
| `GET` | `/api/audit/files` | viewer | List available daily audit files |
| `GET` | `/api/audit/download?date=YYYY-MM-DD` | viewer | Download a specific day's audit file |

## Users

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/users` | admin | List local users (username + display name) |
| `POST` | `/api/users/change-password` | admin | Change password `{"currentPassword":"…","newPassword":"…"}` |
| `POST` | `/api/users/update-profile` | admin | Update display name `{"newDisplayName":"…"}` |
| `GET` | `/api/users/oidc` | admin | List SSO users with roles |
| `PATCH` | `/api/users/oidc` | admin | Update an SSO user's role `{"email":"…","role":"admin\|viewer\|none"}` |
| `DELETE` | `/api/users/oidc` | admin | Remove an SSO user `{"email":"…"}` |

## OIDC Config

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/oidc-config` | admin | Get current OIDC configuration (client secret masked) |

## Webhooks

| Method | Path | Role | Description |
|---|---|---|---|
| `POST` | `/api/webhook` | public | Trigger a git refresh via incoming webhook (GitHub format, validated with `WEBHOOK_SECRET` if set) |

## Session

| Method | Path | Role | Description |
|---|---|---|---|
| `GET` | `/api/me` | viewer | Current user's username, role, and auth configuration |

## Authentication

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/login` | Local username/password login |
| `GET` | `/auth/logout` | Invalidate current session |
| `GET` | `/auth/login` | Initiate OIDC login flow |
| `GET` | `/auth/callback` | OIDC callback |
