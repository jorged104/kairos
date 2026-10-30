<script setup lang="ts">
import { CheckCircle2, Quote, RotateCcw } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createComment, setThreadResolved, type CommentResult, type CommentThread } from '@/api/comments'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { MentionRefs, RichDoc } from '@/types/work'
import CommentEditor from './CommentEditor.vue'
import CommentItem from './CommentItem.vue'
import { showAnchor } from './useComments'

// A thread: its root, its replies (one level, RF-28), answering and
// resolving or reopening it (RF-36 to RF-38). An anchored thread shows the
// commented text, which takes to it in the page (RF-48, RF-51).
const props = defineProps<{
  thread: CommentThread
  refs: MentionRefs
  workItemId: string
  canComment: boolean
  apply: (r: CommentResult) => unknown
}>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const replying = ref(false)
const error = ref<unknown>(null)

async function reply(body: RichDoc) {
  props.apply(await createComment(props.workItemId, body, { threadId: props.thread.id }))
  replying.value = false
}
async function resolve(resolved: boolean) {
  error.value = null
  try {
    props.apply(await setThreadResolved(props.thread.id, resolved))
  } catch (e) {
    error.value = e
  }
}
</script>

<template>
  <section
    class="flex flex-col gap-3 rounded-lg border bg-background p-3"
    :class="{ 'opacity-80': thread.resolved_at }"
    :data-thread="thread.id"
    data-testid="comment-thread"
  >
    <button
      v-if="thread.anchored && thread.quote"
      type="button"
      class="flex items-start gap-1.5 border-l-2 border-amber-400 pl-2 text-left text-sm text-muted-foreground hover:text-foreground disabled:cursor-default disabled:hover:text-muted-foreground"
      :disabled="thread.anchor_deleted"
      data-testid="comment-quote"
      @click="showAnchor(thread.id)"
    >
      <Quote class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span class="line-clamp-2" :class="{ 'line-through': thread.anchor_deleted }">{{ thread.quote }}</span>
      <span v-if="thread.anchor_deleted" class="shrink-0 rounded bg-muted px-1 text-xs no-underline">{{ t('comments.textDeleted') }}</span>
    </button>
    <CommentItem :comment="thread" :refs="refs" :work-item-id="workItemId" :apply="apply">
      <template #actions>
        <button
          v-if="canComment && !thread.resolved_at"
          type="button"
          class="flex items-center gap-1 rounded px-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
          data-testid="thread-resolve"
          @click="resolve(true)"
        >
          <CheckCircle2 class="size-3.5" aria-hidden="true" />{{ t('comments.resolve') }}
        </button>
        <button
          v-else-if="canComment && thread.resolved_at"
          type="button"
          class="flex items-center gap-1 rounded px-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
          data-testid="thread-reopen"
          @click="resolve(false)"
        >
          <RotateCcw class="size-3.5" aria-hidden="true" />{{ t('comments.reopen') }}
        </button>
      </template>
    </CommentItem>
    <p v-if="thread.resolved_at && thread.resolved_by" class="text-xs text-muted-foreground">
      {{ t('comments.resolvedBy', { name: thread.resolved_by.name }) }}
    </p>
    <div v-if="thread.replies.length" class="ml-8 flex flex-col gap-3 border-l pl-3">
      <CommentItem v-for="r in thread.replies" :key="r.id" :comment="r" :refs="refs" :work-item-id="workItemId" :apply="apply" />
    </div>
    <p v-if="error" role="alert" class="text-xs text-destructive">{{ errorMessage(error) }}</p>
    <template v-if="canComment">
      <CommentEditor
        v-if="replying"
        class="ml-8"
        :work-item-id="workItemId"
        :submit="reply"
        :submit-label="t('comments.reply')"
        :placeholder="t('comments.replyPlaceholder')"
        autofocus
        @cancel="replying = false"
      />
      <button
        v-else
        type="button"
        class="ml-8 w-fit rounded px-1 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
        data-testid="thread-reply"
        @click="replying = true"
      >
        {{ t('comments.reply') }}
      </button>
    </template>
  </section>
</template>
