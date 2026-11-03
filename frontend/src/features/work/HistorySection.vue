<script setup lang="ts">
import { useInfiniteQuery } from '@tanstack/vue-query'
import { ChevronDown, ChevronRight, History } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getHistory, type HistoryEntry, type HistoryValue, type Named } from '@/api/metrics'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'

// Who changed what and when, at the end of the sheet and folded when it opens
// (spec 007, RF-47): newest first, 50 at a time (RF-52), with the names of
// the values (RF-49) and the description only as «editó la descripción» (RF-50).
const props = defineProps<{ workItemId: string; version: number }>()
const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()
const open = ref(false)

const { data, error, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
  queryKey: computed(() => ['history', props.workItemId, props.version]),
  queryFn: ({ pageParam }) => getHistory(props.workItemId, pageParam),
  initialPageParam: undefined as number | undefined,
  getNextPageParam: (last) => last.next ?? undefined,
  enabled: open,
})
const entries = computed(() => data.value?.pages.flatMap((p) => p.entries) ?? [])

const isNamed = (v: HistoryValue): v is Named => typeof v === 'object' && v !== null && !Array.isArray(v)
function show(v: HistoryValue): string {
  if (v === null || v === '' || (Array.isArray(v) && v.length === 0)) return t('common.none')
  if (Array.isArray(v)) return v.map(show).join(', ')
  if (isNamed(v)) return v.name || t('history.deleted')
  if (typeof v === 'boolean') return v ? t('common.yes') : t('common.no')
  return String(v)
}
const fieldName = (f: string) => (te(`history.fields.${f}`) ? t(`history.fields.${f}`) : f)
function summary(e: HistoryEntry): string {
  switch (e.kind) {
    case 'created':
      return t('history.created')
    case 'description':
      return t('history.description')
    case 'dependency_added':
    case 'dependency_removed': {
      const c = e.changes[0]!
      return t(`history.${e.kind}`, { type: t(`workItems.depTypes.${c.field}.outgoing`), item: show(c.to) })
    }
    default:
      return ''
  }
}
const when = (at: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(at))
</script>

<template>
  <section class="flex flex-col gap-3" :aria-label="t('history.title')" data-testid="history">
    <button
      type="button"
      class="flex w-fit items-center gap-2 rounded-md text-sm font-medium hover:text-foreground"
      :aria-expanded="open"
      data-testid="history-toggle"
      @click="open = !open"
    >
      <component :is="open ? ChevronDown : ChevronRight" class="size-4 text-muted-foreground" aria-hidden="true" />
      <History class="size-4 text-muted-foreground" aria-hidden="true" />
      {{ t('history.title') }}
    </button>
    <template v-if="open">
      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <ol class="flex flex-col gap-2 border-l pl-4 text-sm">
        <li v-for="(e, i) in entries" :key="i" class="flex flex-col gap-0.5" data-testid="history-entry">
          <p>
            <span class="font-medium">{{ e.actor.name || t('history.system') }}</span>
            <time class="ml-2 text-xs text-muted-foreground" :datetime="e.at">{{ when(e.at) }}</time>
          </p>
          <p v-if="e.kind !== 'updated'" class="text-muted-foreground">{{ summary(e) }}</p>
          <ul v-else class="text-muted-foreground">
            <li v-for="c in e.changes" :key="c.field">
              <template v-if="c.field === 'description'">{{ t('history.description') }}</template>
              <template v-else>
                {{ fieldName(c.field) }}: <span class="line-through">{{ show(c.from) }}</span> → <span class="text-foreground">{{ show(c.to) }}</span>
              </template>
            </li>
          </ul>
        </li>
        <li v-if="data && entries.length === 0" class="text-muted-foreground">{{ t('common.empty') }}</li>
      </ol>
      <Button v-if="hasNextPage" size="sm" variant="outline" class="w-fit" :disabled="isFetchingNextPage" data-testid="history-more" @click="fetchNextPage()">
        {{ t('history.more') }}
      </Button>
    </template>
  </section>
</template>
