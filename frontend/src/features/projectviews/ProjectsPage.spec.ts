import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import type { Board, BoardProject, ListView } from '@/api/projectBoard'
import { createAppI18n } from '@/lib/i18n'
import ProjectsPage from '@/pages/ProjectsPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))
const status = (id: string, name: string, extra = {}) => ({ id, name, color: '#123456', position: 1, is_closed: false, is_initial: false, is_closing: false, retired: false, ...extra })
const project = (id: string, extra: Partial<BoardProject> = {}): BoardProject => ({
  id, identifier: id.toUpperCase(), name: `Proyecto ${id}`, status_id: 's1', lifecycle: 'activo', health: 'en_curso',
  team: { id: 't1', name: 'Equipo' }, owner: { id: 'u1', name: 'Ana Ríos' }, is_program: false, program: null, program_ids: [],
  start_date: '2026-03-01', end_date: '2026-03-31', base_start: null, base_end: null, version: 3, can_manage: true,
  figures: { has_plan: true, plan: 50, real: 30, gap: -20, level: 'late' }, milestones: [], ...extra,
})
const board: Board = {
  today: '2026-03-10',
  projects: [
    project('a', { program: { id: 'pr', name: 'Programa' }, program_ids: ['pr'], milestones: [{ id: 'm', name: 'Entrega', due_date: '2026-03-20', done: false }], base_start: '2026-03-01', base_end: '2026-03-25' }),
    project('b', { owner: { id: 'me', name: 'Yo Mismo' }, status_id: 's2', figures: { has_plan: false, plan: 0, real: 10, gap: 0 }, start_date: null, end_date: null }),
    project('c', { status_id: 'old', can_manage: false }),
  ],
  programs: [{ id: 'pr', name: 'Programa', figures: { has_plan: true, plan: 50, real: 30, gap: -20, level: 'late' } }],
}
const saved: ListView = { id: 'v1', name: 'Mías', shared: false, mine: true, can_edit: true, config: { mode: 'kanban', filters: { owner_id: ['me'] }, sort: '', columns: '', group: '' } }

let fetchMock: ReturnType<typeof vi.fn>
let failStatus = false
beforeEach(() => {
  failStatus = false
  localStorage.clear()
  fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url.startsWith('/api/project-board')) return json(board)
    if (url.startsWith('/api/project-views')) return method === 'POST' ? json({ ...saved, id: 'v2', name: 'Nueva' }, 201) : json({ items: [saved] })
    if (url.startsWith('/api/catalog'))
      return json({
        methodologies: [], work_item_types: [], statuses: [], priorities: [], sizes: [], labels: [],
        project_statuses: [status('s1', 'Propuesta', { is_initial: true }), status('s2', 'En ejecución'), status('s3', 'Cerrado', { is_closed: true, is_closing: true }), status('old', 'Viejo', { retired: true }), status('gone', 'Nadie', { retired: true })],
      })
    if (url.startsWith('/api/users')) return json({ items: [{ id: 'u1', name: 'Ana Ríos' }, { id: 'me', name: 'Yo Mismo' }], total: 2 })
    if (url.startsWith('/api/teams')) return json({ items: [{ id: 't1', name: 'Equipo', parent_id: null, archived_at: null, leaders: [] }] })
    if (method === 'PUT' && url.endsWith('/status')) return failStatus ? json({ error: { code: 'version_conflict', message: 'x' } }, 409) : json({ id: 'a' })
    if (method === 'PATCH') return json({ id: 'a' })
    if (url.startsWith('/api/projects/'))
      return json({ id: 'a', version: 7, name: 'Proyecto a', team: { id: 't1' }, owner: { id: 'u1' }, parent: null, start_date: '2026-03-01', end_date: '2026-03-31', health: 'en_curso', health_explanation: '', custom_values: {} })
    return json({})
  })
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => vi.unstubAllGlobals())

async function render(path = '/projects'): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/projects', component: ProjectsPage }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set({ id: 'me', permissions: ['work:read', 'projects:create:team', 'project_views:share'] } as unknown as Me)
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return { wrapper, router }
}
const calls = (method: string) => fetchMock.mock.calls.filter(([, i]) => ((i as RequestInit | undefined)?.method ?? 'GET') === method)
const body = (c: unknown[]) => JSON.parse(String((c[1] as RequestInit).body))

describe('vistas de proyectos (spec 012)', () => {
  it('abre en Kanban con una columna por estado y las retiradas en uso (RF-28, RF-33 a RF-37, RF-42)', async () => {
    const { wrapper } = await render()
    const cols = wrapper.findAll('[data-testid=project-board-column]')
    expect(cols.map((c) => c.find('h3').text())).toEqual(['Propuesta', 'En ejecución', 'Cerrado', 'Viejo'])
    expect(cols.map((c) => c.find('[data-testid=project-board-count]').text())).toEqual(['1', '1', '0', '1'])
    expect(cols[3]!.text()).toContain('retirado')
    const a = cols[0]!.find('[data-testid=project-card]')
    expect(a.text()).toContain('Programa')
    expect(a.text()).toContain('Atrasado')
    expect(a.find('[data-testid=project-card-link]').attributes('href')).toBe('/projects/a')
    expect(cols[1]!.find('[data-testid=project-card-noplan]').text()).toBe('Sin plazo')
    // Who cannot manage it cannot move it (RF-40).
    expect(cols[3]!.find('[data-testid=project-card]').attributes('draggable')).toBe('false')
    expect(cols[3]!.find('[data-testid=project-card-move]').exists()).toBe(false)
  })

  it('mueve un proyecto con el menú y avisa si falla (RF-38, RF-39, RF-41)', async () => {
    const { wrapper } = await render()
    const card = wrapper.find('[data-testid=project-card]')
    await card.find('[data-testid=project-card-move]').trigger('click')
    const items = card.findAll('[role=menuitem]')
    expect(items.map((i) => i.text())).toEqual(['En ejecución', 'Cerrado'])
    await items[1]!.trigger('click')
    await flushPromises()
    const put = calls('PUT')[0]!
    expect(String(put[0])).toBe('/api/projects/a/status')
    expect(body(put)).toEqual({ version: 3, status_id: 's3' })

    failStatus = true
    await card.find('[data-testid=project-card-move]').trigger('click')
    await card.findAll('[role=menuitem]')[0]!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=project-board-error]').exists()).toBe(true)
  })

  it('arrastrar una tarjeta a otra columna cambia su estado; agrupado por semáforo no (RF-38, RF-44)', async () => {
    const { wrapper, router } = await render()
    const cols = wrapper.findAll('[data-testid=project-board-column]')
    await cols[0]!.find('[data-testid=project-card]').trigger('dragstart', { dataTransfer: { setData: () => {} } })
    await cols[1]!.trigger('drop')
    await flushPromises()
    expect(body(calls('PUT')[0]!)).toEqual({ version: 3, status_id: 's2' })

    await wrapper.find('[data-testid=board-columns]').setValue('level')
    await flushPromises()
    expect(router.currentRoute.value.query.columns).toBe('level')
    expect(wrapper.findAll('[data-testid=project-board-column] h3').map((h) => h.text())).toContain('Sin plazo')
    expect(wrapper.find('[data-testid=project-card-move]').exists()).toBe(false)
  })

  it('filtra por «Yo» desde la dirección y cuenta cada vista (RF-30, RF-63, RF-72)', async () => {
    const { wrapper } = await render('/projects?owner_id=me')
    expect(wrapper.findAll('[data-testid=project-card]')).toHaveLength(1)
    expect(wrapper.find('[data-testid=filter-chip]').text()).toContain('Yo')
    expect(wrapper.find('[data-testid=list-view-all]').text()).toContain('3')
    expect(wrapper.find('[data-testid=list-view]').text()).toContain('Mías · 1')
    expect(wrapper.find('[data-testid=list-views-dirty]').exists()).toBe(true)
  })

  it('abre una vista y guarda la actual como nueva, compartida (RF-70, RF-71, RF-73)', async () => {
    const { wrapper, router } = await render()
    await wrapper.find('[data-testid=list-view]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toMatchObject({ view: 'v1', owner_id: 'me' })
    await wrapper.find('[data-testid=mode-table]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid=list-view-save]').trigger('click')
    await wrapper.find('[data-testid=list-view-name]').setValue('Nueva')
    await wrapper.find('[data-testid=list-view-shared]').setValue(true)
    await wrapper.find('[data-testid=list-view-confirm]').trigger('click')
    await flushPromises()
    expect(body(calls('POST')[0]!)).toEqual({ name: 'Nueva', shared: true, config: { mode: 'table', filters: { owner_id: ['me'] }, sort: '', columns: '', group: '' } })
  })

  it('el Cronograma pinta barras, línea base, hitos y grupos, y mueve fechas con el teclado (RF-46 a RF-58)', async () => {
    const { wrapper } = await render('/projects?mode=timeline')
    const groups = wrapper.findAll('[data-testid=gantt-group]')
    expect(groups.map((g) => g.text().split('·')[0]!.trim())).toEqual(['Programa', 'Sin programa'])
    expect(groups[0]!.text()).toContain('Atrasado')
    expect(wrapper.findAll('[data-testid=gantt-bar]')).toHaveLength(2)
    expect(wrapper.find('[data-testid=gantt-nodates]').text()).toBe('Sin fechas')
    expect(wrapper.findAll('[data-testid=gantt-base]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid=gantt-milestone]')).toHaveLength(1)
    await wrapper.find('[data-testid=gantt-milestones]').setValue(false)
    expect(wrapper.findAll('[data-testid=gantt-milestone]')).toHaveLength(0)

    const bars = wrapper.findAll('[data-testid=gantt-bar]')
    expect(bars[1]!.attributes('tabindex')).toBe('-1') // not managed
    await bars[0]!.trigger('keydown', { key: 'ArrowRight' })
    await flushPromises()
    const patch = calls('PATCH')[0]!
    expect(String(patch[0])).toBe('/api/projects/a')
    expect(body(patch)).toMatchObject({ version: 7, start_date: '2026-03-02', end_date: '2026-04-01' })
    await bars[0]!.trigger('keydown', { key: 'ArrowLeft', shiftKey: true })
    await flushPromises()
    expect(body(calls('PATCH')[1]!)).toMatchObject({ start_date: '2026-03-01', end_date: '2026-03-30' })
  })

  it('la Tabla ordena por cualquier columna (RF-60, RF-61)', async () => {
    const { wrapper, router } = await render('/projects?mode=table')
    expect(wrapper.findAll('[data-testid=project-row]')).toHaveLength(3)
    expect(wrapper.find('[data-testid=project-row]').text()).toContain('Propuesta')
    await wrapper.find('[data-testid=table-sort-name]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.sort).toBe('name')
    await wrapper.find('[data-testid=table-sort-name]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.sort).toBe('-name')
    expect(wrapper.find('[data-testid=project-row]').text()).toContain('Proyecto c')
  })
})
