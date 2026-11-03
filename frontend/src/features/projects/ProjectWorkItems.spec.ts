import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createAppI18n } from '@/lib/i18n'
import type { Catalog, Project, WorkItem, WorkNode } from '@/types/work'
import ProjectWorkItems from './ProjectWorkItems.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))

const catalog: Catalog = {
  methodologies: [],
  work_item_types: [{ id: 'ty', name: 'Bug', color: '#ef4444', position: 1, default_weight: 3, retired: false }],
  statuses: [{ id: 'st', name: 'Nuevo', color: '#3b82f6', position: 1, is_closed: false, is_default: true, retired: false }],
  priorities: [{ id: 'pr', name: 'Normal', color: '#3b82f6', position: 1, is_default: true, retired: false }],
  sizes: [{ id: 'sz-m', name: 'M', position: 3, retired: false }],
  labels: [{ id: 'lb-f', name: 'Frontend', color: '#22c55e', position: 1, retired: false }]
}

const node: WorkNode = {
  id: 'w1', parent_id: null, key: 'KAI-1', title: 'Login', type_id: 'ty', status_id: 'st', priority_id: 'pr', assignee: null, assignees: [], size_id: null, label_ids: [],
  derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position: 0, due_date: null, weight: 3,
}

const item: WorkItem = {
  id: 'w1', key: 'KAI-1', sequence_number: 1, project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, type_id: 'ty',
  status_id: 'st', priority_id: 'pr', title: 'Login', description: null, author: { id: 'u', name: 'U' }, assignee: null, assignees: [], size_id: null, label_ids: [],
  parent: null, start_date: null, due_date: null, estimated_hours: 4, remaining_hours: 4, done_ratio: 20, weight: 3,
  derived: { estimated_hours: 4, remaining_hours: 4, done_ratio: 20 }, position: 0, depth: 0, child_count: 0, version: 7,
  closed_at: null, custom_values: {}, allowed_status_ids: [], dependencies: [], can: { write: true, delete: true },
}

const project: Project = {
  id: 'p1', identifier: 'KAI', name: 'Kairos', description: null, team: { id: 't', name: 'BI' }, owner: { id: 'u', name: 'U' },
  methodology: null, parent: null, template_id: null, is_template: false, lifecycle: 'activo', health: 'en_curso', health_explanation: '',
  start_date: null, end_date: null, depth: 0, version: 1, work_item_type_ids: ['ty'], custom_field_ids: [], custom_values: {},
  can: { manage: true, write_work_items: true, manage_template: false },
}

async function render(extra: Record<string, unknown> = {}) {
  const fetch = vi.fn((url: string, _init?: RequestInit) => {
    if (url.includes('/tree')) return json({ items: [node] })
    if (url.endsWith('/api/catalog')) return json(catalog)
    if (url.endsWith('/api/work-items/w1')) return json(item)
    return json({ items: [] })
  })
  vi.stubGlobal('fetch', fetch)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  const wrapper = mount(ProjectWorkItems, {
    props: { project, ...extra },
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    attachTo: document.body,
  })
  await flushPromises()
  return { wrapper, fetch }
}

describe('árbol de work items', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('edita el peso en la tabla y solo cambia ese campo (RF-156)', async () => {
    const { wrapper, fetch } = await render()
    const cell = wrapper.find('td[aria-label=Peso]')
    expect(cell.text()).toBe('3')
    expect(wrapper.find('th[title]').attributes('title')).toContain('seguimiento')

    await cell.trigger('click')
    await flushPromises()
    await cell.find('input').setValue('0')
    await cell.find('input').trigger('keydown', { key: 'Enter' })
    await flushPromises()

    const patch = fetch.mock.calls.find(([, init]) => init?.method === 'PATCH')
    expect(patch?.[0]).toContain('/api/work-items/w1')
    const body = JSON.parse(String(patch?.[1]?.body))
    expect(body).toMatchObject({ weight: 0, version: 7, title: 'Login', estimated_hours: 4, done_ratio: 20 })
    wrapper.unmount()
  })

  it('edita las etiquetas en su columna (spec 007, RF-43)', async () => {
    const { wrapper, fetch } = await render()
    const row = wrapper.find('[data-testid=work-item-row]')
    await row.find('[data-testid=labels]').trigger('click')
    await row.find('[data-testid=labels-menu] input[type=checkbox]').setValue(true)
    await row.find('[data-testid=labels-done]').trigger('click')
    await flushPromises()
    const patch = fetch.mock.calls.find(([, init]) => init?.method === 'PATCH')
    expect(JSON.parse(String(patch?.[1]?.body))).toMatchObject({ label_ids: ['lb-f'], weight: 3 })

    wrapper.unmount()
  })

  it('con filtros lista las que los cumplen, ordenadas y agrupadas (spec 008, RF-22, RF-56 a RF-58)', async () => {
    const { wrapper, fetch } = await render({ filters: { label_id: ['lb-f'] }, sort: '-due', group: 'status' })
    const call = fetch.mock.calls.map(([url]) => url).find((url) => url.includes('/tree?'))!
    expect(call).toContain('label_id=lb-f')
    expect(call).toContain('sort=-due')
    expect(wrapper.find('[data-testid=table-group-row]').text()).toBe('Nuevo · 1')
    expect(wrapper.find('[data-testid=table-sort]').exists()).toBe(true)
    wrapper.unmount()
  })
})
