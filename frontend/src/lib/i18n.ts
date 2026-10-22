import { createI18n } from 'vue-i18n'
import es from '@/locales/es.json'

export type MessageSchema = typeof es

export function createAppI18n() {
  return createI18n<[MessageSchema], 'es'>({
    legacy: false,
    locale: 'es',
    fallbackLocale: 'es',
    messages: { es },
  })
}
