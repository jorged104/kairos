import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Portfolio, PortfolioRow } from '@/api/portfolio'
import type { Me } from '@/types/api'
import type { Project } from '@/types/work'
import { createAppI18n } from '@/lib/i18n'
import PortfolioPage from '@/pages/PortfolioPage.vue'
import { useAuthStore } from '@/stores/auth'
import PortfolioSettings from './PortfolioSettings.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const row = (id: string, name: string, plan: number, real: number, level?: PortfolioRow['figures']['level'], extra: Partial<PortfolioRow> = {}): PortfolioRow => ({
  id, identifier: id.toUpperCase(), name, lifecycle: 'activo', is_program: false, weight: 1,
  figures: level ? { has_plan: true, plan, real, gap: real - plan, level } : { has_plan: false, plan: 0, real, gap: 0 }, ...extra,
})
const portfolio: Portfolio = {
  today: '2026-03-06',
  programs: [{
    program: row('r', 'Raíz', 80, 60, 'late', { is_program: true }), projects: 3, below_plan: 2, hidden: 1,
    rows: [row('a', 'Alfa', 50, 52, 'on_track'), row('b', 'Beta', 90, 60, 'late'), row('c', 'Gamma', 50, 42, 'at_risk')],
  }],
  loose: { program: null, projects: 1, below_plan: 0, rows: [row('s', 'Suelto', 0, 10)] },
}

const names = (w: ReturnType<typeof mount>) => w.findAll('[data-testid=portfolio-row-link]').map((x) => x.text())

async function render(permissions: string[] = ['work:read']) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push('/portfolio')
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set({ id: 'me', permissions } as unknown as Me)
  const wrapper = mount(PortfolioPage, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return wrapper
}

describe('portafolio (spec 010)', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  beforeEach(() => {
    fetchMock = vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') return json({ id: 'p1', is_program: true, portfolio_weight: 5, effort: 3, version: 2 })
      if (url.startsWith('/api/portfolio')) return json(portfolio)
      if (url.includes('/meetings')) return json({ items: [], total: 0, page: 1, page_size: 3 })
      return json({})
    })
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('muestra los programas y los proyectos sueltos, los más atrasados primero (RF-22 a RF-30, RF-36)', async () => {
    const wrapper = await render()
    const cards = wrapper.findAll('[data-testid=portfolio-card]')
    expect(cards).toHaveLength(2)
    expect(cards[0]!.text()).toContain('3 proyectos · 2 bajo plan')
    // What the user does not see, counted (spec 023, RF-19).
    expect(cards[0]!.find('[data-testid=portfolio-hidden]').text()).toBe('1 proyecto que no ves')
    expect(cards[0]!.text()).toContain('Atrasado')
    expect(cards[1]!.text()).toContain('Sin programa')
    // By gap ascending; without a plan, last.
    expect(names(wrapper)).toEqual(['Beta', 'Gamma', 'Alfa', 'Suelto'])
    expect(wrapper.find('[data-testid=portfolio-row-link]').attributes('href')).toBe('/projects/b')
    expect(wrapper.find('[data-testid=portfolio-program-link]').attributes('href')).toBe('/projects/r')
  })

  it('filtra por semáforo, ordena y lo recuerda (RF-32 a RF-38)', async () => {
    let wrapper = await render()
    await wrapper.find('[data-testid=portfolio-filter-at_risk]').trigger('click')
    expect(names(wrapper)).toEqual(['Gamma'])
    expect(wrapper.findAll('[data-testid=portfolio-card]')).toHaveLength(1) // «Sin programa» has none left
    await wrapper.find('[data-testid=portfolio-filter-on_track]').trigger('click')
    await wrapper.find('[data-testid=sort-name]').trigger('click')
    expect(names(wrapper)).toEqual(['Alfa', 'Gamma'])
    wrapper.unmount()

    wrapper = await render()
    expect(names(wrapper)).toEqual(['Alfa', 'Gamma'])
    await wrapper.find('[data-testid=portfolio-filter-on_track]').trigger('click')
    await wrapper.find('[data-testid=portfolio-filter-at_risk]').trigger('click')
    await wrapper.find('[data-testid=portfolio-filter-ahead]').trigger('click')
    expect(wrapper.find('[data-testid=portfolio-empty]').text()).toContain('Ningún programa ni proyecto cumple el filtro')
    await wrapper.find('[data-testid=portfolio-clear]').trigger('click')
    expect(names(wrapper)).toHaveLength(4)
  })

  it('el administrador marca un programa y fija el peso (RF-1, RF-5, RF-6, RF-8)', async () => {
    const project = { id: 'p1', version: 1, is_template: false, lifecycle: 'activo', is_program: false, portfolio_weight: null, effort: 3 } as unknown as Project
    const wrapper = mount(PortfolioSettings, { props: { project }, global: { plugins: [createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
    expect(wrapper.find('[data-testid=portfolio-effort]').text()).toContain('Pesa su esfuerzo: 3')
    await wrapper.find('[data-testid=portfolio-is-program]').setValue(true)
    await wrapper.find('[data-testid=portfolio-weight]').setValue('5')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const put = fetchMock.mock.calls.find(([, i]) => (i as RequestInit | undefined)?.method === 'PUT')!
    expect(String(put[0])).toBe('/api/projects/p1/portfolio')
    expect(JSON.parse(String((put[1] as RequestInit).body))).toEqual({ version: 1, is_program: true, weight: 5 })
  })

  it('solo el administrador ve crear, añadir y quitar (RF-41, RF-44, RF-46)', async () => {
    const wrapper = await render()
    expect(wrapper.find('[data-testid=portfolio-new]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=portfolio-add-search]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=portfolio-remove]').exists()).toBe(false)
  })
})

describe('crear programas y asignar proyectos (spec 010, RF-41 a RF-48)', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  const project = (id: string, extra = {}) => ({
    id, version: 3, name: 'P', team: { id: 't1', name: 'Equipo' }, owner: { id: 'u1', name: 'Ana' }, parent: null,
    start_date: null, end_date: null, health: 'en_curso', health_explanation: '', custom_values: {}, ...extra,
  })
  beforeEach(() => {
    fetchMock = vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET'
      if (url.startsWith('/api/portfolio')) return json(portfolio)
      if (url.includes('/meetings')) return json({ items: [], total: 0, page: 1, page_size: 3 })
      if (url.startsWith('/api/teams')) return json({ items: [{ id: 't1', name: 'Equipo', parent_id: null, archived_at: null, leaders: [] }] })
      if (method === 'POST' && url === '/api/projects') return json(project('n1', { version: 1 }))
      if (method === 'PUT') return json(project('n1', { version: 2, is_program: true }))
      if (method === 'PATCH') return json(project(url.split('/').at(-1)!, { version: 4 }))
      if (url.startsWith('/api/projects?')) return json({ items: [{ id: 'r', identifier: 'R', name: 'Raíz' }, { id: 'a', identifier: 'A', name: 'Alfa' }, { id: 'x', identifier: 'X', name: 'Equis' }], total: 3, page: 1, page_size: 20 })
      if (url.startsWith('/api/projects/')) return json(project(url.split('/').at(-1)!))
      return json({ items: [] })
    })
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })
  const calls = (method: string) => fetchMock.mock.calls.filter(([, i]) => ((i as RequestInit | undefined)?.method ?? 'GET') === method)
  const body = (c: unknown[]) => JSON.parse(String((c[1] as RequestInit).body))

  it('crea un programa en blanco marcado como programa (RF-42, RF-43)', async () => {
    const wrapper = await render(['work:read', 'portfolio:manage', 'projects:create:all'])
    await wrapper.find('[data-testid=portfolio-new]').trigger('click')
    await wrapper.find('[data-testid=portfolio-new-identifier]').setValue('prg')
    await wrapper.find('[data-testid=portfolio-new-name]').setValue('Programa Nuevo')
    await flushPromises()
    await wrapper.find('[data-testid=portfolio-new-form]').trigger('submit')
    await flushPromises()
    expect(body(calls('POST')[0]!)).toEqual({ identifier: 'PRG', name: 'Programa Nuevo', team_id: 't1' })
    const put = calls('PUT')[0]!
    expect(String(put[0])).toBe('/api/projects/n1/portfolio')
    expect(body(put)).toEqual({ version: 1, is_program: true, weight: null })
    expect(wrapper.find('[data-testid=portfolio-new-form]').exists()).toBe(false)
  })

  it('añade un proyecto al programa y lo quita (RF-44 a RF-47)', async () => {
    const wrapper = await render(['work:read', 'portfolio:manage'])
    expect(wrapper.findAll('[data-testid=portfolio-add-search]')).toHaveLength(1) // not on «Sin programa»
    await wrapper.find('[data-testid=portfolio-add-search]').setValue('e')
    await new Promise((r) => setTimeout(r, 300))
    await flushPromises()
    // Neither the program nor its rows are offered.
    const found = wrapper.findAll('[data-testid=portfolio-add-found]')
    expect(found.map((f) => f.text())).toEqual(['X Equis'])
    await found[0]!.trigger('click')
    await flushPromises()
    let patch = calls('PATCH')[0]!
    expect(String(patch[0])).toBe('/api/projects/x')
    expect(body(patch)).toMatchObject({ version: 3, parent_id: 'r', team_id: 't1', owner_id: 'u1', name: 'P' })

    const cards = wrapper.findAll('[data-testid=portfolio-card]')
    expect(cards[1]!.find('[data-testid=portfolio-remove]').exists()).toBe(false)
    await cards[0]!.find('[data-testid=portfolio-remove]').trigger('click')
    await flushPromises()
    patch = calls('PATCH')[1]!
    expect(String(patch[0])).toBe('/api/projects/b')
    expect(body(patch).parent_id).toBeNull()
  })
})
