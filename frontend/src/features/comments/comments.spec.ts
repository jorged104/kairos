import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import type { Editor } from '@tiptap/vue-3'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'
import type { CommentList, CommentThread } from '@/api/comments'
import PageSection from '@/features/pages/PageSection.vue'
import { withoutComments } from '@/features/richtext/commentMark'
import PageEditor from '@/features/richtext/PageEditor.vue'
import { createAppI18n } from '@/lib/i18n'
import CommentsSection from './CommentsSection.vue'

const doc = (text: string) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] })
const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))

const thread = (id: string, text: string, extra: Partial<CommentThread> = {}): CommentThread => ({
  id, author: { id: 'u1', name: 'Ana Pérez' }, body: doc(text), created_at: '2026-10-29T10:00:00Z', edited_at: null,
  deleted: false, can: { edit: true, delete: true }, quote: null, anchored: false, anchor_deleted: false,
  resolved_at: null, resolved_by: null, replies: [], ...extra,
})

let list: CommentList
let fetchMock: ReturnType<typeof vi.fn>
beforeEach(() => {
  list = {
    threads: [
      thread('t1', '¿Usamos SSO?', {
        replies: [{ id: 'r1', author: { id: 'u2', name: 'Luis' }, body: doc('Sí'), created_at: '2026-10-29T11:00:00Z', edited_at: '2026-10-29T11:05:00Z', deleted: false, can: { edit: false, delete: false } }],
      }),
      thread('t2', 'Ya está', { resolved_at: '2026-10-29T12:00:00Z', resolved_by: { id: 'u2', name: 'Luis' } }),
    ],
    refs: {},
    can_comment: true,
  }
  fetchMock = vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === 'POST') {
      list = { ...list, threads: [...list.threads, thread('t3', 'Nuevo')] }
      return json({ comments: list }, 201)
    }
    return json(list)
  })
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

async function mountIn(component: ReturnType<typeof defineComponent>) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component }] })
  await router.push('/')
  const wrapper = mount(RouterView, {
    global: { plugins: [router, createPinia(), createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

const renderSection = () => mountIn(defineComponent({ setup: () => () => h(CommentsSection, { workItemId: 'w1' }) }))

describe('comentarios de la ficha', () => {
  it('muestra los hilos abiertos con sus respuestas y pliega los resueltos (RF-28 a RF-30, RF-36)', async () => {
    const w = await renderSection()
    const threads = w.findAll('[data-testid=comment-thread]')
    expect(threads).toHaveLength(1)
    expect(threads[0]!.text()).toContain('¿Usamos SSO?')
    expect(threads[0]!.text()).toContain('Sí')
    expect(threads[0]!.text()).toContain('(editado)')
    // Only the author's own comment can be edited.
    expect(threads[0]!.findAll('[data-testid=comment-edit]')).toHaveLength(1)
    const toggle = w.find('[data-testid=resolved-toggle]')
    expect(toggle.text()).toContain('1 hilo resuelto')
    await toggle.trigger('click')
    expect(w.findAll('[data-testid=comment-thread]')).toHaveLength(2)
    expect(w.text()).toContain('Resuelto por Luis')
  })

  it('publica un comentario y la lista se actualiza sin recargar (RF-22, RF-43)', async () => {
    const w = await renderSection()
    await w.find('[data-testid=comment-new]').trigger('click')
    const editor = (w.find('[data-testid=comment-editor]').findComponent(PageEditor).vm as unknown as { editor: Editor }).editor
    editor.commands.setContent(doc('Nuevo'), { emitUpdate: true })
    await nextTick()
    await w.find('[data-testid=comment-submit]').trigger('click')
    await flushPromises()
    const [url, init] = fetchMock.mock.calls.find(([, i]) => (i as RequestInit | undefined)?.method === 'POST')!
    expect(url).toBe('/api/work-items/w1/comments')
    expect(JSON.parse(String((init as RequestInit).body)).body.content[0].content[0].text).toBe('Nuevo')
    expect(w.findAll('[data-testid=comment-thread]')).toHaveLength(2)
  })

  it('quien no puede comentar solo lee (RF-24)', async () => {
    list.can_comment = false
    list.threads[0]!.can = { edit: false, delete: false }
    const w = await renderSection()
    expect(w.find('[data-testid=comment-new]').exists()).toBe(false)
    expect(w.find('[data-testid=comment-editor]').exists()).toBe(false)
    expect(w.find('[data-testid=thread-reply]').exists()).toBe(false)
    expect(w.find('[data-testid=thread-resolve]').exists()).toBe(false)
  })
})

describe('comentarios en la página', () => {
  it('al pegar o duplicar no se copia el anclaje (RF-53)', () => {
    const marked = { type: 'paragraph', content: [{ type: 'text', text: 'hola', marks: [{ type: 'bold' }, { type: 'comment', attrs: { threadId: 't1' } }] }] }
    expect(withoutComments(marked)).toEqual({ type: 'paragraph', content: [{ type: 'text', text: 'hola', marks: [{ type: 'bold' }] }] })
  })

  it('comenta un texto seleccionado y lo marca al guardarse, aunque no se pueda editar (RF-44 a RF-46, RF-49)', async () => {
    const anchor = vi.fn(async () => ({ page: null, page_version: 3, version: 9, refs: {} }))
    const commenting = { workItemId: 'w1', canComment: true, openThreads: ['00000000-0000-4000-8000-000000000001'], anchor, show: vi.fn() }
    const w = await mountIn(
      defineComponent({
        setup: () => () =>
          h(PageSection, {
            owner: { kind: 'work-items', id: 'w1' }, doc: doc('Hola mundo'), pageVersion: 2, editable: false,
            reload: async () => ({ description: null, page_version: 1 }), commenting,
          }),
      }),
    )
    // Only open threads are highlighted.
    expect(document.head.innerHTML).toContain('.page-comment[data-comment="00000000-0000-4000-8000-000000000001"]')

    const page = w.findComponent(PageEditor)
    page.vm.$emit('select', { from: 6, to: 11, quote: 'mundo', rect: new DOMRect(10, 10, 40, 16) })
    await nextTick()
    await document.querySelector<HTMLElement>('[data-testid=comment-selection]')!.click()
    await nextTick()
    const popover = document.querySelector('[data-testid=comment-anchor-popover]')!
    expect(popover.textContent).toContain('mundo')

    const composer = w.findAllComponents(PageEditor).find((c) => c.props('compact'))!
    ;(composer.vm as unknown as { editor: Editor }).editor.commands.setContent(doc('¿Qué mundo?'), { emitUpdate: true })
    await nextTick()
    document.querySelector<HTMLElement>('[data-testid=comment-submit]')!.click()
    await flushPromises()

    expect(anchor).toHaveBeenCalledTimes(1)
    const [input] = anchor.mock.calls[0] as unknown as [{ thread_id: string; quote: string; page: unknown; page_version: number }]
    expect(input.quote).toBe('mundo')
    expect(input.page_version).toBe(2)
    expect(JSON.stringify(input.page)).toContain(`"threadId":"${input.thread_id}"`)
    const editor = (page.vm as unknown as { editor: Editor }).editor
    expect(editor.getHTML()).toContain(`data-comment="${input.thread_id}"`)
    expect(editor.getText()).toBe('Hola mundo') // only marked
    expect(document.querySelector('[data-testid=comment-anchor-popover]')).toBeNull()
  })
})
