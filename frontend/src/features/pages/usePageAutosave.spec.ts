import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { AUTOSAVE_DELAY_MS, MAX_PAGE_CHARS, usePageAutosave } from './usePageAutosave'

type Autosave = ReturnType<typeof usePageAutosave>

const doc = (text: string) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] })
const ok = (version: number) =>
  new Response(JSON.stringify({ page: null, page_version: version, version: 10 + version, refs: { u: { label: 'Ana', state: 'ok' } } }), { status: 200 })
const fail = (status: number, code: string) => new Response(JSON.stringify({ error: { code, message: '' } }), { status })

let wrapper: VueWrapper | null = null
let fetch: ReturnType<typeof vi.fn>

function host(): Autosave {
  let a!: Autosave
  wrapper = mount(
    defineComponent({
      setup() {
        a = usePageAutosave(() => ({ kind: 'work-items', id: 'w1' }), 1)
        return () => null
      },
    }),
  )
  return a
}
const sentBodies = () => fetch.mock.calls.map(([, init]) => JSON.parse(String((init as RequestInit).body)))

beforeEach(() => {
  vi.useFakeTimers()
  fetch = vi.fn()
  vi.stubGlobal('fetch', fetch)
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('autoguardado de la página', () => {
  it('guarda a los 2 s sin escribir, con la versión de la página (RF-31, RF-33)', async () => {
    fetch.mockResolvedValue(ok(2))
    const a = host()
    a.change(doc('ho'))
    await vi.advanceTimersByTimeAsync(1000)
    a.change(doc('hola'))
    expect(a.status.value).toBe('dirty')
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS - 1)
    expect(fetch).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0][0]).toBe('/api/work-items/w1/page')
    expect(sentBodies()[0]).toEqual({ page: doc('hola'), page_version: 1 })
    expect(a.status.value).toBe('saved')
    expect(a.pageVersion.value).toBe(2)
    expect(a.refs.value.u.label).toBe('Ana')
  })

  it('lo que se escribe mientras guarda se guarda después con la versión nueva', async () => {
    let resolve!: (r: Response) => void
    fetch.mockImplementationOnce(() => new Promise<Response>((r) => (resolve = r))).mockResolvedValue(ok(3))
    const a = host()
    a.change(doc('uno'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    expect(a.status.value).toBe('saving')
    a.change(doc('dos'))
    resolve(ok(2))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()
    expect(sentBodies().map((b) => b.page_version)).toEqual([1, 2])
    expect(a.status.value).toBe('saved')
  })

  it('al salir guarda lo pendiente sin esperar (RF-32)', async () => {
    fetch.mockResolvedValue(ok(2))
    const a = host()
    a.change(doc('rápido'))
    await a.flush()
    expect(fetch).toHaveBeenCalledTimes(1)
    a.change(doc('al cerrar'))
    wrapper?.unmount()
    wrapper = null
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('con un conflicto deja de guardar y conserva el texto (RF-34)', async () => {
    fetch.mockResolvedValue(fail(409, 'page_conflict'))
    const a = host()
    a.change(doc('lo mío'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()
    expect(a.status.value).toBe('conflict')
    expect(a.myText()).toContain('lo mío')
    a.change(doc('más'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 5)
    expect(fetch).toHaveBeenCalledTimes(1)

    a.adopt(5)
    fetch.mockResolvedValue(ok(6))
    a.change(doc('sobre lo suyo'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()
    expect(sentBodies().at(-1)?.page_version).toBe(5)
  })

  it('si falla la red reintenta cada vez más tarde, hasta 30 s (RF-35)', async () => {
    fetch.mockRejectedValueOnce(new TypeError('down')).mockRejectedValueOnce(new TypeError('down')).mockResolvedValue(ok(2))
    const a = host()
    a.change(doc('x'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()
    expect(a.status.value).toBe('error')
    expect(a.errorCode.value).toBe('network_error')
    await vi.advanceTimersByTimeAsync(2000) // first retry
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(3999)
    expect(fetch).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1) // second retry, 4 s later
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(3)
    expect(a.status.value).toBe('saved')
  })

  it('un error del servidor que no es de red no se reintenta', async () => {
    fetch.mockResolvedValue(fail(400, 'invalid_rich_text'))
    const a = host()
    a.change(doc('x'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(a.errorCode.value).toBe('invalid_rich_text')
  })

  it('no envía una página de más de 50 000 caracteres (RF-36)', async () => {
    const a = host()
    a.change(doc('a'.repeat(MAX_PAGE_CHARS + 1)))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 2)
    expect(a.status.value).toBe('too_long')
    expect(fetch).not.toHaveBeenCalled()
  })

  it('no guarda las subidas en curso', async () => {
    fetch.mockResolvedValue(ok(2))
    const a = host()
    a.change({ type: 'doc', content: [{ type: 'paragraph' }, { type: 'fileUpload', attrs: { uploadId: 'x' } }] })
    await a.flush()
    expect(JSON.stringify(sentBodies()[0])).not.toContain('fileUpload')
  })
})
