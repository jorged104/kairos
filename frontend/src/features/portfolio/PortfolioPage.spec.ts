import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Portfolio, PortfolioRow } from '@/api/portfolio'
import type { Project } from '@/types/work'
import { createAppI18n } from '@/lib/i18n'
import PortfolioPage from '@/pages/PortfolioPage.vue'
import PortfolioSettings from './PortfolioSettings.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const row = (id: string, name: string, plan: number, real: number, level?: PortfolioRow['figures']['level'], extra: Partial<PortfolioRow> = {}): PortfolioRow => ({
  id, identifier: id.toUpperCase(), name, lifecycle: 'activo', is_program: false, weight: 1,
  figures: level ? { has_plan: true, plan, real, gap: real - plan, level } : { has_plan: false, plan: 0, real, gap: 0 }, ...extra,
})
const portfolio: Portfolio = {
  today: '2026-03-06',
  programs: [{
    program: row('r', 'Raíz', 80, 60, 'late', { is_program: true }), projects: 3, below_plan: 2,
    rows: [row('a', 'Alfa', 50, 52, 'on_track'), row('b', 'Beta', 90, 60, 'late'), row('c', 'Gamma', 50, 42, 'at_risk')],
  }],
  loose: { program: null, projects: 1, below_plan: 0, rows: [row('s', 'Suelto', 0, 10)] },
}

const names = (w: ReturnType<typeof mount>) => w.findAll('[data-testid=portfolio-row-link]').map((x) => x.text())

async function render() {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push('/portfolio')
  const wrapper = mount(PortfolioPage, { global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return wrapper
}

describe('portafolio (spec 010)', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  beforeEach(() => {
    fetchMock = vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') return json({ id: 'p1', is_program: true, portfolio_weight: 5, effort: 3, version: 2 })
      if (url.startsWith('/api/portfolio')) return json(portfolio)
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
})
