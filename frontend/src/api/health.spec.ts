import { describe, expect, it, vi } from 'vitest'
import { fetchReadiness, READINESS_TIMEOUT_MS } from './health'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('fetchReadiness', () => {
  it('marca API y BD disponibles con 200', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, { status: 'ok', database: 'up' }))

    expect(await fetchReadiness(fetchImpl)).toEqual({ api: 'available', database: 'available' })
    expect(fetchImpl).toHaveBeenCalledWith('/health/ready', expect.objectContaining({ signal: expect.any(AbortSignal) }))
  })

  it('marca la BD no disponible con 503 database_unavailable', async () => {
    const body = { error: { code: 'database_unavailable', message: 'x' } }
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(503, body))

    expect(await fetchReadiness(fetchImpl)).toEqual({ api: 'available', database: 'unavailable' })
  })

  it('marca la API no disponible y la BD desconocida ante un error de red', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))

    expect(await fetchReadiness(fetchImpl)).toEqual({ api: 'unavailable', database: 'unknown' })
  })

  it.each([
    ['502 del proxy', jsonResponse(502, {})],
    ['503 con otro código', jsonResponse(503, { error: { code: 'other' } })],
    ['503 sin JSON', new Response('oops', { status: 503 })],
  ])('marca la API no disponible ante %s', async (_, res) => {
    expect(await fetchReadiness(vi.fn().mockResolvedValue(res))).toEqual({ api: 'unavailable', database: 'unknown' })
  })

  it('da la consulta por fallida al superar el timeout', async () => {
    // Never resolves on its own; only the abort signal ends it.
    const fetchImpl = vi.fn(
      (_: RequestInfo | URL, init?: RequestInit) =>
        new Promise<Response>((_, reject) => init?.signal?.addEventListener('abort', () => reject(init.signal?.reason))),
    )

    expect(await fetchReadiness(fetchImpl, 10)).toEqual({ api: 'unavailable', database: 'unknown' })
  })

  it('usa un timeout de 5 segundos', () => {
    expect(READINESS_TIMEOUT_MS).toBe(5_000)
  })
})
