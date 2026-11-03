import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createAppI18n } from '@/lib/i18n'
import MilestonePage from '@/pages/MilestonePage.vue'
import type { Milestone, WorkNode } from '@/types/work'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))

const milestone: Milestone = {
  id: 'm1', project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, phase: null, name: 'Entrega', description: null, page_version: 1,
  due_date: null, completed_on: null, status: 'abierto', overdue: false, position: 0, version: 1,
  progress: { total: 2, closed: 0, open: 2, percent: 0, estimated_hours: 0, remaining_hours: 0, weight: 2, weight_closed: 0 },
  can: { manage: false, link_work_items: true },
}
const node = (id: string, key: string, extra: Partial<WorkNode> = {}): WorkNode => ({
  id, parent_id: null, key, title: `Tarea ${key}`, type_id: 'ty', status_id: 'st', priority_id: 'pr', assignee: null, assignees: [],
  size_id: null, label_ids: [], derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position: 0,
  due_date: null, weight: 1, ...extra,
})
const linked = [{ ...node('w1', 'KAI-1'), inherited: false }, { ...node('w2', 'KAI-2', { parent_id: 'w1' }), inherited: true }]
const found = [node('w1', 'KAI-1'), node('w3', 'KAI-3'), node('w4', 'KAI-4'), node('w5', 'KAI-5')]

let fetchMock: ReturnType<typeof vi.fn>
const posts = (end: string) => fetchMock.mock.calls.filter(([u, i]) => String(u).endsWith(end) && (i as RequestInit | undefined)?.method === 'POST')
const bodyOf = (call: unknown[]) => JSON.parse(String((call[1] as RequestInit).body))

beforeEach(() => {
  fetchMock = vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === 'POST' && url.endsWith('/unlink')) return json({ unlinked: 1 })
    if (init?.method === 'POST') return json({ linked: 2 })
    if (url.endsWith('/api/catalog')) return json({ methodologies: [], work_item_types: [], statuses: [], priorities: [], sizes: [], labels: [] })
    if (url.startsWith('/api/milestones/m1/work-items')) return json({ items: linked })
    if (url.startsWith('/api/milestones/m1')) return json(milestone)
    if (url.startsWith('/api/projects/p1/work-items')) return json({ items: found, total: 120, page: 1, page_size: 50 })
    return json({ items: [] })
  })
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

async function render() {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push('/milestones/m1?add=1')
  const wrapper = mount(MilestonePage, {
    props: { id: 'm1' },
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
  })
  await flushPromises()
  return wrapper
}

describe('asignar tareas a un hito (spec 004)', () => {
  it('busca sin hito, marca varias y las liga de una vez (RF-162 a RF-167)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const wrapper = await render()
    const searches = () => fetchMock.mock.calls.map(([u]) => String(u)).filter((u) => u.startsWith('/api/projects/p1/work-items'))
    expect(searches()[0]).toContain('milestone_id=none')

    await wrapper.find('[data-testid=link-search]').setValue('KAI')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(searches().at(-1)).toContain('q=KAI')
    expect(wrapper.find('[data-testid=link-work-items]').text()).toContain('Se muestran 4 de 120')

    // KAI-1 is already in it, so it can not be marked.
    const checks = wrapper.findAll('[data-testid=link-check]')
    expect(checks[0]!.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('[data-testid=link-result]')[0]!.text()).toContain('En el hito')
    await checks[1]!.setValue(true)
    await checks[2]!.setValue(true)
    expect(wrapper.find('[data-testid=link-add]').text()).toBe('Añadir 2 tareas')
    await wrapper.find('[data-testid=link-add]').trigger('click')
    await flushPromises()
    expect(bodyOf(posts('/api/milestones/m1/work-items')[0]!)).toEqual({ work_item_ids: ['w3', 'w4'] })

    // Marking all the visible ones skips the linked one.
    await wrapper.find('[data-testid=link-all]').setValue(true)
    expect(wrapper.find('[data-testid=link-add]').text()).toBe('Añadir 3 tareas')
  })

  it('quita varias ligadas de una vez; las heredadas no se marcan (RF-168)', async () => {
    const wrapper = await render()
    const marks = wrapper.findAll('[data-testid=milestone-mark]')
    expect(marks).toHaveLength(1) // only the direct one
    await marks[0]!.setValue(true)
    await wrapper.find('[data-testid=milestone-remove-marked]').trigger('click')
    await flushPromises()
    expect(bodyOf(posts('/unlink')[0]!)).toEqual({ work_item_ids: ['w1'] })
  })
})
