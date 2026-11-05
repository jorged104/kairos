<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Bell, BellOff } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getFollowing, setFollowing } from '@/api/notifications'
import { Button } from '@/components/ui/button'

// Follow or stop following a work item: its comments and changes, not the
// mentions, which always arrive (spec 017, RF-2 to RF-4).
const props = defineProps<{ workItemId: string }>()
const { t } = useI18n()
const client = useQueryClient()
const key = computed(() => ['notifications', 'follow', props.workItemId])
const { data } = useQuery({ queryKey: key, queryFn: () => getFollowing(props.workItemId) })
const toggle = useMutation({
  mutationFn: (following: boolean) => setFollowing(props.workItemId, following),
  onSuccess: (out) => client.setQueryData(key.value, out),
})
</script>

<template>
  <Button
    v-if="data"
    size="sm"
    :variant="data.following ? 'secondary' : 'outline'"
    :aria-pressed="data.following"
    :title="t('notifications.follow.hint')"
    :disabled="toggle.isPending.value"
    data-testid="follow"
    @click="toggle.mutate(!data.following)"
  >
    <Bell v-if="data.following" aria-hidden="true" />
    <BellOff v-else aria-hidden="true" />
    {{ data.following ? t('notifications.follow.following') : t('notifications.follow.follow') }}
  </Button>
</template>
