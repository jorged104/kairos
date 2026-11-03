import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'
import ProjectWorkItems from '@/features/projects/ProjectWorkItems.vue'
import { useFullWidth } from '@/features/pages/useFullWidth'
import AppLayout from '@/layouts/AppLayout.vue'
import { createAppI18n } from '@/lib/i18n'
import type { Catalog, Project, WorkItem, WorkNode } from '@/types/work'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))

const catalog: Catalog = {
  methodologies: [],
  work_item_types: [{ id: 'ty', name: 'Historia', color: '#3b82f6', position: 1, default_weight: 1, retired: false }],
  statuses: [{ id: 'st', name: 'Nuevo', color: '#3b82f6', position: 1, is_closed: false, is_default: true, retired: false }],
  priorities: [
    { id: 'pr', name: 'Normal', color: '#3b82f6', position: 1, is_default: true, retired: false },
    { id: 'pr2', name: 'Alta', color: '#ef4444', position: 2, is_default: false, retired: false },
  ],
  sizes: [
    { id: 'sz-m', name: 'M', position: 3, retired: false },
    { id: 'sz-l', name: 'L', position: 4, retired: true },
  ],
  labels: [{ id: 'lb-f', name: 'Frontend', color: '#22c55e', position: 1, retired: false }]
}
const node: WorkNode = {
  id: 'w1', parent_id: null, key: 'KAI-1', title: 'Login con SSO', type_id: 'ty', status_id: 'st', priority_id: 'pr', assignee: null, assignees: [], size_id: null, label_ids: [],
  derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position: 0, due_date: null, weight: 1,
}
const item: WorkItem = {
  id: 'w1', key: 'KAI-1', sequence_number: 1, project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, type_id: 'ty',
  status_id: 'st', priority_id: 'pr', title: 'Login con SSO', author: { id: 'u', name: 'U' }, assignee: null, assignees: [], size_id: null, label_ids: [],
  description: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Criterios de aceptación' }] }] },
  page_version: 3, refs: {},
  parent: null, start_date: null, due_date: null, estimated_hours: null, remaining_hours: null, done_ratio: 0, weight: 1,
  derived: { estimated_hours: 0, remaining_hours: 0, done_ratio: 0 }, position: 0, depth: 0, child_count: 0, version: 4,
  closed_at: null, custom_values: {}, allowed_status_ids: [], dependencies: [], can: { write: true, delete: true },
}
const project: Project = {
  id: 'p1', identifier: 'KAI', name: 'Kairos', description: null, page_version: 1, team: { id: 't', name: 'BI' }, owner: { id: 'u', name: 'U' },
  methodology: null, parent: null, template_id: null, is_template: false, lifecycle: 'activo', health: 'en_curso', health_explanation: '',
  start_date: null, end_date: null, depth: 0, version: 1, work_item_type_ids: ['ty'], custom_field_ids: [], custom_values: {},
  can: { manage: true, write_work_items: true, manage_template: false },
}

const metrics = {
  today: '2026-03-06',
  schedule: {
    has_plan: true, expected: 60, actual: 20, deviation: -40, state: 'late', plan: 60, real: 20, gap: -40,
    in_baseline: true, base_start: '2026-03-01', base_due: '2026-03-08', changed: true, shift_days: 2,
  },
  contribution: { tracking: false, parent: null, project: { no_weight: false, percent: 60, points: 12 } },
  times: { closed: false, lead_days: null, cycle_days: null, age_days: 10, in_progress_days: null, in_status_days: 2.5, missing: false },
  due: { state: 'overdue', days: 3 },
  reopens: 1,
  blocked: { now: true, blockers: [{ hidden: false, id: 'w9', key: 'KAI-9', title: 'Base' }, { hidden: true }], total_days: 1.5 },
}
const entry = (kind: string, changes: unknown[] = []) => ({ at: '2026-03-01T10:00:00Z', actor: { id: 'u', name: 'Ana' }, kind, changes })
const historyPage1 = {
  entries: [
    entry('updated', [
      { field: 'status_id', from: { id: 's1', name: 'Nuevo' }, to: { id: 's2', name: 'En progreso' } },
      { field: 'label_ids', from: [], to: [{ id: 'lb-x', name: '' }] },
    ]),
    entry('description'),
  ],
  next: 41,
}
const historyPage2 = { entries: [entry('created')], next: null }

// The work item as the API has it now: a PATCH changes it and moves its version.
let current: WorkItem
beforeEach(() => {
  current = item
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'PATCH') {
        current = { ...current, ...JSON.parse(String(init.body)), version: current.version + 1 }
        return json(current)
      }
      if (init?.method === 'DELETE') return Promise.resolve(new Response(null, { status: 204 }))
      if (url.endsWith('/tree')) return json({ items: [node] })
      if (url.startsWith('/api/users')) return json({ items: [{ id: 'u2', name: 'Luis Gómez' }], total: 1, page: 1, page_size: 100 })
      if (url.endsWith('/comments')) return json({ threads: [], refs: {}, can_comment: true })
      if (url.includes('/metrics')) return json(metrics)
      if (url.includes('/history')) return json(url.includes('before=') ? historyPage2 : historyPage1)
      if (url.endsWith('/api/catalog')) return json(catalog)
      if (url.includes('/api/work-items/by-key/KAI-1')) return json(current)
      if (url.endsWith('/api/projects/p1')) return json(project)
      return json({ items: [] })
    }),
  )
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
  useFullWidth().on.value = false
  localStorage.clear()
})

async function renderTree(path = '/projects/p1') {
  const page = defineComponent({ setup: () => () => h(ProjectWorkItems, { project }) })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id', component: page },
      { path: '/work-items/:wikey', component: { template: '<div data-testid="full-page" />' } },
      { path: '/projects/:id/x', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  const wrapper = mount(RouterView, {
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    attachTo: document.body,
  })
  await flushPromises()
  return { wrapper, router }
}

describe('vistas del work item', () => {
  it('se abre en un panel junto al árbol, con la URL ?item= (RF-37)', async () => {
    const { wrapper, router } = await renderTree()
    expect(wrapper.find('[data-testid=item-panel]').exists()).toBe(false)
    await wrapper.find('[data-testid=open-item]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.item).toBe('KAI-1')
    const panel = wrapper.find('[data-testid=item-panel]')
    expect((panel.find('#w-title').element as HTMLInputElement).value).toBe('Login con SSO')
    expect(panel.text()).toContain('Criterios de aceptación')
    expect(wrapper.find('[data-testid=work-item-row]').exists()).toBe(true) // the tree stays
  })

  it('«Abrir como página» lleva a su URL propia (RF-38)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    expect(wrapper.find('[data-testid=open-as-page]').attributes('href')).toBe('/work-items/KAI-1')
  })

  it('Escape cierra el panel, salvo que un menú lo use antes (RF-42)', async () => {
    const { wrapper, router } = await renderTree('/projects/p1?item=KAI-1')
    const menu = document.createElement('div')
    menu.dataset.testid = 'suggestions'
    document.body.appendChild(menu)
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(router.currentRoute.value.query.item).toBe('KAI-1')

    menu.remove()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(router.currentRoute.value.query.item).toBeUndefined()
    expect(wrapper.find('[data-testid=item-panel]').exists()).toBe(false)
  })

  it('el botón de cerrar también cierra el panel', async () => {
    const { wrapper, router } = await renderTree('/projects/p1?item=KAI-1')
    await wrapper.find('[data-testid=close-panel]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.item).toBeUndefined()
  })
})

describe('ficha del work item', () => {
  const calls = (method: string) => vi.mocked(fetch).mock.calls.filter(([, init]) => (init as RequestInit | undefined)?.method === method)

  it('cada propiedad se cambia donde se ve, sin botón de editar', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const panel = wrapper.find('[data-testid=item-panel]')
    expect(panel.findAll('button').some((b) => b.text() === 'Editar')).toBe(false)
    await panel.find('#w-priority').setValue('pr2')
    await flushPromises()
    const [[url, init]] = calls('PATCH')
    expect(url).toBe('/api/work-items/w1')
    const body = JSON.parse(String((init as RequestInit).body))
    expect(body).toMatchObject({ priority_id: 'pr2', title: 'Login con SSO', version: 4 })
    expect(body.description).toBeUndefined() // the page saves itself

    const title = panel.find('#w-title')
    await title.setValue('Login con SAML')
    await title.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(JSON.parse(String((calls('PATCH')[1][1] as RequestInit).body))).toMatchObject({ title: 'Login con SAML', priority_id: 'pr2', version: 5 })
  })

  it('Escape en un campo lo deja como estaba, sin cerrar el panel', async () => {
    const { wrapper, router } = await renderTree('/projects/p1?item=KAI-1')
    await wrapper.find('[data-testid=more-properties]').trigger('click')
    const est = wrapper.find('#w-est')
    await est.setValue('8')
    await est.trigger('keydown', { key: 'Escape' })
    await est.trigger('blur')
    await flushPromises()
    expect((est.element as HTMLInputElement).value).toBe('')
    expect(calls('PATCH')).toHaveLength(0)
    expect(router.currentRoute.value.query.item).toBe('KAI-1')
  })

  it('muestra las propiedades principales y pliega las demás tras «Más propiedades» (RF-68, RF-69)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const props = wrapper.find('[data-testid=properties]')
    for (const id of ['w-status', 'w-assignees', 'w-priority', 'w-start', 'w-due', 'w-size', 'w-weight']) expect(props.find(`#${id}`).exists()).toBe(true)
    for (const id of ['w-type', 'w-parent', 'w-est', 'w-rem', 'w-ratio']) expect(props.find(`#${id}`).exists()).toBe(false)
    const toggle = props.find('[data-testid=more-properties]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(toggle.text()).toBe('Más propiedades (6)')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    for (const id of ['w-type', 'w-parent', 'w-est', 'w-rem', 'w-ratio']) expect(props.find(`#${id}`).exists()).toBe(true)
    expect(props.text()).toContain('Autor')
    await toggle.trigger('click')
    expect(props.find('#w-type').exists()).toBe(false)
  })

  it('la talla fija el peso y no ofrece las retiradas (spec 006, RF-5, RF-9, RF-10)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const size = wrapper.find('#w-size')
    expect(size.findAll('option').map((o) => o.text())).toEqual(['—', 'M'])
    await size.setValue('sz-m')
    await flushPromises()
    expect(JSON.parse(String((calls('PATCH')[0]![1] as RequestInit).body))).toMatchObject({ size_id: 'sz-m' })
  })

  it('asigna a varias personas y guarda al cerrar la lista (spec 006, RF-12, RF-17)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const panel = wrapper.find('[data-testid=item-panel]')
    await panel.find('[data-testid=assignees]').trigger('click')
    const menu = panel.find('[data-testid=assignees-menu]')
    expect(menu.text()).toContain('Luis Gómez')
    await menu.find('input[type=checkbox]').setValue(true)
    expect(calls('PATCH')).toHaveLength(0) // saved when the list closes
    await menu.find('[data-testid=assignees-done]').trigger('click')
    await flushPromises()
    expect(JSON.parse(String((calls('PATCH')[0]![1] as RequestInit).body))).toMatchObject({ assignee_ids: ['u2'], assignee_id: 'u2' })
  })

  it('ordena propiedades, métricas, comentarios, página e historial (spec 006, RF-22; spec 007, RF-5, RF-47)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const html = wrapper.find('[data-testid=item-panel]').html()
    const at = ['properties', 'metrics', 'comments', 'page', 'history'].map((s) => html.indexOf(`data-testid="${s}"`))
    expect(at[0]).toBeGreaterThan(-1)
    expect([...at].sort((a, b) => a - b)).toEqual(at)
  })

  it('pone etiquetas desde las propiedades y guarda al cerrar la lista (spec 007, RF-40, RF-42)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const panel = wrapper.find('[data-testid=item-panel]')
    await panel.find('#w-labels').trigger('click')
    const menu = panel.find('[data-testid=labels-menu]')
    expect(menu.text()).toContain('Frontend')
    await menu.find('input[type=checkbox]').setValue(true)
    expect(calls('PATCH')).toHaveLength(0)
    await menu.find('[data-testid=labels-done]').trigger('click')
    await flushPromises()
    expect(JSON.parse(String((calls('PATCH')[0]![1] as RequestInit).body))).toMatchObject({ label_ids: ['lb-f'] })
    expect(panel.find('#w-labels').text()).toContain('Frontend')
  })

  it('las métricas empiezan plegadas y se recuerda si se abren (spec 007, RF-6 a RF-9, RF-27, RF-31)', async () => {
    const first = await renderTree('/projects/p1?item=KAI-1')
    expect(first.wrapper.find('[data-testid=metrics-list]').exists()).toBe(false)
    expect(vi.mocked(fetch).mock.calls.some(([url]) => String(url).includes('/metrics'))).toBe(false)
    await first.wrapper.find('[data-testid=metrics-toggle]').trigger('click')
    await flushPromises()
    const list = first.wrapper.find('[data-testid=metrics-list]')
    expect(String(vi.mocked(fetch).mock.calls.find(([url]) => String(url).includes('/metrics'))![0])).toMatch(/today=\d{4}-\d{2}-\d{2}/)
    // Against the baseline (spec 009, RF-12, RF-43).
    expect(list.find('[data-testid=metric-schedule]').text()).toContain('Atrasado')
    expect(list.find('[data-testid=metric-schedule]').text()).toContain('Plan 60 % · Real 20 % · Brecha -40 pp')
    expect(list.find('[data-testid=metric-baseline]').text()).toContain('+2 días respecto a la línea base')
    expect(list.find('[data-testid=metric-due]').text()).toContain('Vencida, 3 días de retraso')
    expect(list.find('[data-testid=metric-blocked]').text()).toContain('KAI-9')
    expect(list.find('[data-testid=metric-blocked]').text()).toContain('una tarea sin acceso')
    expect(list.find('[data-testid=metric-times]').text()).toContain('En curso desde hace: —')
    first.wrapper.unmount()
    expect(localStorage.getItem('kairos.sheet.metrics')).toBe('1')
  })

  it('el historial muestra los cambios con nombres y carga más (spec 007, RF-47 a RF-52)', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    await wrapper.find('[data-testid=history-toggle]').trigger('click')
    await flushPromises()
    const entries = () => wrapper.findAll('[data-testid=history-entry]')
    expect(entries()).toHaveLength(2)
    expect(entries()[0]!.text()).toContain('Estado: Nuevo → En progreso')
    expect(entries()[0]!.text()).toContain('Etiquetas: — → (eliminado)')
    expect(entries()[1]!.text()).toContain('editó la descripción')
    await wrapper.find('[data-testid=history-more]').trigger('click')
    await flushPromises()
    expect(String(vi.mocked(fetch).mock.calls.at(-1)![0])).toContain('before=41')
    expect(entries()).toHaveLength(3)
    expect(entries()[2]!.text()).toContain('creó la tarea')
    expect(wrapper.find('[data-testid=history-more]').exists()).toBe(false)
  })

  it('se borra con la papelera de arriba a la izquierda, pulsando dos veces', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const bin = wrapper.find('[data-testid=delete-item]')
    expect(bin.element.parentElement?.firstElementChild).toBe(bin.element)
    await bin.trigger('click')
    expect(calls('DELETE')).toHaveLength(0)
    expect(bin.text()).toContain('Pulsa otra vez')
    await bin.trigger('click')
    await flushPromises()
    expect(calls('DELETE')[0][0]).toBe('/api/work-items/w1')
  })

  it('no muestra hitos ni dependencias', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const panel = wrapper.find('[data-testid=item-panel]')
    expect(panel.text()).not.toContain('Dependencias')
    expect(panel.find('[data-testid=wi-milestone]').exists()).toBe(false)
    expect(vi.mocked(fetch).mock.calls.some(([url]) => String(url).includes('/work-items/w1/milestones'))).toBe(false)
  })

  it('el panel ocupa la mitad y su borde cambia el ancho solo en esta visita', async () => {
    const { wrapper } = await renderTree('/projects/p1?item=KAI-1')
    const panel = () => wrapper.find('[data-testid=item-panel]').element as HTMLElement
    expect(panel().style.getPropertyValue('--panel-w')).toBe('50vw')
    const edge = wrapper.find('[data-testid=panel-resize]')
    await edge.trigger('keydown', { key: 'ArrowLeft' })
    expect(panel().style.getPropertyValue('--panel-w')).toBe(`${Math.round(window.innerWidth / 2) + 32}px`)
    await edge.trigger('dblclick')
    expect(panel().style.getPropertyValue('--panel-w')).toBe('50vw')
  })
})

describe('ancho completo', () => {
  async function renderLayout(fullWidthRoute: boolean) {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: AppLayout, children: [{ path: '', component: { template: '<p>x</p>' }, meta: { fullWidth: fullWidthRoute } }] }],
    })
    await router.push('/')
    return mount(RouterView, { global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  }

  it('oculta la navegación solo en las páginas que lo admiten y se recuerda (RF-40, RF-41)', async () => {
    const fullWidth = useFullWidth()
    const page = await renderLayout(true)
    expect(page.find('header').exists()).toBe(true)
    fullWidth.toggle()
    await nextTick()
    expect(page.find('header').exists()).toBe(false)
    expect(page.find('[data-testid=main]').classes()).not.toContain('max-w-6xl')
    expect(localStorage.getItem('kairos.page.fullWidth')).toBe('1')

    const other = await renderLayout(false)
    expect(other.find('header').exists()).toBe(true) // other screens keep their navigation

    fullWidth.toggle()
    await nextTick()
    expect(page.find('header').exists()).toBe(true)
    expect(localStorage.getItem('kairos.page.fullWidth')).toBe('0')
  })

  it('funciona aunque el navegador no deje guardar la elección', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    const page = await renderLayout(true)
    useFullWidth().toggle()
    await nextTick()
    expect(page.find('header').exists()).toBe(false)
    setItem.mockRestore()
  })
})

describe('páginas de proyecto e hito', () => {
  it('el proyecto tiene su página, editable solo para quien lo gestiona (RF-43)', async () => {
    const { default: ProjectSummary } = await import('@/features/projects/ProjectSummary.vue')
    const render = async (manage: boolean) => {
      const p = { ...project, description: item.description, can: { ...project.can, manage } }
      const page = defineComponent({ setup: () => () => h(ProjectSummary, { project: p }) })
      const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: page }] })
      await router.push('/')
      const wrapper = mount(RouterView, {
        global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
      })
      await flushPromises()
      return wrapper
    }
    const manager = await render(true)
    expect(manager.find('[data-testid=page]').text()).toContain('Criterios de aceptación')
    expect(manager.find('[data-testid=save-status]').exists()).toBe(true)
    const reader = await render(false)
    expect(reader.find('[data-testid=page]').classes()).toContain('page-readonly')
    expect(reader.find('[data-testid=save-status]').exists()).toBe(false)
  })
})
