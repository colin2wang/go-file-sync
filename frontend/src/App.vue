<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import TasksView from './views/TasksView.vue'
import WizardView from './views/WizardView.vue'
import SettingsView from './views/SettingsView.vue'
import RunPanel from './components/RunPanel.vue'
import { store, refreshTasks, switchLanguage, switchTheme } from './store'
import { onEvent } from './events'
import { applyFinished, applyLog, applyProgress } from './store'

const i18n = useI18n()
const dismissDemo = ref(false)
const demoText = computed(() => i18n.t('app.demo'))

let offs: Array<() => void> = []

onMounted(async () => {
  await refreshTasks()
  offs = [
    onEvent('task:run-started', (d: any) => {
      const t = store.tasks.find((x) => x.id === d.id)
      if (t) t.running = true
    }),
    onEvent('task:progress', (d: any) => applyProgress(d.id, d.done, d.total, d.path)),
    onEvent('task:log', (d: any) => applyLog(d.id, d.level, d.text, d.time)),
    onEvent('task:run-finished', (d: any) => applyFinished(d.id, d.result)),
    onEvent('sched:changed', () => refreshTasks()),
  ]
})

onUnmounted(() => offs.forEach((f) => f()))

const viewComp = computed(() =>
  store.view === 'wizard' ? WizardView : store.view === 'settings' ? SettingsView : TasksView,
)
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="logo">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none">
          <rect x="2" y="4" width="8.5" height="12" rx="2" stroke="var(--primary)" stroke-width="2" />
          <rect x="13.5" y="8" width="8.5" height="12" rx="2" stroke="var(--primary)" stroke-width="2" opacity="0.55" />
          <path d="M8 20 L14 13 M14 13 h-3.4 M14 13 v3.4" stroke="var(--primary)" stroke-width="2" stroke-linecap="round" />
        </svg>
        <b>FileSync</b>
      </div>
      <nav>
        <button class="nav-item" :class="{ active: store.view === 'tasks' }" @click="store.view = 'tasks'">
          <span class="ico">⇄</span>{{ $t('nav.tasks') }}
        </button>
        <button class="nav-item" :class="{ active: store.view === 'wizard' }" @click="store.editingId = ''; store.view = 'wizard'">
          <span class="ico">＋</span>{{ $t('nav.newTask') }}
        </button>
        <button class="nav-item" :class="{ active: store.view === 'settings' }" @click="store.view = 'settings'">
          <span class="ico">⚙</span>{{ $t('nav.settings') }}
        </button>
      </nav>
      <div class="side-foot">
        <button class="btn ghost sm" :title="$t('settings.language')" @click="switchLanguage(store.settings.language === 'zh-CN' ? 'en-US' : 'zh-CN')">
          {{ store.settings.language === 'zh-CN' ? 'EN' : '中' }}
        </button>
        <button class="btn ghost sm" :title="$t('settings.theme')" @click="switchTheme(store.settings.theme === 'dark' ? 'light' : 'dark')">
          {{ store.settings.theme === 'dark' ? '☀' : '🌙' }}
        </button>
      </div>
    </aside>

    <main class="main">
      <div v-if="store.demo && !dismissDemo" class="demo-banner">
        <span>🧪 {{ demoText }}</span>
        <button class="btn ghost sm" @click="dismissDemo = true">✕</button>
      </div>
      <component :is="viewComp" />
    </main>

    <RunPanel v-if="store.panelOpen && store.running" />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
  overflow: hidden;
}
.side {
  width: 196px;
  flex: none;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 16px 10px;
}
.logo {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 16.5px;
  padding: 2px 10px 18px;
  color: var(--text);
}
nav {
  display: flex;
  flex-direction: column;
  gap: 3px;
  flex: 1;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: none;
  color: var(--text-2);
  font-size: 14px;
  padding: 9px 12px;
  border-radius: 8px;
  cursor: pointer;
  text-align: left;
}
.nav-item:hover { background: var(--tag-bg); color: var(--text); }
.nav-item.active {
  background: var(--primary-weak);
  color: var(--primary);
  font-weight: 600;
}
.nav-item .ico { width: 18px; text-align: center; }
.side-foot {
  display: flex;
  gap: 6px;
  padding: 8px 6px 0;
  border-top: 1px solid var(--border);
}
.main {
  flex: 1;
  overflow: auto;
  display: flex;
  flex-direction: column;
}
.demo-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--primary-weak);
  color: var(--primary);
  padding: 7px 18px;
  font-size: 12.5px;
  flex: none;
}
</style>
