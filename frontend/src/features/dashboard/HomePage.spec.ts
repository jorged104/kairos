import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Dashboard, DashboardTask, DashboardWeek } from '@/api/dashboard'
import { createAppI18n } from '@/lib/i18n'
import HomePage from '@/pages/HomePage.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))
const st = (id: string, name: string, extra = {}) => ({ id, name, color: '#123456', position: 1, is_closed: false, is_default: false, retired: false, ...extra })
const task = (id: string, extra: Partial<DashboardTask> = {}): DashboardTask => ({
  id, key: `ALF-${id}`, title: `Tarea ${id}`, project: { id: 'p1', name: 'Alfa' }, status_id: 'new', type_id: 'act', closed: false,
  due_date: null, closed_on: null, late_days: 0, weight: 1, can_move: true, ...extra,
})
const week = (start: string, extra: Partial<DashboardWeek> = {}): DashboardWeek => ({ start, closed: 0, weight: 0, with_due: 0, on_time: 0, ...extra })
const me = {
  id: 'u1', name: 'Ana Ríos', email: 'ana@x.com', status: 'active', auth_method: 'local', has_password: true,
  roles: [{ role: 'member', source: 'manual' }], teams: [{ id: 't', name: 'Operaciones', source: 'manual' }], permissions: ['work:read'],
} as unknown as Me
let dashboard: Dashboard
let allowed: string[]
let failPatch = false

beforeEach(() => {
  failPatch = false
  allowed = ['doing', 'done']
  dashboard = {
    today: '2026-03-11',
    profile: { open: 4, late: 2, closed_month: 1 },
    tasks: [
      task('1', { due_date: '2026-03-20' }),
      task('2', { due_date: '2026-03-01', late_days: 10 }),
      task('3'),
      task('4', { due_date: '2026-03-08', late_days: 3, can_move: false }),
      task('5', { status_id: 'done', closed: true, closed_on: '2026-03-05' }),
      task('6', { status_id: 'old' }),
    ],
    projects: [
      { id: 'p1', identifier: 'ALF', name: 'Alfa', role: 'tasks', open: 4, late: 2, figures: { has_plan: true, plan: 50, real: 30, gap: -20, level: 'late' } },
      { id: 'p2', identifier: 'BET', name: 'Beta', role: 'owner', open: 0, late: 0, figures: { has_plan: false, plan: 0, real: 0, gap: 0 } },
    ],
    weeks: Array.from({ length: 12 }, (_, i) => week(`2026-01-${String(i + 1).padStart(2, '0')}`, i === 10 ? { closed: 2, weight: 5, with_due: 2, on_time: 1 } : {})),
    load: [{ project: { id: 'p1', name: 'Alfa' }, open: 4, late: 2 }, { project: null, open: 3, late: 0 }],
  }
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    if (url.startsWith('/api/me/dashboard/people')) return json([{ id: 'u2', name: 'Mario Paz' }])
    if (url.startsWith('/api/users/u2/dashboard'))
      return json({
        ...dashboard, user: { id: 'u2', name: 'Mario Paz', roles: ['member'], teams: [{ id: 't2', name: 'Campo' }] },
        // One of a project the lead does not see (spec 023, RF-26).
        tasks: [...dashboard.tasks.map((x) => ({ ...x, can_move: false })), task('0', { id: '', key: '', title: '', project: { id: '', name: '' }, hidden: true, can_move: false })],
      })
    if (url.startsWith('/api/users/')) return json({ error: { code: 'not_found', message: 'x' } }, 404)
    if (url.startsWith('/api/me/dashboard')) return json(dashboard)
    if (url.startsWith('/api/me/points'))
      return json({ xp: 120, coins: 3, level: { id: 'l2', number: 2, name: 'Nivel 2', min_xp: 100 }, next: { id: 'l3', number: 3, name: 'Nivel 3', min_xp: 300 }, last_seq: 1 })
    if (url.startsWith('/api/catalog'))
      return json({
        methodologies: [], work_item_types: [{ id: 'act', name: 'Actividad', color: '#16a34a', position: 1, default_weight: 1 }], priorities: [], sizes: [], labels: [], project_statuses: [],
        statuses: [st('new', 'Nuevo', { is_default: true }), st('doing', 'En progreso'), st('done', 'Cerrado', { is_closed: true }), st('old', 'Viejo', { retired: true }), st('gone', 'Nadie', { retired: true })],
      })
    if (init?.method === 'PATCH') return failPatch ? json({ error: { code: 'invalid_transition', message: 'x' } }, 409) : json({ id: '1' })
    if (url.includes('/by-key/')) return json({ error: { code: 'not_found', message: 'x' } }, 404)
    if (url.startsWith('/api/work-items/')) return json({ id: '1', version: 2, status_id: 'new', allowed_status_ids: allowed, assignees: [], label_ids: [], parent: null, custom_values: {} })
    return json({})
  }))
})
afterEach(() => vi.unstubAllGlobals())

async function render(path = '/', who: Me = me) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: HomePage }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(who)
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return wrapper
}
const patches = () => vi.mocked(fetch).mock.calls.filter(([, i]) => (i as RequestInit | undefined)?.method === 'PATCH')

describe('mi panel (spec 013)', () => {
  it('muestra el perfil con sus cifras y los proyectos en los que aporto (RF-5 a RF-7, RF-23 a RF-26)', async () => {
    const w = await render()
    const card = w.find('[data-testid=profile-card]')
    expect(card.find('[data-testid=profile-name]').text()).toBe('Ana Ríos')
    expect(card.text()).toContain('AR')
    expect(card.text()).toContain('Miembro')
    expect(card.text()).toContain('Operaciones')
    expect([card.find('[data-testid=profile-open]').text(), card.find('[data-testid=profile-late]').text(), card.find('[data-testid=profile-closed]').text()]).toEqual(['4', '2', '1'])
    expect(card.find('[data-testid=profile-account]').attributes('href')).toBe('/account')
    expect(card.find('[data-testid=level-name]').text()).toBe('Nivel 2 · Nivel 2') // spec 014, RF-37
    const projects = w.findAll('[data-testid=my-project]')
    expect(projects.map((p) => p.find('[data-testid=my-project-role]').text())).toEqual(['Con tareas', 'Responsable'])
    expect(projects[0]!.text()).toContain('2 atrasadas')
    expect(projects[0]!.text()).toContain('Atrasado')
    expect(projects[0]!.find('a').attributes('href')).toBe('/projects/p1')
  })

  it('reparte mis tareas por estado, las más atrasadas primero (RF-8 a RF-15)', async () => {
    const w = await render()
    const cols = w.findAll('[data-testid=my-column]')
    expect(cols.map((c) => c.find('h3').text())).toEqual(['Nuevo', 'En progreso', 'Cerrado', 'Viejo'])
    expect(cols[3]!.text()).toContain('retirado')
    expect(cols[0]!.findAll('[data-testid=my-task-link]').map((l) => l.text())).toEqual(['Tarea 2', 'Tarea 4', 'Tarea 1', 'Tarea 3'])
    expect(cols[0]!.find('[data-testid=my-task-late]').text()).toBe('10 días de atraso')
    // Opens in the side panel, as in the projects (RF-15, change of 2026-11-05).
    expect(cols[0]!.find('[data-testid=my-task-link]').attributes('href')).toBe('/?item=ALF-2')
    expect(cols[2]!.text()).toContain('Cerrada el')
    // Who cannot write it cannot move it.
    const four = cols[0]!.findAll('[data-testid=my-task]')[1]!
    expect(four.attributes('draggable')).toBe('false')
    expect(four.find('[data-testid=my-task-move]').exists()).toBe(false)
  })

  it('mueve una tarea arrastrando según su flujo, o con el menú, y avisa si falla (RF-16 a RF-20)', async () => {
    const w = await render()
    allowed = ['done']
    const cols = w.findAll('[data-testid=my-column]')
    await cols[0]!.find('[data-testid=my-task]').trigger('dragstart', { dataTransfer: { setData: () => {} } })
    await flushPromises()
    expect(cols[1]!.attributes('aria-disabled')).toBe('true')
    await cols[1]!.trigger('drop')
    await flushPromises()
    expect(patches()).toHaveLength(0)
    await cols[0]!.find('[data-testid=my-task]').trigger('dragstart', { dataTransfer: { setData: () => {} } })
    await flushPromises()
    await cols[2]!.trigger('drop')
    await flushPromises()
    expect(JSON.parse(String((patches()[0]![1] as RequestInit).body))).toMatchObject({ status_id: 'done' })

    allowed = ['doing', 'done']
    failPatch = true
    const card = w.findAll('[data-testid=my-column]')[0]!.find('[data-testid=my-task]')
    await card.find('[data-testid=my-task-move]').trigger('click')
    await flushPromises()
    // The card with the open menu goes above the cards under it.
    expect(card.classes()).toContain('z-30')
    const items = card.findAll('[role=menuitem]')
    expect(items.map((i) => i.text())).toEqual(['En progreso', 'Cerrado'])
    await items[0]!.trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=my-kanban-error]').exists()).toBe(true)
  })

  it('corta las columnas en 100 y avisa de las que faltan (RF-21), o muestra que no hay tareas (RF-22)', async () => {
    dashboard.tasks = Array.from({ length: 103 }, (_, i) => task(String(i)))
    let w = await render()
    const first = w.find('[data-testid=my-column]')
    expect(first.findAll('[data-testid=my-task]')).toHaveLength(100)
    expect(first.find('[data-testid=my-column-more]').text()).toContain('3 más')
    w.unmount()
    dashboard.tasks = []
    w = await render()
    expect(w.find('[data-testid=my-kanban-empty]').exists()).toBe(true)
  })

  it('dibuja las gráficas con sus tablas, «Sin datos» y «Otros» (RF-28 a RF-35)', async () => {
    const w = await render()
    expect(w.findAll('[data-testid=weekly-bar]')).toHaveLength(12)
    expect(w.find('[data-testid=chart-weekly] svg').attributes('aria-label')).toContain('total 2')
    expect(w.findAll('[data-testid=weekly-table] tbody tr')[10]!.text()).toContain('5')
    expect(w.find('[data-testid=ontime-total]').text()).toBe('50 %')
    expect(w.findAll('[data-testid=ontime-table] tbody tr')[0]!.text()).toContain('Sin datos')
    const rows = w.findAll('[data-testid=load-row]')
    expect(rows.map((r) => r.find('.truncate').text())).toEqual(['Alfa', 'Otros'])
    expect(w.find('[data-testid=load-table]').text()).toContain('Otros')
    w.unmount()

    dashboard.weeks = dashboard.weeks.map((x) => ({ ...x, closed: 0, with_due: 0, on_time: 0 }))
    const empty = await render()
    expect(empty.find('[data-testid=ontime-total]').text()).toBe('Sin datos')
  })

  it('el inicio es Mi panel con lectura de trabajo; sin ella, el estado del sistema (RF-1, RF-2)', { timeout: 20_000 }, async () => {
    vi.stubGlobal('fetch', vi.fn(() => json({})))
    setActivePinia(createPinia())
    useAuthStore().set(me)
    const router = createAppRouter(createMemoryHistory())
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('home')
    useAuthStore().set({ ...me, permissions: [] })
    await router.push('/status')
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('status')
    expect(router.currentRoute.value.path).toBe('/status')
  })

  it('marca cada tarjeta con el color de su tipo y la abre en el panel lateral (RF-14a, RF-15)', async () => {
    const w = await render()
    const stripe = w.find('[data-testid=my-task] [data-testid=type-stripe]')
    expect(stripe.attributes('aria-label')).toBe('Actividad')
    expect(stripe.attributes('title')).toBe('Actividad')
    expect(stripe.find('span').attributes('style')).toContain('background: rgb(22, 163, 74)')
    await w.find('[data-testid=my-task-link]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=item-panel]').exists()).toBe(true)
  })
})

describe('panel de otra persona (spec 013, cambio de 2026-11-06)', () => {
  const lead = { ...me, roles: [{ role: 'lead', source: 'manual' }], permissions: ['work:read', 'dashboard:others:team'] } as unknown as Me

  it('sin el permiso no hay selector (RF-37)', async () => {
    const w = await render()
    expect(w.find('[data-testid=dashboard-person]').exists()).toBe(false)
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u).startsWith('/api/me/dashboard/people'))).toBe(false)
  })

  it('el líder elige a una persona y ve su panel en solo lectura (RF-37 a RF-47)', async () => {
    const w = await render('/', lead)
    const select = w.find('[data-testid=dashboard-person]')
    expect(select.findAll('option').map((o) => o.text())).toEqual(['Mi panel', 'Mario Paz'])
    await select.setValue('u2')
    await flushPromises()
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u).startsWith('/api/users/u2/dashboard'))).toBe(true)
    expect(w.find('h1').text()).toBe('Panel de Mario Paz')
    expect(w.find('[data-testid=dashboard-other]').text()).toContain('solo lectura')
    const card = w.find('[data-testid=profile-card]')
    expect(card.find('[data-testid=profile-name]').text()).toBe('Mario Paz')
    expect(card.text()).toContain('Campo')
    expect(card.find('[data-testid=profile-account]').attributes('href')).toBe('/users/u2')
    expect(w.find('#my-tasks').text()).toBe('Tareas de Mario Paz')
    expect(w.find('#my-projects').text()).toBe('Proyectos de Mario Paz')
    expect(w.findAll('[data-testid=my-task]').every((x) => x.attributes('draggable') === 'false')).toBe(true)
    expect(w.find('[data-testid=my-task-move]').exists()).toBe(false)
    const hidden = w.find('[data-testid=my-task-hidden]').element.closest('article')!
    expect(hidden.textContent).toContain('Tarea de otro proyecto')
    expect(hidden.querySelector('a')).toBeNull()
    expect(w.text()).not.toContain('Arrastra una tarea')
  })

  it('un panel que no se puede ver no muestra datos y ofrece volver (RF-48, RF-49)', async () => {
    const w = await render('/?user=u9', lead)
    expect(w.find('[data-testid=profile-card]').exists()).toBe(false)
    expect(w.text()).toContain('Ese panel no está disponible.')
    expect(w.find('[data-testid=dashboard-back]').attributes('href')).toBe('/')
  })
})
