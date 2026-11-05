import { api } from './client'

// Invitations and links to a password (spec 019). The secret of a link
// travels in the body, never in a URL the server sees (RF-39).

/** Whether an invitation or a link went out by mail, and if not, the link (RF-13, RF-33). */
export interface Delivery {
  sent: boolean
  url?: string
  expires_at?: string
  /** "password": a link to create or reset one; "login": the notice of SSO only (RF-15). */
  kind: 'password' | 'login'
}

export interface LinkInfo {
  purpose: 'invite' | 'reset'
  email: string
  name: string
}

export const resendInvitation = (userId: string) => api<Delivery>(`/api/users/${userId}/invitation`, { method: 'POST' })

export const sendPasswordLink = (userId: string) => api<Delivery>(`/api/users/${userId}/password-link`, { method: 'POST' })

export const forgotPassword = (email: string) => api<void>('/api/auth/password/forgot', { method: 'POST', body: { email } })

export const checkLink = (token: string) => api<LinkInfo>('/api/auth/password/check', { method: 'POST', body: { token } })

export const resetPassword = (token: string, password: string) =>
  api<void>('/api/auth/password/reset', { method: 'POST', body: { token, password } })
