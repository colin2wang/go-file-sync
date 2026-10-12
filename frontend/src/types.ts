// 与 Go 端 json tag 对齐的类型定义
export interface Filters {
  exclude: string[]
  include: string[]
  caseSensitive: boolean
}

export interface Schedule {
  enabled: boolean
  type: 'interval' | 'daily'
  intervalMinutes: number
  dailyTime: string
}

export interface TaskConfig {
  id: string
  name: string
  enabled: boolean
  left: string
  right: string
  mode: 'one-way' | 'one-way-back' | 'two-way'
  incremental: boolean
  mirror: boolean
  conflict: 'newer' | 'left' | 'right' | 'skip'
  backupDeleted: boolean
  filters: Filters
  schedule: Schedule
  created: string
  updated: string
}

export interface RunSummary {
  status: 'ok' | 'error' | 'cancelled'
  copied: number
  deleted: number
  skipped: number
  conflicts: number
  failed: number
  bytes: number
  message?: string
  finished: string
}

export interface TaskSummary extends TaskConfig {
  nextRun: string
  lastRun: string
  lastResult: RunSummary
  running: boolean
}

export interface Action {
  type: 'copy-l2r' | 'copy-r2l' | 'delete-left' | 'delete-right' | 'conflict' | 'skip'
  path: string
  reason: string
  size?: number
}

export interface PlanStats {
  copyL2R: number
  copyR2L: number
  delLeft: number
  delRight: number
  conflict: number
  skip: number
  unchanged: number
  totalBytes: number
}

export interface PreviewResult {
  actions: Action[]
  stats: PlanStats
  leftMissing: boolean
  rightMissing: boolean
  truncated: boolean
  hasPrevState: boolean
}

export interface Settings {
  language: string
  theme: 'dark' | 'light'
}

export interface LogEntry {
  id: string
  level: 'info' | 'warn' | 'error'
  text: string
  time: string
}
