import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { BaselineComparison, CompareItem } from '@/api/schedule'
import { createAppI18n } from '@/lib/i18n'
import type { Project } from '@/types/work'
import BaselineCompare from './BaselineCompare.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))

const project = { id: 'p1', identifier: 'CMP', name: 'Comparar' } as Project
const v1 = { version: 1, reason: null, created_at: '2026-03-01T10:00:00Z', author: { id: 'u', name: 'Laura' } }
const v2 = { version: 2, reason: 'Retraso', created_at: '2026-03-08T10:00:00Z', author: { id: 'u', name: 'Laura' } }
const row = (r: Partial<CompareItem>): CompareItem => ({
  id: 'w', key: 'CMP-1', title: 'T', change: 'unchanged', before_start: null, before_due: null, after_start: null, after_due: null,
  start_shift: null, due_shift: null, deleted: false, elsewhere: false, hidden: false, ...r,
})
const comparison: BaselineComparison = {
  before: v1, after: v2, versions: [v2, v1],
  summary: { added: 0, removed: 1, moved: 1, unchanged: 1, before_end: '2026-03-10', after_end: '2026-03-15', end_shift: 5 },
  items: [
    row({ id: 'a', key: 'CMP-1', title: 'Se retrasa', change: 'moved', before_due: '2026-03-10', after_due: '2026-03-15', due_shift: 5 }),
    row({ id: null, key: 'CMP-2', title: 'Se borra', change: 'removed', before_due: '2026-03-05', deleted: true }),
    row({ id: null, key: '', title: '', change: 'removed', elsewhere: true, hidden: true }),
    row({ id: 'c', key: 'CMP-3', title: 'Igual' }),
  ],
}

async function render(query: Record<string, string>) {
  const fetch = vi.fn((_url: string) => json(comparison))
  vi.stubGlobal('fetch', fetch)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push({ path: '/projects/p1', query })
  const wrapper = mount(BaselineCompare, {
    props: { project },
    global: { plugins: [router, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
  })
  await flushPromises()
  return { wrapper, fetch, router }
}

describe('comparar líneas base (spec 024)', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('muestra el resumen y oculta las tareas sin cambios, y se filtra por tipo (RF-9 a RF-14, RF-19, RF-22)', async () => {
    const { wrapper } = await render({ tab: 'overview', compare: '1' })
    expect(wrapper.find('[data-testid=compare-count-moved]').text()).toBe('1')
    expect(wrapper.find('[data-testid=compare-end-shift]').text()).toBe('+5 días')
    const rows = wrapper.findAll('[data-testid=compare-row]')
    expect(rows).toHaveLength(3)
    expect(rows[0]!.find('a').attributes('href')).toBe('/work-items/CMP-1')
    expect(rows[1]!.text()).toContain('Borrada')
    expect(rows[1]!.find('a').exists()).toBe(false)
    expect(rows[2]!.text()).toContain('Tarea de otro proyecto')

    await wrapper.find('[data-testid=compare-filter-unchanged]').trigger('click')
    expect(wrapper.findAll('[data-testid=compare-row]')).toHaveLength(4)
  })

  it('guarda los lados en la dirección y no compara un lado consigo mismo (RF-6, RF-8)', async () => {
    const { wrapper, fetch, router } = await render({ tab: 'overview', compare: '1' })
    expect((wrapper.find('[data-testid=compare-from]').element as HTMLSelectElement).value).toBe('1')
    await wrapper.find('[data-testid=compare-to]').setValue('today')
    await flushPromises()
    expect(router.currentRoute.value.query).toMatchObject({ from: '1', to: 'today' })
    expect(String(fetch.mock.calls.at(-1)![0])).toContain('from=1&to=today')

    await wrapper.find('[data-testid=compare-to]').setValue('1')
    await flushPromises()
    expect(wrapper.find('[data-testid=compare-same]').exists()).toBe(true)
    expect(wrapper.find('[data-testid=compare-row]').exists()).toBe(false)
  })
})
