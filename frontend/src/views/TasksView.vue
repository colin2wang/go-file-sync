<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import TaskCard from '../components/TaskCard.vue'
import PlanDialog from '../components/PlanDialog.vue'
import { store, refreshTasks, openRunPanel, openWizard } from '../store'
import { api } from '../backend'
import type { PreviewResult } from '../types'

const { t } = useI18n()
const preview = ref<PreviewResult | null>(null)

onMounted(() => refreshTasks())

async function onRun(id: string) {
  openRunPanel(id)
  try {
    await api.runTask(id)
  } catch (e: any) {
    console.error(e)
  }
}

async function onPreview(id: string) {
  try {
    preview.value = await api.previewPlan(id)
  } catch (e: any) {
    alert(t('plan.title') + ': ' + (e?.message || e))
  }
}

async function onDelete(id: string) {
  const task = store.tasks.find((x) => x.id === id)
  if (!confirm(t('tasks.deleteConfirm', { name: task?.name || id }))) return
  await api.deleteTask(id)
  await refreshTasks()
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h2>{{ $t('nav.tasks') }}</h2>
      <span style="flex: 1"></span>
      <button class="btn primary" @click="openWizard('')">＋ {{ $t('nav.newTask') }}</button>
    </div>

    <div v-if="store.demo" class="demo-tip">{{ $t('app.demo') }}</div>

    <div v-if="store.loading" class="empty"><span class="spin"></span></div>

    <div v-else-if="store.tasks.length === 0" class="empty card">
      <div class="empty-ico">⇄</div>
      <div class="empty-title">{{ $t('tasks.empty') }}</div>
      <div class="empty-hint">{{ $t('tasks.emptyHint') }}</div>
      <button class="btn primary" @click="openWizard('')">＋ {{ $t('nav.newTask') }}</button>
    </div>

    <div v-else class="list">
      <TaskCard
        v-for="task in store.tasks"
        :key="task.id"
        :task="task"
        @run="onRun"
        @preview="onPreview"
        @edit="openWizard"
        @delete="onDelete"
      />
    </div>

    <transition name="fade">
      <PlanDialog v-if="preview" :result="preview" @close="preview = null" />
    </transition>
  </div>
</template>

<style scoped>
.page { max-width: 980px; margin: 0 auto; padding: 26px 30px 60px; }
.page-head { display: flex; align-items: center; gap: 12px; margin-bottom: 18px; }
.page-head h2 { font-size: 19px; }
.demo-tip {
  font-size: 12.5px;
  color: var(--warn);
  background: var(--panel);
  border: 1px dashed var(--warn);
  border-radius: 8px;
  padding: 8px 14px;
  margin-bottom: 14px;
}
.list { display: flex; flex-direction: column; gap: 12px; }
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 70px 20px;
  color: var(--text-2);
}
.empty-ico { font-size: 44px; color: var(--text-3); }
.empty-title { font-size: 16px; font-weight: 600; color: var(--text); }
.empty-hint { font-size: 13px; }
</style>
