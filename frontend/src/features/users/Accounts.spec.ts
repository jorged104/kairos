import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { SystemSettings } from '@/api/system'
import MailSettings from '@/features/system/MailSettings.vue'
import { createAppI18n } from '@/lib/i18n'
import ForgotPasswordPage from '@/pages/ForgotPasswordPage.vue'
import PasswordPage from '@/pages/PasswordPage.vue'
import UserDetailPage from '@/pages/UserDetailPage.vue'
import UserNewPage from '@/pages/UserNewPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me, User } from '@/types/api'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const me = { id: 'admin', name: 'Admin', email: 'a@x.com', roles: [{ role: 'admin', source: 'manual' }], teams: [], permissions: ['users:create:all', 'passwords:set', 'system:manage'] } as unknown as Me
const ana = (extra: Partial<User> = {}): User => ({
  id: 'u1', name: 'Ana', status: 'active', teams: [], full_profile: true, email: 'ana@x.com', roles: [], attributes: {}, has_password: false,
  invitation: { expires_at: '2030-01-10T10:00:00Z', expired: false },
  can: { edit: true, set_status: true, offboard: true, assign_roles: true, manage_teams: true, invite: true, password_link: true },
  ...extra,
})

let handler: (url: string, init?: RequestInit) => Promise<Response> | undefined
beforeEach(() => {
  setActivePinia(createPinia())
  handler = () => undefined
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    const own = handler(url, init)
    if (own) return own
    if (url.startsWith('/api/teams')) return json({ items: [] })
    if (url.startsWith('/api/attributes')) return json({ items: [] })
    if (url.startsWith('/api/users/u1/invitation')) return json({ sent: false, url: 'https://kairos.x/password#t=abc', kind: 'password', expires_at: '2030-01-10T10:00:00Z' })
    if (url.startsWith('/api/users/u1/password-link')) return json({ sent: true, kind: 'password' })
    if (url.startsWith('/api/users/u1')) return json(ana())
    if (url.startsWith('/api/users') && init?.method === 'POST') return json(ana({ delivery: { sent: false, url: 'https://kairos.x/password#t=new', kind: 'password' } }), 201)
    return json({})
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
  history.replaceState(null, '', '/')
})
const callsTo = (part: string, method = 'GET') =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).includes(part) && ((i as RequestInit | undefined)?.method ?? 'GET') === method)
const bodyOf = (part: string, method: string) => JSON.parse(String((callsTo(part, method)[0]![1] as RequestInit).body))

async function render(component: unknown, props: Record<string, unknown> = {}, path = '/') {
  useAuthStore().set(me)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', name: 'login', component: { template: '<div />' } }] })
  await router.push(path)
  const w = mount(component as never, {
    props,
    global: { plugins: [router, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return { w, router }
}

describe('alta e invitación (spec 019)', () => {
  it('da de alta sin contraseña y muestra el enlace si el correo no salió (RF-10, RF-13)', async () => {
    const { w } = await render(UserNewPage)
    expect(w.find('input[type=password]').exists()).toBe(false)
    await w.find('#name').setValue('Ana')
    await w.find('#email').setValue('ana@x.com')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/users', 'POST')).not.toHaveProperty('password')
    expect((w.find('[data-testid=delivery-url]').element as HTMLInputElement).value).toBe('https://kairos.x/password#t=new')
  })

  it('en la ficha: invitación pendiente, reenviar y enlace de nueva contraseña (RF-19, RF-20, RF-32)', async () => {
    const { w } = await render(UserDetailPage, { id: 'u1' })
    expect(w.find('[data-testid=user-invitation]').text()).toContain('Invitación pendiente')
    await w.find('[data-testid=user-resend]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/users/u1/invitation', 'POST')).toHaveLength(1)
    expect(w.find('[data-testid=delivery-link]').text()).toContain('vale 7 días')
    await w.find('[data-testid=user-password-link]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=delivery-sent]').text()).toContain('Vale 1 hora')
  })
})

describe('recuperar la contraseña (spec 019)', () => {
  it('pide el enlace y responde igual exista o no la cuenta (RF-25)', async () => {
    const { w } = await render(ForgotPasswordPage)
    await w.find('[data-testid=forgot-email]').setValue('ana@x.com')
    await w.find('[data-testid=forgot-form]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/auth/password/forgot', 'POST')).toEqual({ email: 'ana@x.com' })
    expect(w.find('[data-testid=forgot-done]').text()).toContain('Si esa dirección tiene cuenta')
  })

  it('lee el enlace del fragmento, lo quita de la barra y guarda la contraseña (RF-16, RF-29, RF-39)', async () => {
    history.replaceState(null, '', '/password#t=secreto123')
    handler = (url) => (url.startsWith('/api/auth/password/check') ? json({ purpose: 'invite', email: 'ana@x.com', name: 'Ana' }) : undefined)
    const { w, router } = await render(PasswordPage)
    expect(bodyOf('/api/auth/password/check', 'POST')).toEqual({ token: 'secreto123' })
    expect(location.hash).toBe('')
    expect(w.text()).toContain('Crea tu contraseña')
    await w.find('[data-testid=password-new]').setValue('una-contrasena-larga')
    await w.find('[data-testid=password-repeat]').setValue('otra-distinta-larga')
    await w.find('[data-testid=password-form]').trigger('submit')
    await flushPromises()
    expect(callsTo('/api/auth/password/reset', 'POST')).toHaveLength(0)
    expect(w.text()).toContain('no coinciden')
    await w.find('[data-testid=password-repeat]').setValue('una-contrasena-larga')
    await w.find('[data-testid=password-form]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/auth/password/reset', 'POST')).toEqual({ token: 'secreto123', password: 'una-contrasena-larga' })
    expect(router.currentRoute.value.query.password).toBe('set')
  })

  it('un enlace no válido lo dice y ofrece pedir otro (RF-18, RF-30)', async () => {
    history.replaceState(null, '', '/password#t=viejo')
    handler = (url) => (url.startsWith('/api/auth/password/check') ? json({ error: { code: 'invalid_link', message: 'x' } }, 400) : undefined)
    const { w } = await render(PasswordPage)
    expect(w.find('[data-testid=link-invalid]').text()).toContain('no es válido')
    expect(w.find('[data-testid=password-form]').exists()).toBe(false)
  })
})

describe('correo en Sistema (spec 019)', () => {
  const settings = {
    mail: { enabled: true, host: 'smtp.x', port: 587, security: 'starttls', username: 'k', password_set: true, from_email: 'kairos@x.com', from_name: 'Kairos', ready: true },
  } as unknown as SystemSettings

  it('guarda sin reenviar la contraseña y dice en qué paso falló la prueba (RF-2, RF-4, RF-6, RF-7)', async () => {
    handler = (url, init) => {
      if (url.startsWith('/api/system/settings/mail/test')) return json({ error: { code: 'mail_test_failed', message: 'x', details: { step: 'auth' } } }, 400)
      if (url.startsWith('/api/system/settings/mail') && init?.method === 'PUT') return json(settings)
    }
    const { w } = await render(MailSettings, { settings })
    expect(w.find('[data-testid=mail-password]').attributes('placeholder')).toContain('Déjalo vacío')
    await w.find('[data-testid=mail-host]').setValue('smtp.y')
    await w.find('[data-testid=mail-settings]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/system/settings/mail', 'PUT')).toMatchObject({ host: 'smtp.y', port: 587, password: '', from_email: 'kairos@x.com' })
    await w.find('[data-testid=mail-test]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=mail-test-failed]').text()).toContain('autenticación')
  })
})
