import type { ProjectSchedule } from '@/api/schedule'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouterView, createMemoryHistory, createRouter, type Router } from 'vue-router'
import type { Member, Overview, Suggestion, ViewList } from '@/api/projectScreens'
import type { WorkloadRow } from '@/api/workload'
import { dragged, fromDay, ticks, toDay, today } from '@/features/timeline/dates'
import ProjectPage from '@/pages/ProjectPage.vue'
import { createAppI18n } from '@/lib/i18n'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'
import type { Catalog, Project, WorkItem, WorkNode } from '@/types/work'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))

const catalog: Catalog = {
  methodologies: [],
  work_item_types: [{ id: 'ty', name: 'Historia', color: '#3b82f6', position: 1, default_weight: 1, retired: false }],
  statuses: [
    { id: 'st-new', name: 'Nuevo', color: '#3b82f6', position: 1, is_closed: false, is_default: true, retired: false },
    { id: 'st-doing', name: 'En progreso', color: '#f59e0b', position: 2, is_closed: false, is_default: false, retired: false },
    { id: 'st-done', name: 'Cerrado', color: '#64748b', position: 3, is_closed: true, is_default: false, retired: false },
  ],
  priorities: [{ id: 'pr', name: 'Normal', color: '#3b82f6', position: 1, is_default: true, retired: false }],
  sizes: [{ id: 'sz-m', name: 'M', position: 3, retired: false }],
  labels: [{ id: 'lb-f', name: 'Frontend', color: '#22c55e', position: 1, retired: false }],
}

const node = (id: string, key: string, extra: Partial<WorkNode> = {}): WorkNode => ({
  id, parent_id: null, key, title: `Tarea ${key}`, type_id: 'ty', status_id: 'st-new', priority_id: 'pr', assignee: null, assignees: [],
  size_id: null, label_ids: [], derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position: 0,
  due_date: null, weight: 1, start_date: null, blocked: false, milestone_ids: [], ...extra,
})
const nodes = [
  node('w1', 'KAI-1', { start_date: '2026-03-01', due_date: '2026-03-10', derived_done_ratio: 40, label_ids: ['lb-f'] }),
  node('w2', 'KAI-2', { status_id: 'st-doing', blocked: true }),
]

const project: Project = {
  id: 'p1', identifier: 'KAI', name: 'Kairos', description: null, page_version: 1, team: { id: 't', name: 'BI' }, owner: { id: 'u', name: 'U' },
  methodology: null, parent: null, template_id: null, is_template: false, lifecycle: 'activo', health: 'en_riesgo', health_explanation: 'Retraso en SSO',
  start_date: '2026-03-01', end_date: '2026-03-31', depth: 0, version: 1, work_item_type_ids: ['ty'], custom_field_ids: [], custom_values: {},
  can: { manage: true, write_work_items: true, manage_template: false, manage_team: true }, my_role: 'pm', column_limits: { 'st-doing': 1 },
}

const overview: Overview = {
  today: '2026-03-15', progress: 40, has_plan: true, expected: 48, deviation: -8,
  figures: { has_plan: true, plan: 48, real: 40, gap: -8, level: 'at_risk' },
  plan_series: [{ day: '2026-03-01', plan: 0 }, { day: '2026-03-15', plan: 48 }, { day: '2026-03-31', plan: 100 }],
  statuses: [{ status_id: 'st-new', count: 3 }, { status_id: 'st-done', count: 1 }], total: 4, closed: 1, overdue: 2, blocked: 1,
  next_milestone: { id: 'm1', name: 'Piloto', due_date: '2026-03-20', status: 'abierto', percent: 50, closed: 1, total: 2, days_left: 5, figures: { has_plan: false, plan: 0, real: 50, gap: 0 } },
  milestones: [{ id: 'm1', name: 'Piloto', due_date: '2026-03-20', status: 'abierto', percent: 50, closed: 1, total: 2, figures: { has_plan: true, plan: 80, real: 50, gap: -30, level: 'late' } }],
  risks: [
    { id: 'w9', key: 'KAI-9', title: 'Pruebas de carga', reason: 'overdue', days: 4, deviation: -30, gap: -30, due_date: '2026-03-11', assignees: [] },
    { id: 'w8', key: 'KAI-8', title: 'Diseño', reason: 'behind', days: 0, deviation: -7, gap: -7.5, level: 'at_risk', due_date: '2026-03-25', assignees: [] },
  ],
  load: [], history: [{ day: '2026-03-14', progress: 38 }],
}

const schedule: ProjectSchedule = {
  today: '2026-03-15',
  baseline: { version: 2, reason: 'Cambio de alcance', created_at: '2026-03-10T10:00:00Z', author: { id: 'u', name: 'Ana Pérez' } },
  versions: [{ version: 1, reason: null, created_at: '2026-03-01T10:00:00Z', author: { id: 'u', name: 'Ana Pérez' } }],
  items: [],
}

const members: Member[] = [
  { user: { id: 'u', name: 'Ana Pérez' }, status: 'active', role: 'pm', owner: true, in_team: false, explicit: false, can_work: true, capacity: 10, load: { project: 4, total: 12, items: 2, overloaded: true } },
  { user: { id: 'u2', name: 'Luis Gómez' }, status: 'active', role: 'member', owner: false, in_team: true, explicit: false, can_work: true, capacity: null, load: { project: 1, total: 1, items: 1, overloaded: false } },
]
const teamSchedule: WorkloadRow[] = [
  { user: { id: 'u2', name: 'Luis Gómez' }, free_from: null, open: 0, weight: 0, undated: 0, bars: [] },
  {
    user: { id: 'u', name: 'Ana Pérez' }, free_from: '2026-03-21', open: 2, weight: 4, undated: 0,
    bars: [{ id: 'w1', key: 'KAI-1', title: 'Diseño', project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, start: '2026-03-02', end: '2026-03-20', done_ratio: 40, overdue: false }],
  },
]
const suggestions: Suggestion[] = [{ id: 'w5', key: 'KAI-5', title: 'Sin asignar', weight: 2, due_date: null, label_ids: [], people: [{ id: 'u2', name: 'Luis Gómez' }] }]
let views: ViewList
let fetchMock: ReturnType<typeof vi.fn>
const calls = (method: string) => fetchMock.mock.calls.filter(([, i]) => (i as RequestInit | undefined)?.method === method)
const bodyOf = (call: unknown[]) => JSON.parse(String((call[1] as RequestInit).body))

beforeEach(() => {
  views = { items: [{ id: 'v1', name: 'Frontend', shared: true, mine: false, can_edit: true, count: 1, unavailable: [], config: { filters: { label_id: ['lb-f'] }, sort: '', group: '' } }], all_count: 2 }
  fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url.startsWith('/api/projects/p1/views') && method === 'POST') return json({ ...bodyOf([url, init]), id: 'v2', mine: true, can_edit: true, count: 1, unavailable: [] }, 201)
    if (url.startsWith('/api/projects/p1/views')) return json(views)
    if (url.startsWith('/api/projects/p1/overview')) return json(overview)
    if (url.startsWith('/api/projects/p1/baseline') && method === 'POST') return json({ ...schedule.baseline, version: 3 }, 201)
    if (url.startsWith('/api/projects/p1/schedule')) return json(schedule)
    if (url.startsWith('/api/projects/p1/tree')) return json({ items: nodes })
    if (url.startsWith('/api/projects/p1/members')) return json({ items: members })
    if (url.startsWith('/api/projects/p1/assignment-suggestions')) return json({ items: suggestions })
    if (url.startsWith('/api/projects/p1/team-schedule')) return json({ items: teamSchedule })
    if (url.startsWith('/api/me/dashboard/people')) return json([{ id: 'u2', name: 'Luis Gómez' }])
    if (url.startsWith('/api/projects/p1/milestones')) return json({ items: [] })
    if (url.endsWith('/api/projects/p1')) return json(project)
    if (url.endsWith('/api/catalog')) return json(catalog)
    if (url.startsWith('/api/users')) return json({ items: [{ id: 'u2', name: 'Luis Gómez' }, { id: 'u3', name: 'Marta Ruiz' }], total: 2, page: 1, page_size: 100 })
    if (url.startsWith('/api/work-items/w')) {
      const id = url.split('/')[3]!
      const item = { ...nodes.find((n) => n.id === id), id, version: 3, allowed_status_ids: ['st-doing'], custom_values: {}, assignees: [], label_ids: [] } as unknown as WorkItem
      if (method === 'PATCH') return json(item)
      return json(item)
    }
    return json({ items: [] })
  })
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
  localStorage.clear()
})

async function renderPage(path = '/projects/p1', who?: Me): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/projects/:id', component: ProjectPage, props: true }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const pinia = createPinia()
  setActivePinia(pinia)
  if (who) useAuthStore().set(who)
  const wrapper = mount(RouterView, {
    global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    attachTo: document.body,
  })
  await flushPromises()
  return { wrapper, router }
}

describe('pestañas del proyecto (spec 008)', () => {
  it('abre en Resumen con las cinco vistas delante (RF-1, RF-2, RF-6 a RF-11, RF-15)', async () => {
    const { wrapper } = await renderPage()
    const tabs = wrapper.findAll('[role=tab]').map((x) => x.text())
    expect(tabs).toEqual(['Resumen', 'Tabla', 'Kanban', 'Cronograma', 'Equipo', 'Hitos', 'Reuniones', 'Informes', 'Fases', 'Project History', 'Config'])
    const ov = wrapper.find('[data-testid=overview]')
    expect(ov.find('[data-testid=overview-progress]').text()).toContain('40 %')
    // The traffic light with plan, real and gap (spec 009, RF-35, RF-44, RF-45, RF-47).
    expect(ov.find('[data-testid=overview-progress]').text()).toContain('En riesgo')
    expect(ov.find('[data-testid=overview-progress]').text()).toContain('Plan 48 % · Real 40 % · Brecha -8 pp')
    expect(ov.find('[data-testid=overview-milestones]').text()).toContain('Atrasado')
    expect(ov.find('[data-testid=overview-risks]').text()).toContain('En riesgo, -7,5 pp')
    expect(ov.find('[data-testid=overview-attention]').text()).toContain('2 vencidas')
    expect(ov.find('[data-testid=overview-next]').text()).toContain('faltan 5 días')
    expect(ov.findAll('[data-testid=overview-risk]')[0]!.text()).toContain('Vencida 4 días')
    expect(ov.find('[data-testid=overview-overdue]').attributes('href')).toContain('overdue=true')
  })

  it('el Resumen de un programa muestra su agregado y sus proyectos (spec 010, RF-39, RF-40)', async () => {
    const f = (plan: number, real: number, level: 'late' | 'on_track') => ({ has_plan: true, plan, real, gap: real - plan, level })
    overview.program = {
      program: { id: 'p1', identifier: 'KAI', name: 'Kairos', lifecycle: 'activo', is_program: true, weight: 3, figures: f(80, 60, 'late') },
      projects: 2, below_plan: 1,
      rows: [
        { id: 'c1', identifier: 'C1', name: 'Uno', lifecycle: 'activo', is_program: false, weight: 1, figures: f(50, 50, 'on_track') },
        { id: 'c2', identifier: 'C2', name: 'Dos', lifecycle: 'activo', is_program: false, weight: 2, figures: f(90, 60, 'late') },
      ],
    }
    try {
      const { wrapper } = await renderPage()
      const card = wrapper.find('[data-testid=overview-program]')
      expect(card.text()).toContain('2 proyectos · 1 bajo plan')
      expect(card.text()).toContain('Atrasado')
      expect(card.findAll('[data-testid=portfolio-row-link]').map((x) => x.text())).toEqual(['Dos', 'Uno'])
    } finally {
      delete overview.program
    }
  })

  it('muestra la línea base y pide el motivo para volver a fijarla (spec 009, RF-2, RF-5, RF-6)', async () => {
    const { wrapper } = await renderPage()
    const panel = wrapper.find('[data-testid=baseline]')
    expect(panel.find('[data-testid=baseline-current]').text()).toContain('Versión 2')
    expect(panel.text()).toContain('Cambio de alcance')
    expect(panel.find('[data-testid=baseline-versions]').text()).toContain('Versión 1')
    await panel.find('[data-testid=baseline-set]').trigger('click')
    expect(calls('POST').some(([u]) => String(u).includes('/baseline'))).toBe(false)
    await panel.find('[data-testid=baseline-reason]').setValue('Nuevo alcance')
    await panel.find('form').trigger('submit')
    await flushPromises()
    const post = calls('POST').find(([u]) => String(u).includes('/baseline'))!
    expect(bodyOf(post)).toEqual({ reason: 'Nuevo alcance' })
  })

  it('oculta Equipo a quien no gestiona el equipo (RF-4)', async () => {
    project.can.manage_team = false
    try {
      const { wrapper } = await renderPage('/projects/p1?tab=team')
      expect(wrapper.findAll('[role=tab]').map((x) => x.text())).not.toContain('Equipo')
      expect(wrapper.find('[data-testid=overview]').exists()).toBe(true)
    } finally {
      project.can.manage_team = true
    }
  })
})

describe('filtros y vistas (spec 008)', () => {
  it('añade un filtro como chip en la dirección y lo quita (RF-3, RF-18, RF-21)', async () => {
    const { wrapper, router } = await renderPage('/projects/p1?tab=table')
    await wrapper.find('[data-testid=filter-add]').trigger('click')
    await wrapper.find('[data-testid=filter-field]').setValue('status_id')
    const values = wrapper.findAll('[data-testid=filter-values] input')
    await values[0]!.trigger('click')
    await values[1]!.trigger('click')
    await wrapper.find('[data-testid=filter-apply]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.status_id).toEqual(['st-new', 'st-doing'])
    expect(wrapper.findAll('[data-testid=filter-chip]')).toHaveLength(2)
    expect(fetchMock.mock.calls.some(([u]) => String(u).includes('/tree?') && String(u).includes('status_id=st-new&status_id=st-doing'))).toBe(true)
    await wrapper.find('[data-testid=filter-remove]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.status_id).toBe('st-doing')
  })

  it('abre una vista, avisa de cambios sin guardar y guarda otra compartida (RF-24 a RF-27, RF-32)', async () => {
    const { wrapper, router } = await renderPage('/projects/p1?tab=kanban')
    const buttons = wrapper.findAll('[data-testid=view-button]')
    expect(buttons[0]!.text()).toContain('Frontend · 1')
    await buttons[0]!.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toMatchObject({ view: 'v1', label_id: 'lb-f', tab: 'kanban' })
    expect(localStorage.getItem('kairos.project.p1.view')).toBe('v1')
    expect(wrapper.find('[data-testid=views-dirty]').exists()).toBe(false)

    await wrapper.find('[data-testid=filter-remove]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=views-dirty]').exists()).toBe(true)
    await wrapper.find('[data-testid=view-save]').trigger('click')
    await wrapper.find('[data-testid=view-name]').setValue('Todo el equipo')
    await wrapper.find('[data-testid=view-shared]').setValue(true)
    await wrapper.find('[data-testid=view-confirm]').trigger('click')
    await flushPromises()
    const post = calls('POST').find(([u]) => String(u).endsWith('/views'))!
    expect(bodyOf(post)).toEqual({ name: 'Todo el equipo', shared: true, config: { filters: {}, sort: '', group: '' } })
  })
})

describe('kanban (spec 008)', () => {
  it('mueve una tarjeta arrastrándola o con «Mover a», y marca el límite (RF-62 a RF-66)', async () => {
    const { wrapper } = await renderPage('/projects/p1?tab=kanban')
    const columns = wrapper.findAll('[data-testid=kanban-column]')
    expect(columns.map((c) => c.attributes('aria-label'))).toEqual(['Nuevo', 'En progreso', 'Cerrado'])
    expect(columns[1]!.find('[data-testid=kanban-limit]').text()).toBe('1 / 1')
    const card = columns[0]!.find('[data-testid=kanban-card]')
    await card.trigger('dragstart', { dataTransfer: { setData: () => undefined } })
    await flushPromises()
    // The workflow allows only «En progreso».
    expect(columns[2]!.attributes('aria-disabled')).toBe('true')
    await columns[2]!.trigger('drop')
    await flushPromises()
    expect(calls('PATCH')).toHaveLength(0)
    await card.trigger('dragstart', { dataTransfer: { setData: () => undefined } })
    await flushPromises()
    await columns[1]!.trigger('drop')
    await flushPromises()
    expect(bodyOf(calls('PATCH')[0]!)).toMatchObject({ status_id: 'st-doing' })

    await card.find('[data-testid=kanban-move]').trigger('click')
    await flushPromises()
    const menu = wrapper.find('[data-testid=kanban-move-menu]')
    expect(menu.findAll('[role=menuitem]').map((b) => b.text())).toEqual(['En progreso'])
  })

  it('si el cambio se rechaza la tarjeta se queda y se explica (RF-64)', async () => {
    const { wrapper } = await renderPage('/projects/p1?tab=kanban')
    const original = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) =>
      init?.method === 'PATCH' ? json({ error: { code: 'blocked_by_open_item', message: '' } }, 409) : original(url, init),
    )
    const columns = wrapper.findAll('[data-testid=kanban-column]')
    await columns[0]!.find('[data-testid=kanban-card]').trigger('dragstart', { dataTransfer: { setData: () => undefined } })
    await flushPromises()
    await columns[1]!.trigger('drop')
    await flushPromises()
    expect(wrapper.find('[data-testid=kanban-error]').text()).toContain('la bloquea una tarea abierta')
  })
})

describe('cronograma (spec 008)', () => {
  it('calcula las fechas al arrastrar y no deja el fin antes del inicio (RF-75 a RF-77)', () => {
    const s = toDay('2026-03-01')
    const e = toDay('2026-03-10')
    expect(dragged(s, e, 'move', 3)).toEqual({ start: s + 3, end: e + 3 })
    expect(dragged(s, e, 'start', 2)).toEqual({ start: s + 2, end: e })
    expect(dragged(s, e, 'end', -9)).toEqual({ start: s, end: s })
    expect(dragged(s, e, 'end', -10)).toBeNull()
    expect(ticks(s, s + 70, 'months', 'es').map((x) => x.label)).toHaveLength(3)
    // Days: one tick a day, the first of the month with its name (RF-81).
    const days = ticks(s - 1, s + 1, 'days', 'es').map((x) => x.label)
    expect(days).toHaveLength(3)
    expect(days[1]).toBe('1 mar')
  })

  it('dibuja la línea base detrás de la barra y la colorea con su semáforo (spec 009, RF-13, RF-48)', async () => {
    const later = node('w7', 'KAI-7', { start_date: '2099-01-10', due_date: '2099-01-20' })
    nodes.push(later)
    schedule.items = [{ id: 'w7', figures: { has_plan: true, plan: 50, real: 40, gap: -10, level: 'at_risk' }, in_baseline: true,
      base_start: '2099-01-01', base_due: '2099-01-10', changed: true, shift_days: 10 }]
    try {
      const { wrapper } = await renderPage('/projects/p1?tab=timeline')
      expect(wrapper.findAll('[data-testid=timeline-baseline]')).toHaveLength(1)
      expect(wrapper.findAll('[data-testid=timeline-bar]').map((b) => b.attributes('data-level'))).toContain('at_risk')
    } finally {
      nodes.pop()
      schedule.items = []
    }
  })

  it('arrastra una barra y lista las tareas sin fechas (RF-70, RF-74, RF-75)', async () => {
    const { wrapper } = await renderPage('/projects/p1?tab=timeline')
    expect(wrapper.find('[data-testid=timeline-undated]').text()).toContain('KAI-2')
    const bar = wrapper.find('[data-testid=timeline-bar]')
    bar.element.dispatchEvent(new PointerEvent('pointerdown', { clientX: 100, bubbles: true }))
    const scroller = bar.element.closest('.overflow-x-auto')!
    scroller.dispatchEvent(new PointerEvent('pointermove', { clientX: 114, bubbles: true }))
    scroller.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }))
    await flushPromises()
    // Months: 7 px a day, so 14 px are two days.
    expect(bodyOf(calls('PATCH')[0]!)).toMatchObject({ start_date: '2026-03-03', due_date: '2026-03-12' })
  })

  it('muestra el árbol, la fecha al arrastrar y crea tareas y subtareas (RF-81 a RF-88)', async () => {
    nodes.push(
      node('w3', 'KAI-3', { position: 1 }),
      node('w4', 'KAI-4', { parent_id: 'w3', start_date: '2026-03-05', due_date: '2026-03-08' }),
      node('w5', 'KAI-5', { parent_id: 'w4', start_date: '2026-03-06', due_date: '2026-03-07' }),
    )
    try {
      const { wrapper } = await renderPage('/projects/p1?tab=timeline')
      const rows = () => wrapper.findAll('[data-testid=timeline-row]')
      expect(rows().map((r) => [r.text().match(/KAI-\d/)?.[0], r.attributes('data-level')])).toEqual([['KAI-1', '0'], ['KAI-3', '0'], ['KAI-4', '1'], ['KAI-5', '2']])
      // The undated parent spans its children, without a bar (RF-84).
      expect(rows()[1]!.find('[data-testid=timeline-bar]').exists()).toBe(false)
      expect(rows()[1]!.find('[data-testid=timeline-summary]').exists()).toBe(true)
      await rows()[1]!.find('[data-testid=timeline-toggle]').trigger('click')
      expect(rows()).toHaveLength(2)
      await rows()[1]!.find('[data-testid=timeline-toggle]').trigger('click')
      expect(rows()).toHaveLength(4)

      // The dates it would get, by the pointer (RF-82).
      const end = rows()[2]!.find('[data-testid=timeline-end]')
      end.element.dispatchEvent(new PointerEvent('pointerdown', { clientX: 100, clientY: 50, bubbles: true }))
      const scroller = end.element.closest('.overflow-x-auto')!
      scroller.dispatchEvent(new PointerEvent('pointermove', { clientX: 114, clientY: 50, bubbles: true }))
      await flushPromises()
      const tip = wrapper.find('[data-testid=timeline-tip]')
      expect(tip.text()).toContain('10 mar')
      expect(tip.text()).toContain('6 días')
      scroller.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }))
      await flushPromises()
      expect(wrapper.find('[data-testid=timeline-tip]').exists()).toBe(false)
      expect(bodyOf(calls('PATCH')[0]!)).toMatchObject({ start_date: '2026-03-05', due_date: '2026-03-10' })

      // At the foot of a group: from today, a week (RF-86).
      await wrapper.find('[data-testid=timeline-add]').trigger('click')
      await wrapper.find('[data-testid=timeline-new-title]').setValue('Nueva')
      await wrapper.find('[data-testid=timeline-new-title]').element.form!.dispatchEvent(new Event('submit'))
      await flushPromises()
      const posts = () => calls('POST').filter(([u]) => String(u).endsWith('/api/projects/p1/work-items'))
      expect(bodyOf(posts()[0]!)).toMatchObject({ title: 'Nueva', type_id: 'ty', parent_id: null, start_date: fromDay(today()), due_date: fromDay(today() + 6) })

      // A sub-item takes its parent's dates (RF-88).
      await rows()[2]!.find('[data-testid=timeline-add-child]').trigger('click')
      await wrapper.find('[data-testid=timeline-new-title]').setValue('Hija')
      await wrapper.find('[data-testid=timeline-new-title]').element.form!.dispatchEvent(new Event('submit'))
      await flushPromises()
      expect(bodyOf(posts()[1]!)).toMatchObject({ title: 'Hija', parent_id: 'w4', start_date: '2026-03-05', due_date: '2026-03-08' })
    } finally {
      nodes.splice(2)
    }
  })
})

describe('equipo (spec 008)', () => {
  it('cambia un papel y asigna desde una sugerencia (RF-33, RF-48, RF-49, RF-53)', async () => {
    const { wrapper } = await renderPage('/projects/p1?tab=team')
    const rows = wrapper.findAll('[data-testid=team-member]')
    expect(rows[0]!.find('[data-testid=team-role]').attributes('disabled')).toBeDefined() // the owner is PM
    expect(rows[0]!.find('[data-testid=team-load]').text()).toContain('12 / 10')
    await rows[1]!.find('[data-testid=team-role]').setValue('observer')
    await flushPromises()
    const patch = calls('PATCH').find(([u]) => String(u).includes('/members/u2'))!
    expect(bodyOf(patch)).toEqual({ role: 'observer' })

    await wrapper.find('[data-testid=team-assign]').trigger('click')
    await flushPromises()
    const assign = calls('PATCH').find(([u]) => String(u).endsWith('/api/work-items/w5'))!
    expect(bodyOf(assign)).toMatchObject({ assignee_ids: ['u2'], assignee_id: 'u2' })
    expect(wrapper.find('[data-testid=team-notice]').text()).toBe('KAI-5 asignada a Luis Gómez.')
  })

  it('muestra el cronograma por persona, con enlace al panel solo de quien se puede ver (spec 021, RF-14 a RF-16)', async () => {
    const lead = { id: 'u', name: 'Ana Pérez', permissions: ['work:read', 'dashboard:others:team'] } as unknown as Me
    const { wrapper } = await renderPage('/projects/p1?tab=team', lead)
    const rows = wrapper.findAll('[data-testid=team-schedule] [data-testid=workload-row]')
    expect(rows.map((r) => r.find('[data-testid=workload-person]').text())).toEqual(['Luis Gómez', 'Ana Pérez'])
    expect(rows[0]!.find('a[data-testid=workload-person]').attributes('href')).toBe('/?user=u2')
    expect(rows[1]!.findAll('[data-testid=workload-bar]')).toHaveLength(1)

    // Without the permissions, nobody else's panel: only one's own.
    const { wrapper: other } = await renderPage('/projects/p1?tab=team', { id: 'u9', name: 'Pepe', permissions: ['work:read'] } as unknown as Me)
    expect(other.findAll('[data-testid=team-schedule] a[data-testid=workload-person]')).toHaveLength(0)
  })
})
