import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { MyTeam, WorkloadBar } from '@/api/workload'
import { createAppI18n } from '@/lib/i18n'
import MyTeamPage from '@/pages/MyTeamPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

// Spec 021: Mi equipo and the schedule by person.

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))
const bar = (id: string, project: string, start: string, end: string, extra: Partial<WorkloadBar> = {}): WorkloadBar => ({
  id, key: `${project}-${id}`, title: `Tarea ${id}`, project: { id: project, identifier: project, name: `Proyecto ${project}` },
  start, end, done_ratio: 50, overdue: false, ...extra,
})
const user = (permissions: string[]) =>
  ({ id: 'lead', name: 'Laura', email: 'l@x.com', status: 'active', auth_method: 'local', has_password: true, roles: [], teams: [], permissions }) as unknown as Me
let team: MyTeam

beforeEach(() => {
  team = {
    today: '2026-03-10',
    teams: [{ id: 't1', name: 'Campo' }, { id: 't2', name: 'Oficina' }],
    people: [
      { user: { id: 'u3', name: 'Eva' }, free_from: null, open: 0, weight: 0, undated: 0, bars: [], teams: [{ id: 't2', name: 'Oficina' }], capacity: null, overloaded: false },
      {
        user: { id: 'u2', name: 'Mario' }, free_from: '2026-04-02', open: 4, weight: 6, undated: 1,
        bars: [bar('1', 'ALF', '2026-03-02', '2026-03-06'), bar('2', 'ALF', '2026-03-04', '2026-03-20'), bar('3', 'OTR', '2026-03-01', '2026-04-01', { overdue: false })],
        teams: [{ id: 't1', name: 'Campo' }], capacity: 4, overloaded: true,
      },
    ],
  }
  vi.stubGlobal('fetch', vi.fn((url: string) => (url.startsWith('/api/me/team') ? json(team) : json({}))))
})
afterEach(() => vi.unstubAllGlobals())

async function render(who: Me) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/my-team', component: MyTeamPage }, { path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push('/my-team')
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(who)
  const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  await flushPromises()
  return { wrapper, router }
}

describe('Mi equipo', () => {
  it('lists the people with their load and links each to their panel (RF-21 to RF-23, RF-14)', async () => {
    const { wrapper } = await render(user(['work:read', 'dashboard:others:team']))
    const rows = wrapper.findAll('[data-testid="my-team-person"]')
    expect(rows.map((r) => r.find('a').text())).toEqual(['Eva', 'Mario'])
    expect(rows[1].find('[data-testid="my-team-load"]').text()).toContain('6 / 4')
    expect(rows[1].text()).toContain('Sobrecargada')
    expect(rows[0].text()).toContain('Libre')
    expect(rows[1].find('a').attributes('href')).toBe('/?user=u2')
  })

  it('draws a row per person, free first, with overlapping bars in lanes and a legend by project (RF-1 to RF-7, RF-25)', async () => {
    const { wrapper } = await render(user(['dashboard:others:all']))
    const lines = wrapper.findAll('[data-testid="workload-row"]')
    expect(lines.map((l) => l.find('[data-testid="workload-person"]').text())).toEqual(['Eva', 'Mario'])
    expect(lines[0].find('[data-testid="workload-free"]').text()).toBe('Libre')
    expect(lines[1].find('[data-testid="workload-free"]').text()).toContain('Libre desde')
    expect(lines[1].find('[data-testid="workload-undated"]').text()).toBe('1 sin fecha')
    const bars = lines[1].findAll('[data-testid="workload-bar"]')
    expect(bars).toHaveLength(3)
    expect(new Set(bars.map((b) => b.attributes('style')!.match(/top: (\d+)px/)![1])).size).toBe(3)
    expect(bars[0].attributes('href')).toBe('/my-team?item=OTR-3')
    expect(wrapper.findAll('[data-testid="workload-legend"] li').map((l) => l.text())).toEqual(['ALF · Proyecto ALF', 'OTR · Proyecto OTR'])
  })

  it('shows the detail of a bar on hover and on focus (RF-8)', async () => {
    const { wrapper } = await render(user(['dashboard:others:all']))
    const b = wrapper.find('[data-testid="workload-bar"]')
    await b.trigger('mouseenter')
    expect(wrapper.find('[data-testid="workload-tip"]').text()).toContain('Tarea 3')
    await b.trigger('mouseleave')
    expect(wrapper.find('[data-testid="workload-tip"]').exists()).toBe(false)
    await b.trigger('focus')
    expect(wrapper.find('[data-testid="workload-tip"]').text()).toContain('Proyecto OTR')
  })

  it('filters by team (RF-26)', async () => {
    const { wrapper } = await render(user(['dashboard:others:all']))
    await wrapper.find('[data-testid="my-team-filter"]').setValue('t2')
    expect(wrapper.findAll('[data-testid="my-team-person"]').map((r) => r.find('a').text())).toEqual(['Eva'])
    expect(wrapper.findAll('[data-testid="workload-row"]')).toHaveLength(1)
  })

  it('says the team has nobody (RF-27)', async () => {
    team = { today: '2026-03-10', teams: [], people: [] }
    const { wrapper } = await render(user(['dashboard:others:team']))
    expect(wrapper.find('[data-testid="my-team-empty"]').exists()).toBe(true)
  })

  it('shows no data without the permissions (RF-20)', async () => {
    const { wrapper } = await render(user(['work:read']))
    expect(wrapper.find('[data-testid="my-team-forbidden"]').exists()).toBe(true)
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u).startsWith('/api/me/team'))).toBe(false)
  })
})
