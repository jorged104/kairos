import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { McpConnection } from '@/api/mcp'
import type { SystemSettings } from '@/api/system'
import { createAppI18n } from '@/lib/i18n'
import AccountPage from '@/pages/AccountPage.vue'
import SystemPage from '@/pages/SystemPage.vue'
import { useAuthStore } from '@/stores/auth'
import type { Me } from '@/types/api'
import CommentItem from '@/features/comments/CommentItem.vue'
import type { WorkComment } from '@/api/comments'

const json = (body: unknown, status = 200) => Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
const me = { id: 'u1', name: 'Ana', email: 'a@x.com', roles: [], teams: [], permissions: ['system:manage', 'work:read'], theme_mode: 'system' } as unknown as Me
const connection = (extra: Partial<McpConnection> = {}): McpConnection => ({
  id: 'c1', name: 'Claude del portátil', created_at: '2026-11-01T10:00:00Z', expires_at: '2027-01-30T10:00:00Z',
  last_used_at: '2026-11-04T10:00:00Z', last_client: 'claude-ai', ...extra,
})
const settings = { mcp: { enabled: true } } as unknown as SystemSettings

type Handler = (url: string, init?: RequestInit) => Promise<Response> | undefined
let handler: Handler
let enabled: boolean
beforeEach(() => {
  enabled = true
  handler = () => undefined
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => handler(url, init) ?? defaults(url, init)))
})
afterEach(() => vi.unstubAllGlobals())

function defaults(url: string, init?: RequestInit) {
  if (url.startsWith('/api/me/mcp-connections')) {
    if (init?.method === 'POST') return json({ ...connection({ id: 'c2', name: 'Claude Code', last_used_at: null, last_client: null }), url: 'https://kairos.x/api/mcp/s3cr3t' }, 201)
    if (init?.method === 'DELETE') return json(undefined, 204)
    return json({ enabled, max: 10, items: [connection()] })
  }
  if (url.startsWith('/api/system/mcp-connections')) {
    if (init?.method === 'DELETE') return json(undefined, 204)
    return json({ items: [connection({ user: { id: 'u2', name: 'Bruno', email: 'b@x.com' } })] })
  }
  if (url.startsWith('/api/system/settings')) return json(settings)
  if (url.startsWith('/api/auth/config')) return json({ local_enabled: true, oidc_enabled: false, registration_enabled: false })
  if (url.startsWith('/api/me/points/movements')) return json({ items: [], total: 0, page: 1, page_size: 50 })
  if (url.startsWith('/api/me/points')) return json({ xp: 0, coins: 0, level: { id: 'l1', number: 1, name: 'Nivel 1', min_xp: 0 }, next: null, last_seq: 0 })
  return json({})
}
const callsTo = (prefix: string, method: string) =>
  vi.mocked(fetch).mock.calls.filter(([u, i]) => String(u).startsWith(prefix) && (i as RequestInit | undefined)?.method === method)

async function render(component: unknown, path: string) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().set(me)
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: component as never }] })
  await router.push(path)
  const w = mount({ template: '<RouterView />' }, {
    global: { plugins: [router, pinia, createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  await flushPromises()
  return w
}

describe('conexiones de IA en Mi cuenta (spec 016)', () => {
  it('crea una conexión y enseña su URL una sola vez con el aviso (RF-8 a RF-11, RF-17, RF-18)', async () => {
    const w = await render(AccountPage, '/account')
    expect(w.find('[data-testid=mcp-steps]').exists()).toBe(true)
    expect(w.find('[data-testid=mcp-connection]').text()).toContain('Claude del portátil')
    expect(w.find('[data-testid=mcp-connection]').text()).toContain('claude-ai')
    await w.find('[data-testid=mcp-name]').setValue('Claude Code')
    await w.find('[data-testid=mcp-days]').setValue('30')
    await w.find('[data-testid=mcp-form]').trigger('submit')
    await flushPromises()
    const [post] = callsTo('/api/me/mcp-connections', 'POST')
    expect(JSON.parse(String((post![1] as RequestInit).body))).toEqual({ name: 'Claude Code', days: 30 })
    expect((w.find('[data-testid=mcp-url]').element as HTMLInputElement).value).toBe('https://kairos.x/api/mcp/s3cr3t')
    expect(w.find('[data-testid=mcp-warning]').text()).toContain('No la compartas')
    await w.find('[data-testid=mcp-done]').trigger('click')
    expect(w.find('[data-testid=mcp-url]').exists()).toBe(false)
  })

  it('revoca tras confirmar (RF-20)', async () => {
    const w = await render(AccountPage, '/account')
    await w.find('[data-testid=mcp-revoke-c1]').trigger('click')
    expect(callsTo('/api/me/mcp-connections/c1', 'DELETE')).toHaveLength(0)
    await w.find('[data-testid=mcp-revoke-c1]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/me/mcp-connections/c1', 'DELETE')).toHaveLength(1)
  })

  it('con el acceso desactivado avisa y no deja crear (RF-4)', async () => {
    enabled = false
    const w = await render(AccountPage, '/account')
    expect(w.find('[data-testid=mcp-disabled]').exists()).toBe(true)
    expect(w.find('[data-testid=mcp-form]').exists()).toBe(false)
  })
})

describe('IA en Sistema (spec 016)', () => {
  it('activa o desactiva el acceso y revoca conexiones de otros (RF-2, RF-23, RF-24)', async () => {
    const w = await render(SystemPage, '/admin/system?tab=mcp')
    expect(w.find('[data-testid=mcp-connection]').text()).toContain('Bruno')
    await w.find('[data-testid=mcp-enabled]').setValue(false)
    await flushPromises()
    const [put] = callsTo('/api/system/settings/mcp', 'PUT')
    expect(JSON.parse(String((put![1] as RequestInit).body))).toEqual({ enabled: false })
    await w.find('[data-testid=mcp-revoke-c1]').trigger('click')
    await w.find('[data-testid=mcp-revoke-c1]').trigger('click')
    await flushPromises()
    expect(callsTo('/api/system/mcp-connections/c1', 'DELETE')).toHaveLength(1)
  })
})

describe('marca «vía IA» (spec 016)', () => {
  it('un comentario de una conexión lo dice, uno de la interfaz no (RF-44)', () => {
    const comment: WorkComment = {
      id: 'k1', author: { id: 'u1', name: 'Ana' }, body: { type: 'doc', content: [{ type: 'paragraph' }] },
      created_at: '2026-11-05T10:00:00Z', edited_at: null, deleted: false, can: { edit: false, delete: false },
      via: { connection_id: 'c1', name: 'Claude del portátil', client: 'claude-ai' },
    }
    const props = { refs: {}, workItemId: 'w1', apply: () => {} }
    const plugins = { global: { plugins: [createAppI18n()] } }
    const via = mount(CommentItem, { props: { comment, ...props }, ...plugins })
    expect(via.find('[data-testid=via-ai]').text()).toBe('vía IA · Claude del portátil')
    const ui = mount(CommentItem, { props: { comment: { ...comment, via: null }, ...props }, ...plugins })
    expect(ui.find('[data-testid=via-ai]').exists()).toBe(false)
  })
})
