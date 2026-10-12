<script setup lang="ts">
import { api } from '../backend'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ modelValue: string; placeholder?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
const { t } = useI18n()

async function browse() {
  const dir = await api.pickFolder(t('wizard.pickTitle'))
  if (dir) emit('update:modelValue', dir)
}
</script>

<template>
  <div class="row">
    <input
      class="input"
      :value="modelValue"
      :placeholder="placeholder"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <button class="btn" @click="browse">{{ $t('common.browse') }}</button>
  </div>
</template>

<style scoped>
.row { display: flex; gap: 8px; }
.row .input { flex: 1; font-family: Consolas, monospace; font-size: 13px; }
.row .btn { flex: none; }
</style>
