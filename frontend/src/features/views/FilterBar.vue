<script setup lang="ts">
import { onClickOutside } from '@vueuse/core'
import { Filter, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { FILTER_FIELDS, SINGLE, type FilterField } from './useProjectFilters'
import type { FilterOption } from './useFilterOptions'

// The active filters as chips that go with one click, and «+ Filtro» to add
// one: a field and its values (spec 008, RF-18 to RF-21). The project list
// brings its own fields (spec 012, RF-62).
const props = withDefaults(
  defineProps<{
    filters: Record<string, string[]>
    options: Partial<Record<string, FilterOption[]>>
    label: (field: string, value: string) => string
    available: (field: string, value: string) => boolean
    fields?: readonly string[]
    kindOf?: (field: string) => 'text' | 'date' | 'one' | 'many'
    /** Where the names of the fields are in the translations. */
    labels?: string
  }>(),
  {
    fields: () => FILTER_FIELDS,
    kindOf: (f: string) => (f === 'q' ? 'text' : f === 'due_before' || f === 'due_after' ? 'date' : SINGLE.includes(f as FilterField) ? 'one' : 'many'),
    labels: 'filters.fields',
  },
)
const emit = defineEmits<{ set: [field: FilterField, values: string[]]; remove: [field: string, value: string] }>()
const { t } = useI18n()
const fieldName = (f: string) => t(`${props.labels}.${f}`)

const chips = computed(() =>
  Object.entries(props.filters).flatMap(([field, values]) =>
    values.map((value) => ({ field, value, text: props.label(field, value), ok: props.available(field, value) })),
  ),
)

const adding = ref(false)
const panel = ref<HTMLElement | null>(null)
onClickOutside(panel, () => (adding.value = false))
const field = ref<FilterField>(props.fields[0] as FilterField)
const picked = ref<string[]>([])
const text = ref('')
const kind = computed(() => props.kindOf(field.value))
watch(field, () => {
  picked.value = [...(props.filters[field.value] ?? [])]
  text.value = props.filters[field.value]?.[0] ?? ''
})
function open() {
  adding.value = !adding.value
  picked.value = [...(props.filters[field.value] ?? [])]
  text.value = props.filters[field.value]?.[0] ?? ''
}
function toggle(v: string) {
  if (kind.value === 'one') picked.value = picked.value[0] === v ? [] : [v]
  else picked.value = picked.value.includes(v) ? picked.value.filter((x) => x !== v) : [...picked.value, v]
}
function add() {
  emit('set', field.value, kind.value === 'text' || kind.value === 'date' ? (text.value.trim() ? [text.value.trim()] : []) : picked.value)
  adding.value = false
}
</script>

<template>
  <section class="relative flex flex-wrap items-center gap-2 rounded-lg border bg-background p-2" :aria-label="t('filters.title')" data-testid="filter-bar">
    <Filter class="ml-1 size-4 text-muted-foreground" aria-hidden="true" />
    <span
      v-for="c in chips"
      :key="c.field + c.value"
      class="inline-flex h-8 items-center gap-1 rounded-full bg-muted pr-1 pl-3 text-sm"
      :class="!c.ok && 'line-through opacity-70'"
      :title="c.ok ? undefined : t('filters.unavailable')"
      data-testid="filter-chip"
    >
      <span class="text-muted-foreground">{{ fieldName(c.field) }}</span>
      <span class="font-medium">{{ c.text }}</span>
      <span v-if="!c.ok" class="text-xs text-muted-foreground">({{ t('filters.unavailable') }})</span>
      <button
        type="button"
        class="grid size-6 place-items-center rounded-full hover:bg-background"
        :aria-label="t('filters.remove', { field: fieldName(c.field), value: c.text })"
        data-testid="filter-remove"
        @click="emit('remove', c.field, c.value)"
      >
        <X class="size-3" aria-hidden="true" />
      </button>
    </span>
    <span v-if="chips.length === 0" class="text-sm text-muted-foreground">{{ t('filters.none') }}</span>
    <Button size="sm" variant="outline" class="rounded-full border-dashed" :aria-expanded="adding" data-testid="filter-add" @click="open">
      {{ t('filters.add') }}
    </Button>
    <slot />

    <div
      v-if="adding"
      ref="panel"
      class="absolute top-full left-0 z-30 mt-1 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-3 rounded-lg border bg-popover p-3 shadow-lg"
      role="dialog"
      :aria-label="t('filters.add')"
      data-testid="filter-panel"
    >
      <label class="flex flex-col gap-1 text-xs font-medium text-muted-foreground">
        {{ t('filters.field') }}
        <select v-model="field" class="h-9 rounded-md border bg-background px-2 text-sm text-foreground" data-testid="filter-field">
          <option v-for="f in fields" :key="f" :value="f">{{ fieldName(f) }}</option>
        </select>
      </label>
      <input
        v-if="kind === 'text'"
        v-model="text"
        type="search"
        class="h-9 rounded-md border bg-background px-2 text-sm"
        :aria-label="fieldName('q')"
        data-testid="filter-text"
        @keydown.enter.prevent="add"
      />
      <input v-else-if="kind === 'date'" v-model="text" type="date" class="h-9 rounded-md border bg-background px-2 text-sm" :aria-label="fieldName(field)" />
      <ul v-else class="flex max-h-60 flex-col overflow-y-auto" data-testid="filter-values">
        <li v-for="o in options[field] ?? []" :key="o.value">
          <label class="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-sm hover:bg-muted">
            <input
              :type="kind === 'one' ? 'radio' : 'checkbox'"
              :name="`filter-${field}`"
              class="size-4 accent-primary"
              :checked="picked.includes(o.value)"
              @click="toggle(o.value)"
            />
            <span v-if="o.color" class="size-2 rounded-full" :style="{ background: o.color }" />
            <span class="truncate">{{ o.label }}</span>
          </label>
        </li>
      </ul>
      <div class="flex justify-end gap-2">
        <Button size="sm" variant="ghost" @click="adding = false">{{ t('common.cancel') }}</Button>
        <Button size="sm" data-testid="filter-apply" @click="add">{{ t('filters.apply') }}</Button>
      </div>
    </div>
  </section>
</template>
