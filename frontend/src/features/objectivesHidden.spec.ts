import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

describe('objetivos ocultos', () => {
  it('sus rutas llevan al inicio', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('{}'))))
    setActivePinia(createPinia())
    useAuthStore().set({ id: 'u', name: 'A', roles: [], teams: [], permissions: ['work:read'] } as unknown as Me)
    const router = createAppRouter(createMemoryHistory())
    await router.push('/objectives')
    expect(router.currentRoute.value.name).toBe('home')
    await router.push('/objectives/x')
    expect(router.currentRoute.value.name).toBe('home')
    expect(router.hasRoute('objectives')).toBe(false)
    vi.unstubAllGlobals()
  })
})
