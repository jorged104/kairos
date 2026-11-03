<script setup lang="ts">
import { onClickOutside } from '@vueuse/core'
import { Check, LoaderCircle } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn } from '@/lib/utils'
import type { Label } from '@/types/work'
import LabelChips from './LabelChips.vue'

// The labels of a work item, chosen with boxes in a list that searches, like
// the assignees: the change saves when the list closes (spec 007, RF-40 to
// RF-43). A retired label only shows where it already is (RF-39). The tree
// filter uses it too, with every label as an option.
const props = withDefaults(
  defineProps<{
    id?: string
    value: string[]
    /** The catalog, retired ones included. */
    options: Label[]
    label: string
    save: (ids: string[]) => Promise<unknown>
    editable?: boolean
    max?: number
    /** Chips shown before «+N». */
    visible?: number
    placeholder?: string
    class?: string
  }>(),
  { id: undefined, editable: true, max: 10, visible: Infinity, placeholder: undefined, class: '' },
)
const { t } = useI18n()

const root = ref<HTMLElement>()
const search = ref<HTMLInputElement>()
const open = ref(false)
const query = ref('')
const draft = ref<string[]>([])
const state = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')

const byId = computed(() => new Map(props.options.map((l) => [l.id, l])))
const current = computed(() => props.value.map((id) => byId.value.get(id)).filter((l): l is Label => !!l))
const choices = computed(() => {
  const q = query.value.trim().toLocaleLowerCase()
  return props.options
    .filter((l) => !l.retired || props.value.includes(l.id))
    .filter((l) => !q || l.name.toLocaleLowerCase().includes(q))
})
const full = computed(() => draft.value.length >= props.max)

async function toggleOpen() {
  if (!props.editable || state.value === 'saving') return
  if (open.value) return close()
  draft.value = [...props.value]
  query.value = ''
  open.value = true
  await nextTick()
  search.value?.focus()
}

function toggle(id: string) {
  const i = draft.value.indexOf(id)
  if (i >= 0) draft.value.splice(i, 1)
  else if (!full.value) draft.value.push(id)
}

let savedTimer: ReturnType<typeof setTimeout> | undefined
async function close() {
  if (!open.value) return
  open.value = false
  const before = props.value
  if (before.length === draft.value.length && before.every((id) => draft.value.includes(id))) return
  state.value = 'saving'
  try {
    await props.save([...draft.value])
    state.value = 'saved'
    clearTimeout(savedTimer)
    savedTimer = setTimeout(() => state.value === 'saved' && (state.value = 'idle'), 1500)
  } catch {
    state.value = 'error'
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault() // the panel closes with Escape only when nothing uses it
    e.stopPropagation()
    void close()
  }
}

onClickOutside(root, () => void close())
</script>

<template>
  <span ref="root" :class="cn('relative flex min-w-0 items-center', props.class)" @keydown="onKeydown">
    <button
      :id="id"
      type="button"
      :class="
        cn(
          '-mx-1.5 flex min-h-8 w-full min-w-0 items-center gap-2 rounded-md border border-transparent px-1.5 py-1 text-left text-sm outline-none',
          editable && 'hover:bg-muted/60 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
          !editable && 'cursor-default',
          state === 'error' && 'border-destructive',
        )
      "
      :aria-label="label"
      :aria-expanded="editable ? open : undefined"
      :aria-invalid="state === 'error' || undefined"
      :disabled="state === 'saving'"
      data-testid="labels"
      @click="toggleOpen"
    >
      <LabelChips :labels="current" :max="visible">
        <template #empty>
          <span class="text-muted-foreground">{{ placeholder ?? t('common.none') }}</span>
        </template>
      </LabelChips>
    </button>
    <LoaderCircle v-if="state === 'saving'" class="absolute right-1 size-3.5 animate-spin text-primary" aria-hidden="true" />
    <Check v-else-if="state === 'saved'" class="absolute right-1 size-3.5 text-green-700" aria-hidden="true" />
    <div
      v-if="open"
      class="absolute top-full left-0 z-40 mt-1 flex w-64 flex-col gap-1 rounded-lg border bg-popover p-2 text-popover-foreground shadow-lg"
      role="dialog"
      :aria-label="label"
      data-testid="labels-menu"
    >
      <input
        ref="search"
        v-model="query"
        type="search"
        class="h-8 rounded-md border bg-background px-2 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        :placeholder="t('labels.search')"
        :aria-label="t('labels.search')"
      />
      <ul class="max-h-60 overflow-y-auto">
        <li v-for="l in choices" :key="l.id">
          <label class="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-sm hover:bg-muted">
            <input
              type="checkbox"
              class="size-4 accent-primary"
              :checked="draft.includes(l.id)"
              :disabled="full && !draft.includes(l.id)"
              @change="toggle(l.id)"
            />
            <span class="size-2 shrink-0 rounded-full" :style="{ background: l.color }" />
            <span class="truncate" :class="l.retired && 'text-muted-foreground line-through'">{{ l.name }}</span>
          </label>
        </li>
        <li v-if="choices.length === 0" class="px-1.5 py-1 text-sm text-muted-foreground">{{ t('common.empty') }}</li>
      </ul>
      <p v-if="full && max !== Infinity" class="px-1.5 text-xs text-muted-foreground">{{ t('labels.max', { max }) }}</p>
      <button type="button" class="self-end rounded-md px-2 py-1 text-sm font-medium hover:bg-muted" data-testid="labels-done" @click="close">
        {{ t('workItems.done') }}
      </button>
    </div>
  </span>
</template>
