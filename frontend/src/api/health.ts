export type ServiceStatus = 'available' | 'unavailable' | 'unknown'

export interface ReadinessState {
  api: ServiceStatus
  database: ServiceStatus
}

export const READINESS_TIMEOUT_MS = 5_000

const apiDown: ReadinessState = { api: 'unavailable', database: 'unknown' }

function isErrorCode(body: unknown, code: string): boolean {
  if (typeof body !== 'object' || body === null || !('error' in body)) return false
  const error = (body as { error: unknown }).error
  return typeof error === 'object' && error !== null && (error as { code?: unknown }).code === code
}

/**
 * Asks the API whether it and the database are ready. Never throws:
 * timeouts, network errors and unexpected responses mean the API is down.
 */
export async function fetchReadiness(
  fetchImpl: typeof fetch = fetch,
  timeoutMs = READINESS_TIMEOUT_MS,
): Promise<ReadinessState> {
  try {
    const res = await fetchImpl('/health/ready', { signal: AbortSignal.timeout(timeoutMs) })
    if (res.ok) return { api: 'available', database: 'available' }
    if (res.status === 503 && isErrorCode(await res.json(), 'database_unavailable')) {
      return { api: 'available', database: 'unavailable' }
    }
  } catch {
    // Timeout, network failure or a non-JSON body: treated as API down below.
  }
  return apiDown
}
