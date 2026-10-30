import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { getCatalog, listCustomFields } from '@/api/work'
import type { Catalog } from '@/types/work'

const empty: Catalog = { methodologies: [], work_item_types: [], statuses: [], priorities: [], sizes: [] }

/** The catalog of spec 004 with lookups by id; retired elements included. */
export function useCatalog() {
  const { data } = useQuery({ queryKey: ['catalog'], queryFn: getCatalog, staleTime: 60_000 })
  const catalog = computed(() => data.value ?? empty)
  const byId = <T extends { id: string }>(list: T[]) => new Map(list.map((x) => [x.id, x]))
  return {
    catalog,
    types: computed(() => byId(catalog.value.work_item_types)),
    statuses: computed(() => byId(catalog.value.statuses)),
    priorities: computed(() => byId(catalog.value.priorities)),
    methodologies: computed(() => byId(catalog.value.methodologies)),
    sizes: computed(() => byId(catalog.value.sizes ?? [])),
  }
}

export function useCustomFields() {
  const { data } = useQuery({ queryKey: ['custom-fields'], queryFn: listCustomFields, staleTime: 60_000 })
  return computed(() => data.value ?? [])
}
