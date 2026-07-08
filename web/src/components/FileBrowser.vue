<template>
  <div class="file-browser" v-if="visible">
    <div class="browser-overlay" @click.self="close"></div>
    <div class="browser-modal">
      <div class="browser-header">
        <h3>{{ t.selectDir }}</h3>
        <button class="close-btn" @click="close">×</button>
      </div>
      <div class="browser-path">
        <input v-model="currentPath" @keyup.enter="loadDir(currentPath)" placeholder="Path...">
        <button @click="loadDir(currentPath)">Go</button>
      </div>
      <div class="browser-drives" v-if="drives.length > 0">
        <button 
          v-for="drive in drives" 
          :key="drive"
          :class="{ active: currentPath.startsWith(drive) }"
          @click="loadDir(drive)"
        >
          {{ drive }}
        </button>
      </div>
      <div class="browser-list">
        <div 
          v-for="file in files" 
          :key="file.path"
          class="browser-item"
          :class="{ directory: file.is_dir }"
          @dblclick="file.is_dir ? loadDir(file.path) : selectFile(file)"
        >
          <span class="file-icon">{{ file.is_dir ? '📁' : '📄' }}</span>
          <span class="file-name">{{ file.name }}</span>
          <span class="file-size" v-if="!file.is_dir">{{ formatSize(file.size) }}</span>
        </div>
        <div v-if="files.length === 0" class="empty">Empty directory</div>
      </div>
      <div class="browser-footer">
        <input v-model="selectedPath" readonly placeholder="Selected path...">
        <button class="btn-primary" @click="confirmSelect">{{ t.confirm }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, inject } from 'vue'
import messages from '../i18n'

interface FileItem {
  name: string
  path: string
  is_dir: boolean
  size: number
}

const props = defineProps<{
  visible: boolean
  initialPath?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'select', path: string): void
}>()

const lang = inject<any>('lang')
const t = ref(messages.zh)

watch(lang, (val) => {
  t.value = messages[val] || messages.zh
}, { immediate: true })

const files = ref<FileItem[]>([])
const currentPath = ref(props.initialPath || 'C:\\')
const selectedPath = ref('')
const drives = ref<string[]>([])

watch(() => props.visible, (val) => {
  if (val) {
    loadDrives()
    loadDir(currentPath.value)
  }
})

async function loadDrives() {
  const res = await fetch('/api/drives')
  const data = await res.json()
  if (Array.isArray(data)) {
    drives.value = data
  }
}

async function loadDir(path: string) {
  const res = await fetch(`/api/files?path=${encodeURIComponent(path)}`)
  const data = await res.json()
  if (Array.isArray(data)) {
    files.value = data
    currentPath.value = path
  }
}

function selectFile(file: FileItem) {
  selectedPath.value = file.path
}

function confirmSelect() {
  if (selectedPath.value) {
    emit('select', selectedPath.value)
    emit('update:visible', false)
  }
}

function close() {
  emit('update:visible', false)
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}
</script>

<style scoped>
.file-browser {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
}

.browser-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
}

.browser-modal {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 600px;
  max-height: 80vh;
  background: #1c2938;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.browser-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #253341;
}

.browser-header h3 {
  margin: 0;
  font-size: 16px;
}

.close-btn {
  background: none;
  border: none;
  color: #8899a6;
  font-size: 24px;
  cursor: pointer;
  line-height: 1;
}

.close-btn:hover {
  color: #e1e8ed;
}

.browser-path {
  display: flex;
  gap: 8px;
  padding: 12px 20px;
  border-bottom: 1px solid #253341;
}

.browser-path input {
  flex: 1;
  padding: 8px 12px;
  background: #0f1419;
  border: 1px solid #253341;
  border-radius: 6px;
  color: #e1e8ed;
  font-family: monospace;
}

.browser-path button {
  padding: 8px 16px;
  background: #253341;
  border: none;
  border-radius: 6px;
  color: #e1e8ed;
  cursor: pointer;
}

.browser-path button:hover {
  background: #3a4a5c;
}

.browser-drives {
  display: flex;
  gap: 4px;
  padding: 8px 20px;
  border-bottom: 1px solid #253341;
}

.browser-drives button {
  padding: 4px 12px;
  background: #253341;
  border: 1px solid transparent;
  border-radius: 4px;
  color: #8899a6;
  cursor: pointer;
  font-size: 12px;
}

.browser-drives button:hover,
.browser-drives button.active {
  background: #1da1f2;
  color: white;
}

.browser-list {
  flex: 1;
  overflow-y: auto;
  max-height: 400px;
  padding: 8px 0;
}

.browser-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 20px;
  cursor: pointer;
}

.browser-item:hover {
  background: #253341;
}

.browser-item.directory {
  color: #1da1f2;
}

.file-icon {
  width: 20px;
  text-align: center;
}

.file-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  color: #8899a6;
  font-size: 12px;
}

.empty {
  text-align: center;
  padding: 40px;
  color: #8899a6;
}

.browser-footer {
  display: flex;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid #253341;
}

.browser-footer input {
  flex: 1;
  padding: 8px 12px;
  background: #0f1419;
  border: 1px solid #253341;
  border-radius: 6px;
  color: #e1e8ed;
  font-family: monospace;
}

.btn-primary {
  padding: 8px 16px;
  background: #1da1f2;
  border: none;
  border-radius: 6px;
  color: white;
  cursor: pointer;
  font-weight: 500;
}

.btn-primary:hover {
  background: #1a91da;
}
</style>
