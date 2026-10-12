// 应用级 reactive 单例（替代 Pinia，少一个依赖）
import { reactive } from 'vue'
import type { TaskSummary, Settings, LogEntry, RunSummary } from './types'
import { api } from './backend'

export type ViewName = 'tasks' | 'wizard' | 'settings'

interface RunningState {
  id: string
  name: string
  done: number
  total: number
  logs: LogEntry[]
  finished: RunSummary | null
}

export const store = reactive({
  view: 'tasks' as ViewName,
  tasks: [] as TaskSummary[],
  loading: false,
  settings: { language: 'zh-CN', theme: 'light' } as Settings,
  editingId: '' as string, // '' = 新建
  running: null as RunningState | null, // 执行面板当前展示的任务
  panelOpen: false,
  demo: false,
})

export async function refreshTasks() {
  store.loading = true
  try {
    store.tasks = await api.listTasks()
  } finally {
    store.loading = false
  }
}

export function openRunPanel(id: string) {
  const t = store.tasks.find((x) => x.id === id)
  store.running = { id, name: t?.name || id, done: 0, total: 0, logs: [], finished: null }
  store.panelOpen = true
}

export function closeRunPanel() {
  store.panelOpen = false
}

export function openWizard(editingId: string) {
  store.editingId = editingId
  store.view = 'wizard'
}

export function applyProgress(id: string, done: number, total: number, path: string) {
  const r = store.running
  if (!r || r.id !== id) return
  r.done = done
  r.total = total
  r.logs.push({ id, level: 'info', text: path, time: new Date().toLocaleTimeString() })
  trimLogs(r)
}

export function applyLog(id: string, level: string, text: string, time: string) {
  const r = store.running
  if (!r || r.id !== id) return
  r.logs.push({ id, level: level as LogEntry['level'], text, time })
  trimLogs(r)
}

export function applyFinished(id: string, result: RunSummary) {
  const r = store.running
  if (!r || r.id !== id) return
  r.finished = result
  const t = store.tasks.find((x) => x.id === id)
  if (t) {
    t.running = false
    t.lastResult = result
  }
}

function trimLogs(r: RunningState) {
  if (r.logs.length > 400) r.logs.splice(0, r.logs.length - 400)
}

export async function applySettings(s: Settings) {
  store.settings = s
  document.documentElement.setAttribute('data-theme', s.theme)
  await api.saveSettings(s)
}

export async function switchLanguage(lang: string) {
  await applySettings({ ...store.settings, language: lang })
  ;(window as any).__i18n.locale.value = lang
}

export async function switchTheme(theme: 'dark' | 'light') {
  await applySettings({ ...store.settings, theme })
}

export function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function fmtTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  const today = new Date()
  const sameDay = d.toDateString() === today.toDateString()
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  if (sameDay) return hm
  return `${d.getMonth() + 1}/${d.getDate()} ${hm}`
}

export function fmtNextRun(iso: string): string {
  if (!iso) return '—'
  const diff = new Date(iso).getTime() - Date.now()
  if (isNaN(diff)) return '—'
  if (diff <= 0) return '⏱'
  const mins = Math.round(diff / 60000)
  if (mins < 60) return `${mins} min`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours} h ${mins % 60} min`
  return fmtTime(iso)
}
