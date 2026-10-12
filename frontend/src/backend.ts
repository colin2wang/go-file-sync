// 后端 API 封装：桌面端直连 window.go，浏览器端回退 Mock（演示模式）
import type { TaskConfig, TaskSummary, PreviewResult, Settings, Action } from './types'
import { emitMock } from './events'

export const isDesktop = typeof (window as any).go !== 'undefined' && !!(window as any).go?.main?.App

function call<T>(method: string, ...args: any[]): Promise<T> {
  if (!isDesktop) return mockCall(method, ...args) as Promise<T>
  return (window as any).go.main.App[method](...args) as Promise<T>
}

export const api = {
  listTasks: () => call<TaskSummary[]>('ListTasks'),
  getTask: (id: string) => call<TaskConfig>('GetTask', id),
  newTask: () => call<TaskConfig>('NewTask'),
  saveTask: (cfg: TaskConfig) => call<TaskConfig>('SaveTask', cfg),
  deleteTask: (id: string) => call<void>('DeleteTask', id),
  previewPlan: (id: string) => call<PreviewResult>('PreviewPlan', id),
  runTask: (id: string) => call<void>('RunTask', id),
  cancelTask: (id: string) => call<void>('CancelTask', id),
  pickFolder: (title: string) => call<string>('PickFolder', title),
  renderYAML: (cfg: TaskConfig) => call<string>('RenderYAML', cfg),
  parseYAML: (text: string) => call<TaskConfig>('ParseYAML', text),
  getSettings: () => call<Settings>('GetSettings'),
  saveSettings: (s: Settings) => call<void>('SaveSettings', s),
  configDir: () => call<string>('ConfigDir'),
  openInExplorer: (p: string) => call<void>('OpenInExplorer', p),
}

/* ============================== Mock 后端 ============================== */

const nowISO = () => new Date().toISOString()

function defaultFilters() {
  return { exclude: ['*.tmp', 'Thumbs.db'], include: [], caseSensitive: false }
}

const mockTasks: TaskSummary[] = [
  {
    id: 'mock-1', name: '照片库双向同步', enabled: true,
    left: 'D:\\照片库', right: 'E:\\备份\\照片库',
    mode: 'two-way', incremental: true, mirror: false, conflict: 'newer', backupDeleted: true,
    filters: { exclude: ['*.tmp', 'Thumbs.db', 'desktop.ini'], include: [], caseSensitive: false },
    schedule: { enabled: true, type: 'interval', intervalMinutes: 30, dailyTime: '02:00' },
    created: nowISO(), updated: nowISO(),
    nextRun: new Date(Date.now() + 12 * 60000).toISOString(),
    lastRun: new Date(Date.now() - 18 * 60000).toISOString(),
    lastResult: { status: 'ok', copied: 5, deleted: 0, skipped: 1204, conflicts: 0, failed: 0, bytes: 86_214_912, finished: nowISO() },
    running: false,
  },
  {
    id: 'mock-2', name: '项目文档归档', enabled: true,
    left: 'C:\\Users\\me\\Documents', right: 'D:\\Archive\\Documents',
    mode: 'one-way', incremental: true, mirror: true, conflict: 'newer', backupDeleted: true,
    filters: defaultFilters(),
    schedule: { enabled: true, type: 'daily', intervalMinutes: 60, dailyTime: '02:00' },
    created: nowISO(), updated: nowISO(),
    nextRun: tomorrow2AM(),
    lastRun: new Date(Date.now() - 9 * 3600_000).toISOString(),
    lastResult: { status: 'ok', copied: 12, deleted: 3, skipped: 3310, conflicts: 0, failed: 0, bytes: 154_618_880, finished: nowISO() },
    running: false,
  },
]

function tomorrow2AM(): string {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  d.setHours(2, 0, 0, 0)
  return d.toISOString()
}

let mockSettings: Settings = { language: 'zh-CN', theme: 'light' }

// 与 Go RenderYAML 同格式的 TS 复刻（仅演示模式）
function mockQuote(s: string): string {
  if (s === '') return '""'
  if (s === 'true' || s === 'false' || s === 'null' || s === 'yes' || s === 'no') return `"${s}"`
  if (s.trim() !== s) return `"${s}"`
  if (/[:#{}[\],&*!|>'"%@`]/.test(s) || s.includes(': ')) return `"${s.replace(/"/g, '\\"')}"`
  if (/^[\d.+-]+$/.test(s)) return `"${s}"`
  return s
}

function mockRenderYAML(c: TaskConfig): string {
  const L: string[] = []
  const w = (s = '') => L.push(s)
  w('# ===================== FileSync 任务配置 =====================')
  w('# name    : 任务名称（显示在任务列表）')
  w('# enabled : false 时该任务不参与手动与定时同步')
  w()
  w(`name: ${mockQuote(c.name)}`)
  w(`enabled: ${c.enabled}`)
  w()
  w('# -----------------------------------------------------------')
  w('# left  : 左侧文件夹（one-way 模式下作为源）')
  w('# right : 右侧文件夹（one-way 模式下作为目标）')
  w('# mode  : one-way = 左→右 | one-way-back = 右→左 | two-way = 双向保持一致')
  w(`left: ${mockQuote(c.left)}`)
  w(`right: ${mockQuote(c.right)}`)
  w(`mode: ${c.mode}`)
  w()
  w('# -----------------------------------------------------------')
  w('# incremental    : true 时仅同步有变化的文件（大小+修改时间比对），提高效率')
  w('# mirror         : 单向模式下，源端已删除的文件在目标端同步删除')
  w('# conflict       : 双向模式冲突策略：newer 较新者胜 | left 以左为准 | right 以右为准 | skip 跳过')
  w('# backup_deleted : 删除或覆盖前备份到目标目录的 .filesync-trash/')
  w(`incremental: ${c.incremental}`)
  w(`mirror: ${c.mirror}`)
  w(`conflict: ${c.conflict}`)
  w(`backup_deleted: ${c.backupDeleted}`)
  w()
  w('# -----------------------------------------------------------')
  w('# 过滤规则：glob 通配（* 任意、? 单字符、** 跨目录；尾部 / 表示目录）')
  w('# exclude: 排除项；include: 非空时仅同步匹配项')
  w('filters:')
  if (c.filters.exclude.length === 0) {
    w('  exclude: []')
  } else {
    w('  exclude:')
    c.filters.exclude.forEach((x) => w(`    - ${mockQuote(x)}`))
  }
  if (c.filters.include.length === 0) {
    w('  include: []')
  } else {
    w('  include:')
    c.filters.include.forEach((x) => w(`    - ${mockQuote(x)}`))
  }
  w(`  case_sensitive: ${c.filters.caseSensitive}`)
  w()
  w('# -----------------------------------------------------------')
  w('# 定时同步：type = interval（每 N 分钟）| daily（每天 HH:MM）')
  w('schedule:')
  w(`  enabled: ${c.schedule.enabled}`)
  w(`  type: ${c.schedule.type}`)
  w(`  interval_minutes: ${c.schedule.intervalMinutes}`)
  w(`  daily_time: ${mockQuote(c.schedule.dailyTime)}`)
  return L.join('\n') + '\n'
}

function mockPlanFor(t: TaskSummary): PreviewResult {
  const paths = [
    { p: '工作总结 2026Q4.docx', s: 48_128, why: 'modified' },
    { p: '会议纪要/评审会 10-09.md', s: 8_192, why: 'new' },
    { p: '设计稿/logo-v3.psd', s: 24_117_248, why: 'new' },
    { p: '旧版需求.pdf', s: 1_048_576, why: t.mode === 'one-way' && t.mirror ? 'delete' : 'new' },
    { p: '数据/客户表.xlsx', s: 92_160, why: 'modified' },
  ]
  const actions = paths.map((x, i) => ({
    type: x.why === 'delete' ? 'delete-right' : i % 3 === 2 && t.mode === 'two-way' ? 'copy-r2l' : t.mode === 'one-way-back' ? 'copy-r2l' : t.mode === 'one-way' ? 'copy-l2r' : i === 2 ? 'copy-l2r' : 'copy-l2r',
    path: x.p,
    reason: x.why,
    size: x.s,
  })) as Action[]
  return {
    actions,
    stats: {
      copyL2R: 3, copyR2L: 1, delLeft: 0, delRight: 1, conflict: 0, skip: 0,
      unchanged: 233, totalBytes: 25_266_944,
    },
    leftMissing: false, rightMissing: false, truncated: false, hasPrevState: true,
  }
}

let mockTimers: any[] = []

function mockCall(method: string, ...args: any[]): Promise<any> {
  const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))
  switch (method) {
    case 'ListTasks':
      return Promise.resolve(JSON.parse(JSON.stringify(mockTasks)))
    case 'GetTask': {
      const t = mockTasks.find((x) => x.id === args[0])
      return t ? Promise.resolve({ ...t }) : Promise.reject(new Error('not found'))
    }
    case 'NewTask':
      return Promise.resolve({
        id: '', name: '', enabled: true, left: '', right: '',
        mode: 'one-way', incremental: true, mirror: false, conflict: 'newer', backupDeleted: true,
        filters: defaultFilters(),
        schedule: { enabled: false, type: 'interval', intervalMinutes: 60, dailyTime: '02:00' },
        created: '', updated: '',
      })
    case 'SaveTask': {
      const c = { ...(args[0] as TaskConfig) }
      if (!c.id) c.id = 'mock-' + Math.random().toString(36).slice(2, 8)
      c.updated = nowISO()
      const idx = mockTasks.findIndex((x) => x.id === c.id)
      const summary: TaskSummary = { ...c, nextRun: '', lastRun: '', lastResult: { status: 'ok', copied: 0, deleted: 0, skipped: 0, conflicts: 0, failed: 0, bytes: 0, finished: '' }, running: false }
      if (idx >= 0) mockTasks[idx] = summary
      else mockTasks.push(summary)
      return Promise.resolve({ ...c })
    }
    case 'DeleteTask': {
      const del = mockTasks.findIndex((x) => x.id === args[0])
      if (del >= 0) mockTasks.splice(del, 1)
      return Promise.resolve()
    }
    case 'PreviewPlan':
      return sleep(600).then(() => {
        const t = mockTasks.find((x) => x.id === args[0])!
        return mockPlanFor(t)
      }) as Promise<any>
    case 'RunTask': {
      const id = args[0] as string
      const t = mockTasks.find((x) => x.id === id)
      if (!t || t.running) return Promise.resolve()
      t.running = true
      emitMock('task:run-started', { id, name: t.name })
      emitMock('task:log', { id, level: 'info', text: '开始扫描左侧文件夹…', time: new Date().toLocaleTimeString() })
      const files = ['docs/方案.md', 'docs/预算.xlsx', '图片/封面.png', '图片/示意图.svg', '数据/明细.csv', '数据/汇总.csv', 'README.md']
      const total = files.length
      files.forEach((f, i) => {
        mockTimers.push(setTimeout(() => {
          emitMock('task:log', { id, level: 'info', text: `← 复制 ${f}`, time: new Date().toLocaleTimeString() })
          emitMock('task:progress', { id, done: i + 1, total, path: f })
        }, 350 * (i + 1)))
      })
      mockTimers.push(setTimeout(() => {
        t.running = false
        t.lastRun = nowISO()
        t.lastResult = { status: 'ok', copied: total, deleted: 0, skipped: 41, conflicts: 0, failed: 0, bytes: 3_145_728, finished: nowISO() }
        emitMock('task:log', { id, level: 'info', text: '同步完成', time: new Date().toLocaleTimeString() })
        emitMock('task:run-finished', { id, result: t.lastResult })
      }, 350 * (total + 1)))
      return sleep(50) as Promise<any>
    }
    case 'CancelTask':
      mockTimers.forEach(clearTimeout)
      mockTimers = []
      mockTasks.forEach((x) => (x.running = false))
      return Promise.resolve()
    case 'IsRunning':
      return Promise.resolve(!!mockTasks.find((x) => x.id === args[0])?.running)
    case 'PickFolder':
      return sleep(200).then(() => '/home/user/演示/已选择的文件夹') as Promise<any>
    case 'RenderYAML':
      return Promise.resolve(mockRenderYAML(args[0] as TaskConfig))
    case 'ParseYAML':
      return Promise.reject(new Error('浏览器演示模式不支持 YAML 导入，请在桌面应用中使用'))
    case 'GetSettings':
      return Promise.resolve({ ...mockSettings })
    case 'SaveSettings':
      mockSettings = { ...(args[0] as Settings) }
      return Promise.resolve()
    case 'ConfigDir':
      return Promise.resolve('C:\\Users\\me\\AppData\\Roaming\\FileSync\\tasks')
    case 'OpenInExplorer':
      return Promise.resolve()
    default:
      return Promise.reject(new Error('unknown mock method: ' + method))
  }
}
