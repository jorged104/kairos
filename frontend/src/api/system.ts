import type { Theme } from '@/lib/theme'
import { api } from './client'

// Settings of the system (spec 015). The secrets never come back: only
// whether they are set (RF-6).

export type AuthMode = 'local' | 'oidc' | 'both'

export interface SSOTest {
  ok: boolean
  email?: string
  name?: string
  error?: string
  at: string
}

export interface SystemSettings {
  version: number
  access: { mode: AuthMode; registration_enabled: boolean; default_role: '' | 'viewer' | 'member' | 'lead' }
  sso: {
    issuer: string
    discovery_url: string
    client_id: string
    client_secret_set: boolean
    redirect_url: string
    configured: boolean
    test: SSOTest | null
    tested: boolean
  }
  storage: {
    kind: 'db' | 's3'
    endpoint: string
    bucket: string
    region: string
    access_key: string
    secret_key_set: boolean
    use_ssl: boolean
    files_count: number
    locked: boolean
  }
  appearance: Theme
  emergency_available: boolean
  emergency_email: string
}

export const getSystemSettings = () => api<SystemSettings>('/api/system/settings')

const put = (section: string, body: unknown) => api<SystemSettings>(`/api/system/settings/${section}`, { method: 'PUT', body })

export const saveAccess = (body: SystemSettings['access'] & { accept_no_emergency?: boolean }) => put('access', body)

export const saveSSO = (body: { issuer: string; discovery_url: string; client_id: string; client_secret: string }) => put('sso', body)

export const saveStorage = (body: Omit<SystemSettings['storage'], 'secret_key_set' | 'files_count' | 'locked'> & { secret_key: string }) =>
  put('storage', body)

export const saveAppearance = (body: Theme) => put('appearance', body)

/** Where the browser goes to test the SSO; it comes back to the SSO tab (RF-28). */
export const ssoTestUrl = '/api/auth/oidc/start?test=1'

export const setThemeMode = (theme_mode: 'light' | 'dark' | 'system') =>
  api<void>('/api/me/preferences', { method: 'PUT', body: { theme_mode } })
