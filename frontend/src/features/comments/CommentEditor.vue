<script setup lang="ts">
import type { JSONContent } from '@tiptap/vue-3'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { commentFilesPath } from '@/api/comments'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { textLength } from '@/features/pages/usePageAutosave'
import PageEditor from '@/features/richtext/PageEditor.vue'
import { pageForSave } from '@/features/richtext/pageExtensions'
import type { RichDoc } from '@/types/work'

// Writes a comment: the page's blocks, mentions and files, compact (RF-26).
// Ctrl+Enter publishes it.
const props = withDefaults(
  defineProps<{
    workItemId: string
    submit: (doc: RichDoc) => Promise<unknown>
    initial?: RichDoc
    submitLabel?: string
    placeholder?: string
    autofocus?: boolean
  }>(),
  { initial: null, submitLabel: undefined, placeholder: undefined, autofocus: false },
)
const emit = defineEmits<{ cancel: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const MAX = 10_000
const doc = ref<JSONContent | null>(props.initial)
const generation = ref(0) // remounts the editor empty after publishing
const sending = ref(false)
const error = ref<unknown>(null)
const editorRef = ref<InstanceType<typeof PageEditor> | null>(null)

const hasFiles = (d: JSONContent | null): boolean =>
  !!d && (d.type === 'image' || d.type === 'fileAttachment' || (d.content ?? []).some(hasFiles))
const length = computed(() => textLength(doc.value))
const empty = computed(() => length.value === 0 && !hasFiles(doc.value))
const tooLong = computed(() => length.value > MAX)

async function send() {
  if (empty.value || tooLong.value || sending.value) return
  sending.value = true
  error.value = null
  try {
    await props.submit(pageForSave(doc.value!))
    doc.value = null
    generation.value++
  } catch (e) {
    error.value = e
  } finally {
    sending.value = false
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    void send()
  } else if (e.key === 'Escape' && !document.querySelector('[data-testid=suggestions]')) {
    e.preventDefault() // the panel closes with Escape only when nothing uses it
    emit('cancel')
  }
}
// Answering or editing starts with the cursor in the editor.
watch(
  () => editorRef.value?.editor,
  (e) => {
    if (e && props.autofocus) e.commands.focus('end')
  },
)
defineExpose({ focus: () => editorRef.value?.editor?.commands.focus() })
</script>

<template>
  <div class="flex flex-col gap-2 rounded-lg border bg-background p-2 focus-within:ring-3 focus-within:ring-ring/30" data-testid="comment-editor" @keydown="onKeydown">
    <PageEditor
      :key="generation"
      ref="editorRef"
      :doc="initial"
      editable
      compact
      :placeholder="placeholder ?? t('comments.placeholder')"
      :owner="{ kind: 'work-items', id: workItemId, filesPath: commentFilesPath(workItemId) }"
      @change="doc = $event"
    />
    <p v-if="error" role="alert" class="text-xs text-destructive">{{ errorMessage(error) }}</p>
    <div class="flex items-center justify-end gap-2">
      <span v-if="tooLong" class="mr-auto text-xs text-destructive" role="alert">{{ t('comments.tooLong') }}</span>
      <span v-else class="mr-auto text-xs text-muted-foreground">{{ t('comments.hint') }}</span>
      <Button size="sm" variant="ghost" type="button" @click="emit('cancel')">{{ t('common.cancel') }}</Button>
      <Button size="sm" type="button" :disabled="empty || tooLong || sending" data-testid="comment-submit" @click="send">
        {{ submitLabel ?? t('comments.publish') }}
      </Button>
    </div>
  </div>
</template>
