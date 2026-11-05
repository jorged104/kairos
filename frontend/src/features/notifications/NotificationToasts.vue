<script setup lang="ts">
import { Bell, X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import type { AppNotification } from '@/api/notifications'
import { useNotificationsSync, useNotificationText } from './useNotifications'
import { useOpenNotification } from './useOpenNotification'

// The toasts of new notifications: 6 s, closable, they lead to the element;
// announced politely without taking the focus (RF-30 to RF-32).
const { t } = useI18n()
const router = useRouter()
const { toasts, dismiss } = useNotificationsSync()
const { subject, summary } = useNotificationText()
const actions = useOpenNotification()

async function go(id: number, n?: AppNotification) {
  dismiss(id)
  if (n) await actions.open(n)
  else await router.push('/notifications')
}
</script>

<template>
  <div class="pointer-events-none fixed right-4 bottom-4 z-50 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2" role="status" aria-live="polite" data-testid="toasts">
    <TransitionGroup name="card">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="pointer-events-auto flex items-start gap-2 rounded-xl border bg-popover p-3 text-sm text-popover-foreground shadow-lg"
        data-testid="toast"
      >
        <Bell class="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true" />
        <button type="button" class="flex min-w-0 flex-1 flex-col gap-0.5 text-left" data-testid="toast-open" @click="go(toast.id, toast.notification)">
          <template v-if="toast.notification">
            <span class="font-medium">{{ summary(toast.notification) }}</span>
            <span class="truncate text-muted-foreground">{{ subject(toast.notification) }}</span>
            <span v-if="toast.notification.last.excerpt" class="line-clamp-2 text-xs text-muted-foreground">«{{ toast.notification.last.excerpt }}»</span>
          </template>
          <span v-else class="font-medium">{{ t('notifications.many', { n: toast.count }) }}</span>
        </button>
        <button type="button" class="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground" :aria-label="t('notifications.close')" @click="dismiss(toast.id)">
          <X class="size-4" aria-hidden="true" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
