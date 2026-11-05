import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { getUnread, type AppNotification, type NotificationsMessage } from '@/api/notifications'
import { onRealtime, realtimeConnected } from '@/composables/useRealtime'

// The notifications of the tab (spec 017): the count on the bell, kept
// fresh by the realtime connection, or every 60 s without it (RF-33 to RF-38).

export const unreadKey = ['notifications', 'unread'] as const
export const listKey = ['notifications', 'list'] as const

/** The unread count; every bell and page share it. */
export function useUnread() {
  return useQuery({
    queryKey: unreadKey,
    queryFn: getUnread,
    refetchInterval: () => (realtimeConnected.value ? false : 60_000),
  })
}

/** A toast: one notification, or several at once folded in one (RF-32). */
export interface Toast {
  id: number
  notification?: AppNotification
  count?: number
}

export const MAX_TOASTS = 3
export const TOAST_MS = 6000

/**
 * Applies what the realtime connection brings to the cached count and lists,
 * and raises the toasts. Once per tab, in the layout.
 */
export function useNotificationsSync() {
  const client = useQueryClient()
  const route = useRoute()
  const toasts = ref<Toast[]>([])
  let next = 1

  // The element on screen does not toast (RF-31).
  const onScreen = (n: AppNotification) => {
    const s = n.subject
    if (s.type === 'work_item') return route.params.wikey === s.key || route.query.item === s.key
    return route.path === (s.type === 'milestone' ? `/milestones/${s.id}` : `/projects/${s.id}`)
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((x) => x.id !== id)
  }

  function raise(n: AppNotification) {
    if (onScreen(n)) return
    const shown = toasts.value.filter((x) => x.notification?.subject.id !== n.subject.id)
    const toast: Toast = { id: next++, notification: n }
    if (shown.length >= MAX_TOASTS) {
      const count = shown.reduce((sum, x) => sum + (x.count ?? 1), 0) + 1
      toasts.value = [{ id: next++, count }]
    } else {
      toasts.value = [...shown, toast]
    }
    const id = toasts.value[toasts.value.length - 1]!.id
    setTimeout(() => dismiss(id), TOAST_MS)
  }

  const stop = onRealtime((m) => {
    if (m.type === 'connected') {
      void client.invalidateQueries({ queryKey: ['notifications'] })
      return
    }
    if (m.type !== 'notifications') return
    const msg = m as unknown as NotificationsMessage
    client.setQueryData(unreadKey, { unread: msg.unread })
    void client.invalidateQueries({ queryKey: listKey })
    if (msg.notification && !msg.notification.read_at) raise(msg.notification)
  })
  onUnmounted(stop)

  return { toasts, dismiss }
}

/** The sentence of a notification: who did what (RF-23). */
export function useNotificationText() {
  const { t } = useI18n()
  const who = (n: AppNotification) => {
    const names = n.actors.map((a) => a.name)
    const rest = n.actors_total - names.length
    if (rest > 0) return t('notifications.who.more', { names: names.join(', '), n: rest })
    if (names.length === 2) return t('notifications.who.two', { a: names[0], b: names[1] })
    return names[0] ?? ''
  }
  const subject = (n: AppNotification) => (n.subject.key ? `${n.subject.key} ${n.subject.title}` : n.subject.title)
  const summary = (n: AppNotification) =>
    t(`notifications.kinds.${n.last.kind}`, { who: who(n), from: n.last.from ?? '', to: n.last.to ?? '' }, n.actors_total > 1 ? 2 : 1)
  return { who, subject, summary }
}
