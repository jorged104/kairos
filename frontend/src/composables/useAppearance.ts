import { useQuery } from '@tanstack/vue-query'
import { onBeforeUnmount, ref, watch, watchEffect } from 'vue'
import { fetchAuthConfig } from '@/api/auth'
import { applyTheme, DEFAULT_THEME, isDark } from '@/lib/theme'
import { useAuthStore } from '@/stores/auth'

/**
 * Applies the appearance of the system, also on the login (spec 015, RF-45),
 * and the person's light or dark mode, or the system's without a session
 * (RF-48 to RF-51).
 */
export function useAppearance() {
  const auth = useAuthStore()
  const { data } = useQuery({ queryKey: ['auth', 'config'], queryFn: fetchAuthConfig, staleTime: 60_000 })
  watch(
    () => data.value?.theme,
    (theme) => applyTheme(theme ?? DEFAULT_THEME),
    { immediate: true },
  )

  const media = typeof window !== 'undefined' && window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null
  const prefersDark = ref(media?.matches ?? false)
  const onChange = (e: MediaQueryListEvent) => (prefersDark.value = e.matches)
  media?.addEventListener?.('change', onChange)
  onBeforeUnmount(() => media?.removeEventListener?.('change', onChange))

  watchEffect(() => {
    document.documentElement.classList.toggle('dark', isDark(auth.me?.theme_mode ?? 'system', prefersDark.value))
  })
}
