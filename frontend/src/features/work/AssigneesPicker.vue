<script setup lang="ts">
import { onClickOutside } from '@vueuse/core'
import { Check, LoaderCircle } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn } from '@/lib/utils'
import type { Ref as Person } from '@/types/work'
import AvatarStack from './AvatarStack.vue'

// The assignees of a work item, chosen with boxes in a list that searches.
// The change saves when the list closes (spec 006, RF-17).
const props = withDefaults(
  defineProps<{
    id?: string
    value: Person[]
    /** People that can be added. */
    options: Person[]
    label: string
    save: (ids: string[]) => Promise<unknown>
    editable?: boolean
    max?: number
    class?: string
  }>(),
  { id: undefined, editable: true, max: 20, class: '' },
)
const { t } = useI18n()

const root = ref<HTMLElement>()
const search = ref<HTMLInputElement>()
const open = ref(false)
const query = ref('')
const draft = ref<string[]>([])
const state = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')

// The current ones first, also when they are no longer among the options (RF-15).
const people = computed(() => {
  const current = props.value
  const others = props.options.filter((o) => !current.some((c) => c.id === o.id))
  const q = query.value.trim().toLocaleLowerCase()
  return [...current, ...others].filter((p) => !q || p.name.toLocaleLowerCase().includes(q))
})
const full = computed(() => draft.value.length >= props.max)

async function toggleOpen() {
  if (!props.editable || state.value === 'saving') return
  if (open.value) return close()
  draft.value = props.value.map((p) => p.id)
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
  const before = props.value.map((p) => p.id)
  if (before.length === draft.value.length && before.every((id, i) => id === draft.value[i])) return
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
          '-mx-1.5 flex h-8 w-full min-w-0 items-center gap-2 rounded-md border border-transparent px-1.5 text-left text-sm outline-none',
          editable && 'hover:bg-muted/60 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
          !editable && 'cursor-default',
          state === 'error' && 'border-destructive',
        )
      "
      :aria-label="label"
      :aria-expanded="editable ? open : undefined"
      :aria-invalid="state === 'error' || undefined"
      :disabled="state === 'saving'"
      data-testid="assignees"
      @click="toggleOpen"
    >
      <AvatarStack :people="value" />
    </button>
    <LoaderCircle v-if="state === 'saving'" class="absolute right-1 size-3.5 animate-spin text-primary" aria-hidden="true" />
    <Check v-else-if="state === 'saved'" class="absolute right-1 size-3.5 text-green-700" aria-hidden="true" />
    <div
      v-if="open"
      class="absolute top-full left-0 z-40 mt-1 flex w-64 flex-col gap-1 rounded-lg border bg-popover p-2 text-popover-foreground shadow-lg"
      role="dialog"
      :aria-label="label"
      data-testid="assignees-menu"
    >
      <input
        ref="search"
        v-model="query"
        type="search"
        class="h-8 rounded-md border bg-background px-2 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        :placeholder="t('workItems.searchPeople')"
        :aria-label="t('workItems.searchPeople')"
      />
      <ul class="max-h-60 overflow-y-auto">
        <li v-for="p in people" :key="p.id">
          <label class="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-sm hover:bg-muted">
            <input
              type="checkbox"
              class="size-4 accent-primary"
              :checked="draft.includes(p.id)"
              :disabled="full && !draft.includes(p.id)"
              @change="toggle(p.id)"
            />
            <span class="truncate">{{ p.name }}</span>
          </label>
        </li>
        <li v-if="people.length === 0" class="px-1.5 py-1 text-sm text-muted-foreground">{{ t('common.empty') }}</li>
      </ul>
      <p v-if="full" class="px-1.5 text-xs text-muted-foreground">{{ t('workItems.maxAssignees', { max }) }}</p>
      <button type="button" class="self-end rounded-md px-2 py-1 text-sm font-medium hover:bg-muted" data-testid="assignees-done" @click="close">
        {{ t('workItems.done') }}
      </button>
    </div>
  </span>
</template>
