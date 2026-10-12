import { createI18n } from 'vue-i18n'
import { messages } from './lang'

export function makeI18n(locale: string) {
  return createI18n({
    legacy: false,
    locale: locale || 'zh-CN',
    fallbackLocale: 'zh-CN',
    messages,
  })
}
