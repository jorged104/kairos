import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createAppI18n } from '@/lib/i18n'
import es from '@/locales/es.json'
import LoginPage from './LoginPage.vue'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status })

async function render(path: string, config: Record<string, boolean>, loginResponse?: Response) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(url === '/api/auth/config' ? json(200, config) : (loginResponse ?? json(500, {}))),
    ),
  )
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: LoginPage },
      { path: '/register', name: 'register', component: { template: '<div />' } },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  const wrapper = mount(LoginPage, {
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
  })
  await flushPromises()
  return { wrapper, router }
}

describe('LoginPage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('muestra solo los métodos habilitados (RF-6, RF-12)', async () => {
    const local = await render('/login', { local_enabled: true, oidc_enabled: false, registration_enabled: false })
    expect(local.wrapper.find('[data-testid=local-login]').exists()).toBe(true)
    expect(local.wrapper.find('[data-testid=sso-login]').exists()).toBe(false)
    expect(local.wrapper.find('[data-testid=register-link]').exists()).toBe(false)

    const oidc = await render('/login', { local_enabled: false, oidc_enabled: true, registration_enabled: false })
    expect(oidc.wrapper.find('[data-testid=local-login]').exists()).toBe(false)
    expect(oidc.wrapper.find('[data-testid=sso-login]').exists()).toBe(true)

    const both = await render('/login', { local_enabled: true, oidc_enabled: true, registration_enabled: true })
    expect(both.wrapper.find('[data-testid=local-login]').exists()).toBe(true)
    expect(both.wrapper.find('[data-testid=sso-login]').exists()).toBe(true)
    expect(both.wrapper.find('[data-testid=register-link]').exists()).toBe(true)
  })

  it('traduce el código de error de la API, no su mensaje (RF-125)', async () => {
    const { wrapper } = await render(
      '/login',
      { local_enabled: true, oidc_enabled: false, registration_enabled: false },
      json(401, { error: { code: 'invalid_credentials', message: 'mensaje técnico' } }),
    )
    await wrapper.find('#email').setValue('a@x.com')
    await wrapper.find('#password').setValue('una-contrasena-larga')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const text = wrapper.get('[data-testid=login-error]').text()
    expect(text).toBe(es.errors.invalid_credentials)
    expect(text).not.toContain('mensaje técnico')
  })

  it('traduce el error que devuelve el login SSO en la URL', async () => {
    const { wrapper } = await render('/login?error=idp_unavailable', { local_enabled: false, oidc_enabled: true, registration_enabled: false })
    expect(wrapper.get('[data-testid=login-error]').text()).toBe(es.errors.idp_unavailable)
  })

  it('usa un texto genérico para códigos desconocidos', async () => {
    const { wrapper } = await render('/login?error=algo_raro', { local_enabled: true, oidc_enabled: false, registration_enabled: false })
    expect(wrapper.get('[data-testid=login-error]').text()).toBe(es.errors.unknown)
  })

  it('tras entrar vuelve a la ruta original (RF-122)', async () => {
    const me = { id: 'u1', name: 'Ana', permissions: [], roles: [], teams: [] }
    const { wrapper, router } = await render(
      '/login?redirect=/users%3Fq%3Dana',
      { local_enabled: true, oidc_enabled: false, registration_enabled: false },
      json(200, me),
    )
    await wrapper.find('#email').setValue('a@x.com')
    await wrapper.find('#password').setValue('una-contrasena-larga')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/users?q=ana')
  })
})
