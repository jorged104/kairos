<script setup lang="ts">
import { ChevronDown, ChevronRight, MessageSquare } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createComment } from '@/api/comments'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { RichDoc } from '@/types/work'
import CommentEditor from './CommentEditor.vue'
import CommentThread from './CommentThread.vue'
import { useComments } from './useComments'

// The comments of a work item, right below its properties as in Notion
// (spec 006, RF-22): the open threads oldest first (RF-29) and the resolved
// ones folded (RF-36).
const props = defineProps<{ workItemId: string }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { list, error, apply } = useComments(() => props.workItemId)

const open = computed(() => (list.value?.threads ?? []).filter((th) => !th.resolved_at))
const resolved = computed(() => (list.value?.threads ?? []).filter((th) => th.resolved_at))
const total = computed(() => (list.value?.threads ?? []).reduce((n, th) => n + (th.deleted ? 0 : 1) + th.replies.length, 0))
const showResolved = ref(false)
const writing = ref(false)

async function publish(body: RichDoc) {
  apply(await createComment(props.workItemId, body))
}
</script>

<template>
  <section class="flex flex-col gap-3" :aria-label="t('comments.title')" data-testid="comments">
    <h2 class="flex items-center gap-2 text-sm font-medium">
      <MessageSquare class="size-4 text-muted-foreground" aria-hidden="true" />
      {{ t('comments.title') }}
      <span v-if="total" class="text-muted-foreground">{{ total }}</span>
    </h2>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <template v-if="list">
      <CommentThread
        v-for="th in open"
        :key="th.id"
        :thread="th"
        :refs="list.refs"
        :work-item-id="workItemId"
        :can-comment="list.can_comment"
        :apply="apply"
      />
      <template v-if="list.can_comment">
        <CommentEditor v-if="writing || open.length === 0" :work-item-id="workItemId" :submit="publish" @cancel="writing = false" />
        <button
          v-else
          type="button"
          class="w-full rounded-lg border border-dashed px-3 py-2 text-left text-sm text-muted-foreground hover:bg-muted/50"
          data-testid="comment-new"
          @click="writing = true"
        >
          {{ t('comments.placeholder') }}
        </button>
      </template>
      <p v-else-if="list.threads.length === 0" class="text-sm text-muted-foreground">{{ t('comments.none') }}</p>
      <div v-if="resolved.length" class="flex flex-col gap-2">
        <button
          type="button"
          class="flex w-fit items-center gap-1 rounded-md px-1.5 py-0.5 text-sm text-muted-foreground hover:bg-muted/60 hover:text-foreground"
          :aria-expanded="showResolved"
          data-testid="resolved-toggle"
          @click="showResolved = !showResolved"
        >
          <component :is="showResolved ? ChevronDown : ChevronRight" class="size-4" aria-hidden="true" />
          {{ t('comments.resolved', { count: resolved.length }, resolved.length) }}
        </button>
        <template v-if="showResolved">
          <CommentThread
            v-for="th in resolved"
            :key="th.id"
            :thread="th"
            :refs="list.refs"
            :work-item-id="workItemId"
            :can-comment="list.can_comment"
            :apply="apply"
          />
        </template>
      </div>
    </template>
  </section>
</template>
