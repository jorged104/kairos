<script setup lang="ts">
import { AlertTriangle, Check, CircleDot, LoaderCircle } from '@lucide/vue'
import { ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave } from 'vue-router'
import type { PageOwner, SavedPage } from '@/api/pages'
import { Button } from '@/components/ui/button'
import PageEditor from '@/features/richtext/PageEditor.vue'
import type { MentionRefs, RichDoc } from '@/types/work'
import { usePageAutosave } from './usePageAutosave'

// The page of a work item, milestone or project, saved as it is written
// (RF-31 to RF-36). Who can't edit it reads it (RF-13).
const props = defineProps<{
  owner: PageOwner
  doc: RichDoc
  pageVersion: number
  refs?: MentionRefs
  editable: boolean
  /** Reads the owner again, to take someone else's version after a conflict. */
  reload: () => Promise<{ description: RichDoc; page_version: number; refs?: MentionRefs }>
}>()
const emit = defineEmits<{ saved: [SavedPage] }>()
const { t } = useI18n()

// The editor's content is only replaced when the page is reloaded; the
// changes the user types stay in the editor.
const shown = shallowRef<RichDoc>(props.doc)
const autosave = usePageAutosave(() => props.owner, props.pageVersion, props.refs ?? {}, { onSaved: (s) => emit('saved', s) })
const copied = ref(false)

async function copyMine() {
  try {
    await navigator.clipboard.writeText(autosave.myText())
    copied.value = true
  } catch {
    copied.value = false
  }
}

async function takeTheirs() {
  const fresh = await props.reload()
  shown.value = fresh.description
  autosave.adopt(fresh.page_version, fresh.refs ?? {})
  copied.value = false
}

onBeforeRouteLeave(() => autosave.flush())
defineExpose({ flush: autosave.flush, hasChanges: autosave.hasChanges })
</script>

<template>
  <section class="flex flex-col gap-2" :aria-label="t('page.label')">
    <div
      v-if="autosave.status.value === 'conflict'"
      role="alert"
      class="flex flex-wrap items-center gap-3 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-950 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-100"
      data-testid="page-conflict"
    >
      <AlertTriangle class="size-4 flex-none" aria-hidden="true" />
      <span class="min-w-0 flex-[1_1_16rem]">{{ t('page.conflict.message') }}</span>
      <Button size="sm" variant="outline" type="button" data-testid="copy-mine" @click="copyMine">
        {{ copied ? t('page.conflict.copied') : t('page.conflict.copy') }}
      </Button>
      <Button size="sm" type="button" data-testid="take-theirs" @click="takeTheirs">{{ t('page.conflict.reload') }}</Button>
    </div>
    <div v-if="editable" class="flex items-center gap-1.5 text-xs" aria-live="polite" data-testid="save-status">
      <template v-if="autosave.status.value === 'saved'">
        <Check class="size-3.5 text-emerald-700 dark:text-emerald-400" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.saved') }}</span>
      </template>
      <template v-else-if="autosave.status.value === 'dirty'">
        <CircleDot class="size-3.5 text-amber-700 dark:text-amber-400" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.dirty') }}</span>
      </template>
      <template v-else-if="autosave.status.value === 'saving'">
        <LoaderCircle class="size-3.5 animate-spin text-muted-foreground" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.saving') }}</span>
      </template>
      <span v-else-if="autosave.status.value === 'too_long'" class="text-destructive" role="alert">{{ t('page.save.tooLong') }}</span>
      <span v-else-if="autosave.status.value === 'error'" class="text-destructive" role="alert">
        {{ t('page.save.error', { reason: t(`errors.${autosave.errorCode.value ?? 'unknown'}`) }) }}
      </span>
    </div>
    <PageEditor
      :doc="shown"
      :refs="autosave.refs.value"
      :editable="editable && autosave.status.value !== 'conflict'"
      :owner="owner"
      @change="autosave.change"
    />
  </section>
</template>
