import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import NamedCatalog from '@/features/catalog/NamedCatalog.vue'
import ProjectSummary from '@/features/projects/ProjectSummary.vue'
import { createAppI18n } from '@/lib/i18n'
import type { Project } from '@/types/work'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const status = (id: string, name: string, extra = {}) => ({ id, name, color: '#123456', position: 1, is_closed: false, is_initial: false, is_closing: false, retired: false, ...extra })
const statuses = [status('s1', 'Propuesta', { is_initial: true }), status('s2', 'En ejecución'), status('s3', 'Cerrado', { is_closed: true, is_closing: true }), status('s4', 'Viejo', { retired: true })]

afterEach(() => vi.unstubAllGlobals())
const plugins = () => [createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }] as const]
const body = (fetch: ReturnType<typeof vi.fn>, method: string) =>
  JSON.parse(String((fetch.mock.calls.find(([, i]) => (i as RequestInit | undefined)?.method === method)![1] as RequestInit).body))

describe('estados de proyecto (spec 012)', () => {
  it('el catálogo marca el estado inicial y el de cierre, y crea estados abiertos (RF-1 a RF-5)', async () => {
    const fetch = vi.fn((_u: string, _i?: RequestInit) => json({}))
    vi.stubGlobal('fetch', fetch)
    const wrapper = mount(NamedCatalog, {
      props: { kind: 'project-statuses', title: 'Estados de proyecto', items: statuses, closable: true, staged: true },
      global: { plugins: plugins() },
      attachTo: document.body,
    })
    const initial = wrapper.findAll('input[aria-label="Inicial"]')
    expect(initial.map((i) => (i.element as HTMLInputElement).checked)).toEqual([true, false, false, false])
    expect((wrapper.findAll('input[aria-label="De cierre"]')[2]!.element as HTMLInputElement).checked).toBe(true)
    await initial[1]!.setValue(true)
    await flushPromises()
    expect(String(fetch.mock.calls[0]![0])).toBe('/api/project-statuses/s2')
    expect(body(fetch, 'PATCH')).toMatchObject({ name: 'En ejecución', is_initial: true, is_closing: false, is_closed: false })

    await wrapper.find('[data-testid=quick-add] button').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid=quick-add] input[aria-label="Nuevo estado de proyecto"]').setValue('Revisión')
    await wrapper.find('[data-testid=quick-add] form').trigger('submit')
    await flushPromises()
    expect(body(fetch, 'POST')).toMatchObject({ name: 'Revisión', is_closed: false, is_initial: false, is_closing: false })
    wrapper.unmount()
  })

  it('la ficha cambia el estado del proyecto a quien lo gestiona (RF-26)', async () => {
    const fetch = vi.fn((url: string, init?: RequestInit) => {
      if (url.startsWith('/api/catalog'))
        return json({ methodologies: [], work_item_types: [], statuses: [], priorities: [], sizes: [], labels: [], project_statuses: statuses })
      if (init?.method === 'PUT') return json({ ...project, status_id: 's3', lifecycle: 'cerrado', version: 2 })
      return json({ items: [] })
    })
    vi.stubGlobal('fetch', fetch)
    const project = {
      id: 'p1', identifier: 'KAI', name: 'Kairos', description: null, page_version: 1, team: { id: 't', name: 'BI' }, owner: { id: 'u', name: 'U' },
      methodology: null, parent: null, template_id: null, is_template: false, lifecycle: 'activo', health: 'en_curso', health_explanation: '',
      start_date: null, end_date: null, depth: 0, version: 1, work_item_type_ids: [], custom_field_ids: [], custom_values: {},
      can: { manage: true, write_work_items: true, manage_template: false }, status_id: 's2',
    } as unknown as Project
    const wrapper = mount(ProjectSummary, { props: { project }, global: { plugins: plugins(), stubs: { PageSection: true } } })
    await flushPromises()
    const select = wrapper.find('[data-testid=project-status]')
    // Retired statuses are not offered unless the project has one (RF-10).
    expect(select.findAll('option').map((o) => o.text())).toEqual(['Propuesta', 'En ejecución', 'Cerrado'])
    await select.setValue('s3')
    await flushPromises()
    expect(body(fetch, 'PUT')).toEqual({ version: 1, status_id: 's3' })

    const reader = mount(ProjectSummary, {
      props: { project: { ...project, can: { ...project.can, manage: false } } },
      global: { plugins: plugins(), stubs: { PageSection: true } },
    })
    await flushPromises()
    expect(reader.find('[data-testid=project-status]').exists()).toBe(false)
    expect(reader.text()).toContain('En ejecución')
  })
})
