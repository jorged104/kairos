<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listProjects } from '@/api/work'
import { Input } from '@/components/ui/input'

// Search of an active project to hang under a program (spec 010, RF-44).
const props = defineProps<{ programId: string; exclude: string[]; disabled?: boolean }>()
const emit = defineEmits<{ pick: [id: string] }>()
const { t } = useI18n()

const search = ref('')
const q = refDebounced(computed(() => search.value.trim()), 250)
const { data } = useQuery({
  queryKey: computed(() => ['projects', 'portfolio-add', q.value]),
  queryFn: () => listProjects({ q: q.value, lifecycle: 'activo', page_size: 20 }),
  enabled: computed(() => q.value.length > 0),
})
const found = computed(() => (data.value?.items ?? []).filter((p) => p.id !== props.programId && !props.exclude.includes(p.id)))
function pick(id: string) {
  search.value = ''
  emit('pick', id)
}
</script>

<template>
  <div class="relative w-full max-w-80">
    <Input
      v-model="search"
      type="search"
      :placeholder="t('portfolio.add.placeholder')"
      :aria-label="t('portfolio.add.label')"
      :disabled="disabled"
      data-testid="portfolio-add-search"
    />
    <ul v-if="q && search && found.length" class="absolute top-full z-10 mt-1 w-full rounded-md border bg-background shadow" role="listbox">
      <li v-for="p in found" :key="p.id">
        <button type="button" class="w-full px-2 py-1.5 text-left text-sm hover:bg-muted" data-testid="portfolio-add-found" @click="pick(p.id)">
          <span class="font-mono text-xs text-muted-foreground">{{ p.identifier }}</span> {{ p.name }}
        </button>
      </li>
    </ul>
    <p v-else-if="q && search && data" class="absolute top-full z-10 mt-1 w-full rounded-md border bg-background px-2 py-1.5 text-xs text-muted-foreground shadow">
      {{ t('portfolio.add.none') }}
    </p>
  </div>
</template>
