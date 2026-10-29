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
  priorities: [{ id: 'pr', name: 'Normal', color: '#3b82f6', position: 1, is_default: true, retired: false }],
}
const node: WorkNode = {
  id: 'w1', parent_id: null, key: 'KAI-1', title: 'Login con SSO', type_id: 'ty', status_id: 'st', priority_id: 'pr', assignee: null,
  derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position: 0, due_date: null, weight: 1,
}
const item: WorkItem = {
  id: 'w1', key: 'KAI-1', sequence_number: 1, project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, type_id: 'ty',
  status_id: 'st', priority_id: 'pr', title: 'Login con SSO', author: { id: 'u', name: 'U' }, assignee: null,
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

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url.endsWith('/tree')) return json({ items: [node] })
      if (url.endsWith('/api/catalog')) return json(catalog)
      if (url.includes('/api/work-items/by-key/KAI-1')) return json(item)
      if (url.endsWith('/api/projects/p1')) return json(project)
      return json({ items: [] })
    }),
  )
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
  useFullWidth().on.value = false
})

async function renderTree(path = '/projects/p1') {
  const page = defineComponent({ setup: () => () => h(ProjectWorkItems, { project }) })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id', component: page },
      { path: '/work-items/:wikey', component: { template: '<div data-testid="full-page" />' } },
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
    expect(panel.text()).toContain('Login con SSO')
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
