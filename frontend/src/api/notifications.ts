import type { Ref } from '@/types/work'
import type { Via } from './mcp'
import { api } from './client'

// Notifications (spec 017): one per element while unread, with the events
// joined in it (RF-21 to RF-24).

export type NotificationKind = 'comment' | 'reply' | 'mention' | 'status' | 'thread_resolved' | 'thread_reopened'

export interface AppNotification {
  id: string
  subject: {
    type: 'work_item' | 'milestone' | 'project' | 'meeting'
    id: string
    key?: string
    title: string
    project_id: string
    project_name: string
  }
  actors: Ref[]
  actors_total: number
  events: number
  mention: boolean
  last: { kind: NotificationKind; comment_id?: string; excerpt?: string; from?: string; to?: string }
  via: Via | null
  read_at: string | null
  updated_at: string
}

export interface NotificationPage {
  items: AppNotification[]
  total: number
  unread: number
  page: number
  page_size: number
}

/** What the realtime connection brings: the new count and, for a new event, its notification. */
export interface NotificationsMessage {
  type: 'notifications'
  unread: number
  notification?: AppNotification
}

export const getNotifications = (q: { unread?: boolean; page?: number; page_size?: number } = {}) =>
  api<NotificationPage>('/api/notifications', {
    query: { unread: q.unread ? 'true' : undefined, page: q.page, page_size: q.page_size },
  })

export const getUnread = () => api<{ unread: number }>('/api/notifications/unread')

export const markRead = (id: string, read: boolean) => api<void>(`/api/notifications/${id}/read`, { method: 'PUT', body: { read } })

export const markAllRead = () => api<void>('/api/notifications/read-all', { method: 'POST' })

export const getFollowing = (workItemId: string) => api<{ following: boolean }>(`/api/work-items/${workItemId}/follow`)

export const setFollowing = (workItemId: string, following: boolean) =>
  api<{ following: boolean }>(`/api/work-items/${workItemId}/follow`, { method: 'PUT', body: { following } })

/** Where a notification leads: the element, at the comment when the last event is one (RF-39, RF-41). */
export function notificationTarget(n: AppNotification): string {
  switch (n.subject.type) {
    case 'work_item':
      return `/work-items/${n.subject.key}` + (n.last.comment_id ? `?comment=${n.last.comment_id}` : '')
    case 'milestone':
      return `/milestones/${n.subject.id}`
    case 'meeting':
      return `/meetings/${n.subject.id}`
    default:
      return `/projects/${n.subject.id}`
  }
}
