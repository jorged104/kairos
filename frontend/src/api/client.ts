/** An error answered by the API in the common format. The UI shows it by code (RF-125). */
export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly details: Record<string, string>

  constructor(code: string, status: number, details: Record<string, string> = {}) {
    super(code)
    this.code = code
    this.status = status
    this.details = details
  }
}

let onUnauthenticated: (() => void) | null = null

/** Registers what to do when any request finds the session gone (401). */
export function setUnauthenticatedHandler(handler: () => void) {
  onUnauthenticated = handler
}

interface RequestOptions {
  method?: string
  body?: unknown
  query?: Record<string, string | number | undefined>
}

export function errorFrom(status: number, body: unknown): ApiError {
  if (typeof body === 'object' && body !== null && 'error' in body) {
    const error = (body as { error: { code?: unknown; details?: unknown } }).error
    if (typeof error?.code === 'string') {
      const details = typeof error.details === 'object' && error.details !== null ? (error.details as Record<string, string>) : {}
      return new ApiError(error.code, status, details)
    }
  }
  return new ApiError('unknown', status)
}

export async function api<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let url = path
  if (options.query) {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(options.query)) {
      if (value !== undefined && value !== '') params.set(key, String(value))
    }
    const qs = params.toString()
    if (qs) url += `?${qs}`
  }
  let res: Response
  try {
    res = await fetch(url, {
      method: options.method ?? 'GET',
      credentials: 'same-origin',
      headers: options.body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    })
  } catch {
    throw new ApiError('network_error', 0)
  }
  if (res.status === 204) return undefined as T
  const body: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const error = errorFrom(res.status, body)
    if (res.status === 401 && onUnauthenticated) onUnauthenticated()
    throw error
  }
  return body as T
}
