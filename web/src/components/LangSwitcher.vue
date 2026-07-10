<template>
  <div class="lang-switcher">
    <button
      v-for="lang in languages"
      :key="lang.code"
      :class="{ active: currentLang === lang.code }"
      @click="setLang(lang.code)"
    >
      {{ lang.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { inject } from 'vue'

const languages = [
  { code: 'zh', label: '中文' },
  { code: 'en', label: 'EN' },
  { code: 'ja', label: '日本語' }
]

const currentLang = inject<import('vue').Ref<string>>('lang')!

function setLang(code: string) {
  currentLang.value = code
  localStorage.setItem('lang', code)
}
</script>

<style scoped>
.lang-switcher {
  display: flex;
  gap: 4px;
}

button {
  padding: 4px 10px;
  border: 1px solid #253341;
  background: transparent;
  color: #8899a6;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
}

button:hover {
  background: #253341;
  color: #e1e8ed;
}

button.active {
  background: #1da1f2;
  color: white;
  border-color: #1da1f2;
}
</style>
