<script setup lang="ts">
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { LoaderCircle, RotateCcw, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { removeUpload, uploads } from './uploads'

// The placeholder of a file being uploaded: its progress, or the error with
// retry and remove (RF-19, RF-20). It is never saved with the page.
const props = defineProps(nodeViewProps)
const { t } = useI18n()

const upload = computed(() => uploads.get(props.node.attrs.uploadId as string))
const percent = computed(() => Math.round((upload.value?.progress ?? 0) * 100))
const remove = () => removeUpload(props.editor, props.node.attrs.uploadId as string)
</script>

<template>
  <NodeViewWrapper
    contenteditable="false"
    class="my-2 flex flex-col gap-1.5 rounded-lg border border-dashed px-3 py-2 text-sm"
    :class="upload?.error ? 'border-destructive/50 bg-destructive/5' : ''"
    data-testid="file-block"
  >
    <div class="flex items-center gap-2">
      <LoaderCircle v-if="upload && !upload.error" class="size-4 animate-spin text-muted-foreground" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate font-medium">{{ upload?.name }}</span>
      <span v-if="upload && !upload.error" class="text-xs text-muted-foreground">{{ t('page.files.uploading', { percent }) }}</span>
      <template v-if="upload?.error">
        <Button size="sm" variant="outline" type="button" data-testid="file-retry" @click="upload.retry()">
          <RotateCcw class="size-3.5" aria-hidden="true" />{{ t('page.files.retry') }}
        </Button>
      </template>
      <Button size="sm" variant="ghost" type="button" :aria-label="t('page.files.remove')" data-testid="file-remove" @click="remove">
        <X class="size-4" aria-hidden="true" />
      </Button>
    </div>
    <div v-if="upload && !upload.error" class="h-1 rounded bg-muted" role="progressbar" :aria-valuenow="percent" aria-valuemin="0" aria-valuemax="100">
      <div class="h-1 rounded bg-primary transition-[width]" :style="{ width: `${percent}%` }" />
    </div>
    <p v-if="upload?.error" class="text-destructive" role="alert">{{ t(`errors.${upload.error}`) }}</p>
  </NodeViewWrapper>
</template>
