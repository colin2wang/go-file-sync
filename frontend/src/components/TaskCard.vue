<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TaskSummary } from '../types'
import { fmtTime, fmtNextRun, fmtBytes } from '../store'

const props = defineProps<{ task: TaskSummary }>()
const emit = defineEmits<{
  (e: 'run', id: string): void
  (e: 'preview', id: string): void
  (e: 'edit', id: string): void
  (e: 'delete', id: string): void
}>()
const { t } = useI18n()

const modeBadge = computed(() => t('mode.' + props.task.mode))
const scheduleText = computed(() => {
  const s = props.task.schedule
  if (!s.enabled) return t('tasks.scheduleOff')
  if (s.type === 'daily') return t('tasks.scheduleDaily', { t: s.dailyTime })
  return t('tasks.scheduleEvery', { n: s.intervalMinutes })
})
const lastBadge = computed(() => {
  const r = props.task.lastResult
  if (!r || !r.status) return null
  if (r.status === 'ok') return { cls: 'ok', text: t('tasks.lastOk') }
  if (r.status === 'error') return { cls: 'err', text: t('tasks.lastError') }
  return { cls: 'warn', text: t('tasks.lastCancelled') }
})
const lastDetail = computed(() => {
  const r = props.task.lastResult
  if (!r || !r.status || r.status !== 'ok') return ''
  return t('run.stats', { c: r.copied, d: r.deleted, s: r.skipped, f: r.failed })
})
const dirArrow = computed(() =>
  props.task.mode === 'one-way' ? '→' : props.task.mode === 'one-way-back' ? '←' : '⇄',
)
</script>

<template>
  <div class="card tc" :class="{ dim: !task.enabled }">
    <div class="tc-head">
      <span class="tc-name">
        {{ task.name }}
        <span v-if="task.running" class="spin" style="margin-left: 6px"></span>
      </span>
      <span class="badge primary">{{ modeBadge }} {{ dirArrow }}</span>
      <span class="badge" :class="task.enabled ? '' : 'warn'">
        {{ task.enabled ? $t('common.enabled') : $t('common.disabled') }}
      </span>
      <span style="flex: 1"></span>
      <button class="btn primary sm" :disabled="task.running" @click="emit('run', task.id)">
        {{ task.running ? $t('tasks.running') : $t('tasks.run') }}
      </button>
      <button class="btn sm" @click="emit('preview', task.id)">{{ $t('tasks.preview') }}</button>
      <button class="btn sm" @click="emit('edit', task.id)">{{ $t('common.edit') }}</button>
      <button class="btn sm danger" @click="emit('delete', task.id)">{{ $t('common.delete') }}</button>
    </div>
    <div class="tc-paths">
      <span class="path" :title="task.left">{{ task.left }}</span>
      <span class="arrow">{{ dirArrow }}</span>
      <span class="path" :title="task.right">{{ task.right }}</span>
    </div>
    <div class="tc-meta">
      <span>⏱ {{ scheduleText }}</span>
      <span v-if="task.schedule.enabled">· {{ $t('tasks.nextRun') }}: <b>{{ fmtNextRun(task.nextRun) }}</b></span>
      <span>· {{ $t('tasks.lastRun') }}: {{ fmtTime(task.lastRun) }}</span>
      <span v-if="lastBadge" class="badge" :class="lastBadge.cls">{{ lastBadge.text }}</span>
      <span v-if="lastDetail" class="detail">{{ lastDetail }}</span>
    </div>
  </div>
</template>

<style scoped>
.tc { padding: 14px 18px; display: flex; flex-direction: column; gap: 9px; }
.tc.dim { opacity: 0.62; }
.tc-head { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.tc-name { font-size: 15px; font-weight: 600; margin-right: 4px; }
.tc-paths {
  display: flex;
  align-items: center;
  gap: 10px;
  font-family: Consolas, monospace;
  font-size: 12.8px;
  color: var(--text-2);
}
.tc-paths .path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}
.tc-paths .arrow { color: var(--primary); font-weight: 700; }
.tc-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12.5px;
  color: var(--text-3);
}
.tc-meta .detail { color: var(--text-2); }
</style>
