<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ yaml: string; error?: string }>()
const { t } = useI18n()
const copied = ref(false)

// mini YAML 高亮：注释 / 键 / 字符串 / 布尔与数字
const html = computed(() => {
  const esc = props.yaml
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return esc.replace(/(#[^\n]*)|(^[\w.-]+:)|('[^']*'|"[^"]*")|\b(true|false|null)\b|(- .*)$/gm, (m, c, k, s, b) => {
    if (c) return `<span class="yc">${c}</span>`
    if (k) return `<span class="yk">${k}</span>`
    if (s) return `<span class="ys">${s}</span>`
    if (b) return `<span class="yb">${b}</span>`
    return m
  })
})

async function copy() {
  try {
    await navigator.clipboard.writeText(props.yaml)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = props.yaml
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    ta.remove()
  }
  copied.value = true
  setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="yp">
    <div class="yp-bar">
      <span class="yp-title">YAML</span>
      <span v-if="error" class="yp-err">{{ error }}</span>
      <span style="flex: 1"></span>
      <button class="btn sm" @click="copy">{{ copied ? $t('common.copied') : $t('common.copy') }}</button>
      <slot name="extra"></slot>
    </div>
    <div class="yaml-view" v-html="html"></div>
  </div>
</template>

<style scoped>
.yp {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 8px;
}
.yp-bar {
  display: flex;
  align-items: center;
  gap: 10px;
}
.yp-title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1.5px;
  color: var(--text-3);
}
.yp-err {
  color: var(--err);
  font-size: 12px;
}
</style>
