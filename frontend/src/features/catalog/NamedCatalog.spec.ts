import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createAppI18n } from '@/lib/i18n'
import NamedCatalog from './NamedCatalog.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const bug = { id: 'ty', name: 'Bug', color: '#ef4444', position: 1, default_weight: 3, retired: false }

function render(weighted: boolean) {
  const fetch = vi.fn((_url: string, _init?: RequestInit) => json({}))
  vi.stubGlobal('fetch', fetch)
  const wrapper = mount(NamedCatalog, {
    props: { kind: weighted ? 'work-item-types' : 'priorities', title: 'Tipos', items: [bug], weighted },
    global: { plugins: [createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    attachTo: document.body,
  })
  return { wrapper, fetch }
}
const body = (fetch: ReturnType<typeof render>['fetch'], method: string) =>
  JSON.parse(String(fetch.mock.calls.find(([, init]) => init?.method === method)?.[1]?.body))

describe('catálogo de tipos', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('edita el peso por defecto de un tipo (RF-150, RF-156)', async () => {
    const { wrapper, fetch } = render(true)
    const cell = wrapper.find('td[aria-label="Peso por defecto"]')
    expect(cell.text()).toBe('3')
    await cell.trigger('click')
    await flushPromises()
    await cell.find('input').setValue('5')
    await cell.find('input').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(body(fetch, 'PATCH')).toMatchObject({ name: 'Bug', color: '#ef4444', position: 1, default_weight: 5 })
    wrapper.unmount()
  })

  it('crea un tipo con su peso por defecto desde la fila rápida', async () => {
    const { wrapper, fetch } = render(true)
    await wrapper.find('[data-testid=quick-add] button').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid=quick-add] input[aria-label="Nuevo tipo"]').setValue('Riesgo')
    await wrapper.find('[data-testid=quick-add] input[type=number]').setValue('0')
    await wrapper.find('[data-testid=quick-add] form').trigger('submit')
    await flushPromises()
    expect(body(fetch, 'POST')).toMatchObject({ name: 'Riesgo', default_weight: 0 })
    wrapper.unmount()
  })

  it('las demás listas no tienen peso', () => {
    const { wrapper } = render(false)
    expect(wrapper.find('td[aria-label="Peso por defecto"]').exists()).toBe(false)
    wrapper.unmount()
  })
})

describe('catálogo de tallas (spec 006)', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('una talla solo tiene nombre y posición, sin color ni peso (RF-1)', async () => {
    const fetch = vi.fn((_url: string, _init?: RequestInit) => json({}))
    vi.stubGlobal('fetch', fetch)
    const m = { id: 'sz', name: 'M', position: 3, retired: false }
    const wrapper = mount(NamedCatalog, {
      props: { kind: 'work-item-sizes', title: 'Tallas', items: [m], sized: true },
      global: { plugins: [createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] },
      attachTo: document.body,
    })
    expect(wrapper.find('td[aria-label=Color]').exists()).toBe(false)
    expect(wrapper.find('td[aria-label=Peso]').exists()).toBe(false)
    expect(wrapper.find('input[type=number]').exists()).toBe(false)
    const cell = wrapper.find('td[aria-label=Nombre]')
    await cell.trigger('click')
    await flushPromises()
    await cell.find('input').setValue('Mediana')
    await cell.find('input').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    const [url, init] = fetch.mock.calls.find(([, i]) => i?.method === 'PATCH')!
    expect(url).toBe('/api/work-item-sizes/sz')
    expect(JSON.parse(String(init!.body))).toEqual({ name: 'Mediana', position: 3 })
    wrapper.unmount()
  })
})
