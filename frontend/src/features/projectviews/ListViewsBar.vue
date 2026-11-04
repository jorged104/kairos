<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { onClickOutside } from '@vueuse/core'
import { Share2, Trash2 } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createListView, deleteListView, listListViews, updateListView, type ListView, type ListViewConfig } from '@/api/projectBoard'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { emptyConfig, sameConfig } from './filters'
import { remembered } from './useListState'

// «Todos», the shared views and one's own, each with its count; «Cambios sin
// guardar» when the list moved away from the open view (spec 012, RF-69 to
// RF-79).
const props = defineProps<{
  viewId: string
  config: ListViewConfig
  /** The projects each view shows, counted in the browser (RF-72). */
  count: (config: ListViewConfig | null) => number
  canShare: boolean
}>()
const emit = defineEmits<{ open: [view: ListView | null] }>()
const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const key = ['project-views']
const { data } = useQuery({ queryKey: key, queryFn: listListViews })
const views = computed(() => data.value?.items ?? [])
const active = computed(() => views.value.find((v) => v.id === props.viewId) ?? null)
// «Todos» has no filters but keeps the mode in use.
const dirty = computed(() => {
  const base = active.value?.config ?? { ...emptyConfig(), mode: props.config.mode }
  return !sameConfig(base, props.config)
})

// The last view used opens again when the address brings none (RF-79).
onMounted(async () => {
  if (props.viewId !== 'all' || Object.keys(props.config.filters).length) return
  const last = remembered()
  if (last.view === 'all') return
  const list = await qc.ensureQueryData({ queryKey: key, queryFn: listListViews })
  const v = list.items.find((x) => x.id === last.view)
  if (v) emit('open', v)
})

const saving = ref<null | 'new' | 'rename'>(null)
const name = ref('')
const shared = ref(false)
const error = ref<unknown>(null)
const panel = ref<HTMLElement | null>(null)
onClickOutside(panel, () => (saving.value = null))
function startSave(mode: 'new' | 'rename') {
  saving.value = saving.value === mode ? null : mode
  name.value = mode === 'rename' ? (active.value?.name ?? '') : ''
  shared.value = mode === 'rename' ? !!active.value?.shared : false
  error.value = null
}
const refresh = () => qc.invalidateQueries({ queryKey: key })

async function save() {
  error.value = null
  try {
    const v =
      saving.value === 'rename' && active.value
        ? await updateListView(active.value.id, { name: name.value, shared: shared.value, config: active.value.config })
        : await createListView({ name: name.value, shared: shared.value, config: props.config })
    await refresh()
    emit('open', v)
    saving.value = null
  } catch (e) {
    error.value = e
  }
}
async function updateActive() {
  if (!active.value) return
  error.value = null
  try {
    await updateListView(active.value.id, { name: active.value.name, shared: active.value.shared, config: props.config })
    await refresh()
  } catch (e) {
    error.value = e
  }
}
const armed = ref(false)
async function remove() {
  if (!active.value) return
  if (!armed.value) {
    armed.value = true
    setTimeout(() => (armed.value = false), 3000)
    return
  }
  armed.value = false
  try {
    await deleteListView(active.value.id)
    await refresh()
    emit('open', null)
  } catch (e) {
    error.value = e
  }
}
</script>

<template>
  <section class="relative flex flex-wrap items-center gap-1.5" :aria-label="t('views.title')" data-testid="list-views-bar">
    <span class="mr-1 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{{ t('views.title') }}</span>
    <Button size="sm" :variant="viewId === 'all' ? 'default' : 'outline'" :aria-pressed="viewId === 'all'" data-testid="list-view-all" @click="emit('open', null)">
      {{ t('projectViews.all') }} <span class="opacity-70">· {{ count(null) }}</span>
    </Button>
    <Button
      v-for="v in views"
      :key="v.id"
      size="sm"
      :variant="viewId === v.id ? 'default' : 'outline'"
      :aria-pressed="viewId === v.id"
      data-testid="list-view"
      @click="emit('open', v)"
    >
      <Share2 v-if="v.shared" class="size-3.5" :aria-label="t('views.shared')" />
      {{ v.name }} <span class="opacity-70">· {{ count(v.config) }}</span>
    </Button>

    <span class="flex-1" />
    <template v-if="dirty">
      <span class="text-xs font-medium text-amber-700 dark:text-amber-400" data-testid="list-views-dirty">{{ t('views.dirty') }}</span>
      <Button v-if="active?.can_edit" size="sm" variant="outline" data-testid="list-view-update" @click="updateActive">{{ t('views.update') }}</Button>
    </template>
    <template v-if="active?.can_edit">
      <Button size="sm" variant="ghost" data-testid="list-view-rename" @click="startSave('rename')">{{ t('views.rename') }}</Button>
      <Button size="sm" :variant="armed ? 'destructive' : 'ghost'" :aria-label="armed ? t('views.confirmDelete') : t('views.delete')" data-testid="list-view-delete" @click="remove">
        <Trash2 aria-hidden="true" /><span v-if="armed">{{ t('views.confirmDelete') }}</span>
      </Button>
    </template>
    <Button size="sm" data-testid="list-view-save" @click="startSave('new')">{{ t('views.saveAs') }}</Button>

    <div
      v-if="saving"
      ref="panel"
      class="absolute top-full right-0 z-30 mt-1 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-3 rounded-lg border bg-popover p-3 shadow-lg"
      role="dialog"
      :aria-label="saving === 'new' ? t('views.saveAs') : t('views.rename')"
      data-testid="list-view-panel"
    >
      <label class="flex flex-col gap-1 text-xs font-medium text-muted-foreground">
        {{ t('views.name') }}
        <input v-model="name" maxlength="100" class="h-9 rounded-md border bg-background px-2 text-sm text-foreground" data-testid="list-view-name" @keydown.enter.prevent="save" />
      </label>
      <label v-if="canShare" class="flex items-center gap-2 text-sm">
        <input v-model="shared" type="checkbox" class="size-4 accent-primary" data-testid="list-view-shared" />
        {{ t('projectViews.shareAll') }}
      </label>
      <p class="text-xs text-muted-foreground">{{ t('projectViews.hint') }}</p>
      <p v-if="error" role="alert" class="text-xs text-destructive">{{ errorMessage(error) }}</p>
      <div class="flex justify-end gap-2">
        <Button size="sm" variant="ghost" @click="saving = null">{{ t('common.cancel') }}</Button>
        <Button size="sm" :disabled="!name.trim()" data-testid="list-view-confirm" @click="save">{{ t('common.save') }}</Button>
      </div>
    </div>
    <p v-if="error && !saving" role="alert" class="w-full text-xs text-destructive">{{ errorMessage(error) }}</p>
  </section>
</template>
