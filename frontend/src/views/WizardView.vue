<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TaskConfig } from '../types'
import { api } from '../backend'
import { store, refreshTasks } from '../store'
import FolderInput from '../components/FolderInput.vue'
import FilterEditor from '../components/FilterEditor.vue'
import YamlPane from '../components/YamlPane.vue'

const { t } = useI18n()

const cfg = ref<TaskConfig | null>(null)
const yaml = ref('')
const yamlErr = ref('')
const step = ref(0)
const saving = ref(false)
const importOpen = ref(false)
const importText = ref('')
const importErr = ref('')

const steps = computed(() => [
  t('wizard.stepBasic'),
  t('wizard.stepFolders'),
  t('wizard.stepStrategy'),
  t('wizard.stepFilters'),
  t('wizard.stepSchedule'),
])

onMounted(async () => {
  cfg.value = store.editingId ? await api.getTask(store.editingId) : await api.newTask()
  yaml.value = await api.renderYAML(cfg.value)
})

// 表单任意变化 → 防抖渲染 YAML（所见即所得）
let timer: any = null
watch(
  cfg,
  () => {
    if (!cfg.value) return
    clearTimeout(timer)
    timer = setTimeout(async () => {
      try {
        yaml.value = await api.renderYAML(cfg.value!)
        yamlErr.value = ''
      } catch (e: any) {
        yamlErr.value = String(e?.message || e)
      }
    }, 220)
  },
  { deep: true },
)

const canNext = computed(() => {
  if (!cfg.value) return false
  if (step.value === 0) return cfg.value.name.trim() !== ''
  if (step.value === 1) return cfg.value.left.trim() !== '' && cfg.value.right.trim() !== ''
  return true
})

async function save() {
  if (!cfg.value) return
  saving.value = true
  try {
    await api.saveTask(cfg.value)
    store.view = 'tasks'
    store.editingId = ''
    await refreshTasks()
  } catch (e: any) {
    alert(String(e?.message || e))
  } finally {
    saving.value = false
  }
}

function back() {
  store.view = 'tasks'
  store.editingId = ''
}

async function doImport() {
  try {
    cfg.value = await api.parseYAML(importText.value)
    importOpen.value = false
    importErr.value = ''
  } catch (e: any) {
    importErr.value = String(e?.message || e)
  }
}

const modeCards = computed(() => [
  { v: 'one-way', icon: 'L ⟶ R', label: t('wizard.modeOneWay'), desc: t('wizard.modeOneWayDesc') },
  { v: 'one-way-back', icon: 'L ⟵ R', label: t('wizard.modeOneWayBack'), desc: t('wizard.modeOneWayBackDesc') },
  { v: 'two-way', icon: 'L ⟷ R', label: t('wizard.modeTwoWay'), desc: t('wizard.modeTwoWayDesc') },
])
</script>

<template>
  <div class="wizard">
    <div class="w-left">
      <div class="w-head">
        <h2>{{ store.editingId ? $t('wizard.editTitle') : $t('wizard.newTitle') }}</h2>
        <button class="btn ghost sm" @click="importOpen = true">{{ $t('common.import') }}</button>
      </div>

      <div class="steps">
        <button
          v-for="(s, i) in steps"
          :key="i"
          class="step"
          :class="{ active: i === step, done: i < step }"
          @click="step = i"
        >
          <span class="dot">{{ i < step ? '✓' : i + 1 }}</span>
          <span class="step-label">{{ s }}</span>
        </button>
      </div>

      <div v-if="cfg" class="w-form card">
        <!-- 步骤 0：基本信息 -->
        <div v-show="step === 0" class="fg">
          <label>{{ $t('wizard.taskName') }}</label>
          <input v-model="cfg.name" class="input" :placeholder="$t('wizard.taskNamePh')" />
          <label class="switch-row">
            <span>{{ $t('wizard.taskEnabled') }}</span>
            <input v-model="cfg.enabled" type="checkbox" class="sw" />
          </label>
        </div>

        <!-- 步骤 1：文件夹与模式 -->
        <div v-show="step === 1" class="fg">
          <label>{{ $t('wizard.leftFolder') }}</label>
          <FolderInput v-model="cfg.left" placeholder="/path/to/left" />
          <label>{{ $t('wizard.rightFolder') }}</label>
          <FolderInput v-model="cfg.right" placeholder="/path/to/right" />
          <label>{{ $t('wizard.syncMode') }}</label>
          <div class="mode-cards">
            <button
              v-for="m in modeCards"
              :key="m.v"
              class="mode-card"
              :class="{ active: cfg.mode === m.v }"
              type="button"
              @click="cfg.mode = m.v as any"
            >
              <span class="mode-icon mono">{{ m.icon }}</span>
              <span class="mode-label">{{ m.label }}</span>
              <span class="mode-desc">{{ m.desc }}</span>
            </button>
          </div>
        </div>

        <!-- 步骤 2：同步策略 -->
        <div v-show="step === 2" class="fg">
          <label class="switch-row">
            <span>{{ $t('wizard.incremental') }}<small>{{ $t('wizard.incrementalHint') }}</small></span>
            <input v-model="cfg.incremental" type="checkbox" class="sw" />
          </label>
          <label class="switch-row">
            <span>{{ $t('wizard.mirror') }}<small>{{ $t('wizard.mirrorHint') }}</small></span>
            <input v-model="cfg.mirror" type="checkbox" class="sw" />
          </label>
          <label class="switch-row">
            <span>{{ $t('wizard.backupDeleted') }}<small>{{ $t('wizard.backupDeletedHint') }}</small></span>
            <input v-model="cfg.backupDeleted" type="checkbox" class="sw" />
          </label>
          <template v-if="cfg.mode === 'two-way'">
            <label>{{ $t('wizard.conflict') }}</label>
            <select v-model="cfg.conflict" class="input">
              <option value="newer">{{ $t('conflict.newer') }}</option>
              <option value="left">{{ $t('conflict.left') }}</option>
              <option value="right">{{ $t('conflict.right') }}</option>
              <option value="skip">{{ $t('conflict.skip') }}</option>
            </select>
          </template>
        </div>

        <!-- 步骤 3：过滤规则 -->
        <div v-show="step === 3" class="fg">
          <p class="fg-hint">{{ $t('wizard.filtersHint') }}</p>
          <FilterEditor
            v-model="cfg.filters.exclude"
            :label="$t('wizard.excludeRules')"
            :placeholder="$t('wizard.excludePh')"
          />
          <FilterEditor
            v-model="cfg.filters.include"
            :label="$t('wizard.includeRules')"
            :placeholder="$t('wizard.includePh')"
          />
          <label class="switch-row">
            <span>{{ $t('wizard.caseSensitive') }}</span>
            <input v-model="cfg.filters.caseSensitive" type="checkbox" class="sw" />
          </label>
        </div>

        <!-- 步骤 4：定时任务 -->
        <div v-show="step === 4" class="fg">
          <label class="switch-row">
            <span>{{ $t('wizard.scheduleEnabled') }}</span>
            <input v-model="cfg.schedule.enabled" type="checkbox" class="sw" />
          </label>
          <template v-if="cfg.schedule.enabled">
            <label>{{ $t('wizard.scheduleType') }}</label>
            <select v-model="cfg.schedule.type" class="input">
              <option value="interval">{{ $t('wizard.typeInterval') }}</option>
              <option value="daily">{{ $t('wizard.typeDaily') }}</option>
            </select>
            <div v-if="cfg.schedule.type === 'interval'" class="inline-field">
              <label>{{ $t('wizard.everyMinutes') }}</label>
              <input
                v-model.number="cfg.schedule.intervalMinutes"
                type="number"
                min="1"
                class="input"
                style="max-width: 140px"
              />
            </div>
            <div v-else class="inline-field">
              <label>{{ $t('wizard.dailyTime') }}</label>
              <input v-model="cfg.schedule.dailyTime" type="time" class="input" style="max-width: 140px" />
            </div>
          </template>
        </div>
      </div>

      <div class="w-foot">
        <button class="btn" :disabled="step === 0" @click="step--">← {{ $t('wizard.prev') }}</button>
        <button class="btn ghost" @click="back">{{ $t('common.cancel') }}</button>
        <span style="flex: 1"></span>
        <button v-if="step < 4" class="btn primary" :disabled="!canNext" @click="step++">
          {{ $t('wizard.next') }} →
        </button>
        <button v-else class="btn primary" :disabled="saving" @click="save">
          {{ saving ? $t('common.loading') : '💾 ' + $t('common.save') }}
        </button>
      </div>
    </div>

    <div class="w-right">
      <YamlPane :yaml="yaml" :error="yamlErr" />
    </div>

    <!-- YAML 导入弹层 -->
    <transition name="fade">
      <div v-if="importOpen" class="mask" @click.self="importOpen = false">
        <div class="card import-box">
          <div class="imp-head">
            <b>{{ $t('common.import') }}</b>
            <button class="btn ghost sm" @click="importOpen = false">✕</button>
          </div>
          <p class="fg-hint">{{ $t('wizard.importHint') }}</p>
          <textarea v-model="importText" class="input mono imp-area" spellcheck="false"></textarea>
          <div v-if="importErr" class="imp-err">{{ importErr }}</div>
          <div style="display: flex; justify-content: flex-end; gap: 8px">
            <button class="btn primary" @click="doImport">{{ $t('wizard.parseBack') }}</button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.wizard {
  display: flex;
  gap: 18px;
  padding: 26px 30px;
  height: 100%;
  max-width: 1240px;
  margin: 0 auto;
  min-height: 0;
}
.w-left { flex: 1.15; display: flex; flex-direction: column; min-height: 0; }
.w-right { flex: 1; min-width: 380px; min-height: 0; }
.w-head { display: flex; align-items: center; margin-bottom: 14px; gap: 10px; }
.w-head h2 { font-size: 19px; }
.w-head .btn { margin-left: auto; }
.steps { display: flex; gap: 4px; margin-bottom: 14px; flex-wrap: wrap; }
.step {
  display: flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border);
  background: var(--panel);
  color: var(--text-2);
  border-radius: 99px;
  padding: 6px 13px 6px 7px;
  font-size: 12.5px;
  cursor: pointer;
}
.step .dot {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--tag-bg);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11.5px;
}
.step.active {
  border-color: var(--primary);
  color: var(--primary);
  font-weight: 600;
}
.step.active .dot { background: var(--primary); color: var(--primary-text); }
.step.done .dot { background: var(--ok); color: #fff; }
.w-form { flex: 1; overflow: auto; padding: 18px 20px; }
.fg { display: flex; flex-direction: column; gap: 14px; }
.fg > label { font-size: 12.5px; color: var(--text-2); font-weight: 600; }
.fg-hint { font-size: 12.5px; color: var(--text-3); margin: 0; }
.switch-row {
  display: flex !important;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  cursor: pointer;
  font-weight: 500 !important;
  color: var(--text) !important;
  font-size: 13.5px !important;
}
.switch-row small {
  display: block;
  color: var(--text-3);
  font-weight: 400;
  font-size: 12px;
  margin-top: 2px;
}
.mode-cards { display: flex; gap: 10px; }
.mode-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: center;
  border: 1.5px solid var(--border);
  border-radius: 10px;
  background: var(--panel-2);
  padding: 14px 8px;
  cursor: pointer;
  color: var(--text-2);
}
.mode-card:hover { border-color: var(--text-3); }
.mode-card.active { border-color: var(--primary); background: var(--primary-weak); color: var(--primary); }
.mode-icon { font-size: 15px; font-weight: 700; letter-spacing: 1px; }
.mode-label { font-size: 13px; font-weight: 600; }
.mode-desc { font-size: 11.5px; color: var(--text-3); text-align: center; }
.inline-field { display: flex; align-items: center; gap: 10px; }
.inline-field label { font-size: 12.5px; color: var(--text-2); margin: 0; }
.w-foot { display: flex; gap: 8px; padding-top: 14px; }
.mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 60;
}
.import-box { width: 640px; max-width: 92vw; padding: 16px 18px; }
.imp-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.imp-area { height: 300px; resize: vertical; font-size: 12.5px; }
.imp-err { color: var(--err); font-size: 12.5px; }
</style>
