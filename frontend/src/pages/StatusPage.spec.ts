import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { READINESS_REFRESH_MS } from '@/features/health/useReadiness'
import { createAppI18n } from '@/lib/i18n'
import StatusPage from './StatusPage.vue'

const ok = () => new Response(JSON.stringify({ status: 'ok', database: 'up' }), { status: 200 })
const dbDown = () =>
  new Response(JSON.stringify({ error: { code: 'database_unavailable', message: 'x' } }), { status: 503 })

function render() {
  const queryClient = new QueryClient()
  return mount(StatusPage, {
    global: { plugins: [createAppI18n(), [VueQueryPlugin, { queryClient }]] },
  })
}

describe('StatusPage', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('consulta al abrir y vuelve a consultar cada 30 s, actualizando sin recargar', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(ok()).mockResolvedValueOnce(dbDown())
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = render()
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid=status-database]').attributes('data-status')).toBe('available')

    await vi.advanceTimersByTimeAsync(READINESS_REFRESH_MS - 1)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid=status-database]').attributes('data-status')).toBe('unavailable')
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
  })

  it('usa un intervalo de 30 segundos', () => {
    expect(READINESS_REFRESH_MS).toBe(30_000)
  })
})
