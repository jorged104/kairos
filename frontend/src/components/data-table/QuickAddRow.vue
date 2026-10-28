<script setup lang="ts">
import { LoaderCircle, Plus } from '@lucide/vue'
import { nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The last row of a table, where a new element is created by typing its name
// and pressing Enter in any of its fields or the «Crear» button (RF-145, RF-146).
// It stays open for the next one; Esc closes it.
const props = withDefaults(
  defineProps<{
    colspan: number
    /** Text of the closed row, e.g. «Nuevo work item». */
    label: string
    placeholder: string
    submit: (title: string) => Promise<unknown>
    /** Opens focused, e.g. a child row; closing it emits `close`. */
    open?: boolean
    indent?: string
  }>(),
  { open: false, indent: '0' },
)
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const active = ref(props.open)
const title = ref('')
const pending = ref(false)
const error = ref<unknown>(null)
// A required field left empty, kept apart from the API error (RF-147).
const missing = ref(false)
const form = ref<HTMLFormElement>()
const input = ref<HTMLInputElement>()
const opener = ref<HTMLButtonElement>()

async function show() {
  active.value = true
  await nextTick()
  input.value?.focus()
}
onMounted(() => props.open && show())

async function close() {
  active.value = false
  title.value = ''
  error.value = null
  missing.value = false
  emit('close')
  await nextTick()
  opener.value?.focus()
}

// Enter in a select, date or color field doesn't submit a form by itself.
function onEnter(event: KeyboardEvent) {
  if (event.isComposing || (event.target as HTMLElement).closest('button')) return
  event.preventDefault()
  create()
}

async function create() {
  if (pending.value) return
  const value = title.value.trim()
  const invalid = !value ? input.value : form.value?.querySelector<HTMLElement>(':invalid')
  if (invalid) {
    missing.value = true
    invalid.focus()
    return
  }
  missing.value = false
  pending.value = true
  error.value = null
  try {
    await props.submit(value)
    title.value = ''
    await nextTick()
    input.value?.focus()
  } catch (e) {
    error.value = e
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <tr class="border-b last:border-0" :class="active && 'bg-primary/5'" data-testid="quick-add">
    <td :colspan="colspan" class="px-2 py-1">
      <!-- Sticky and as wide as the visible table, so the button stays on a narrow screen (RF-148). -->
      <form
        v-if="active"
        ref="form"
        novalidate
        class="sticky left-0 flex max-w-[100cqw] flex-wrap items-center gap-2"
        :style="{ paddingLeft: indent }"
        @submit.prevent="create"
        @keydown.enter="onEnter"
        @keydown.esc.prevent="close"
        @input="missing = false"
      >
        <Plus class="size-4 shrink-0 text-primary" aria-hidden="true" />
        <input
          ref="input"
          v-model="title"
          class="h-8 min-w-48 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          :placeholder="placeholder"
          :aria-label="label"
          :disabled="pending"
          required
        />
        <slot />
        <span class="hidden text-xs text-muted-foreground sm:inline">{{ t('table.quickAddHint') }}</span>
        <Button type="submit" size="sm" :disabled="pending">
          <LoaderCircle v-if="pending" class="animate-spin" aria-hidden="true" />{{ t('common.create') }}
        </Button>
        <p v-if="missing || error" role="alert" class="basis-full text-xs text-destructive">
          {{ missing ? t('errors.required') : errorMessage(error) }}
        </p>
      </form>
      <button
        v-else
        ref="opener"
        type="button"
        class="flex h-8 items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
        @click="show"
      >
        <Plus class="size-4" aria-hidden="true" />{{ label }}
      </button>
    </td>
  </tr>
</template>
