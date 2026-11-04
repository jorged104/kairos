import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Movement, PointsSummary } from '@/api/gamification'
import LevelProgress from '@/features/gamification/LevelProgress.vue'
import PointsHistory from '@/features/gamification/PointsHistory.vue'
import PointsNotice from '@/features/gamification/PointsNotice.vue'
import { createAppI18n } from '@/lib/i18n'
import GamificationAdminPage from '@/pages/GamificationAdminPage.vue'
import RankingPage from '@/pages/RankingPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status }))
const level = (number: number, name: string, min_xp: number) => ({ id: `l${number}`, number, name, min_xp })
const summary = (extra: Partial<PointsSummary> = {}): PointsSummary => ({
  xp: 150, coins: 3, level: level(2, 'Nivel 2', 100), next: level(3, 'Nivel 3', 300), last_seq: 4, ...extra,
})
const movement = (seq: number, extra: Partial<Movement> = {}): Movement => ({
  id: `m${seq}`, seq, xp: 20, coins: 4, reason: 'close', note: null, created_at: '2026-03-10T10:00:00Z',
  work_item: { id: 'w', key: 'KAI-1', title: 'Informe' }, ...extra,
})
const me = (extra: Partial<Me> = {}) =>
  ({ id: 'u1', name: 'Ana', email: 'a@x.com', roles: [], permissions: ['work:read', 'gamification:manage'],
    teams: [{ id: 't1', name: 'BI', source: 'manual' }, { id: 't2', name: 'Reportes', source: 'manual' }], ...extra }) as unknown as Me

type Handler = (url: string, init?: RequestInit) => Promise<Response> | undefined
let handler: Handler
beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => handler(url, init) ?? json({})))
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
const calls = (method: string, prefix: string) =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).startsWith(prefix) && ((i as RequestInit | undefined)?.method ?? 'GET') === method)

async function render(component: unknown, props: Record<string, unknown> = {}, user = me()) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(user)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }] })
  await router.push('/')
  const w = mount(defineComponent({ render: () => h(component as never, props) }), {
    global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return w
}

describe('nivel y puntos (spec 014)', () => {
  it('muestra el nivel, el XP, las monedas y lo que falta (RF-37)', async () => {
    const w = await render(LevelProgress, { points: summary() })
    expect(w.find('[data-testid=level-name]').text()).toBe('Nivel 2 · Nivel 2')
    expect(w.find('[data-testid=points-xp]').text()).toBe('150 XP')
    expect(w.find('[data-testid=points-coins]').text()).toBe('3 monedas')
    expect(w.find('[role=progressbar]').attributes('aria-valuenow')).toBe('25')
    expect(w.find('[data-testid=level-missing]').text()).toBe('Te faltan 150 XP para Nivel 3')
  })

  it('en el nivel más alto no hay barra (RF-38)', async () => {
    const w = await render(LevelProgress, { points: summary({ next: null, coins: 1 }) })
    expect(w.find('[role=progressbar]').exists()).toBe(false)
    expect(w.find('[data-testid=level-max]').exists()).toBe(true)
    expect(w.find('[data-testid=points-coins]').text()).toBe('1 moneda')
  })

  it('lista el historial con enlace, motivo y páginas (RF-39)', async () => {
    handler = (url) => {
      if (url.startsWith('/api/me/points/movements'))
        return json({
          items: [movement(2, { reason: 'adjust', work_item: null, note: 'Migración', xp: -5, coins: 0 }), movement(1)],
          total: 120, page: url.includes('page=2') ? 2 : 1, page_size: 50,
        })
    }
    const w = await render(PointsHistory)
    const rows = w.findAll('[data-testid=points-movement]')
    expect(rows[0]!.text()).toContain('Ajuste')
    expect(rows[0]!.text()).toContain('Migración')
    expect(rows[0]!.text()).toContain('-5')
    expect(rows[1]!.find('[data-testid=points-movement-link]').attributes('href')).toBe('/work-items/KAI-1')
    expect(w.text()).toContain('Página 1 de 3')
    await w.find('[data-testid=points-next]').trigger('click')
    await flushPromises()
    expect(calls('GET', '/api/me/points/movements').some(([u]) => String(u).includes('page=2'))).toBe(true)
  })

  it('avisa de lo ganado al cerrar y de la subida de nivel (RF-41, RF-42)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let current = summary({ xp: 90, level: level(1, 'Nivel 1', 0), next: level(2, 'Aprendiz', 100), last_seq: 3 })
    handler = (url) => {
      if (url.startsWith('/api/me/points/movements')) return json({ items: [movement(5), movement(4, { reason: 'reopen', xp: -10, coins: -2 })], total: 5, page: 1, page_size: 50 })
      if (url.startsWith('/api/me/points')) return json(current)
    }
    const w = await render(PointsNotice)
    expect(w.find('[data-testid=points-notice]').exists()).toBe(false) // the first reading only sets the cursor

    current = summary({ xp: 110, level: level(2, 'Aprendiz', 100), last_seq: 5 })
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(w.find('[data-testid=points-notice-gain]').text()).toBe('+20 XP · +4 monedas')
    expect(w.find('[data-testid=points-notice-level]').text()).toBe('¡Subes a Aprendiz!')
    expect(calls('GET', '/api/me/points/movements')[0]![0]).toContain('after_seq=3')

    await vi.advanceTimersByTimeAsync(6_000)
    await flushPromises()
    expect(w.find('[data-testid=points-notice]').exists()).toBe(false)
  })
})

describe('ranking (spec 014)', () => {
  const ranking = (period: string) => ({
    team: { id: 't1', name: 'BI' }, period, from: period === 'all' ? null : '2026-03-01', to: period === 'all' ? null : '2026-03-31',
    items: [
      { position: 1, user: { id: 'u2', name: 'Mario' }, level: level(3, 'Nivel 3', 300), xp: 40 },
      { position: 2, user: { id: 'u1', name: 'Ana' }, level: level(1, 'Nivel 1', 0), xp: 0 },
    ],
  })
  beforeEach(() => {
    handler = (url) => {
      if (url.startsWith('/api/gamification/teams/')) return json(ranking(new URL(url, 'http://x').searchParams.get('period') ?? 'month'))
    }
  })

  it('muestra el ranking del mes de mi primer equipo y cambia de periodo y equipo (RF-43 a RF-48)', async () => {
    const w = await render(RankingPage)
    expect(calls('GET', '/api/gamification/teams/t1/ranking')[0]![0]).toContain('period=month')
    const rows = w.findAll('[data-testid=ranking-row]')
    expect(rows.map((r) => r.find('[data-testid=ranking-position]').text())).toEqual(['1', '2'])
    expect(rows[0]!.text()).toContain('Nivel 3 · Nivel 3')
    expect(rows[1]!.text()).toContain('tú')
    expect(w.find('[data-testid=ranking-range]').text()).toContain('1 mar')
    await w.find('[data-testid=ranking-period-all]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid=ranking-range]').exists()).toBe(false)
    await w.find('[data-testid=ranking-team] select, select[data-testid=ranking-team]').setValue('t2')
    await flushPromises()
    expect(calls('GET', '/api/gamification/teams/t2/ranking')[0]![0]).toContain('period=all')
  })

  it('sin equipos no hay ranking (RF-51)', async () => {
    const w = await render(RankingPage, {}, me({ teams: [] }))
    expect(w.find('[data-testid=ranking-no-team]').exists()).toBe(true)
    expect(calls('GET', '/api/gamification/teams/')).toHaveLength(0)
  })
})

describe('administración de la gamificación (spec 014)', () => {
  let failLevel = false
  let failAdjust = false
  beforeEach(() => {
    failLevel = false
    failAdjust = false
    handler = (url, init) => {
      const method = init?.method ?? 'GET'
      if (url.startsWith('/api/gamification/values'))
        return json({ no_size: { xp: 5, coins: 1 }, sizes: [{ size_id: 's1', name: 'M', retired: false, xp: 20, coins: 4 }, { size_id: 's2', name: 'XL', retired: true, xp: 80, coins: 16 }] })
      if (url.startsWith('/api/gamification/levels')) {
        if (method === 'POST' && failLevel) return json({ error: { code: 'level_conflict', message: 'x' } }, 409)
        if (method === 'GET') return json({ items: [level(1, 'Nivel 1', 0), level(2, 'Nivel 2', 100)] })
        return json(level(3, 'Nuevo', 200))
      }
      if (url.startsWith('/api/users')) return json({ items: [{ id: 'u1', name: 'Ana' }, { id: 'u2', name: 'Mario' }], total: 2, page: 1, page_size: 100 })
      if (url.startsWith('/api/teams')) return json([])
      if (url.endsWith('/adjustments')) return failAdjust ? json({ error: { code: 'points_negative', message: 'x' } }, 409) : json(summary(), 201)
      if (url.startsWith('/api/gamification/users/')) {
        if (url.includes('/movements')) return json({ items: [], total: 0, page: 1, page_size: 50 })
        return json(summary())
      }
    }
  })

  it('guarda la tabla de puntos, con las tallas retiradas marcadas (RF-24, RF-26)', async () => {
    const w = await render(GamificationAdminPage)
    const rows = w.findAll('[data-testid=point-value-row]')
    expect(rows.map((r) => r.text())).toEqual(['Sin talla', 'M', 'XL · retirada'])
    await rows[1]!.find('[data-testid=point-value-xp]').setValue('25')
    await w.find('[data-testid=point-values]').trigger('submit')
    await flushPromises()
    const body = JSON.parse(String((calls('PUT', '/api/gamification/values')[0]![1] as RequestInit).body))
    expect(body).toEqual({ no_size: { xp: 5, coins: 1 }, sizes: [{ size_id: 's1', xp: 25, coins: 4 }, { size_id: 's2', xp: 80, coins: 16 }] })
  })

  it('protege el primer nivel, crea niveles y muestra el conflicto (RF-30 a RF-32)', async () => {
    const w = await render(GamificationAdminPage)
    const levels = w.findAll('[data-testid=level-row]')
    expect(levels[0]!.find('[data-testid=level-min]').attributes('disabled')).toBeDefined()
    expect(levels[0]!.find('[data-testid=level-delete]').exists()).toBe(false)
    await levels[1]!.find('[data-testid=level-delete]').trigger('click')
    await flushPromises()
    expect(calls('DELETE', '/api/gamification/levels/l2')).toHaveLength(1)

    failLevel = true
    await w.find('[data-testid=level-add-name]').setValue('Experto')
    await w.find('[data-testid=level-add-min]').setValue('100')
    await w.find('[data-testid=level-add]').trigger('submit')
    await flushPromises()
    expect(JSON.parse(String((calls('POST', '/api/gamification/levels')[0]![1] as RequestInit).body))).toEqual({ name: 'Experto', min_xp: 100 })
    expect(w.find('[data-testid=levels-error]').text()).toBe('Ya hay un nivel con ese XP mínimo.')
  })

  it('ajusta los puntos de otra persona, nunca los propios, y muestra el error (RF-18 a RF-21, RF-40)', async () => {
    const w = await render(GamificationAdminPage)
    const pick = w.find('[data-testid=person-pick] select, select[data-testid=person-pick]')
    await pick.setValue('u1')
    await flushPromises()
    expect(w.find('[data-testid=adjust-self]').exists()).toBe(true)
    expect(w.find('[data-testid=adjust-submit]').attributes('disabled')).toBeDefined()

    await pick.setValue('u2')
    await flushPromises()
    expect(w.find('[data-testid=person-points] [data-testid=level-name]').exists()).toBe(true)
    await w.find('[data-testid=adjust-xp]').setValue('-500')
    await w.find('[data-testid=adjust-note]').setValue('Corrección')
    failAdjust = true
    await w.find('[data-testid=adjust-form]').trigger('submit')
    await flushPromises()
    expect(JSON.parse(String((calls('POST', '/api/gamification/users/u2/adjustments')[0]![1] as RequestInit).body))).toEqual({ xp: -500, coins: 0, note: 'Corrección' })
    expect(w.find('[data-testid=adjust-error]').text()).toBe('El ajuste dejaría el saldo por debajo de 0.')
  })
})
