import { computed } from 'vue'
import { useRoute, useRouter, type LocationQuery } from 'vue-router'
import type { Filters, SavedView, ViewConfig } from '@/api/projectScreens'

/** The filters of the work items (spec 008, RF-18), as the API names them. */
export const FILTER_FIELDS = [
  'status_id', 'type_id', 'priority_id', 'assignee_id', 'label_id', 'size_id', 'milestone_id',
  'closed', 'blocked', 'overdue', 'due_before', 'due_after', 'q',
] as const
export type FilterField = (typeof FILTER_FIELDS)[number]

/** Fields with one value at most; the rest take several, joined with «o» (RF-20). */
export const SINGLE: FilterField[] = ['closed', 'blocked', 'overdue', 'due_before', 'due_after', 'q']

export const GROUPS = ['', 'milestone', 'status', 'assignee', 'type'] as const

const list = (v: LocationQuery[string]): string[] => (Array.isArray(v) ? v : v ? [v] : []).filter((x): x is string => !!x)

/** The same config, ignoring the order of the values. */
export function sameConfig(a: ViewConfig, b: ViewConfig): boolean {
  const keys = new Set([...Object.keys(a.filters), ...Object.keys(b.filters)])
  for (const k of keys) {
    const x = [...(a.filters[k] ?? [])].sort().join('\u0000')
    const y = [...(b.filters[k] ?? [])].sort().join('\u0000')
    if (x !== y) return false
  }
  return (a.sort || '') === (b.sort || '') && (a.group || '') === (b.group || '')
}

export const emptyConfig = (): ViewConfig => ({ filters: {}, sort: '', group: '' })

const lastViewKey = (projectId: string) => `kairos.project.${projectId}.view`

/** The last view used in a project, remembered in this browser (RF-32). */
export function rememberedView(projectId: string): string | null {
  try {
    return localStorage.getItem(lastViewKey(projectId))
  } catch {
    return null
  }
}
function remember(projectId: string, id: string) {
  try {
    localStorage.setItem(lastViewKey(projectId), id)
  } catch {
    // Not remembered this time.
  }
}

/**
 * The filters, order and grouping of a project's table, kanban and timeline,
 * kept in the page's address so a link opens the same (spec 008, RF-3, RF-22).
 */
export function useProjectFilters(projectId: () => string) {
  const route = useRoute()
  const router = useRouter()

  const filters = computed<Filters>(() => {
    const out: Filters = {}
    for (const f of FILTER_FIELDS) {
      const v = list(route.query[f])
      if (v.length) out[f] = v
    }
    return out
  })
  const sort = computed(() => (typeof route.query.sort === 'string' ? route.query.sort : ''))
  const group = computed<ViewConfig['group']>(() => {
    const g = route.query.group
    return typeof g === 'string' && (GROUPS as readonly string[]).includes(g) ? (g as ViewConfig['group']) : ''
  })
  const config = computed<ViewConfig>(() => ({ filters: filters.value, sort: sort.value, group: group.value }))
  const viewId = computed(() => (typeof route.query.view === 'string' ? route.query.view : 'all'))
  const filtering = computed(() => Object.keys(filters.value).length > 0)

  /** Replaces the config; the rest of the address (tab, item) stays. */
  function apply(next: Partial<ViewConfig>, view?: string) {
    const c = { ...config.value, ...next }
    const query: Record<string, string | string[]> = {}
    for (const [k, v] of Object.entries(route.query)) {
      if ((FILTER_FIELDS as readonly string[]).includes(k) || k === 'sort' || k === 'group' || k === 'view') continue
      if (typeof v === 'string') query[k] = v
    }
    for (const [k, v] of Object.entries(c.filters)) if (v.length) query[k] = v.length === 1 ? v[0]! : v
    if (c.sort) query.sort = c.sort
    if (c.group) query.group = c.group
    const id = view ?? viewId.value
    if (id && id !== 'all') query.view = id
    return router.replace({ query })
  }

  function setValues(field: FilterField, values: string[]) {
    const f = { ...filters.value }
    if (values.length) f[field] = SINGLE.includes(field) ? values.slice(0, 1) : [...new Set(values)]
    else delete f[field]
    return apply({ filters: f })
  }
  function removeValue(field: string, value: string) {
    return setValues(field as FilterField, (filters.value[field] ?? []).filter((v) => v !== value))
  }

  /** Opens a saved view, or «Todas» without one (RF-23). */
  function openView(view: SavedView | null) {
    remember(projectId(), view?.id ?? 'all')
    return apply(view ? { ...emptyConfig(), ...view.config, filters: { ...view.config.filters } } : emptyConfig(), view?.id ?? 'all')
  }

  return { filters, sort, group, config, viewId, filtering, apply, setValues, removeValue, openView }
}
