import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'

/**
 * Turns an API error into a catalog text by its code, never the API message
 * (RF-125). Unknown codes fall back to a generic text.
 */
export function useErrorMessage() {
  const { t, te } = useI18n()
  return (error: unknown): string => {
    const code = error instanceof ApiError ? error.code : typeof error === 'string' ? error : 'unknown'
    return te(`errors.${code}`) ? t(`errors.${code}`) : t('errors.unknown')
  }
}
