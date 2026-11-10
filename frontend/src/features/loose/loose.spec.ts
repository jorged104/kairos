import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import type { Dashboard, DashboardTask } from '@/api/dashboard'
import type { Claimable, TeamTask } from '@/api/loose'
import { createAppI18n } from '@/lib/i18n'
import HomePage from '@/pages/HomePage.vue'
import MyTeamPage from '@/pages/MyTeamPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'
import type { WorkItem } from '@/types/work'
import LooseBar from './LooseBar.vue'

// Spec 022: tareas sin proyecto.

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))
const who = (id: string, permissions: string[]) =>
  ({ id, name: id, email: `${id}@x.com`, status: 'active', auth_method: 'local', has_password: true, roles: [], teams: [], permissions }) as unknown as Me
const member = who('u1', ['work:read', 'loose:create:own'])
const lead = who('lead', ['work:read', 'loose:create:own', 'loose:assign:team', 'dashboard:others:team'])
const task = (id: string, extra: Partial<DashboardTask> = {}): DashboardTask => ({
  id, key: `ALF-${id}`, title: `Tarea ${id}`, project: { id: 'p1', name: 'Alfa' }, status_id: 'new', type_id: 'act', closed: false,
  due_date: null, closed_on: null, late_days: 0, weight: 1, can_move: true, ...extra,
})
let dashboard: Dashboard
let claimable: Claimable[]
let teamTasks: TeamTask[]
let claimStatus: number

beforeEach(() => {
  claimStatus = 200
  dashboard = {
    today: '2026-03-10', profile: { open: 3, late: 0, closed_month: 0 }, projects: [], load: [],
    weeks: Array.from({ length: 12 }, (_, i) => ({ start: `2026-01-${String(i + 1).padStart(2, '0')}`, closed: 0, weight: 0, with_due: 0, on_time: 0 })),
    tasks: [
      task('1'),
      task('2', { key: 'TAREA-2', title: 'Suelta', loose: { team: null, mode: '' } }),
      task('3', { key: 'TAREA-3', title: 'Formación', loose: { team: { id: 't1', name: 'Campo' }, mode: 'each' } }),
    ],
  }
  claimable = [{ id: 'c1', key: 'TAREA-9', title: 'Incidencia', due_date: '2026-03-01', weight: 1, team: { id: 't1', name: 'Campo' }, author: { id: 'lead', name: 'Laura' } }]
  teamTasks = [
    { id: 'c1', key: 'TAREA-9', title: 'Incidencia', due_date: null, mode: 'claim', team: { id: 't1', name: 'Campo' }, closed: false, created_at: '2026-03-08T10:00:00Z', people: [], done: 0, total: 0 },
    {
      id: 'g1', key: 'TAREA-4', title: 'Formación', due_date: '2026-03-30', mode: 'each', team: { id: 't1', name: 'Campo' }, closed: false, created_at: '2026-03-01T10:00:00Z',
      people: [{ id: 'u1', name: 'Ana', done: true }, { id: 'u2', name: 'Mario', done: false }], done: 1, total: 2,
    },
  ]
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url.includes('/by-key/')) return json({ error: { code: 'not_found', message: 'x' } }, 404)
    if (url.startsWith('/api/loose-tasks')) return json({ id: 'n1', key: 'TAREA-7' }, 201)
    if (/\/work-items\/\w+\/claim$/.test(url)) return claimStatus === 200 ? json({ id: 'c1', key: 'TAREA-9' }) : json({ error: { code: 'already_taken', message: 'x' } }, claimStatus)
    if (url.startsWith('/api/me/claimable-tasks')) return json({ items: claimable })
    if (url.startsWith('/api/me/team-tasks')) return json({ items: teamTasks })
    if (url.startsWith('/api/me/team')) return json({ today: '2026-03-10', teams: [{ id: 't1', name: 'Campo' }], people: [] })
    if (url.startsWith('/api/me/dashboard/people')) return json([{ id: 'u2', name: 'Mario' }])
    if (url.startsWith('/api/me/dashboard')) return json(dashboard)
    if (url.startsWith('/api/me/points')) return json({ xp: 0, coins: 0, level: { id: 'l1', number: 1, name: 'Nivel 1', min_xp: 0 }, next: null, last_seq: 0 })
    if (url.startsWith('/api/teams'))
      return json({ items: [{ id: 't1', name: 'Campo', parent_id: null, external_ref: null, archived_at: null, leaders: [{ user_id: 'lead', name: 'Laura' }], member_count: 2 },
        { id: 't9', name: 'Ajeno', parent_id: null, external_ref: null, archived_at: null, leaders: [], member_count: 1 }] })
    if (url.startsWith('/api/projects')) return json({ items: [{ id: 'p1', identifier: 'ALF', name: 'Alfa', can: { write_work_items: true } }, { id: 'p2', identifier: 'BET', name: 'Beta', can: { write_work_items: false } }], total: 2 })
    if (url.includes('/move') || url.includes('/release')) return json({ id: 'w1', key: method === 'POST' && url.includes('/move') ? 'ALF-5' : 'TAREA-2' })
    if (url.startsWith('/api/catalog')) return json({ methodologies: [], work_item_types: [], priorities: [{ id: 'pr', name: 'Normal', retired: false }], sizes: [], labels: [], project_statuses: [], statuses: [{ id: 'new', name: 'Nuevo', color: '#000', position: 1, is_closed: false, is_default: true, retired: false }] })
    return json({})
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

async function render(component: object, path: string, me: Me): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component }, { path: '/my-team', component }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(me)
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] }, attachTo: document.body })
  await flushPromises()
  return { wrapper, router }
}
const posts = (path: string) => vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).startsWith(path) && (i as RequestInit | undefined)?.method === 'POST')
const bodyOf = (call: unknown[]) => JSON.parse(String((call[1] as RequestInit).body))
const dialog = () => document.querySelector<HTMLFormElement>('[data-testid=loose-dialog]')
async function type(selector: string, value: string) {
  const el = document.querySelector<HTMLInputElement | HTMLSelectElement>(selector)!
  el.value = value
  el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input'))
  await flushPromises()
}

describe('Mi panel (spec 022)', () => {
  it('creates a task for oneself, without choosing whom, and opens it (RF-1 to RF-4)', async () => {
    const { wrapper, router } = await render(HomePage, '/', member)
    await wrapper.find('[data-testid=new-loose]').trigger('click')
    await flushPromises()
    expect(dialog()).not.toBeNull()
    expect(document.querySelector('[data-testid=loose-to-team]')).toBeNull()
    await type('[data-testid=loose-title]', 'Renovar el certificado')
    await type('[data-testid=loose-due]', '2026-03-20')
    dialog()!.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(bodyOf(posts('/api/loose-tasks')[0]!)).toEqual({ title: 'Renovar el certificado', due_date: '2026-03-20', weight: null, priority_id: null, user_id: 'u1' })
    expect(router.currentRoute.value.query.item).toBe('TAREA-7')
    expect(dialog()).toBeNull()
  })

  it('lets a lead give it to a team they lead, one per person (RF-5, RF-7)', async () => {
    const { wrapper } = await render(HomePage, '/', lead)
    await wrapper.find('[data-testid=new-loose]').trigger('click')
    await flushPromises()
    ;(document.querySelector('[data-testid=loose-to-team]') as HTMLButtonElement).click()
    await flushPromises()
    const options = [...document.querySelectorAll('[data-testid=loose-team] option')].map((o) => o.textContent?.trim())
    expect(options).toEqual(['Elige un equipo', 'Campo'])
    await type('[data-testid=loose-title]', 'Formación')
    await type('[data-testid=loose-team]', 't1')
    ;(document.querySelector('[data-testid=loose-mode-each]') as HTMLInputElement).click()
    await flushPromises()
    expect(document.querySelector('[data-testid=loose-submit]')!.textContent).toContain('Crear una por persona')
    dialog()!.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(bodyOf(posts('/api/loose-tasks')[0]!)).toMatchObject({ title: 'Formación', team_id: 't1', mode: 'each' })
  })

  it('marks the tasks without a project and filters by origin (RF-30, RF-31)', async () => {
    const { wrapper } = await render(HomePage, '/', member)
    const marks = wrapper.findAll('[data-testid=my-task-loose]').map((x) => x.text())
    expect(marks).toEqual(['Sin proyecto', 'Campo · Cada persona la hace'])
    expect(wrapper.find('[data-testid=my-origin-loose]').text()).toBe('Sin proyecto · 2')
    await wrapper.find('[data-testid=my-origin-projects]').trigger('click')
    expect(wrapper.findAll('[data-testid=my-task]')).toHaveLength(1)
    await wrapper.find('[data-testid=my-origin-loose]').trigger('click')
    expect(wrapper.findAll('[data-testid=my-task]')).toHaveLength(2)
  })

  it('shows what to take and takes it, or says somebody was first (RF-18 to RF-20)', async () => {
    const { wrapper } = await render(HomePage, '/', member)
    const card = wrapper.find('[data-testid=claimable-task]')
    expect(card.text()).toContain('Incidencia')
    expect(card.text()).toContain('Publicada por Laura')
    claimStatus = 409
    await card.find('[data-testid=claim]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=claimable]').text()).toContain('Ya la tomó otra persona.')
    expect(posts('/api/work-items/c1/claim')).toHaveLength(1)
  })
})

describe('Mi equipo (spec 022)', () => {
  it('lists the team tasks: who has them and how far each person goes (RF-33 to RF-35)', async () => {
    const { wrapper } = await render(MyTeamPage, '/my-team?tab=tasks', lead)
    const claim = wrapper.find('[data-testid=team-task-claim]')
    expect(claim.find('[data-testid=team-task-untaken]').text()).toBe('Sin tomar · 2 días')
    const each = wrapper.find('[data-testid=team-task-each]')
    expect(each.find('[data-testid=team-task-progress]').text()).toBe('1 de 2')
    expect(each.findAll('li').map((l) => l.text())).toEqual(['Ana', 'Mario'])
    expect(wrapper.find('[data-testid=new-team-task]').exists()).toBe(true)
  })
})

describe('ficha de una tarea sin proyecto (spec 022)', () => {
  const item = (loose: WorkItem['loose']) => ({ id: 'w1', key: 'TAREA-2', version: 3, loose }) as unknown as WorkItem
  async function bar(w: WorkItem) {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }] })
    const wrapper = mount(LooseBar, { props: { item: w }, global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
    await flushPromises()
    return wrapper
  }

  it('takes a team task, or gives it back (RF-19, RF-22)', async () => {
    const take = await bar(item({ team: { id: 't1', name: 'Campo' }, mode: 'claim', group: null, done: 0, total: 0, can: { claim: true, release: false, move: false } }))
    expect(take.find('[data-testid=loose-label]').text()).toBe('Campo · Una persona la toma')
    await take.find('[data-testid=loose-claim]').trigger('click')
    await flushPromises()
    expect(posts('/api/work-items/w1/claim')).toHaveLength(1)
    expect(take.emitted('changed')).toHaveLength(1)

    const back = await bar(item({ team: { id: 't1', name: 'Campo' }, mode: 'claim', group: null, done: 0, total: 0, can: { claim: false, release: true, move: true } }))
    await back.find('[data-testid=loose-release]').trigger('click')
    await flushPromises()
    expect(posts('/api/work-items/w1/release')).toHaveLength(1)
  })

  it('shows the progress of «each person» (RF-24) and moves to a project one can write in (RF-37)', async () => {
    const each = await bar(item({ team: { id: 't1', name: 'Campo' }, mode: 'each', group: null, done: 2, total: 5, can: { claim: false, release: false, move: false } }))
    expect(each.find('[data-testid=loose-progress]').text()).toBe('2 de 5')
    expect(each.find('[data-testid=loose-move]').exists()).toBe(false)

    const own = await bar(item({ team: null, mode: '', group: null, done: 0, total: 0, can: { claim: false, release: false, move: true } }))
    expect(own.find('[data-testid=loose-label]').text()).toBe('Sin proyecto')
    await own.find('[data-testid=loose-move]').trigger('click')
    await flushPromises()
    expect(own.findAll('[data-testid=loose-move-project] option').map((o) => o.text())).toEqual(['Elige un proyecto', 'ALF · Alfa'])
    await own.find('[data-testid=loose-move-project]').setValue('p1')
    await own.find('form').trigger('submit')
    await flushPromises()
    expect(bodyOf(posts('/api/work-items/w1/move')[0]!)).toEqual({ project_id: 'p1', version: 3 })
    expect((own.emitted('changed')![0]![0] as WorkItem).key).toBe('ALF-5')
  })
})
