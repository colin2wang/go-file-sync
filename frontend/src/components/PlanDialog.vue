<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PreviewResult, Action } from '../types'
import { fmtBytes } from '../store'

const props = defineProps<{ result: PreviewResult }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()

const st = computed(() => props.result.stats)

const actions = computed(() => props.result.actions.map((a) => ({ ...a, reasonText: t('reason.' + a.reason) })))

function typeBadge(a: Action): { cls: string; text: string } {
  switch (a.type) {
    case 'copy-l2r': return { cls: 'primary', text: 'L → R' }
    case 'copy-r2l': return { cls: 'primary', text: 'R → L' }
    case 'delete-left': return { cls: 'err', text: '✕ L' }
    case 'delete-right': return { cls: 'err', text: '✕ R' }
    case 'conflict': return { cls: 'warn', text: '⚠' }
    default: return { cls: '', text: '=' }
  }
}

function count(a: Action): boolean {
  return a.type !== 'skip'
}
</script>

<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal plan">
      <div class="modal-head">
        <b>{{ $t('plan.title') }}</b>
        <span style="flex: 1"></span>
        <button class="btn ghost sm" @click="emit('close')">✕</button>
      </div>

      <div class="plan-stats">
        <span class="badge primary">→{{ st.copyL2R }}</span>
        <span class="badge primary">←{{ st.copyR2L }}</span>
        <span class="badge err">✕{{ st.delLeft + st.delRight }}</span>
        <span v-if="st.conflict" class="badge warn">⚠{{ st.conflict }}</span>
        <span v-if="st.skip" class="badge">{{ st.skip }}</span>
        <span v-if="st.unchanged" class="badge ok">✓{{ st.unchanged }}</span>
        <span class="bytes">{{ fmtBytes(st.totalBytes) }}</span>
      </div>
      <div v-if="result.leftMissing || result.rightMissing" class="plan-note warn">
        {{ result.leftMissing ? $t('plan.leftMissing') : '' }}
        {{ result.rightMissing ? $t('plan.rightMissing') : '' }}
      </div>

      <div class="plan-table">
        <table>
          <thead>
            <tr>
              <th style="width: 84px">{{ $t('plan.colType') }}</th>
              <th>{{ $t('plan.colPath') }}</th>
              <th style="width: 190px">{{ $t('plan.colReason') }}</th>
              <th style="width: 90px; text-align: right">{{ $t('plan.colSize') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(a, i) in actions" :key="i" :class="{ skipped: a.type === 'skip' }">
              <td><span class="badge" :class="typeBadge(a).cls" v-if="count(a)">{{ typeBadge(a).text }}</span></td>
              <td class="mono">{{ a.path }}</td>
              <td class="reason">{{ a.reasonText }}</td>
              <td class="num">{{ a.size ? fmtBytes(a.size) : '' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="result.truncated" class="plan-note">
          {{ $t('plan.truncated') }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plan { width: 860px; max-width: 94vw; height: 640px; max-height: 88vh; display: flex; flex-direction: column; }
.plan-head { padding: 14px 18px; border-bottom: 1px solid var(--border); display: flex; align-items: center; }
.plan-stats {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 12px 18px 4px;
  flex-wrap: wrap;
}
.plan-stats .bytes { margin-left: auto; color: var(--text-3); font-size: 12.5px; }
.plan-note { padding: 6px 18px; font-size: 12.5px; color: var(--warn); }
.plan-table { overflow: auto; padding: 8px 12px 16px; }
.reason { color: var(--text-2); font-size: 12.5px; }
.num { text-align: right; color: var(--text-2); font-size: 12.5px; }
tr.skipped { opacity: 0.55; }
</style>
