<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { store, applySettings } from '../store'
import { api, isDesktop } from '../backend'

const { t, locale } = useI18n()

async function setLanguage(lang: string) {
  store.settings.language = lang
  locale.value = lang
  ;(window as any).__i18n.locale = lang
  await api.saveSettings(store.settings)
  await applySettings(store.settings)
}

async function setTheme(theme: 'dark' | 'light') {
  store.settings.theme = theme
  await api.saveSettings(store.settings)
  await applySettings(store.settings)
}

async function openConfigDir() {
  const dir = await api.configDir()
  if (isDesktop) {
    await api.openInExplorer(dir)
  } else {
    alert(dir)
  }
}
</script>

<template>
  <div class="page">
    <h2>{{ $t('settings.title') }}</h2>

    <div class="card sec">
      <div class="sec-title">{{ $t('settings.language') }}</div>
      <div class="opt-row">
        <button
          class="btn"
          :class="{ 'opt-active': store.settings.language === 'zh-CN' }"
          @click="setLanguage('zh-CN')"
        >
          🇨🇳 中文
        </button>
        <button
          class="btn"
          :class="{ 'opt-active': store.settings.language === 'en-US' }"
          @click="setLanguage('en-US')"
        >
          🇺🇸 English
        </button>
      </div>
    </div>

    <div class="card sec">
      <div class="sec-title">{{ $t('settings.theme') }}</div>
      <div class="opt-row">
        <button
          class="btn"
          :class="{ 'opt-active': store.settings.theme === 'light' }"
          @click="setTheme('light')"
        >
          ☀ {{ $t('settings.light') }}
        </button>
        <button
          class="btn"
          :class="{ 'opt-active': store.settings.theme === 'dark' }"
          @click="setTheme('dark')"
        >
          ☾ {{ $t('settings.dark') }}
        </button>
      </div>
    </div>

    <div class="card sec">
      <div class="sec-title">{{ $t('settings.configDir') }}</div>
      <p class="sec-hint">{{ $t('settings.configDirHint') }}</p>
      <div class="opt-row">
        <button class="btn" @click="openConfigDir">📁 {{ $t('settings.openDir') }}</button>
      </div>
    </div>

    <div class="card sec">
      <div class="sec-title">{{ $t('settings.about') }}</div>
      <p class="sec-hint">{{ $t('settings.aboutText') }}</p>
    </div>
  </div>
</template>

<style scoped>
.page { max-width: 720px; margin: 0 auto; padding: 26px 30px 60px; }
h2 { font-size: 19px; margin-bottom: 18px; }
.sec { padding: 16px 20px; margin-bottom: 14px; }
.sec-title { font-weight: 600; font-size: 14px; margin-bottom: 10px; }
.sec-hint { font-size: 12.5px; color: var(--text-3); margin: 0 0 10px; line-height: 1.6; }
.opt-row { display: flex; gap: 8px; flex-wrap: wrap; }
.opt-active {
  border-color: var(--primary) !important;
  color: var(--primary);
  background: var(--primary-weak);
  font-weight: 600;
}
</style>
