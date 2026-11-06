export interface OmniHealth {
  status: string
  lastCheck: string
  downSince?: string
  error?: string
}

export interface GitInfo {
  name?: string
  repo: string
  branch: string
  sha: string
  shortSha: string
  commitMessage: string
  commitAuthor?: string
  lastSync: string
  syncError?: string
}

export interface ReconcileInfo {
  type: string
  status: string
  startedAt: string
  finishedAt: string
}

export interface NodeGroup {
  name?: string
  count: number
  machineClass?: string
  machines?: string[]
  extensions?: string[]
}

export interface ResourceInfo {
  id: string
  type: string
  status: string
  provisionType?: string
  diff?: string
  fileContent?: string
  liveContent?: string
  error?: string
  talosVersion?: string
  kubernetesVersion?: string
  controlPlane?: NodeGroup
  workers?: NodeGroup[]
  clusterReady?: string
  kubernetesApiReady?: string
  controlplaneReady?: string
  clusterPhase?: string
  machinesHealthy?: number
  machinesTotal?: number
  /** User labels on the Omni Cluster resource (omni.sidero.dev/* hidden). */
  labels?: Record<string, string>
  /** Labels declared in the cluster's Git template. */
  templateLabels?: Record<string, string>
  etcdStatus?: string
  wireGuardStatus?: string
  lastBackupTime?: string
  backupEnabled?: boolean
  clusterExtensions?: string[]
  machineExtensions?: Record<string, string[]>
  machineHostnames?: Record<string, string>
  lastSyncResult?: string
  lastSyncError?: string
  lastSyncTime?: string
  lastSyncSHA?: string
  lastSyncAuthor?: string
  lastSyncMessage?: string
  syncStatusSince?: string
  createdAt?: string
  autoSync?: boolean
  repoName?: string
  /** Names of the maintenance windows that include this cluster (computed). */
  maintenanceWindows?: string[]
  /** Combined status of those windows (computed). */
  maintenanceStatus?: MaintenanceStatus
}

export type Weekday = 'mon' | 'tue' | 'wed' | 'thu' | 'fri' | 'sat' | 'sun'

/** One weekly range in server time. End <= start means it ends the next day. */
export interface MaintenanceTimeRange {
  days: Weekday[]
  start: string // "HH:MM"
  end: string // "HH:MM"
}

/** A named window: one weekly time range applied to one or more clusters. */
export interface MaintenanceWindow extends MaintenanceTimeRange {
  id: string
  name: string
  clusters: string[]
  /** Clusters with all of these labels are included too (AND), now and later. */
  selector?: Record<string, string>
}

/** Computed by the server; *Text fields are already in server time, e.g. "Sat 06:00". */
export interface MaintenanceStatus {
  open: boolean
  currentEnd?: string
  nextStart?: string
  currentEndText?: string
  nextStartText?: string
}

export interface MaintenanceWindowView extends MaintenanceWindow {
  status: MaintenanceStatus
  /** Clusters the window applies to right now (by name or label). */
  matchedClusters: string[]
}

export interface LogEntry {
  timestamp: string
  level: string
  label: string
  message: string
}

export interface RepoConfigView {
  name: string
  url: string
  branch: string
  hasToken: boolean
  clustersPath: string
  mcPath: string
  fromConfig: boolean
}

export interface SnapshotData {
  serverStartedAt: string
  appVersion: string
  omniEndpoint: string
  omniVersion: string
  omniHealth: OmniHealth
  omniEnvLocked: boolean
  omniConfigured: boolean
  omniHasStoredKey: boolean
  git: GitInfo
  repos?: GitInfo[]
  repoConfigs?: RepoConfigView[]
  lastReconcile: ReconcileInfo
  machineClasses: ResourceInfo[]
  clusters: ResourceInfo[]
  clustersEnabled: boolean
  repoClusterMap?: Record<string, string[]>
  repoMachineClassMap?: Record<string, string[]>
  maintenanceWindows?: MaintenanceWindowView[]
  /** Zone maintenance window times are read in, e.g. "UTC" or "Europe/Amsterdam". */
  maintenanceTimezone?: string
  logs: LogEntry[]
  logLevel: string
}

export interface ManifestStatus {
  phase: string
  kind: string
  name: string
  namespace: string
  group: string
}

export interface ManifestGroupStatus {
  phase: string
  mode: string
  manifests: Record<string, ManifestStatus>
}

export interface ClusterManifestStatus {
  groups: Record<string, ManifestGroupStatus>
  outOfSync: number
  total: number
  lastError: string
}

export interface AuditEntry {
  timestamp: string
  user: string
  action: string
  resource: string
  kind: string
}

export interface MeResponse {
  username: string
  role: string
  authDisabled: boolean
  oidcEnabled: boolean
}
