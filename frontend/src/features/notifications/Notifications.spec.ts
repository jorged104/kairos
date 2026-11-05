import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { AppNotification } from '@/api/notifications'
import { realtimeConnected, startRealtime, stopRealtime } from '@/composables/useRealtime'
import { createAppI18n } from '@/lib/i18n'
import NotificationsPage from '@/pages/NotificationsPage.vue'
import FollowButton from './FollowButton.vue'
import NotificationBell from './NotificationBell.vue'
import NotificationToasts from './NotificationToasts.vue'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const note = (extra: Partial<AppNotification> = {}): AppNotification => ({
  id: 'n1',
  subject: { type: 'work_item', id: 'w1', key: 'KAI-42', title: 'Revisar informe', project_id: 'p1', project_name: 'Kairos' },
  actors: [{ id: 'u2', name: 'Ana' }, { id: 'u3', name: 'Luis' }],
  actors_total: 4,
  events: 3,
  mention: false,
  last: { kind: 'comment', comment_id: 'c9', excerpt: 'Faltan las cifras' },
  via: null,
  read_at: null,
  updated_at: '2026-11-05T10:00:00Z',
  ...extra,
})

let unread = 3
let follow = true
beforeEach(() => {
  unread = 3
  follow = true
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    if (url.startsWith('/api/notifications/unread')) return json({ unread })
    if (url.startsWith('/api/notifications/read-all')) return json(undefined, 204)
    if (url.includes('/read')) return json(undefined, 204)
    if (url.startsWith('/api/notifications')) return json({ items: [note()], total: 1, unread, page: 1, page_size: 20 })
    if (url.endsWith('/follow')) {
      if (init?.method === 'PUT') follow = JSON.parse(String(init.body)).following
      return json({ following: follow })
    }
    return json({})
  }))
})
afterEach(() => {
  stopRealtime()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
const callsTo = (part: string, method = 'GET') =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).includes(part) && ((i as RequestInit | undefined)?.method ?? 'GET') === method)

async function render(component: unknown, path = '/', props: Record<string, unknown> = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/work-items/:wikey', component: { template: '<div />' } },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  const w = mount(component as never, {
    props,
    attachTo: document.body,
    global: { plugins: [router, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return { w, router }
}

// A WebSocket that the test drives.
class FakeSocket {
  static last: FakeSocket | null = null
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  constructor(public url: string) {
    FakeSocket.last = this
  }
  close() {
    this.onclose?.()
  }
  send(m: unknown) {
    this.onmessage?.({ data: JSON.stringify(m) })
  }
}

describe('campana (spec 017)', () => {
  it('cuenta las no leídas, despliega las últimas y las marca todas (RF-26, RF-27, RF-43)', async () => {
    unread = 120
    const { w } = await render(NotificationBell)
    expect(w.find('[data-testid=notification-count]').text()).toBe('99+')
    await w.find('[data-testid=notification-bell]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/notifications?page_size=20')).toHaveLength(1)
    const item = w.find('[data-testid=notification]')
    expect(item.text()).toContain('Ana, Luis y 2 más comentaron')
    expect(item.text()).toContain('KAI-42 Revisar informe')
    expect(item.text()).toContain('«Faltan las cifras»')
    expect(item.text()).toContain('3 novedades')
    await w.find('[data-testid=notification-read-all]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/notifications/read-all', 'POST')).toHaveLength(1)
    w.unmount()
  })

  it('al pulsar una la marca leída y lleva al comentario (RF-39 a RF-41)', async () => {
    const { w, router } = await render(NotificationBell)
    await w.find('[data-testid=notification-bell]').trigger('click')
    await flushPromises()
    await w.find('[data-testid=notification-open]').trigger('click')
    await flushPromises()
    const [put] = callsTo('/api/notifications/n1/read', 'PUT')
    expect(JSON.parse(String((put![1] as RequestInit).body))).toEqual({ read: true })
    expect(router.currentRoute.value.fullPath).toBe('/work-items/KAI-42?comment=c9')
    w.unmount()
  })
})

describe('tiempo real (spec 017)', () => {
  it('actualiza el contador y muestra un aviso, salvo en la ficha abierta (RF-30, RF-31, RF-33)', async () => {
    vi.stubGlobal('WebSocket', FakeSocket)
    startRealtime()
    FakeSocket.last!.onopen!()
    expect(realtimeConnected.value).toBe(true)
    // One app: the bell and the toasts share the cache.
    const both = { components: { NotificationBell, NotificationToasts }, template: '<div><NotificationBell /><NotificationToasts /></div>' }
    const { w, router } = await render(both)
    FakeSocket.last!.send({ type: 'notifications', unread: 7, notification: note() })
    await flushPromises()
    expect(w.find('[data-testid=notification-count]').text()).toBe('7')
    expect(w.findAll('[data-testid=toast]')).toHaveLength(1)
    expect(w.find('[data-testid=toasts]').attributes('aria-live')).toBe('polite')

    await router.push('/work-items/KAI-50')
    FakeSocket.last!.send({ type: 'notifications', unread: 8, notification: note({ id: 'n2', subject: { ...note().subject, id: 'w2', key: 'KAI-50' } }) })
    await flushPromises()
    expect(w.findAll('[data-testid=toast]')).toHaveLength(1)
    w.unmount()
  })

  it('pliega los avisos si llegan más de 3 y los quita a los 6 s (RF-30, RF-32)', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', FakeSocket)
    startRealtime()
    const { w } = await render(NotificationToasts)
    for (let i = 1; i <= 4; i++) {
      FakeSocket.last!.send({ type: 'notifications', unread: i, notification: note({ id: `n${i}`, subject: { ...note().subject, id: `w${i}`, key: `KAI-${i}` } }) })
    }
    await flushPromises()
    expect(w.findAll('[data-testid=toast]')).toHaveLength(1)
    expect(w.text()).toContain('4 notificaciones nuevas')
    vi.advanceTimersByTime(6000)
    await flushPromises()
    expect(w.findAll('[data-testid=toast]')).toHaveLength(0)
    w.unmount()
  })

  it('reconecta esperando cada vez más (RF-36)', () => {
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', FakeSocket)
    startRealtime()
    const first = FakeSocket.last!
    first.close()
    expect(FakeSocket.last).toBe(first)
    vi.advanceTimersByTime(1000)
    const second = FakeSocket.last!
    expect(second).not.toBe(first)
    second.close()
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.last).toBe(second)
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.last).not.toBe(second)
  })
})

describe('página y seguir (spec 017)', () => {
  it('filtra las no leídas (RF-28)', async () => {
    const { w } = await render(NotificationsPage)
    await w.find('[data-testid=filter-unread]').trigger('click')
    await flushPromises()
    expect(callsTo('unread=true').length).toBeGreaterThan(0)
    w.unmount()
  })

  it('sigue o deja de seguir una tarea (RF-2 a RF-4)', async () => {
    const { w } = await render(FollowButton, '/', { workItemId: 'w1' })
    expect(w.find('[data-testid=follow]').text()).toBe('Siguiendo')
    await w.find('[data-testid=follow]').trigger('click')
    await flushPromises()
    const [put] = callsTo('/api/work-items/w1/follow', 'PUT')
    expect(JSON.parse(String((put![1] as RequestInit).body))).toEqual({ following: false })
    expect(w.find('[data-testid=follow]').text()).toBe('Seguir')
    w.unmount()
  })
})
