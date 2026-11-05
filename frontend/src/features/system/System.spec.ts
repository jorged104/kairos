import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { SystemSettings } from '@/api/system'
import { createAppI18n } from '@/lib/i18n'
import AccountPage from '@/pages/AccountPage.vue'
import LoginPage from '@/pages/LoginPage.vue'
import SystemPage from '@/pages/SystemPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const settings = (extra: Partial<SystemSettings> = {}): SystemSettings => ({
  version: 3,
  access: { mode: 'both', registration_enabled: false, default_role: 'member' },
  sso: {
    issuer: 'https://idp.x/realms/k', discovery_url: '', client_id: 'kairos', client_secret_set: true,
    redirect_url: 'https://kairos.x/api/auth/oidc/callback', configured: true, test: null, tested: false,
  },
  storage: { kind: 'db', endpoint: '', bucket: '', region: '', access_key: '', secret_key_set: false, use_ssl: true, files_count: 0, locked: false },
  appearance: { palette: 'neutral', primary: 'default', radius: 'soft' },
  emergency_available: false,
  emergency_email: '',
  ...extra,
})
const me = { id: 'u1', name: 'Ana', email: 'a@x.com', roles: [], teams: [], permissions: ['system:manage'], theme_mode: 'system' } as unknown as Me

type Handler = (url: string, init?: RequestInit) => Promise<Response> | undefined
let handler: Handler
let current: SystemSettings
beforeEach(() => {
  current = settings()
  handler = () => undefined
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => handler(url, init) ?? defaults(url, init)))
})
afterEach(() => vi.unstubAllGlobals())

function defaults(url: string, init?: RequestInit) {
  if (url.startsWith('/api/system/settings')) {
    if (init?.method === 'PUT') return json(current)
    return json(current)
  }
  if (url.startsWith('/api/auth/config')) return json({ local_enabled: false, oidc_enabled: true, registration_enabled: false, emergency_enabled: true })
  if (url.startsWith('/api/me/preferences')) return json(undefined, 204)
  if (url.startsWith('/api/me/mcp-connections')) return json({ enabled: false, max: 10, items: [] })
  if (url.startsWith('/api/me/points/movements')) return json({ items: [], total: 0, page: 1, page_size: 50 })
  if (url.startsWith('/api/me/points')) return json({ xp: 0, coins: 0, level: { id: 'l1', number: 1, name: 'Nivel 1', min_xp: 0 }, next: null, last_seq: 0 })
  return json({})
}
const bodyOf = (prefix: string, method = 'PUT') => {
  const call = vi.mocked(fetch).mock.calls.find(([u, i]) => String(u).startsWith(prefix) && (i as RequestInit | undefined)?.method === method)
  return call ? JSON.parse(String((call[1] as RequestInit).body)) : undefined
}

async function render(component: unknown, path = '/admin/system') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(me)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: component as never }] })
  await router.push(path)
  const w = mount({ template: '<RouterView />' }, {
    global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return { w, router }
}

describe('sistema: acceso (spec 015)', () => {
  it('guarda el modo, el registro y el rol por defecto (RF-11 a RF-16)', async () => {
    const { w } = await render(SystemPage)
    expect(w.findAll('[data-testid=default-role] option, select[data-testid=default-role] option').map((o) => o.text())).not.toContain('Administrador')
    await w.find('[data-testid=mode-local]').setValue(true)
    await w.find('[data-testid=registration]').setValue(true)
    await w.find('[data-testid=access-settings]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/system/settings/access')).toEqual({ mode: 'local', registration_enabled: true, default_role: 'member', accept_no_emergency: false })
  })

  it('avisa de que hace falta el login de prueba, y pide confirmación sin acceso de emergencia (RF-13, RF-23)', async () => {
    let calls = 0
    handler = (url, init) => {
      if (url.startsWith('/api/system/settings/access') && init?.method === 'PUT') {
        calls++
        return calls === 1 ? json({ error: { code: 'no_emergency_access', message: 'x' } }, 409) : json(current)
      }
    }
    const { w } = await render(SystemPage)
    await w.find('[data-testid=mode-oidc]').setValue(true)
    expect(w.find('[data-testid=mode-needs-test]').exists()).toBe(true)
    await w.find('[data-testid=access-settings]').trigger('submit')
    await flushPromises()
    expect(w.find('[data-testid=no-emergency]').exists()).toBe(true)
    expect(w.find('[data-testid=access-save]').attributes('disabled')).toBeDefined()
    await w.find('[data-testid=accept-no-emergency]').setValue(true)
    await w.find('[data-testid=access-settings]').trigger('submit')
    await flushPromises()
    const puts = vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).startsWith('/api/system/settings/access') && (i as RequestInit).method === 'PUT')
    expect(JSON.parse(String((puts[1]![1] as RequestInit).body)).accept_no_emergency).toBe(true)
  })
})

describe('sistema: SSO (spec 015)', () => {
  it('muestra la URL de retorno, conserva el secreto vacío y enseña el error (RF-7, RF-25, RF-27)', async () => {
    handler = (url, init) => {
      if (url.startsWith('/api/system/settings/sso') && init?.method === 'PUT')
        return json({ error: { code: 'sso_discovery_failed', message: 'x', details: { reason: 'timeout' } } }, 400)
    }
    const { w } = await render(SystemPage, '/admin/system?tab=sso')
    expect((w.find('[data-testid=sso-redirect]').element as HTMLInputElement).value).toBe('https://kairos.x/api/auth/oidc/callback')
    expect(w.find('[data-testid=sso-client-secret]').attributes('placeholder')).toContain('Déjalo vacío')
    await w.find('[data-testid=sso-issuer]').setValue('https://otro.x')
    await w.find('[data-testid=sso-settings] form').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/system/settings/sso')).toEqual({ issuer: 'https://otro.x', discovery_url: '', client_id: 'kairos', client_secret: '' })
    expect(w.find('[data-testid=sso-error]').text()).toContain('no responde')
  })

  it('bloquea el SSO mientras es el único acceso y muestra la última prueba (RF-29, RF-31, RF-32)', async () => {
    current = settings({
      access: { mode: 'oidc', registration_enabled: false, default_role: 'member' },
      sso: { ...settings().sso, tested: false, test: { ok: true, email: 'ana@idp.x', name: 'Ana', at: '2026-11-04T10:00:00Z' } },
    })
    const { w } = await render(SystemPage, '/admin/system?tab=sso')
    expect(w.find('[data-testid=sso-in-use]').exists()).toBe(true)
    expect(w.find('[data-testid=sso-settings] fieldset').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid=sso-test-ok]').text()).toContain('ana@idp.x')
    expect(w.find('[data-testid=sso-test-ok]').text()).toContain('repítela')
  })
})

describe('sistema: almacenamiento (spec 015)', () => {
  it('pide los datos del S3 y deja fijo el sitio cuando hay archivos (RF-34, RF-37, RF-38)', async () => {
    current = settings({
      storage: { kind: 's3', endpoint: 's3.x:9000', bucket: 'k', region: 'eu', access_key: 'AK', secret_key_set: true, use_ssl: true, files_count: 3, locked: true },
    })
    const { w } = await render(SystemPage, '/admin/system?tab=storage')
    expect(w.find('[data-testid=storage-locked]').text()).toContain('3 archivos')
    expect(w.find('[data-testid=s3-bucket]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid=s3-access-key]').attributes('disabled')).toBeUndefined()
    await w.find('[data-testid=s3-region]').setValue('us')
    await w.find('[data-testid=storage-settings]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/system/settings/storage')).toEqual({ kind: 's3', endpoint: 's3.x:9000', bucket: 'k', region: 'us', access_key: 'AK', secret_key: '', use_ssl: true })
  })
})

describe('sistema: apariencia (spec 015)', () => {
  it('previsualiza en claro y oscuro, guarda y restablece (RF-39 a RF-47)', async () => {
    const { w } = await render(SystemPage, '/admin/system?tab=appearance')
    await w.find('[data-testid=palette-zinc]').trigger('click')
    await w.find('[data-testid=primary-violet]').trigger('click')
    await w.find('[data-testid=radius-round]').trigger('click')
    const light = w.find('[data-testid=preview-light]').attributes('style')
    const dark = w.find('[data-testid=preview-dark]').attributes('style')
    expect(light).toContain('--radius: 1rem')
    expect(light).not.toBe(dark)
    expect(w.find('[data-testid=preview-dark]').classes()).toContain('dark')
    await w.find('[data-testid=appearance-save]').trigger('click')
    await flushPromises()
    expect(bodyOf('/api/system/settings/appearance')).toEqual({ palette: 'zinc', primary: 'violet', radius: 'round' })
    vi.mocked(fetch).mockClear()
    await w.find('[data-testid=appearance-reset]').trigger('click')
    await flushPromises()
    expect(bodyOf('/api/system/settings/appearance')).toEqual({ palette: 'neutral', primary: 'default', radius: 'soft' })
  })
})

describe('modo claro u oscuro y acceso de emergencia (spec 015)', () => {
  it('cada persona elige claro, oscuro o el del sistema en Mi cuenta (RF-48 a RF-50)', async () => {
    const { w } = await render(AccountPage, '/account')
    expect(w.find('[data-testid=theme-mode-system]').attributes('aria-pressed')).toBe('true')
    await w.find('[data-testid=theme-mode-dark]').trigger('click')
    await flushPromises()
    expect(bodyOf('/api/me/preferences')).toEqual({ theme_mode: 'dark' })
    expect(useAuthStore().me?.theme_mode).toBe('dark')
  })

  it('con solo SSO, el login ofrece el acceso de emergencia (RF-19, RF-20)', async () => {
    const { w } = await render(LoginPage, '/login')
    expect(w.find('[data-testid=local-login]').exists()).toBe(false)
    await w.find('[data-testid=emergency-link]').trigger('click')
    expect(w.find('[data-testid=local-login]').exists()).toBe(true)
    expect(w.find('[data-testid=emergency-hint]').exists()).toBe(true)
  })
})
