import { createI18n } from 'vue-i18n'
import es from '@/locales/es.json'

export type MessageSchema = typeof es

export function createAppI18n() {
  return createI18n<[MessageSchema], 'es'>({
    legacy: false,
    locale: 'es',
    fallbackLocale: 'es',
    messages: { es },
    datetimeFormats: {
      es: {
        long: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' },
      },
    },
  })
}
