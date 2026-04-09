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
  logs: LogEntry[]
  logLevel: string
}

export interface MeResponse {
  username: string
  role: string
  authDisabled: boolean
  oidcEnabled: boolean
}
