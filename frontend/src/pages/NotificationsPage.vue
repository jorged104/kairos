<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getNotifications } from '@/api/notifications'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import NotificationItem from '@/features/notifications/NotificationItem.vue'
import { listKey } from '@/features/notifications/useNotifications'
import { useOpenNotification } from '@/features/notifications/useOpenNotification'

// Every notification, 50 a page, all or only the unread (spec 017, RF-28, RF-29).
const { t } = useI18n()
const errorMessage = useErrorMessage()
const unreadOnly = ref(false)
const page = ref(1)
watch(unreadOnly, () => (page.value = 1))
const { data, error } = useQuery({
  queryKey: computed(() => [...listKey, 'page', unreadOnly.value, page.value]),
  queryFn: () => getNotifications({ unread: unreadOnly.value, page: page.value, page_size: 50 }),
})
const pages = computed(() => Math.max(1, Math.ceil((data.value?.total ?? 0) / 50)))
const actions = useOpenNotification()
</script>

<template>
  <div class="flex max-w-3xl flex-col gap-4" data-testid="notifications-page">
    <div class="flex flex-wrap items-center gap-3">
      <h1 class="font-heading text-xl font-semibold">{{ t('notifications.title') }}</h1>
      <div class="flex rounded-lg border p-0.5 text-sm" role="group" :aria-label="t('notifications.filter')">
        <button
          v-for="opt in [false, true]"
          :key="String(opt)"
          type="button"
          class="rounded-md px-3 py-1"
          :class="unreadOnly === opt ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
          :aria-pressed="unreadOnly === opt"
          :data-testid="opt ? 'filter-unread' : 'filter-all'"
          @click="unreadOnly = opt"
        >
          {{ opt ? t('notifications.unreadOnly') : t('notifications.all') }}
        </button>
      </div>
      <Button v-if="data?.unread" size="sm" variant="outline" class="ml-auto" data-testid="notifications-read-all" @click="actions.readAll()">
        {{ t('notifications.readAll') }}
      </Button>
    </div>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <div v-if="data" class="flex flex-col gap-0.5 rounded-xl border p-1">
      <p v-if="!data.items.length" class="px-3 py-8 text-center text-sm text-muted-foreground" data-testid="notifications-empty">
        {{ unreadOnly ? t('notifications.emptyUnread') : t('notifications.empty') }}
      </p>
      <NotificationItem v-for="n in data.items" :key="n.id" :notification="n" @open="actions.open" @toggle="actions.toggle" />
    </div>
    <div v-if="pages > 1" class="flex items-center gap-2 text-sm">
      <Button size="sm" variant="outline" :disabled="page === 1" @click="page--">{{ t('common.previous') }}</Button>
      <span class="text-muted-foreground">{{ t('notifications.page', { page, pages }) }}</span>
      <Button size="sm" variant="outline" :disabled="page === pages" @click="page++">{{ t('common.next') }}</Button>
    </div>
  </div>
</template>
