<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { deleteComment, updateComment, type CommentResult, type WorkComment } from '@/api/comments'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AvatarStack from '@/features/work/AvatarStack.vue'
import PageEditor from '@/features/richtext/PageEditor.vue'
import type { MentionRefs } from '@/types/work'
import CommentEditor from './CommentEditor.vue'

// One comment: who, when, whether it was edited (RF-30), and its own
// actions; editing happens in place (RF-31).
const props = defineProps<{ comment: WorkComment; refs: MentionRefs; workItemId: string; apply: (r: CommentResult) => unknown }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const editing = ref(false)
const error = ref<unknown>(null)
const when = computed(() =>
  new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(props.comment.created_at)),
)

async function save(body: unknown) {
  props.apply(await updateComment(props.comment.id, body as WorkComment['body']))
  editing.value = false
}
async function remove() {
  error.value = null
  try {
    props.apply(await deleteComment(props.comment.id))
  } catch (e) {
    error.value = e
  }
}
</script>

<template>
  <article class="flex gap-2" data-testid="comment">
    <AvatarStack :people="[comment.author]" :max="1" bare class="mt-0.5 shrink-0" />
    <div class="flex min-w-0 flex-1 flex-col gap-1">
      <header class="flex flex-wrap items-baseline gap-x-2 text-sm">
        <span class="font-medium">{{ comment.author.name }}</span>
        <time class="text-xs text-muted-foreground" :datetime="comment.created_at">{{ when }}</time>
        <span v-if="comment.edited_at && !comment.deleted" class="text-xs text-muted-foreground">{{ t('comments.edited') }}</span>
        <span class="ml-auto flex gap-1">
          <slot name="actions" />
          <button
            v-if="comment.can.edit && !editing"
            type="button"
            class="rounded px-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
            data-testid="comment-edit"
            @click="editing = true"
          >
            {{ t('common.edit') }}
          </button>
          <button
            v-if="comment.can.delete"
            type="button"
            class="rounded px-1 text-xs text-muted-foreground hover:bg-muted hover:text-destructive"
            data-testid="comment-delete"
            @click="remove"
          >
            {{ t('comments.delete') }}
          </button>
        </span>
      </header>
      <p v-if="comment.deleted" class="text-sm text-muted-foreground italic">{{ t('comments.deleted') }}</p>
      <CommentEditor
        v-else-if="editing"
        :work-item-id="workItemId"
        :initial="comment.body"
        :submit="save"
        :submit-label="t('common.save')"
        autofocus
        @cancel="editing = false"
      />
      <PageEditor v-else :doc="comment.body" :refs="refs" :editable="false" compact class="text-sm" />
      <p v-if="error" role="alert" class="text-xs text-destructive">{{ errorMessage(error) }}</p>
    </div>
  </article>
</template>
