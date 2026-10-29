<script setup lang="ts">
import { Check, LoaderCircle } from '@lucide/vue'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CellKind, CellOption } from '@/components/data-table/types'
import { cn } from '@/lib/utils'

// A property of the work item edited where it is shown, without an edit mode:
// a select saves on change; a field saves on Enter or on leaving it, and Esc
// restores it. The error itself is shown by the sheet.
const props = withDefaults(
  defineProps<{
    id: string
    value: string
    label: string
    save: (value: string) => Promise<unknown>
    kind?: Exclude<CellKind, 'boolean' | 'color'>
    options?: CellOption[]
    /** The empty choice of a select; none when the field is required. */
    emptyLabel?: string
    editable?: boolean
    required?: boolean
    min?: number
    max?: number
    step?: number
    class?: string
  }>(),
  { kind: 'text', options: () => [], editable: true, required: false, class: '' },
)
const { t } = useI18n()

const draft = ref(props.value)
const state = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
watch(
  () => props.value,
  (v) => {
    draft.value = v
    if (state.value === 'error') state.value = 'idle'
  },
)

let savedTimer: ReturnType<typeof setTimeout> | undefined
async function commit() {
  const v = props.kind === 'text' ? draft.value.trim() : draft.value
  if (v === props.value || state.value === 'saving') return
  if (props.required && v === '') {
    draft.value = props.value
    return
  }
  state.value = 'saving'
  try {
    await props.save(v)
    state.value = 'saved'
    clearTimeout(savedTimer)
    savedTimer = setTimeout(() => state.value === 'saved' && (state.value = 'idle'), 1500)
  } catch {
    state.value = 'error'
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault() // the panel closes with Escape only when nothing uses it
    draft.value = props.value
    state.value = 'idle'
    ;(event.target as HTMLElement).blur()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    void commit()
  }
}

const control = (extra: string) =>
  cn(
    '-mx-1.5 h-8 w-full min-w-0 rounded-md border border-transparent bg-transparent px-1.5 text-sm outline-none',
    'enabled:hover:bg-muted/60 focus-visible:border-ring focus-visible:bg-background focus-visible:ring-3 focus-visible:ring-ring/50',
    'disabled:cursor-default disabled:opacity-100',
    state.value === 'error' && 'border-destructive',
    extra,
    props.class,
  )
</script>

<template>
  <span class="relative flex items-center">
    <select
      v-if="kind === 'select'"
      :id="id"
      v-model="draft"
      :class="control(editable ? 'cursor-pointer' : 'appearance-none')"
      :disabled="!editable || state === 'saving'"
      :aria-label="label"
      :aria-invalid="state === 'error' || undefined"
      @change="commit"
      @keydown.esc="onKeydown"
    >
      <option v-if="!required" value="">{{ emptyLabel ?? t('common.none') }}</option>
      <option v-for="o in options" :key="o.value" :value="o.value" :disabled="o.disabled">{{ o.label }}</option>
    </select>
    <input
      v-else
      :id="id"
      v-model="draft"
      :type="kind"
      :class="control('')"
      :placeholder="t('common.none')"
      :readonly="!editable"
      :disabled="state === 'saving'"
      :required="required"
      :min="min"
      :max="max"
      :step="step"
      :aria-label="label"
      :aria-invalid="state === 'error' || undefined"
      @keydown="onKeydown"
      @blur="commit"
    />
    <LoaderCircle v-if="state === 'saving'" class="absolute right-1 size-3.5 animate-spin text-primary" aria-hidden="true" />
    <Check v-else-if="state === 'saved'" class="absolute right-1 size-3.5 text-green-700" aria-hidden="true" />
  </span>
</template>
