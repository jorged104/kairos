<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { Bell } from '@lucide/vue'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getNotifications, type AppNotification } from '@/api/notifications'
import NotificationItem from './NotificationItem.vue'
import { listKey, useUnread } from './useNotifications'
import { useOpenNotification } from './useOpenNotification'

// The bell of the sidebar: the unread count and the 20 latest, unread first
// (RF-26, RF-27).
defineProps<{ collapsed?: boolean }>()
const { t } = useI18n()
const { data: unreadData } = useUnread()
const unread = computed(() => unreadData.value?.unread ?? 0)
const label = computed(() => (unread.value > 99 ? '99+' : String(unread.value)))

const open = ref(false)
const { data: list } = useQuery({
  queryKey: [...listKey, 'bell'],
  queryFn: () => getNotifications({ page_size: 20 }),
  enabled: open,
})
const actions = useOpenNotification()
async function go(n: AppNotification) {
  open.value = false
  await actions.open(n)
}

const root = ref<HTMLElement | null>(null)
// Closes with a click outside or with Escape.
function away(e: Event) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}
function esc(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}
function listen(on: boolean) {
  if (on) {
    document.addEventListener('pointerdown', away)
    document.addEventListener('keydown', esc)
  } else {
    document.removeEventListener('pointerdown', away)
    document.removeEventListener('keydown', esc)
  }
}
watch(open, listen)
onUnmounted(() => listen(false))
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
      :class="collapsed && 'lg:justify-center lg:px-0'"
      :aria-label="t('notifications.bell', { n: unread })"
      :aria-expanded="open"
      :title="collapsed ? t('notifications.title') : undefined"
      data-testid="notification-bell"
      @click="open = !open"
    >
      <span class="relative">
        <Bell class="size-4 shrink-0" aria-hidden="true" />
        <span
          v-if="unread"
          class="absolute -top-2 -right-2.5 min-w-4 rounded-full bg-primary px-1 text-center text-[10px] leading-4 font-semibold text-primary-foreground tabular-nums"
          data-testid="notification-count"
          >{{ label }}</span
        >
      </span>
      <span :class="collapsed && 'lg:sr-only'">{{ t('notifications.title') }}</span>
    </button>
    <div
      v-if="open"
      class="fixed inset-x-2 bottom-16 z-50 flex max-h-[70vh] flex-col overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-xl animate-in fade-in-0 zoom-in-95 duration-150 motion-reduce:animate-none sm:absolute sm:inset-x-auto sm:bottom-0 sm:left-full sm:ml-2 sm:w-96"
      role="dialog"
      :aria-label="t('notifications.title')"
      data-testid="notification-dropdown"
    >
      <header class="flex items-center gap-2 border-b px-3 py-2">
        <h2 class="text-sm font-semibold">{{ t('notifications.title') }}</h2>
        <button
          v-if="unread"
          type="button"
          class="ml-auto text-xs text-primary hover:underline"
          data-testid="notification-read-all"
          @click="actions.readAll()"
        >
          {{ t('notifications.readAll') }}
        </button>
      </header>
      <div class="flex-1 overflow-y-auto p-1">
        <p v-if="list && !list.items.length" class="px-3 py-6 text-center text-sm text-muted-foreground">{{ t('notifications.empty') }}</p>
        <NotificationItem v-for="n in list?.items ?? []" :key="n.id" :notification="n" @open="go" @toggle="actions.toggle" />
      </div>
      <RouterLink to="/notifications" class="border-t px-3 py-2 text-center text-sm text-primary hover:bg-muted" data-testid="notification-all" @click="open = false">
        {{ t('notifications.seeAll') }}
      </RouterLink>
    </div>
  </div>
</template>
