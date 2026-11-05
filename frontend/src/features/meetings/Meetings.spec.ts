import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Meeting } from '@/api/meetings'
import { createAppI18n } from '@/lib/i18n'
import PageEditor from '@/features/richtext/PageEditor.vue'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import type { Project } from '@/types/work'
import MeetingSheet from './MeetingSheet.vue'
import MeetingsTab from './MeetingsTab.vue'
import MeetingTemplatesAdmin from './MeetingTemplatesAdmin.vue'
import ProgramMeetings from './ProgramMeetings.vue'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const doc = (text: string) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] })
const meeting = (extra: Partial<Meeting> = {}): Meeting => ({
  id: 'm1', owner: { type: 'project', id: 'p1' }, project: { id: 'p1', identifier: 'KAI', name: 'Kairos' },
  title: 'Seguimiento semanal', starts_at: '2030-01-10T16:00:00Z', location: 'https://meet.example.com/x',
  description: doc('Acta de la reunión'), page_version: 3, refs: {}, version: 2, template_id: null,
  created_at: '2030-01-01T10:00:00Z', updated_at: '2030-01-01T10:00:00Z',
  tasks: [{ id: 'w1', key: 'KAI-7', title: 'Enviar el acta', status_id: 'new' }],
  can: { write: true, comment: true }, ...extra,
})
const project = { id: 'p1', identifier: 'KAI', name: 'Kairos', lifecycle: 'activo', work_item_type_ids: ['act'],
  can: { manage: true, write_work_items: true, manage_template: false } } as unknown as Project
const catalog = {
  methodologies: [], priorities: [], sizes: [], labels: [], project_statuses: [],
  work_item_types: [{ id: 'act', name: 'Actividad', color: '#16a34a', position: 1, default_weight: 1, retired: false }, { id: 'epic', name: 'Épica', color: '#000', position: 0, default_weight: 1, retired: false }],
  statuses: [{ id: 'new', name: 'Nuevo', color: '#000', position: 1, is_closed: false, is_default: true, retired: false }],
}

let current: Meeting
beforeEach(() => {
  setActivePinia(createPinia())
  current = meeting()
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url.startsWith('/api/catalog')) return json(catalog)
    if (url.startsWith('/api/meetings/m1/comments')) return json({ threads: [], refs: {}, can_comment: true })
    if (url.startsWith('/api/meetings/m1') && method === 'PATCH') return json({ ...current, ...JSON.parse(String(init!.body)), version: 3 })
    if (url.startsWith('/api/meetings/m1') && method === 'DELETE') return json(undefined, 204)
    if (url.startsWith('/api/meetings/m1')) return json(current)
    if (url.startsWith('/api/projects/p1/meetings') && method === 'POST') return json(meeting({ id: 'm2' }), 201)
    if (url.startsWith('/api/projects/p1/meetings'))
      return json({ items: [{ id: 'm1', title: 'Seguimiento semanal', starts_at: '2030-01-10T16:00:00Z', location: 'Sala 1', tasks: 2 }], total: 1, page: 1, page_size: 50 })
    if (url.startsWith('/api/projects/p1/work-items') && method === 'POST') return json({ id: 'w9', key: 'KAI-9', title: 'Acta' }, 201)
    if (url.startsWith('/api/projects/p1')) return json(project)
    if (url.startsWith('/api/meeting-templates/global')) return json({ items: [{ id: 'g1', scope: 'global', name: 'Comité', title: 'Comité', location: '', page: null, position: 0, retired: false, project_id: null }] })
    if (url.startsWith('/api/meeting-templates') && method === 'POST') return json({ id: 'g2' }, 201)
    if (url.startsWith('/api/meeting-templates'))
      return json({ items: [
        { id: 'g1', scope: 'global', name: 'Comité', title: 'Comité de seguimiento', location: 'Sala 2', page: null, position: 0, retired: false, project_id: null },
        { id: 't1', scope: 'project', name: 'Diaria', title: 'Diaria', location: '', page: null, position: 0, retired: false, project_id: 'p1' },
      ] })
    return json({})
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})
const callsTo = (part: string, method: string) =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).includes(part) && ((i as RequestInit | undefined)?.method ?? 'GET') === method)
const bodyOf = (part: string, method: string) => JSON.parse(String((callsTo(part, method)[0]![1] as RequestInit).body))

async function render(component: unknown, props: Record<string, unknown> = {}, path = '/') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const w = mount(component as never, {
    props,
    attachTo: document.body,
    global: { plugins: [router, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return { w, router }
}

describe('pestaña Reuniones (spec 018)', () => {
  it('lista las próximas y abre una en el panel (RF-38 a RF-41)', async () => {
    const { w } = await render(MeetingsTab, { project })
    const row = w.find('[data-testid=meeting-row]')
    expect(row.text()).toContain('Seguimiento semanal')
    expect(row.text()).toContain('Sala 1')
    expect(row.text()).toContain('2 tareas')
    expect(row.find('a').attributes('href')).toBe('/?meeting=m1')
    await w.find('[data-testid=meetings-past]').trigger('click')
    await flushPromises()
    expect(callsTo('when=past', 'GET').length).toBeGreaterThan(0)
    w.unmount()
  })

  it('al pulsar una reunión de la lista, la abre editable en el panel (RF-41)', async () => {
    const { w, router } = await render(MeetingsTab, { project }, '/projects/p1?tab=meetings')
    await w.find('[data-testid=meeting-row] a').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.meeting).toBe('m1')
    const sheet = w.find('[data-testid=item-panel] [data-testid=meeting-sheet]')
    expect(sheet.exists()).toBe(true)
    await sheet.find('[data-testid=meeting-title]').setValue('Seguimiento quincenal')
    await sheet.find('[data-testid=meeting-form]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/meetings/m1', 'PATCH')).toMatchObject({ version: 2, title: 'Seguimiento quincenal' })
    w.unmount()
  })

  it('crea una reunión desde una plantilla, que propone título y lugar (RF-34, RF-35)', async () => {
    const { w } = await render(MeetingsTab, { project })
    await w.find('[data-testid=meeting-new]').trigger('click')
    await flushPromises()
    const options = w.findAll('[data-testid=meeting-template] option').map((o) => o.text())
    expect(options).toEqual(['En blanco', 'Comité', 'Diaria'])
    await w.find('[data-testid=meeting-template]').setValue('g1')
    await nextTick()
    expect((w.find('[data-testid=meeting-new-title]').element as HTMLInputElement).value).toBe('Comité de seguimiento')
    await w.find('[data-testid=meeting-new-starts]').setValue('2030-01-10T10:00')
    await w.find('[data-testid=meeting-create]').trigger('submit')
    await flushPromises()
    const body = bodyOf('/api/projects/p1/meetings', 'POST')
    expect(body).toMatchObject({ title: 'Comité de seguimiento', location: 'Sala 2', template_id: 'g1' })
    expect(body.starts_at).toMatch(/Z$/) // with its zone (RF-7)
    w.unmount()
  })
})

describe('ficha de la reunión (spec 018)', () => {
  it('muestra sus datos y tareas, guarda con la versión y borra tras confirmar (RF-1, RF-6, RF-11, RF-15, RF-27)', async () => {
    const { w } = await render(MeetingSheet, { id: 'm1', panel: true })
    expect(w.find('[data-testid=meeting-link]').attributes('href')).toBe('https://meet.example.com/x')
    expect(w.find('[data-testid=meeting-link]').attributes('target')).toBe('_blank')
    expect(w.find('[data-testid=meeting-tasks]').text()).toContain('KAI-7')
    expect(w.find('[data-testid=meeting-tasks]').text()).toContain('Nuevo')
    expect(callsTo('/api/meetings/m1/comments', 'GET').length).toBeGreaterThan(0) // RF-20
    await w.find('[data-testid=meeting-location]').setValue('Sala 3')
    await w.find('[data-testid=meeting-form]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/meetings/m1', 'PATCH')).toMatchObject({ version: 2, title: 'Seguimiento semanal', location: 'Sala 3' })
    await w.find('[data-testid=meeting-delete]').trigger('click')
    expect(callsTo('/api/meetings/m1', 'DELETE')).toHaveLength(0)
    await w.find('[data-testid=meeting-delete]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/meetings/m1', 'DELETE')).toHaveLength(1)
    expect(w.emitted('close')).toBeTruthy()
    w.unmount()
  })

  it('crea una tarea desde el texto seleccionado y la menciona tras él (RF-23 a RF-26)', async () => {
    const { w } = await render(MeetingSheet, { id: 'm1' })
    const editor = w.findComponent(PageEditor)
    editor.vm.$emit('select', { from: 1, to: 5, quote: 'Acta', rect: new DOMRect(10, 10, 40, 10) })
    await nextTick()
    ;(document.querySelector('[data-testid=task-selection]') as HTMLButtonElement).click()
    await nextTick()
    const type = document.querySelector('[data-testid=task-type]') as HTMLSelectElement
    expect([...type.options].map((o) => o.text)).toEqual(['Actividad']) // the project's types
    ;(document.querySelector('[data-testid=task-popover]') as HTMLFormElement).requestSubmit()
    await flushPromises()
    expect(bodyOf('/api/projects/p1/work-items', 'POST')).toEqual({ type_id: 'act', title: 'Acta', meeting_id: 'm1' })
    const saved = (editor.vm as unknown as { editor: { getJSON: () => unknown } }).editor.getJSON()
    expect(JSON.stringify(saved)).toContain('"kind":"work_item"')
    expect(JSON.stringify(saved)).toContain('"id":"w9"')
    w.unmount()
  })

  it('quien no puede escribir solo la lee (RF-8)', async () => {
    current = meeting({ can: { write: false, comment: false } })
    const { w } = await render(MeetingSheet, { id: 'm1' })
    expect(w.find('[data-testid=meeting-form]').exists()).toBe(false)
    expect(w.find('[data-testid=meeting-title-read]').text()).toBe('Seguimiento semanal')
    expect(w.find('[data-testid=meeting-delete]').exists()).toBe(false)
    w.unmount()
  })

  it('se abre en el panel lateral con ?meeting= (RF-41)', async () => {
    const { w } = await render(ItemPanel, {}, '/projects/p1?tab=meetings&meeting=m1')
    expect(w.find('[data-testid=item-panel] [data-testid=meeting-sheet]').exists()).toBe(true)
    w.unmount()
  })
})

describe('plantillas y portafolio (spec 018)', () => {
  it('el catálogo crea y retira plantillas globales (RF-29, RF-31)', async () => {
    const { w } = await render(MeetingTemplatesAdmin)
    expect(w.find('[data-testid=template-row]').text()).toContain('Comité')
    await w.find('[data-testid=template-new]').trigger('click')
    await w.find('[data-testid=template-name]').setValue('Retro')
    await w.find('[data-testid=template-title]').setValue('Retrospectiva')
    await w.find('[data-testid=template-form]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/meeting-templates', 'POST')).toMatchObject({ name: 'Retro', title: 'Retrospectiva', location: '' })
    await w.find('[data-testid=template-retire-g1]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/meeting-templates/g1/retire', 'POST')).toHaveLength(1)
    w.unmount()
  })

  it('el portafolio muestra las próximas 3 reuniones del programa (RF-43)', async () => {
    const { w } = await render(ProgramMeetings, { projectId: 'p1' })
    expect(callsTo('/api/projects/p1/meetings?when=upcoming&page=1&page_size=3', 'GET')).toHaveLength(1)
    expect(w.find('[data-testid=program-meeting] a').attributes('href')).toBe('/meetings/m1')
    expect(w.find('[data-testid=program-meetings-all]').attributes('href')).toBe('/projects/p1?tab=meetings')
    w.unmount()
  })
})
