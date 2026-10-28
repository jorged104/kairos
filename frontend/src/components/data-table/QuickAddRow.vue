<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The last row of a table, where a new element is created by typing its name
// and pressing Enter. It stays open for the next one; Esc closes it.
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
  emit('close')
  await nextTick()
  opener.value?.focus()
}

async function create() {
  const value = title.value.trim()
  if (!value || pending.value) return
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
      <form v-if="active" class="flex flex-wrap items-center gap-2" :style="{ paddingLeft: indent }" @submit.prevent="create" @keydown.esc.prevent="close">
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
        <span class="text-xs text-muted-foreground">{{ t('table.quickAddHint') }}</span>
        <button type="submit" class="sr-only">{{ t('common.create') }}</button>
        <p v-if="error" role="alert" class="basis-full text-xs text-destructive">{{ errorMessage(error) }}</p>
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
