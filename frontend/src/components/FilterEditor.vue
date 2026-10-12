<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ label: string; placeholder: string; modelValue: string[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string[]): void }>()
const { t } = useI18n()
const draft = ref('')

function commit() {
  const v = draft.value.trim()
  if (!v) return
  if (!props.modelValue.includes(v)) emit('update:modelValue', [...props.modelValue, v])
  draft.value = ''
}
function removeAt(i: number) {
  const next = [...props.modelValue]
  next.splice(i, 1)
  emit('update:modelValue', next)
}
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    commit()
  } else if (e.key === 'Backspace' && !draft.value && props.modelValue.length) {
    removeAt(props.modelValue.length - 1)
  }
}
</script>

<template>
  <div class="fi">
    <label>{{ label }}</label>
    <div class="tag-input">
      <span v-for="(rule, i) in modelValue" :key="i" class="tag">
        {{ rule }}
        <button type="button" @click="removeAt(i)">✕</button>
      </span>
      <input
        v-model="draft"
        :placeholder="modelValue.length ? '' : placeholder"
        spellcheck="false"
        @keydown="onKeydown"
        @blur="commit"
      />
    </div>
  </div>
</template>

<style scoped>
.fi label {
  display: block;
  font-size: 12.5px;
  color: var(--text-2);
  margin-bottom: 6px;
}
</style>
