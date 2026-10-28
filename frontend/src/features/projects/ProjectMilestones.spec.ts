import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createAppI18n } from '@/lib/i18n'
import type { Milestone, Project } from '@/types/work'
import ProjectMilestones from './ProjectMilestones.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))

const milestone: Milestone = {
  id: 'm1', project: { id: 'p1', identifier: 'KAI', name: 'Kairos' }, phase: null, name: 'Entrega', description: null,
  due_date: '2020-01-01', completed_on: null, status: 'abierto', overdue: true, position: 0, version: 1,
  progress: { total: 4, closed: 1, open: 3, percent: 25, estimated_hours: 0, remaining_hours: 0 }, can: { manage: false },
}

function project(manage: boolean, lifecycle: Project['lifecycle'] = 'activo'): Project {
  return {
    id: 'p1', identifier: 'KAI', name: 'Kairos', description: null, team: { id: 't', name: 'BI' }, owner: { id: 'u', name: 'U' },
    methodology: null, parent: null, template_id: null, is_template: false, lifecycle, health: 'en_curso', health_explanation: '',
    start_date: null, end_date: null, depth: 0, version: 1, work_item_type_ids: [], custom_field_ids: [], custom_values: {},
    can: { manage, write_work_items: manage, manage_template: false },
  }
}

async function render(p: Project) {
  vi.stubGlobal('fetch', vi.fn((url: string) => json(url.endsWith('/milestones') ? { items: [milestone] } : { items: [] })))
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  const wrapper = mount(ProjectMilestones, {
    props: { project: p },
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
  })
  await flushPromises()
  return wrapper
}

describe('hitos del proyecto', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('muestra avance y atraso, y el alta solo a quien gestiona un proyecto activo (RF-137)', async () => {
    const viewer = await render(project(false))
    expect(viewer.find('[data-testid=milestone-row]').text()).toContain('Atrasado')
    expect(viewer.find('[data-testid=milestone-row]').text()).toContain('1 de 4 work items cerrados')
    expect(viewer.find('[data-testid=quick-add]').exists()).toBe(false)
    expect(viewer.find('[data-grid-cell]').exists()).toBe(false)

    expect((await render(project(true))).find('[data-testid=quick-add]').exists()).toBe(true)
    expect((await render(project(true, 'cerrado'))).find('[data-testid=quick-add]').exists()).toBe(false)
  })
})
