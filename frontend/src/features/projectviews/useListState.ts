import { computed } from 'vue'
import { useRoute, useRouter, type LocationQuery } from 'vue-router'
import { MODES, type ListView, type ListViewConfig, type Mode } from '@/api/projectBoard'
import { COLUMNS, FIELDS, GROUPS, emptyConfig, kindOf, type Filters } from './filters'

const list = (v: LocationQuery[string]): string[] => (Array.isArray(v) ? v : v ? [v] : []).filter((x): x is string => !!x)
const one = <T extends string>(v: LocationQuery[string], allowed: readonly T[]): T | '' =>
  typeof v === 'string' && (allowed as readonly string[]).includes(v) ? (v as T) : ''

const KEY = 'kairos.projects.list'

/** The last view and mode used, remembered in this browser (RF-79). */
export function remembered(): { view: string; mode: Mode | '' } {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (v && typeof v.view === 'string') return { view: v.view, mode: (MODES as readonly string[]).includes(v.mode) ? v.mode : '' }
  } catch {
    // nothing remembered
  }
  return { view: 'all', mode: '' }
}
function remember(view: string, mode: Mode | '') {
  try {
    localStorage.setItem(KEY, JSON.stringify({ view, mode }))
  } catch {
    // Not remembered this time.
  }
}

/**
 * The mode, the open view, the filters, the order, the columns and the
 * grouping of the project list, in the page's address so a link opens the
 * same (spec 012, RF-30, RF-31).
 */
export function useListState() {
  const route = useRoute()
  const router = useRouter()

  const filters = computed<Filters>(() => {
    const out: Filters = {}
    for (const f of FIELDS) {
      const v = list(route.query[f])
      if (v.length) out[f] = v
    }
    return out
  })
  const config = computed<ListViewConfig>(() => ({
    mode: one(route.query.mode, MODES),
    filters: filters.value,
    sort: typeof route.query.sort === 'string' ? route.query.sort : '',
    columns: one(route.query.columns, COLUMNS),
    group: one(route.query.group, GROUPS),
  }))
  const mode = computed<Mode>(() => config.value.mode || 'kanban')
  const viewId = computed(() => (typeof route.query.view === 'string' ? route.query.view : 'all'))

  function apply(next: Partial<ListViewConfig>, view?: string) {
    const c = { ...config.value, ...next }
    const query: Record<string, string | string[]> = {}
    for (const [k, v] of Object.entries(c.filters)) if (v.length) query[k] = v.length === 1 ? v[0]! : v
    if (c.mode) query.mode = c.mode
    if (c.sort) query.sort = c.sort
    if (c.columns) query.columns = c.columns
    if (c.group) query.group = c.group
    const id = view ?? viewId.value
    if (id !== 'all') query.view = id
    remember(id, c.mode)
    return router.replace({ query })
  }

  function setValues(field: string, values: string[]) {
    const f = { ...filters.value }
    if (values.length) f[field] = kindOf(field) === 'many' ? [...new Set(values)] : values.slice(0, 1)
    else delete f[field]
    return apply({ filters: f })
  }
  const removeValue = (field: string, value: string) => setValues(field, (filters.value[field] ?? []).filter((v) => v !== value))

  /** Opens a saved view, or «Todos» without one (RF-69, RF-73). */
  function openView(view: ListView | null) {
    // «Todos» keeps the mode in use.
    return apply(view ? { ...emptyConfig(), ...view.config, filters: { ...view.config.filters } } : { ...emptyConfig(), mode: config.value.mode }, view?.id ?? 'all')
  }

  return { filters, config, mode, viewId, apply, setValues, removeValue, openView }
}
