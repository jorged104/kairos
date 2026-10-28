<script setup lang="ts">
import { Check, LoaderCircle } from '@lucide/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { cn } from '@/lib/utils'
import type { CellKind, CellOption } from './types'

// A table cell edited in place, like a spreadsheet: click, Enter or typing
// starts editing; Enter, Tab or leaving the cell saves; Esc restores the value.
const props = withDefaults(
  defineProps<{
    value: string
    label: string
    save: (value: string) => Promise<unknown>
    kind?: CellKind
    /** Choices for a select; a function loads them when editing starts. */
    options?: CellOption[] | (() => Promise<CellOption[]>)
    editable?: boolean
    required?: boolean
    class?: string
  }>(),
  { kind: 'text', options: () => [], editable: true, required: false, class: '' },
)
const emit = defineEmits<{ reload: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const cell = ref<HTMLTableCellElement>()
const input = ref<HTMLInputElement | HTMLSelectElement>()
const editing = ref(false)
const draft = ref(props.value)
const loaded = ref<CellOption[]>([])
const state = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const error = ref<unknown>(null)
const conflict = computed(() => error.value instanceof ApiError && error.value.code === 'version_conflict')
const choices = computed(() => (Array.isArray(props.options) ? props.options : loaded.value))
const shown = computed(() =>
  props.kind === 'select' ? (choices.value.find((o) => o.value === props.value)?.label ?? props.value) : props.value,
)
// Instant kinds save on change and never enter an edit mode.
const instant = computed(() => props.kind === 'boolean' || props.kind === 'color')

watch(
  () => props.value,
  (v) => {
    if (!editing.value) draft.value = v
  },
)

async function start(initial?: string) {
  if (!props.editable || editing.value || instant.value) return
  error.value = null
  state.value = 'idle'
  draft.value = initial ?? props.value
  if (props.kind === 'select' && typeof props.options === 'function') {
    try {
      loaded.value = await props.options()
    } catch (e) {
      fail(e)
      return
    }
  }
  editing.value = true
  await nextTick()
  input.value?.focus()
  if (initial === undefined && input.value instanceof HTMLInputElement && props.kind === 'text') input.value.select()
}

function fail(e: unknown) {
  error.value = e
  state.value = 'error'
}

let savedTimer: ReturnType<typeof setTimeout> | undefined
async function commit(value = draft.value) {
  if (state.value === 'saving') return
  const v = props.kind === 'text' ? value.trim() : value
  if (v === props.value) {
    editing.value = false
    state.value = 'idle'
    error.value = null
    return
  }
  if (props.required && v === '') {
    fail('required')
    return
  }
  state.value = 'saving'
  try {
    await props.save(v)
    editing.value = false
    error.value = null
    state.value = 'saved'
    clearTimeout(savedTimer)
    savedTimer = setTimeout(() => state.value === 'saved' && (state.value = 'idle'), 1500)
  } catch (e) {
    fail(e)
  }
}

function cancel() {
  editing.value = false
  draft.value = props.value
  error.value = null
  state.value = 'idle'
  cell.value?.focus()
}

function onCellKeydown(event: KeyboardEvent) {
  if (event.target !== cell.value || !props.editable || instant.value) return
  if (event.key === 'Enter' || event.key === 'F2') {
    event.preventDefault()
    start()
  } else if ((event.key === 'Delete' || event.key === 'Backspace') && !props.required && props.kind !== 'select') {
    event.preventDefault()
    commit('')
  } else if (props.kind === 'text' && event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
    event.preventDefault()
    start(event.key)
  }
}

async function onInputKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    cancel()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    await commit()
    if (!editing.value) cell.value?.focus()
  }
}

// Leaving the cell saves it, unless the save failed: then the value stays for Esc or a retry.
function onBlur() {
  if (editing.value && state.value !== 'error') commit()
}

defineExpose({ start })
</script>

<template>
  <td
    ref="cell"
    :class="
      cn(
        'relative h-10 border-r px-2 align-middle whitespace-nowrap last:border-r-0 focus:outline-2 focus:-outline-offset-2 focus:outline-primary/40',
        editable && !instant && 'cursor-text hover:bg-muted/50',
        editing && 'bg-background outline-2 -outline-offset-2 outline-primary',
        state === 'error' && 'outline-2 -outline-offset-2 outline-destructive',
        props.class,
      )
    "
    :tabindex="editable ? 0 : undefined"
    :data-grid-cell="editable ? '' : undefined"
    :aria-label="label"
    :aria-invalid="state === 'error' || undefined"
    @click="start()"
    @keydown="onCellKeydown"
  >
    <template v-if="kind === 'boolean'">
      <input
        type="checkbox"
        class="size-4 accent-primary"
        :checked="value === 'true'"
        :disabled="!editable || state === 'saving'"
        :aria-label="label"
        @change="commit(($event.target as HTMLInputElement).checked ? 'true' : 'false')"
      />
    </template>
    <template v-else-if="kind === 'color'">
      <span class="flex items-center gap-2">
        <input
          type="color"
          class="size-6 cursor-pointer rounded border-0 bg-transparent p-0"
          :value="value"
          :disabled="!editable || state === 'saving'"
          :aria-label="label"
          @change="commit(($event.target as HTMLInputElement).value)"
        />
        <span class="font-mono text-xs text-muted-foreground uppercase">{{ value }}</span>
      </span>
    </template>
    <template v-else-if="editing">
      <select
        v-if="kind === 'select'"
        ref="input"
        v-model="draft"
        class="h-8 w-full bg-transparent text-sm outline-none"
        :aria-label="label"
        @change="commit()"
        @keydown="onInputKeydown"
        @blur="onBlur"
      >
        <option v-if="!required" value="">{{ t('common.none') }}</option>
        <option v-for="o in choices" :key="o.value" :value="o.value" :disabled="o.disabled">{{ o.label }}</option>
      </select>
      <input
        v-else
        ref="input"
        v-model="draft"
        :type="kind"
        class="h-8 w-full min-w-24 bg-transparent text-sm outline-none"
        :aria-label="label"
        @keydown="onInputKeydown"
        @blur="onBlur"
      />
    </template>
    <span v-else class="flex min-h-8 items-center gap-2">
      <slot :shown="shown">
        <span :class="{ 'text-muted-foreground': !shown }">{{ shown || t('common.none') }}</span>
      </slot>
    </span>
    <LoaderCircle v-if="state === 'saving'" class="absolute top-1/2 right-1.5 size-3.5 -translate-y-1/2 animate-spin text-primary" aria-hidden="true" />
    <Check v-else-if="state === 'saved'" class="absolute top-1/2 right-1.5 size-3.5 -translate-y-1/2 text-green-700" aria-hidden="true" />
    <span
      v-if="state === 'error'"
      role="alert"
      class="absolute top-full left-0 z-20 mt-0.5 flex max-w-80 items-center gap-2 rounded-md border border-destructive/30 bg-background px-2 py-1 text-xs whitespace-normal text-destructive shadow-sm"
    >
      {{ errorMessage(error) }}
      <button v-if="conflict" type="button" class="font-medium underline" @mousedown.prevent @click.stop="emit('reload'); cancel()">
        {{ t('table.reload') }}
      </button>
    </span>
  </td>
</template>
