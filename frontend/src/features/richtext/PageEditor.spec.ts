import { DragHandle } from '@tiptap/extension-drag-handle-vue-3'
import type { Editor } from '@tiptap/vue-3'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createAppI18n } from '@/lib/i18n'
import type { MentionRefs, RichDoc } from '@/types/work'
import PageEditor from './PageEditor.vue'
import { startUploads, uploads } from './uploads'

let wrapper: VueWrapper | null = null
afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
  uploads.clear()
  vi.unstubAllGlobals()
})

async function render(doc: RichDoc, editable = true, refs: MentionRefs = {}) {
  wrapper = mount(PageEditor, {
    props: { doc, editable, refs, owner: { kind: 'work-items', id: 'w1' } },
    global: { plugins: [createAppI18n()] },
    attachTo: document.body,
  })
  await flushPromises()
  const editor = (wrapper.vm as unknown as { editor: Editor }).editor
  return { wrapper, editor }
}

function type(editor: Editor, text: string) {
  for (const ch of text) {
    const { from, to } = editor.state.selection
    const handled = editor.view.someProp('handleTextInput', (f) => f(editor.view, from, to, ch, () => editor.state.tr.insertText(ch, from, to)))
    if (!handled) editor.view.dispatch(editor.state.tr.insertText(ch, from, to))
  }
}

function key(editor: Editor, k: string) {
  editor.view.dom.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }))
}

const options = () => [...document.querySelectorAll('[data-testid=suggestions] [role=option]')].map((o) => o.querySelector('span')?.textContent)
const p = (text: string) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] })

describe('página tipo Notion', () => {
  it('el menú / filtra, se elige con flechas y Enter, y Escape lo cierra sin borrar (RF-7 a RF-9)', async () => {
    const { editor } = await render(null)
    editor.commands.focus('start')
    type(editor, '/')
    await flushPromises()
    expect(options()).toContain('Lista de tareas')
    type(editor, 'lis')
    await flushPromises()
    expect(options()).toEqual(['Lista', 'Lista numerada', 'Lista de tareas'])

    key(editor, 'ArrowDown')
    key(editor, 'ArrowDown')
    key(editor, 'ArrowDown')
    key(editor, 'ArrowUp')
    await nextTick()
    expect(document.querySelector('[aria-selected=true]')?.textContent).toContain('Lista de tareas')
    key(editor, 'Enter')
    await flushPromises()
    expect(JSON.stringify(editor.getJSON())).toContain('"taskList"')
    expect(editor.getText()).not.toContain('/lis')

    type(editor, '/cit')
    await flushPromises()
    expect(options()).toEqual(['Cita'])
    key(editor, 'Escape')
    await flushPromises()
    expect(document.querySelector('[data-testid=suggestions]')).toBeNull()
    expect(editor.getText()).toContain('/cit')
  })

  it('no abre el menú / en mitad de una palabra (RF-7)', async () => {
    const { editor } = await render(p('y'))
    editor.commands.focus('end')
    type(editor, '/')
    await flushPromises()
    expect(document.querySelector('[data-testid=suggestions]')).toBeNull()
  })

  it('el menú del bloque convierte, duplica y borra (RF-12)', async () => {
    const { wrapper, editor } = await render({
      type: 'doc',
      content: [p('uno').content[0], p('dos').content[0]],
    })
    const pointAt = (index: number) => {
      let pos = 0
      for (let i = 0; i < index; i++) pos += editor.state.doc.child(i).nodeSize
      wrapper.findComponent(DragHandle).props('onNodeChange')?.({ node: editor.state.doc.child(index), pos, editor })
    }
    pointAt(0)
    await wrapper.find('[data-testid=block-handle]').trigger('click')
    await wrapper.find('[data-testid=block-menu] button').trigger('click')
    await wrapper.find('[data-testid=convert-heading2]').trigger('click')
    expect(editor.getJSON().content?.[0]).toMatchObject({ type: 'heading', attrs: { level: 2 } })

    pointAt(1)
    await wrapper.find('[data-testid=block-handle]').trigger('click')
    await wrapper.find('[data-testid=block-duplicate]').trigger('click')
    expect(editor.getText({ blockSeparator: '|' })).toBe('uno|dos|dos')

    pointAt(0)
    await wrapper.find('[data-testid=block-handle]').trigger('click')
    await wrapper.find('[data-testid=block-delete]').trigger('click')
    expect(editor.getText({ blockSeparator: '|' })).toBe('dos|dos')
  })

  it('en modo lectura no hay menú, asa ni casillas que marcar (RF-13)', async () => {
    const doc = {
      type: 'doc',
      content: [{ type: 'taskList', content: [{ type: 'taskItem', attrs: { checked: false }, content: [{ type: 'paragraph', content: [{ type: 'text', text: 'x' }] }] }] }],
    }
    const { wrapper, editor } = await render(doc, false)
    expect(wrapper.find('[data-testid=block-handle]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=file-picker]').exists()).toBe(false)
    expect(wrapper.find('li[data-checked] input').attributes('disabled')).toBeDefined()
    type(editor, '/')
    await flushPromises()
    expect(document.querySelector('[data-testid=suggestions]')).toBeNull()
  })

  it('marcar una tarea es un cambio de la página (RF-3)', async () => {
    const doc = {
      type: 'doc',
      content: [{ type: 'taskList', content: [{ type: 'taskItem', attrs: { checked: false }, content: [{ type: 'paragraph', content: [{ type: 'text', text: 'x' }] }] }] }],
    }
    const { wrapper } = await render(doc)
    await wrapper.find('li[data-checked] input').setValue(true)
    const changes = wrapper.emitted('change') as unknown[][]
    expect(JSON.stringify(changes.at(-1)?.[0])).toContain('"checked":true')
  })

  it('las menciones leen sus refs: nombre actual, sin acceso o eliminado (RF-27 a RF-29)', async () => {
    const mention = (id: string, kind: string) => ({ type: 'mention', attrs: { id, kind, label: 'viejo' } })
    const doc = { type: 'doc', content: [{ type: 'paragraph', content: [mention('u1', 'user'), mention('w1', 'work_item'), mention('w2', 'work_item'), mention('w3', 'work_item')] }] }
    const { wrapper } = await render(doc, false, {
      u1: { label: 'Ana Ruiz', state: 'ok' },
      w1: { label: 'KAI-8 Autenticación', state: 'ok' },
      w2: { state: 'no_access' },
      w3: { state: 'deleted' },
    })
    const texts = wrapper.findAll('.mention').map((m) => m.text())
    expect(texts).toEqual(['@Ana Ruiz', '#KAI-8 Autenticación', 'sin acceso', 'eliminado'])
    expect(wrapper.find('.mention a').attributes('href')).toBe('/work-items/KAI-8')
    expect(wrapper.html()).not.toContain('viejo')
  })

  it('una subida muestra su progreso, y si falla, el error con reintentar y quitar (RF-19, RF-20)', async () => {
    const sends: { onload?: () => void; upload: { onprogress?: (e: ProgressEvent) => void }; status: number; response: unknown }[] = []
    vi.stubGlobal(
      'XMLHttpRequest',
      class {
        upload: { onprogress?: (e: ProgressEvent) => void } = {}
        onload?: () => void
        onerror?: () => void
        status = 0
        response: unknown = null
        withCredentials = false
        responseType = ''
        open() {}
        send() {
          sends.push(this)
        }
      },
    )
    const { wrapper, editor } = await render(null)
    startUploads(editor, { kind: 'work-items', id: 'w1' }, [new File(['x'], 'captura.png', { type: 'image/png' })], 0)
    await flushPromises()
    sends[0].upload.onprogress?.({ lengthComputable: true, loaded: 1, total: 4 } as ProgressEvent)
    await flushPromises()
    expect(wrapper.find('[data-testid=file-block]').text()).toContain('25 %')

    sends[0].status = 415
    sends[0].response = { error: { code: 'file_type_not_allowed' } }
    sends[0].onload?.()
    await flushPromises()
    expect(wrapper.find('[data-testid=file-block] [role=alert]').text()).toContain('Tipo de archivo no admitido')

    await wrapper.find('[data-testid=file-retry]').trigger('click')
    sends[1].status = 201
    sends[1].response = { id: 'f1', kind: 'image', name: 'captura.png', size: 1, content_type: 'image/png' }
    sends[1].onload?.()
    await flushPromises()
    expect(wrapper.find('[data-testid=file-block]').exists()).toBe(false)
    expect(editor.getJSON().content?.some((n) => n.type === 'image' && n.attrs?.fileId === 'f1')).toBe(true)
  })

  it('rechaza en el navegador un archivo de más de 10 MB y se puede quitar (RF-16)', async () => {
    const { wrapper, editor } = await render(null)
    const big = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'grande.pdf')
    startUploads(editor, { kind: 'work-items', id: 'w1' }, [big], 0)
    await flushPromises()
    expect(wrapper.find('[data-testid=file-block] [role=alert]').text()).toContain('10 MB')
    await wrapper.find('[data-testid=file-remove]').trigger('click')
    expect(editor.getJSON().content?.some((n) => n.type === 'fileUpload')).toBe(false)
  })
})
