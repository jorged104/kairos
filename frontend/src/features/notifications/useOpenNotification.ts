import { useQueryClient } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'
import { markAllRead, markRead, notificationTarget, type AppNotification } from '@/api/notifications'
import { listKey, unreadKey } from './useNotifications'

/** Opening, reading and unreading notifications, shared by the bell and the page (RF-39 to RF-43). */
export function useOpenNotification() {
  const client = useQueryClient()
  const router = useRouter()
  const refresh = () => Promise.all([client.invalidateQueries({ queryKey: unreadKey }), client.invalidateQueries({ queryKey: listKey })])

  async function open(n: AppNotification) {
    if (!n.read_at) await markRead(n.id, true).catch(() => undefined)
    void refresh()
    await router.push(notificationTarget(n))
  }
  async function toggle(n: AppNotification) {
    await markRead(n.id, !n.read_at)
    await refresh()
  }
  async function readAll() {
    await markAllRead()
    await refresh()
  }
  return { open, toggle, readAll }
}
