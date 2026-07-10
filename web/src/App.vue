<template>
  <div class="app">
    <header class="header">
      <h1>{{ t.title }}</h1>
      <div class="header-right">
        <div class="header-stats">
          <span class="stat">{{ t.stats.totalTasks }}: {{ tasks.length }}</span>
          <span class="stat success">{{ t.stats.successCount }}: {{ stats.success_count || 0 }}</span>
          <span class="stat error">{{ t.stats.failCount }}: {{ stats.fail_count || 0 }}</span>
          <span class="stat info">{{ t.stats.monitoredFiles || '监控文件' }}: {{ syncStats.monitored_files }}</span>
          <span class="stat success">{{ t.stats.syncedFiles || '同步文件' }}: {{ syncStats.synced_files }}</span>
        </div>
        <LangSwitcher />
      </div>
    </header>

    <main class="main">
      <div class="panel task-panel">
        <div class="panel-header">
          <h2>{{ t.tasks }}</h2>
          <button class="btn btn-primary" @click="showAddTask = true">{{ t.addTask }}</button>
        </div>
        
        <div class="task-list">
          <div v-for="task in tasks" :key="task.id" class="task-card" :class="{ disabled: !task.enabled }">
            <div class="task-header">
              <span class="task-name">{{ task.name }}</span>
              <div class="task-actions">
                <label class="toggle">
                  <input type="checkbox" :checked="task.enabled" @change="toggleTask(task)">
                  <span class="toggle-slider"></span>
                </label>
                <button class="btn-icon" @click="editTask(task)">✏️</button>
                <button class="btn-icon danger" @click="deleteTask(task.id)">🗑️</button>
              </div>
            </div>
            <div class="task-paths">
              <div class="path">
                <span class="path-label">{{ t.sourcePath }}:</span>
                <span class="path-value">{{ task.source_path }}</span>
              </div>
              <div class="path-arrow">→</div>
              <div class="path">
                <span class="path-label">{{ t.targetPath }}:</span>
                <span class="path-value">{{ task.target_path }}</span>
              </div>
            </div>
            <div class="task-meta">
              <span class="meta-item">{{ getDirectionLabel(task.sync_direction) }}</span>
              <span class="meta-item">⏱️ {{ task.monitor_interval }}s</span>
            </div>
          </div>

          <div v-if="tasks.length === 0" class="empty-state">
            {{ t.noTasks }}
          </div>
        </div>
      </div>

      <div class="panel log-panel">
        <div class="panel-header">
          <h2>{{ t.logs }}</h2>
        </div>
        <div class="log-list">
          <div v-for="log in logs" :key="log.id" class="log-item" :class="log.status">
            <span class="log-time">{{ formatTime(log.created_at) }}</span>
            <span class="log-action">{{ log.action }}</span>
            <span class="log-file">{{ log.file_path }}</span>
            <span class="log-status" :class="log.status">{{ (log.status === 'success' || log.status === 'synced') ? t.synced : t.failed }}</span>
          </div>
          <div v-if="logs.length === 0" class="empty-state">
            {{ t.noLogs }}
          </div>
        </div>
      </div>
    </main>

    <!-- Add/Edit Task Modal -->
    <div class="modal-overlay" v-if="showAddTask || editingTask" @click.self="closeModal">
      <div class="modal">
        <h3>{{ editingTask ? t.editTask : t.addTask }}</h3>
        <div class="form-group">
          <label>{{ t.taskName }}</label>
          <input v-model="formData.name" :placeholder="t.taskName">
        </div>
        <div class="form-group">
          <label>{{ t.sourcePath }}</label>
          <div class="path-input">
            <input v-model="formData.source_path" readonly>
            <button @click="openBrowser('source')">{{ t.browse }}</button>
          </div>
        </div>
        <div class="form-group">
          <label>{{ t.targetPath }}</label>
          <div class="path-input">
            <input v-model="formData.target_path" readonly>
            <button @click="openBrowser('target')">{{ t.browse }}</button>
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>{{ t.syncDirection }}</label>
            <select v-model="formData.sync_direction">
              <option value="one_way_upload">{{ t.directionUpload }}</option>
              <option value="one_way_download">{{ t.directionDownload }}</option>
              <option value="two_way">{{ t.directionTwoWay }}</option>
            </select>
          </div>
          <div class="form-group">
            <label>{{ t.monitorInterval }}</label>
            <input type="number" v-model.number="formData.monitor_interval" min="1" max="3600">
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="closeModal">{{ t.cancel }}</button>
          <button class="btn btn-primary" @click="saveTask">{{ t.save }}</button>
        </div>
      </div>
    </div>

    <!-- File Browser -->
    <FileBrowser 
      v-model:visible="showBrowser"
      :initial-path="browserInitialPath"
      @select="onFileSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, provide } from 'vue'
import messages from './i18n'
import LangSwitcher from './components/LangSwitcher.vue'
import FileBrowser from './components/FileBrowser.vue'

interface SyncTask {
  id: number
  name: string
  source_path: string
  target_path: string
  monitor_interval: number
  sync_direction: string
  enabled: boolean
}

interface SyncLog {
  id: number
  task_name: string
  action: string
  file_path: string
  status: string
  created_at: string
}

interface Stats {
  total_tasks: number
  enabled_tasks: number
  success_count: number
  fail_count: number
}

const currentLang = ref(localStorage.getItem('lang') || 'zh')
const t = ref(messages.zh)

// Initialize translations
t.value = messages[currentLang.value] || messages.zh

// Watch for language changes
watch(currentLang, (val) => {
  t.value = messages[val] || messages.zh
})

// Provide language state to child components
provide('lang', currentLang)

const tasks = ref<SyncTask[]>([])
const logs = ref<SyncLog[]>([])
const stats = ref<Stats>({ total_tasks: 0, enabled_tasks: 0, success_count: 0, fail_count: 0 })
const syncStats = ref<{ monitored_files: number; synced_files: number }>({ monitored_files: 0, synced_files: 0 })
const showAddTask = ref(false)
const editingTask = ref<SyncTask | null>(null)
const showBrowser = ref(false)
const browserInitialPath = ref('C:\\')
const browserTarget = ref<'source' | 'target'>('source')

const formData = ref({
  name: '',
  source_path: '',
  target_path: '',
  monitor_interval: 5,
  sync_direction: 'one_way_upload'
})

async function safeJson(res: Response) {
  const text = await res.text()
  try { return JSON.parse(text) } catch { return null }
}

async function loadTasks() {
  const res = await fetch('/api/tasks')
  const data = await safeJson(res)
  tasks.value = Array.isArray(data) ? data : []
}

async function loadLogs() {
  const res = await fetch('/api/logs?limit=50')
  const data = await safeJson(res)
  logs.value = Array.isArray(data) ? data : []
}

async function loadStats() {
  const res = await fetch('/api/stats')
  const data = await safeJson(res)
  stats.value = data || { total_tasks: 0, enabled_tasks: 0, success_count: 0, fail_count: 0 }
}

async function loadSyncStats() {
  const res = await fetch('/api/sync-stats')
  const data = await safeJson(res)
  syncStats.value = data || { monitored_files: 0, synced_files: 0 }
}

async function saveTask() {
  const url = editingTask.value
    ? `/api/tasks/${editingTask.value.id}`
    : '/api/tasks'
  
  const method = editingTask.value ? 'PUT' : 'POST'
  
  await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(formData.value)
  })
  
  closeModal()
  loadTasks()
  loadStats()
}

async function deleteTask(id: number) {
  if (!confirm(t.value.deleteConfirm)) return
  await fetch(`/api/tasks/${id}`, { method: 'DELETE' })
  loadTasks()
  loadStats()
}

async function toggleTask(task: SyncTask) {
  await fetch(`/api/tasks/${task.id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...task, enabled: !task.enabled })
  })
  loadTasks()
}

function editTask(task: SyncTask) {
  editingTask.value = task
  formData.value = {
    name: task.name,
    source_path: task.source_path,
    target_path: task.target_path,
    monitor_interval: task.monitor_interval,
    sync_direction: task.sync_direction
  }
}

function closeModal() {
  showAddTask.value = false
  editingTask.value = null
  formData.value = {
    name: '',
    source_path: '',
    target_path: '',
    monitor_interval: 5,
    sync_direction: 'one_way_upload'
  }
}

function openBrowser(target: 'source' | 'target') {
  browserTarget.value = target
  browserInitialPath.value = target === 'source' ? formData.value.source_path : formData.value.target_path
  showBrowser.value = true
}

function onFileSelect(path: string) {
  if (browserTarget.value === 'source') {
    formData.value.source_path = path
  } else {
    formData.value.target_path = path
  }
}

function getDirectionLabel(direction: string): string {
  switch (direction) {
    case 'one_way_upload': return t.value.directionUpload
    case 'one_way_download': return t.value.directionDownload
    case 'two_way': return t.value.directionTwoWay
    default: return direction
  }
}

function formatTime(ts: string) {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}

onMounted(() => {
  loadTasks()
  loadLogs()
  loadStats()
  loadSyncStats()
  setInterval(loadLogs, 60000) // Refresh logs every 60 seconds
  setInterval(loadStats, 60000) // Refresh stats every 60 seconds
  setInterval(loadSyncStats, 60000) // Refresh sync stats every 60 seconds
})
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: #0f1419;
  color: #e1e8ed;
  min-height: 100vh;
}

.app {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 0;
  border-bottom: 1px solid #253341;
  margin-bottom: 24px;
}

.header h1 {
  font-size: 24px;
  color: #1da1f2;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-stats {
  display: flex;
  gap: 12px;
}

.stat {
  padding: 6px 12px;
  background: #1c2938;
  border-radius: 16px;
  font-size: 12px;
}

.stat.success { color: #00c853; }
.stat.error { color: #ff5252; }
.stat.info { color: #1da1f2; }

.main {
  display: grid;
  grid-template-columns: 1fr 400px;
  gap: 24px;
}

.panel {
  background: #1c2938;
  border-radius: 12px;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #253341;
}

.panel-header h2 {
  font-size: 16px;
  font-weight: 600;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: all 0.2s;
}

.btn-primary {
  background: #1da1f2;
  color: white;
}

.btn-primary:hover {
  background: #1a91da;
}

.btn-secondary {
  background: #253341;
  color: #e1e8ed;
}

.btn-secondary:hover {
  background: #3a4a5c;
}

.task-list {
  padding: 16px;
  max-height: 500px;
  overflow-y: auto;
}

.task-card {
  background: #253341;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
  transition: all 0.2s;
}

.task-card:hover {
  background: #2c3e50;
}

.task-card.disabled {
  opacity: 0.5;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.task-name {
  font-weight: 600;
  font-size: 15px;
}

.task-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  padding: 4px;
  opacity: 0.7;
  transition: opacity 0.2s;
}

.btn-icon:hover { opacity: 1; }
.btn-icon.danger:hover { color: #ff5252; }

.toggle {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
}

.toggle input { opacity: 0; width: 0; height: 0; }

.toggle-slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background: #3a4a5c;
  border-radius: 22px;
  transition: 0.3s;
}

.toggle-slider:before {
  position: absolute;
  content: "";
  height: 16px;
  width: 16px;
  left: 3px;
  bottom: 3px;
  background: white;
  border-radius: 50%;
  transition: 0.3s;
}

.toggle input:checked + .toggle-slider {
  background: #1da1f2;
}

.toggle input:checked + .toggle-slider:before {
  transform: translateX(18px);
}

.task-paths {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  font-size: 13px;
}

.path {
  flex: 1;
  background: #0f1419;
  padding: 8px 12px;
  border-radius: 6px;
  overflow: hidden;
}

.path-label {
  color: #8899a6;
  margin-right: 8px;
}

.path-value {
  word-break: break-all;
}

.path-arrow {
  color: #1da1f2;
  font-size: 18px;
}

.task-meta {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #8899a6;
}

.log-list {
  padding: 12px;
  max-height: 500px;
  overflow-y: auto;
  font-family: "SF Mono", Monaco, monospace;
  font-size: 12px;
}

.log-item {
  display: flex;
  gap: 12px;
  padding: 8px;
  border-radius: 4px;
  margin-bottom: 4px;
}

.log-item:hover {
  background: #253341;
}

.log-time {
  color: #8899a6;
  white-space: nowrap;
}

.log-action {
  color: #1da1f2;
  min-width: 60px;
}

.log-file {
  flex: 1;
  word-break: break-all;
  color: #e1e8ed;
}

.log-status {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  text-transform: uppercase;
}

.log-status.success {
  background: #00c85320;
  color: #00c853;
}

.log-status.failed {
  background: #ff525220;
  color: #ff5252;
}

.empty-state {
  text-align: center;
  padding: 40px;
  color: #8899a6;
}

.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: #1c2938;
  border-radius: 12px;
  padding: 24px;
  width: 100%;
  max-width: 500px;
}

.modal h3 {
  margin-bottom: 20px;
  font-size: 18px;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #8899a6;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  background: #0f1419;
  border: 1px solid #253341;
  border-radius: 6px;
  color: #e1e8ed;
  font-size: 14px;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #1da1f2;
}

.path-input {
  display: flex;
  gap: 8px;
}

.path-input input {
  flex: 1;
  background: #0f1419;
  border: 1px solid #253341;
  border-radius: 6px;
  color: #e1e8ed;
  padding: 10px 12px;
  font-family: monospace;
}

.path-input button {
  padding: 10px 16px;
  background: #253341;
  border: none;
  border-radius: 6px;
  color: #e1e8ed;
  cursor: pointer;
}

.path-input button:hover {
  background: #3a4a5c;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

@media (max-width: 900px) {
  .main {
    grid-template-columns: 1fr;
  }
}
</style>
