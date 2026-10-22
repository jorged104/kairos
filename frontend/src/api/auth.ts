import type { AuthConfig, Me } from '@/types/api'
import { api } from './client'

export const fetchAuthConfig = () => api<AuthConfig>('/api/auth/config')

export const fetchMe = () => api<Me>('/api/me')

export const login = (email: string, password: string) =>
  api<Me>('/api/auth/login', { method: 'POST', body: { email, password } })

export const register = (name: string, email: string, password: string) =>
  api<Me>('/api/auth/register', { method: 'POST', body: { name, email, password } })

export const logout = () => api<{ redirect_url: string }>('/api/auth/logout', { method: 'POST' })

export const changeOwnPassword = (currentPassword: string, newPassword: string) =>
  api<void>('/api/me/password', {
    method: 'PUT',
    body: { current_password: currentPassword, new_password: newPassword },
  })

/** URL that starts the SSO login and comes back to returnTo (RF-15, RF-122). */
export function ssoStartUrl(returnTo: string): string {
  return `/api/auth/oidc/start?${new URLSearchParams({ return_to: returnTo })}`
}
