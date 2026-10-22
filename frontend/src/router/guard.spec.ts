import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter, safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

const me: Me = {
  id: 'u1',
  name: 'Ana',
  email: 'ana@x.com',
  status: 'active',
  auth_method: 'local',
  has_password: true,
  roles: [{ role: 'member', source: 'manual' }],
  teams: [],
  permissions: ['directory:read', 'self:password'],
}

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status })
const unauthenticated = () => json(401, { error: { code: 'unauthenticated', message: 'x' } })

describe('guardia del router', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => vi.unstubAllGlobals())

  it('sin sesión redirige al login guardando la ruta original (RF-121)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(unauthenticated()))
    const router = createAppRouter(createMemoryHistory())
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/')
  })

  it('tras iniciar sesión vuelve a la ruta original (RF-122)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(unauthenticated()))
    const router = createAppRouter(createMemoryHistory())
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('login')

    useAuthStore().set(me)
    await router.push(safeRedirect(router.currentRoute.value.query.redirect))
    expect(router.currentRoute.value.name).toBe('status')
  })

  it('con sesión, el login lleva directamente a la ruta pedida', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(200, me)))
    const router = createAppRouter(createMemoryHistory())
    await router.push('/login?redirect=/')
    expect(router.currentRoute.value.name).toBe('status')
  })

  it('el registro es público', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(unauthenticated()))
    const router = createAppRouter(createMemoryHistory())
    await router.push('/register')
    expect(router.currentRoute.value.name).toBe('register')
  })

  it('solo acepta redirecciones internas', () => {
    expect(safeRedirect('/users?q=a')).toBe('/users?q=a')
    expect(safeRedirect('//evil.test')).toBe('/')
    expect(safeRedirect('https://evil.test')).toBe('/')
    expect(safeRedirect(undefined)).toBe('/')
  })
})
