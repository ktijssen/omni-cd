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

## Maintenance Windows

| Method | Path | Role | Description |
|---|---|---|---|
| `POST` | `/api/maintenance-windows` | admin | Create a window (body below, without `id`); returns the window with its new `id` |
| `PUT` | `/api/maintenance-windows` | admin | Replace the window with the given `id` |
| `DELETE` | `/api/maintenance-windows` | admin | Delete a window `{"id":"…"}` |

Windows and their current status are part of the state snapshot (`maintenanceWindows`, plus `maintenanceTimezone`).

```json
{
  "id": "3f9c2a1b7d4e8f60",
  "name": "Weekend production",
  "clusters": ["staging"],
  "selector": { "env": "production" },
  "days": ["tue", "thu"],
  "start": "22:00",
  "end": "02:00"
}
```

- Times are in the **server's timezone**: the `TZ` environment variable, UTC when unset. With `TZ=Europe/Amsterdam` windows follow daylight-saving changes.
- Each window has one time range. `days` are `mon`–`sun` and name the day the window **starts**. When `end` is not after `start` the window ends the next day: Tuesday 22:00–02:00 runs until Wednesday 02:00. `start` is inclusive, `end` exclusive.
- The window applies to the clusters listed in `clusters` **plus** every cluster that has all labels in `selector` (`key: value`, AND). A selector also matches clusters created or labelled later. Labels are matched against the live Omni labels and the `labels:` in the cluster's Git template, so a cluster that is only in Git still matches. At least one of `clusters` or `selector` is required; `omni.sidero.dev/*` keys are not allowed.
- In the state snapshot each window has `matchedClusters`: the clusters it applies to right now.
- `name` must be unique (case-insensitive). Every cluster must be known to Omni CD; on `PUT`, clusters already in the window are kept even if they have since disappeared.
- A cluster may be in several windows; it can be synced automatically when any of them is open (overlapping or back-to-back windows count as one).
- Errors return `{"error": "..."}` with `400`, or `404` for an unknown `id`.
