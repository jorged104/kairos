import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { createAppI18n } from '@/lib/i18n'
import { useAuthStore } from '@/stores/auth'
import type { Me, Permission } from '@/types/api'

const me = (permissions: Permission[]) => ({ id: 'u', name: 'Ana Ríos', email: 'ana@x.com', roles: [], teams: [], permissions }) as unknown as Me

beforeEach(() => localStorage.clear())

async function render(permissions: Permission[], path = '/') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(me(permissions))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: AppLayout, children: [{ path: ':p(.*)*', component: { template: '<p>x</p>' } }] }],
  })
  await router.push(path)
  const w = mount(RouterView, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return { w, router }
}
const labels = (w: Awaited<ReturnType<typeof render>>['w'], sel: string) => w.find(sel).findAll('[data-testid=nav-link]').map((l) => l.text())

describe('navegación lateral', () => {
  it('separa el trabajo de la configuración, y esta muestra la administración según permisos', async () => {
    const { w } = await render(['work:read'])
    const all = w.findAll('[data-testid=nav-link]').map((l) => l.text())
    expect(all.slice(0, 4)).toEqual(['Mi panel', 'Proyectos', 'Portafolio', 'Seguimiento'])
    expect(labels(w, '[data-testid=nav-settings]')).toEqual(['Mi cuenta', 'Usuarios', 'Equipos', 'Estado'])

    const admin = await render(['work:read', 'attributes:manage', 'idp_mappings:manage', 'catalog:manage', 'audit:read'])
    expect(labels(admin.w, '[data-testid=nav-settings]')).toEqual([
      'Mi cuenta', 'Usuarios', 'Equipos', 'Atributos', 'Homologaciones', 'Catálogo', 'Auditoría', 'Estado',
    ])
  })

  it('marca la sección actual, también en sus páginas hijas', async () => {
    const { w } = await render(['work:read'], '/work-items/ALF-1')
    const current = w.findAll('[aria-current=page]')
    expect(current.map((c) => c.text())).toEqual(['Proyectos'])
  })

  it('pliega la configuración y el menú, y lo recuerda', async () => {
    const { w } = await render(['work:read'])
    const toggle = w.find('[data-testid=nav-settings-toggle]')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(localStorage.getItem('kairos.nav.settings')).toBe('0')

    await w.find('[data-testid=nav-collapse]').trigger('click')
    expect(localStorage.getItem('kairos.nav.collapsed')).toBe('1')
    expect(w.find('[data-testid=sidebar]').classes()).toContain('lg:w-16')
  })

  it('en pantallas pequeñas se abre como cajón y se cierra al navegar', async () => {
    const { w, router } = await render(['work:read'])
    const side = w.find('[data-testid=sidebar]')
    expect(side.classes()).toContain('-translate-x-full')
    await w.find('[data-testid=nav-open]').trigger('click')
    expect(side.classes()).toContain('translate-x-0')
    await router.push('/projects')
    await flushPromises()
    expect(side.classes()).toContain('-translate-x-full')
  })
})
