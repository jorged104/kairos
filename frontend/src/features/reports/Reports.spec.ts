import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { SystemSettings } from '@/api/system'
import AISettings from '@/features/system/AISettings.vue'
import { createAppI18n } from '@/lib/i18n'
import type { Project } from '@/types/work'
import ReportEditor from './ReportEditor.vue'
import ReportsTab from './ReportsTab.vue'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const ndjson = (events: unknown[]) =>
  Promise.resolve(
    new Response(
      new ReadableStream({
        start(c) {
          const enc = new TextEncoder()
          // Split across chunks, as the network does.
          const text = events.map((e) => JSON.stringify(e)).join('\n') + '\n'
          c.enqueue(enc.encode(text.slice(0, 25)))
          c.enqueue(enc.encode(text.slice(25)))
          c.close()
        },
      }),
      { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } },
    ),
  )

const project = { id: 'p1', name: 'Programa', can: { manage: true } } as unknown as Project
const report = {
  id: 'r1', project_id: 'p1', template_id: 't1', template_name: 'Estado del proyecto', title: 'Semana 1',
  author: { id: 'u1', name: 'Ana' }, updated_by: { id: 'u1', name: 'Ana' }, created_at: '2026-11-05T10:00:00Z', updated_at: '2026-11-05T10:00:00Z',
  version: 3, content: '# Informe\n\nBien.', doc: { type: 'doc', content: [] }, can: { manage: true, generate: true },
}

let handler: (url: string, init?: RequestInit) => Promise<Response> | undefined
beforeEach(() => {
  setActivePinia(createPinia())
  handler = () => undefined
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => handler(url, init) ?? json({})))
  vi.stubGlobal('confirm', vi.fn(() => true))
})
afterEach(() => vi.unstubAllGlobals())

const callsTo = (part: string, method = 'GET') =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).includes(part) && ((i as RequestInit | undefined)?.method ?? 'GET') === method)
const bodyOf = (part: string, method: string, n = 0) => JSON.parse(String((callsTo(part, method)[n]![1] as RequestInit).body))

async function render(component: unknown, props: Record<string, unknown> = {}, path = '/') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const w = mount(component as never, {
    props,
    global: { plugins: [router, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return { w, router }
}

describe('servicio de IA en Sistema (spec 020)', () => {
  const settings = { ai: { enabled: true, base_url: 'https://ia.x/v1', model: 'm', api_key_set: true, agent_available: false, ready: false } } as unknown as SystemSettings

  it('avisa de los datos enviados, guarda sin reenviar la clave y dice en qué falló la prueba (RF-8, RF-9, RF-13, RF-14)', async () => {
    handler = (url, init) => {
      if (url.startsWith('/api/system/settings/ai/test')) return json({ error: { code: 'ai_test_failed', message: 'x', details: { step: 'tools' } } }, 400)
      if (url.startsWith('/api/system/settings/ai') && init?.method === 'PUT') return json(settings)
    }
    const { w } = await render(AISettings, { settings })
    expect(w.find('[data-testid=ai-privacy]').text()).toContain('envía al servicio de IA')
    expect(w.find('[data-testid=ai-no-agent]').exists()).toBe(true)
    expect(w.find('[data-testid=ai-api-key]').attributes('placeholder')).toContain('Déjalo vacío')
    await w.find('[data-testid=ai-model]').setValue('otro')
    await w.find('[data-testid=ai-settings]').trigger('submit')
    await flushPromises()
    expect(bodyOf('/api/system/settings/ai', 'PUT')).toEqual({ enabled: true, base_url: 'https://ia.x/v1', model: 'otro', api_key: '' })
    await w.find('[data-testid=ai-test]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=ai-test-failed]').text()).toContain('no admite consultas')
  })
})

describe('pestaña Informes (spec 020)', () => {
  it('lista los informes y solo ofrece generar si se puede (RF-29, RF-30, RF-34)', async () => {
    handler = (url) => (url.startsWith('/api/projects/p1/reports') ? json({ items: [report], can: { manage: false, generate: false } }) : undefined)
    const { w } = await render(ReportsTab, { project })
    const rows = w.findAll('[data-testid=report-row]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Semana 1')
    expect(rows[0]!.text()).toContain('Estado del proyecto · por Ana')
    expect(w.find('[data-testid=report-new]').exists()).toBe(false)
  })

  it('abre un informe guardado sin botones si no lo gestiona (RF-31, RF-34)', async () => {
    handler = (url) => {
      if (url.startsWith('/api/projects/p1/reports')) return json({ items: [report], can: { manage: false, generate: false } })
      if (url.startsWith('/api/reports/r1')) return json({ ...report, can: { manage: false, generate: false } })
    }
    const { w } = await render(ReportsTab, { project }, '/?report=r1')
    expect(w.find('[data-testid=report-title]').text()).toBe('Semana 1')
    expect(w.find('[data-testid=report-edit]').exists()).toBe(false)
    expect(w.find('[data-testid=report-delete]').exists()).toBe(false)
  })
})

describe('generar y editar un informe (spec 020)', () => {
  const can = { manage: true, generate: true }

  it('muestra el plan, la consulta y el texto, y guarda lo generado como «ai» (RF-36 a RF-42, RF-62, RF-70)', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    handler = (url, init) => {
      if (url.startsWith('/api/report-templates')) return json({ items: [{ id: 't1', name: 'Estado del proyecto', description: 'Situación', archived: false }] })
      if (url.startsWith('/api/projects/p1/reports/generate'))
        return gate.then(() =>
          ndjson([
            { type: 'todos', todos: [{ content: 'Leer el árbol', status: 'completed' }, { content: 'Escribir', status: 'in_progress' }] },
            { type: 'tool', tool: 'list_work_items', project: 'Subproyecto' },
            { type: 'text', delta: 'Voy a mirar' },
            { type: 'reset' },
            { type: 'text', delta: '# Informe\n\n' },
            { type: 'text', delta: 'Todo bien.' },
            { type: 'done' },
          ]),
        )
      if (url.startsWith('/api/projects/p1/reports') && init?.method === 'POST') return json({ ...report, id: 'r9', title: 'Estado', content: '# Informe\n\nTodo bien.' }, 201)
    }
    const { w, router } = await render(ReportEditor, { project, can })
    expect((w.find('[data-testid=report-template]').element as HTMLSelectElement).value).toBe('t1')
    await w.find('[data-testid=report-instructions]').setValue('Destaca riesgos')
    await w.find('[data-testid=report-setup]').trigger('submit')
    await flushPromises()
    expect(w.find('[data-testid=report-generating]').exists()).toBe(true)
    expect(bodyOf('/api/projects/p1/reports/generate', 'POST')).toEqual({ template_id: 't1', instructions: 'Destaca riesgos' })
    release()
    await flushPromises()
    await flushPromises()

    const content = w.find('[data-testid=report-content-input]').element as HTMLTextAreaElement
    expect(content.value).toBe('# Informe\n\nTodo bien.')
    expect((w.find('[data-testid=report-title-input]').element as HTMLInputElement).value).toContain('Estado del proyecto · ')
    await w.find('[data-testid=report-save]').trigger('click')
    await flushPromises()
    expect(bodyOf('/api/projects/p1/reports', 'POST', 1)).toMatchObject({ template_id: 't1', content: '# Informe\n\nTodo bien.', origin: 'ai' })
    expect(router.currentRoute.value.query.report).toBe('r9')
  })

  it('enseña el plan y la consulta mientras trabaja (RF-37, RF-38)', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    handler = (url) => {
      if (url.startsWith('/api/report-templates')) return json({ items: [{ id: 't1', name: 'Estado', description: '', archived: false }] })
      if (url.startsWith('/api/projects/p1/reports/generate'))
        return Promise.resolve(
          new Response(
            new ReadableStream({
              async start(c) {
                const enc = new TextEncoder()
                c.enqueue(enc.encode(JSON.stringify({ type: 'todos', todos: [{ content: 'Leer el árbol', status: 'in_progress' }] }) + '\n'))
                c.enqueue(enc.encode(JSON.stringify({ type: 'tool', tool: 'list_milestones', project: 'Programa' }) + '\n'))
                await gate
                c.enqueue(enc.encode(JSON.stringify({ type: 'text', delta: 'x' }) + '\n' + JSON.stringify({ type: 'done' }) + '\n'))
                c.close()
              },
            }),
          ),
        )
    }
    const { w } = await render(ReportEditor, { project, can })
    await w.find('[data-testid=report-setup]').trigger('submit')
    await flushPromises()
    expect(w.find('[data-testid=report-todos]').text()).toContain('Leer el árbol')
    expect(w.find('[data-testid=report-tool]').text()).toBe('Leyendo los hitos de Programa')
    release()
    await flushPromises()
  })

  it('un error a medias conserva el texto recibido (RF-73, RF-74)', async () => {
    handler = (url) => {
      if (url.startsWith('/api/report-templates')) return json({ items: [{ id: 't1', name: 'Estado', description: '', archived: false }] })
      if (url.startsWith('/api/projects/p1/reports/generate'))
        return ndjson([{ type: 'text', delta: '# Medio informe' }, { type: 'error', code: 'ai_quota' }])
    }
    const { w } = await render(ReportEditor, { project, can })
    await w.find('[data-testid=report-setup]').trigger('submit')
    await flushPromises()
    await flushPromises()
    expect(w.find('[data-testid=report-error]').text()).toContain('no tiene cuota')
    expect((w.find('[data-testid=report-content-input]').element as HTMLTextAreaElement).value).toBe('# Medio informe')
  })

  it('pide un cambio, lo deshace y guarda a mano con su versión (RF-55 a RF-60, RF-63, RF-70)', async () => {
    handler = (url, init) => {
      if (url.startsWith('/api/reports/r1') && init?.method === 'PUT') return json({ ...report, version: 4, content: 'mano' })
      if (url.startsWith('/api/reports/r1')) return json(report)
      if (url.startsWith('/api/projects/p1/reports/generate')) return ndjson([{ type: 'text', delta: '# Con riesgos' }, { type: 'done' }])
    }
    const { w } = await render(ReportEditor, { project, can, reportId: 'r1' })
    const content = () => (w.find('[data-testid=report-content-input]').element as HTMLTextAreaElement).value
    expect(content()).toBe('# Informe\n\nBien.')

    await w.find('[data-testid=report-change-input]').setValue('Añade riesgos')
    await w.find('[data-testid=report-change]').trigger('submit')
    await flushPromises()
    await flushPromises()
    expect(bodyOf('/api/projects/p1/reports/generate', 'POST')).toEqual({ draft: '# Informe\n\nBien.', change: 'Añade riesgos', template_id: 't1' })
    expect(content()).toBe('# Con riesgos')
    await w.find('[data-testid=report-undo]').trigger('click')
    expect(content()).toBe('# Informe\n\nBien.')

    await w.find('[data-testid=report-content-input]').setValue('mano')
    await w.find('[data-testid=report-save]').trigger('click')
    await flushPromises()
    expect(bodyOf('/api/reports/r1', 'PUT')).toEqual({ title: 'Semana 1', content: 'mano', origin: 'ai_edited', version: 3 })
  })

  it('si otro guardó antes, deja copiar el texto (RF-64, RF-65)', async () => {
    handler = (url, init) => {
      if (url.startsWith('/api/reports/r1') && init?.method === 'PUT') return json({ error: { code: 'version_conflict', message: 'x' } }, 409)
      if (url.startsWith('/api/reports/r1')) return json(report)
    }
    const writeText = vi.fn(() => Promise.resolve())
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
    const { w } = await render(ReportEditor, { project, can, reportId: 'r1' })
    await w.find('[data-testid=report-content-input]').setValue('mi versión')
    await w.find('[data-testid=report-save]').trigger('click')
    await flushPromises()
    await w.find('[data-testid=report-copy]').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith('mi versión')
  })
})
