import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import type { Tracking } from '@/api/tracking'
import { createAppI18n } from '@/lib/i18n'
import TrackingPage from '@/pages/TrackingPage.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const fig = (plan: number, real: number, level?: 'late' | 'on_track') => ({ has_plan: !!level, plan, real, gap: real - plan, level })
const task = (id: string, key: string, extra = {}) => ({ id, key, title: `Tarea ${key}`, project_id: 'p1', parent_title: 'Padre', ratio: 40, start_date: null, due_date: '2026-03-05', closed_on: null, ...extra })
const tracking: Tracking = {
  today: '2026-03-10', matched: 52, mine: true,
  projects: [{
    id: 'p1', identifier: 'ALF', name: 'Alfa', owner: { id: 'u1', name: 'Ana' }, figures: fig(60, 40, 'late'),
    milestones: [
      { id: 'm1', name: 'Entrega', due_date: '2026-03-08', status: 'abierto', bar_start: '2026-03-01', bar_end: '2026-03-20', figures: fig(60, 40, 'late'), state: 'late' },
      { id: 'm2', name: 'Cierre', due_date: '2026-04-30', status: 'abierto', bar_start: null, bar_end: null, figures: fig(0, 0), state: 'no_data' },
    ],
  }],
  summary: { projects: 1, milestones: 2, met: 0, figures: fig(60, 40, 'late') },
  columns: {
    late: { total: 101, items: [task('w1', 'ALF-1', { late_days: 5 })] },
    in_progress: { total: 0, items: [] },
    done: { total: 1, items: [task('w2', 'ALF-2', { closed_on: '2026-03-09' })] },
    upcoming: { total: 0, items: [] },
  },
}

let fetchMock: ReturnType<typeof vi.fn>
beforeEach(() => {
  fetchMock = vi.fn((url: string) => {
    if (url.startsWith('/api/tracking')) return json(tracking)
    if (url.startsWith('/api/portfolio')) return json({ today: '2026-03-10', programs: [{ program: { id: 'pr1', name: 'Programa Uno' }, projects: 0, below_plan: 0, rows: [] }], loose: { program: null, projects: 0, below_plan: 0, rows: [] } })
    if (url.startsWith('/api/users')) return json({ items: [{ id: 'u1', name: 'Ana' }], total: 1 })
    return json({ items: [] })
  })
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => vi.unstubAllGlobals())

async function render(path = '/tracking'): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/tracking', component: TrackingPage }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return { wrapper, router }
}
const lastTracking = () => String(fetchMock.mock.calls.map(([u]) => String(u)).filter((u) => u.startsWith('/api/tracking')).at(-1))

describe('seguimiento (spec 011)', () => {
  it('muestra indicadores, calendario y columnas de mis proyectos (RF-6, RF-9, RF-11 a RF-42)', async () => {
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid=tracking-default]').text()).toContain('tus proyectos')
    expect(wrapper.find('[data-testid=tracking-limit]').text()).toContain('51 más quedan fuera')
    expect(wrapper.find('[data-testid=tracking-summary]').text()).toContain('Atrasado')
    const ms = wrapper.findAll('[data-testid=tracking-milestone]')
    expect(ms.map((m) => m.attributes('data-state'))).toEqual(['late', 'no_data'])
    expect(ms[0]!.find('[data-testid=tracking-bar]').exists()).toBe(true)
    expect(ms[0]!.find('[data-testid=tracking-target]').exists()).toBe(true)
    expect(ms[1]!.find('[data-testid=tracking-bar]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=tracking-legend]').text()).toContain('En ejecución')
    const late = wrapper.find('[data-testid=tracking-col-late]')
    expect(late.find('[data-testid=tracking-col-total]').text()).toBe('101')
    expect(late.text()).toContain('5 días de atraso')
    expect(late.text()).toContain('Alfa · Padre')
    expect(late.find('[data-testid=tracking-more]').text()).toContain('100 más')
    expect(late.find('[data-testid=tracking-task]').attributes('href')).toBe('/work-items/ALF-1')
    expect(ms[0]!.find('a').attributes('href')).toBe('/milestones/m1')
  })

  it('pone los filtros en la dirección y los manda a la API (RF-2 a RF-8)', async () => {
    const { wrapper, router } = await render('/tracking?owner_id=u1')
    expect(lastTracking()).toContain('owner_id=u1')
    expect(wrapper.text()).toContain('Responsable: Ana')
    await wrapper.find('[data-testid=tracking-program] ').setValue('pr1')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ program_id: 'pr1', owner_id: ['u1'] })
    expect(lastTracking()).toContain('program_id=pr1')
    await wrapper.find('[data-testid=tracking-clear]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({})
  })
})
