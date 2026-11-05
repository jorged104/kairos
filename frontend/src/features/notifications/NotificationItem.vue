<script setup lang="ts">
import { AtSign } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import type { AppNotification } from '@/api/notifications'
import ViaBadge from '@/features/mcp/ViaBadge.vue'
import { useNotificationText } from './useNotifications'

// One notification: what, where, who, when; unread ones stand out (RF-23, RF-24).
defineProps<{ notification: AppNotification }>()
const emit = defineEmits<{ open: [AppNotification]; toggle: [AppNotification] }>()
const { t, d } = useI18n()
const { subject, summary } = useNotificationText()
</script>

<template>
  <div
    class="group flex gap-2 rounded-lg px-3 py-2 text-sm transition-colors hover:bg-muted"
    :class="!notification.read_at && 'bg-primary/5'"
    data-testid="notification"
    :data-unread="!notification.read_at"
  >
    <span class="mt-1.5 size-2 shrink-0 rounded-full" :class="notification.read_at ? 'bg-transparent' : 'bg-primary'" aria-hidden="true" />
    <button type="button" class="flex min-w-0 flex-1 flex-col gap-0.5 text-left" data-testid="notification-open" @click="emit('open', notification)">
      <span class="flex flex-wrap items-center gap-x-1.5">
        <span class="font-medium">{{ summary(notification) }}</span>
        <span v-if="notification.mention" class="inline-flex items-center gap-0.5 rounded bg-primary/10 px-1 text-xs text-primary" data-testid="notification-mention">
          <AtSign class="size-3" aria-hidden="true" />{{ t('notifications.mention') }}
        </span>
        <ViaBadge v-if="notification.via" :via="notification.via" />
      </span>
      <span class="truncate text-muted-foreground">{{ subject(notification) }}</span>
      <span v-if="notification.last.excerpt" class="line-clamp-2 text-xs text-muted-foreground" data-testid="notification-excerpt">«{{ notification.last.excerpt }}»</span>
      <span class="text-xs text-muted-foreground">
        {{ d(notification.updated_at, 'long') }}
        <template v-if="notification.events > 1"> · {{ t('notifications.events', { n: notification.events }) }}</template>
      </span>
    </button>
    <button
      type="button"
      class="self-start rounded px-1.5 py-0.5 text-xs text-muted-foreground opacity-0 group-hover:opacity-100 hover:bg-background hover:text-foreground focus-visible:opacity-100"
      data-testid="notification-toggle"
      @click="emit('toggle', notification)"
    >
      {{ notification.read_at ? t('notifications.markUnread') : t('notifications.markRead') }}
    </button>
  </div>
</template>
