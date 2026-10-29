import type { Editor } from '@tiptap/vue-3'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'
import { createAppI18n } from '@/lib/i18n'
import PageEditor from '@/features/richtext/PageEditor.vue'
import PageSection from './PageSection.vue'
import { AUTOSAVE_DELAY_MS } from './usePageAutosave'

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

const doc = (text: string) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] })

// PageSection guards leaving the route, so it is mounted as a route's page.
async function render(props: Record<string, unknown>) {
  const page = defineComponent({ setup: () => () => h(PageSection, props as never) })
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: page }] })
  await router.push('/')
  const wrapper = mount(RouterView, { global: { plugins: [createAppI18n(), router] }, attachTo: document.body })
  await flushPromises()
  return wrapper
}

describe('sección de la página', () => {
  it('ante un conflicto avisa, copia mi texto y recarga la versión del otro (RF-34)', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ error: { code: 'page_conflict' } }), { status: 409 })))
    const writeText = vi.fn(async () => {})
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const reload = vi.fn(async () => ({ description: doc('lo suyo'), page_version: 7, refs: {} }))
    const wrapper = await render({ owner: { kind: 'work-items', id: 'w1' }, doc: doc('inicio'), pageVersion: 1, editable: true, reload })
    const editor = (wrapper.findComponent(PageEditor).vm as unknown as { editor: Editor }).editor
    editor.commands.insertContent(' y lo mío')
    await nextTick()
    expect(wrapper.find('[data-testid=save-status]').text()).toBe('Cambios sin guardar')
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()

    expect(wrapper.find('[data-testid=page-conflict]').text()).toContain('Otra persona cambió esta página')
    expect(editor.isEditable).toBe(false)
    expect(editor.getText()).toContain('lo mío') // kept on screen
    await wrapper.find('[data-testid=copy-mine]').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('lo mío'))

    await wrapper.find('[data-testid=take-theirs]').trigger('click')
    await flushPromises()
    expect(reload).toHaveBeenCalled()
    expect(wrapper.find('[data-testid=page-conflict]').exists()).toBe(false)
    expect(editor.getText()).toBe('lo suyo')
    expect(editor.isEditable).toBe(true)
    wrapper.unmount()
  })

  it('quien no puede editar solo lee, sin estado de guardado (RF-13)', async () => {
    const wrapper = await render({ owner: { kind: 'work-items', id: 'w1' }, doc: doc('texto'), pageVersion: 1, editable: false, reload: vi.fn() })
    expect(wrapper.find('[data-testid=save-status]').exists()).toBe(false)
    expect(wrapper.text()).toContain('texto')
    wrapper.unmount()
  })
})
