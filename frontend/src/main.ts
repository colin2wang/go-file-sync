import { createApp } from 'vue'
import App from './App.vue'
import { makeI18n } from './i18n'
import { api, isDesktop } from './backend'
import { store, applySettings } from './store'
import type { Settings } from './types'
import './style.css'

async function bootstrap() {
  let settings: Settings = { language: 'zh-CN', theme: 'light' }
  try {
    settings = await api.getSettings()
  } catch {
    /* 默认值兜底 */
  }
  document.documentElement.setAttribute('data-theme', settings.theme || 'light')
  const i18n = makeI18n(settings.language || 'zh-CN')
  ;(window as any).__i18n = i18n.global
  store.settings = settings
  store.demo = !isDesktop

  createApp(App).use(i18n).mount('#app')
  await applySettings(settings)
}

bootstrap()
