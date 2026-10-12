<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { store, closeRunPanel } from '../store'
import { api } from '../backend'
const { t } = useI18n()

const run = computed(() => store.running)

function pct(): number {
  const r = run.value
  if (!r) return 0
  if (r.total > 0) return Math.round((r.done / r.total) * 100)
  return r.finished ? 100 : 0
}
</script>

<template>
  <transition name="fade">
    <div v-if="run && store.panelOpen" class="panel">
      <div class="p-head">
        <b>{{ run.name }}</b>
        <span v-if="!run.finished" class="spin"></span>
        <span style="flex: 1"></span>
        <button v-if="!run.finished" class="btn sm danger" @click="api.cancelTask(run.id)">
          {{ $t('run.cancel') }}
        </button>
        <button class="btn ghost sm" @click="closeRunPanel()">✕</button>
      </div>

      <div class="p-body">
        <div class="p-progress">
          <div class="pct-row">
            <span v-if="run.total > 0">{{ $t('run.done', { n: run.done, m: run.total }) }}</span>
            <span v-else>{{ $t('run.doneNoTotal', { n: run.done }) }}</span>
            <b>{{ pct() }}%</b>
          </div>
          <div class="bar"><div class="bar-in" :style="{ width: pct() + '%' }"></div></div>
          <div class="current mono">{{ $t('run.waiting') }}</div>
        </div>

        <div v-if="run.finished" class="p-result" :class="run.finished.status">
          {{
            run.finished.status === 'ok'
              ? $t('run.finishedOk')
              : run.finished.status === 'cancelled'
                ? $t('run.finishedCancel')
                : $t('run.finishedError')
          }}
          · {{ $t('run.stats', { c: run.finished.copied, d: run.finished.deleted, s: run.finished.skipped, f: run.finished.failed }) }}
          <span v-if="run.finished.message"> · {{ run.finished.message }}</span>
        </div>

        <div class="logs mono">
          <div v-for="(l, i) in run.logs" :key="i" :class="'lv-' + l.level">
            <span class="lt">{{ l.time }}</span>{{ l.text }}
          </div>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.panel {
  position: fixed;
  right: 0;
  top: 0;
  bottom: 0;
  width: 440px;
  max-width: 92vw;
  background: var(--panel);
  border-left: 1px solid var(--border);
  box-shadow: -12px 0 32px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  z-index: 40;
}
.p-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 16px;
  border-bottom: 1px solid var(--border);
}
.p-body { flex: 1; display: flex; flex-direction: column; padding: 14px 16px; gap: 12px; min-height: 0; }
.pct-row { display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px; }
.bar {
  height: 8px;
  background: var(--bg-soft);
  border-radius: 5px;
  overflow: hidden;
}
.bar-in {
  height: 100%;
  background: var(--primary);
  border-radius: 5px;
  transition: width 0.25s ease;
}
.current {
  margin-top: 7px;
  font-size: 12px;
  color: var(--text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.p-result {
  font-size: 13px;
  padding: 9px 12px;
  border-radius: 8px;
  background: var(--panel-2);
}
.p-result.ok { color: var(--ok); }
.p-result.error { color: var(--err); }
.p-result.cancelled { color: var(--warn); }
.logs {
  flex: 1;
  overflow: auto;
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.7;
}
.lt { color: var(--text-3); margin-right: 8px; }
.lv-warn { color: var(--warn); }
.lv-error { color: var(--err); }
</style>
