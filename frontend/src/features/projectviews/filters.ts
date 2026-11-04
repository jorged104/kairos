import type { BoardProject, ListViewConfig, Mode } from '@/api/projectBoard'
import type { Level } from '@/api/schedule'

// Filtering, sorting and grouping the project list, the same for the three
// modes and for the count of each view (spec 012, RF-62 to RF-68, RF-72).

export const FIELDS = ['status_id', 'level', 'health', 'program_id', 'team_id', 'owner_id', 'end', 'end_from', 'end_to', 'q'] as const
export type Field = (typeof FIELDS)[number]
export type Kind = 'many' | 'text' | 'date'
export const kindOf = (f: string): Kind => (f === 'q' ? 'text' : f === 'end_from' || f === 'end_to' ? 'date' : 'many')

export const END_VALUES = ['overdue', 'month', 'quarter', 'none'] as const

export type Filters = Record<string, string[]>

/** A level of the traffic light, or «Sin plazo». */
export const levelOf = (p: BoardProject): Level | 'none' => (p.figures.has_plan && p.figures.level ? p.figures.level : 'none')

export interface Context {
  /** Who looks: «Yo» (RF-63). */
  me: string
  /** YYYY-MM-DD. */
  today: string
}

const quarter = (d: string) => `${d.slice(0, 4)}-${Math.floor((Number(d.slice(5, 7)) - 1) / 3)}`

function endMatches(p: BoardProject, v: string, today: string): boolean {
  const end = p.end_date
  switch (v) {
    case 'overdue':
      return !!end && end < today && p.lifecycle === 'activo'
    case 'month':
      return !!end && end.slice(0, 7) === today.slice(0, 7)
    case 'quarter':
      return !!end && quarter(end) === quarter(today)
    default:
      return !end
  }
}

function fieldMatches(p: BoardProject, field: string, values: string[], ctx: Context): boolean {
  switch (field) {
    case 'status_id':
      return values.includes(p.status_id)
    case 'level':
      return values.includes(levelOf(p))
    case 'health':
      return values.includes(p.health)
    // Every project below the program, at any depth (RF-67).
    case 'program_id':
      return values.some((v) => (v === 'none' ? !p.program : p.program_ids.includes(v)))
    case 'team_id':
      return values.includes(p.team.id)
    case 'owner_id':
      return values.some((v) => (v === 'me' ? ctx.me : v) === p.owner.id)
    case 'end':
      return values.some((v) => endMatches(p, v, ctx.today))
    case 'end_from':
      return !!p.end_date && p.end_date >= values[0]!
    case 'end_to':
      return !!p.end_date && p.end_date <= values[0]!
    case 'q': {
      const q = values[0]!.trim().toLowerCase()
      return !q || p.name.toLowerCase().includes(q) || p.identifier.toLowerCase().includes(q)
    }
    default:
      return true
  }
}

/** Values of one field join with «o», fields with «y» (RF-65); values not
 * available are left out (RF-78). */
export function filter(rows: BoardProject[], filters: Filters, ctx: Context, available: (f: string, v: string) => boolean = () => true) {
  const active = Object.entries(filters)
    .map(([f, vs]) => [f, vs.filter((v) => available(f, v))] as const)
    .filter(([, vs]) => vs.length > 0)
  return rows.filter((p) => active.every(([f, vs]) => fieldMatches(p, f, vs, ctx)))
}

/** The orders of the Kanban and the Timeline (RF-68); the Table sorts by any column (RF-61). */
export const SORTS = ['gap', 'end', 'name', 'real'] as const
export const TABLE_SORTS = ['identifier', 'name', 'status', 'program', 'owner', 'health', 'real', 'plan', 'gap', 'start', 'end'] as const
export type SortKey = (typeof SORTS)[number]
/** The most behind first while nothing else is chosen (RF-68). */
export const DEFAULT_SORT = 'gap'

const HEALTH_ORDER = ['en_curso', 'en_riesgo', 'fuera_de_curso']

/** «-key» is descending. Without a plan or a date, always last. Statuses go
 * in the order of the catalog. */
export function sort(rows: BoardProject[], by: string, statusOrder: string[] = []): BoardProject[] {
  const desc = by.startsWith('-')
  const key = (desc ? by.slice(1) : by) || DEFAULT_SORT
  const value = (p: BoardProject): string | number | null => {
    switch (key) {
      case 'identifier':
        return p.identifier
      case 'name':
        return p.name
      case 'status':
        return statusOrder.indexOf(p.status_id)
      case 'program':
        return p.program?.name ?? null
      case 'owner':
        return p.owner.name
      case 'health':
        return HEALTH_ORDER.indexOf(p.health)
      case 'real':
        return p.figures.real
      case 'plan':
        return p.figures.has_plan ? p.figures.plan : null
      case 'start':
        return p.start_date
      case 'end':
        return p.end_date
      default:
        return p.figures.has_plan ? p.figures.gap : null
    }
  }
  const sign = desc ? -1 : 1
  return [...rows].sort((a, b) => {
    const va = value(a)
    const vb = value(b)
    if (va === null || vb === null) return va === vb ? a.name.localeCompare(b.name) : va === null ? 1 : -1
    const c = typeof va === 'string' ? va.localeCompare(vb as string) : va - (vb as number)
    return c === 0 ? a.name.localeCompare(b.name) : sign * c
  })
}

/** What forms the columns of the Kanban (RF-43). */
export const COLUMNS = ['status', 'level', 'health', 'program'] as const
export type Columns = (typeof COLUMNS)[number]
/** How the rows of the Timeline group (RF-51). */
export const GROUPS = ['program', 'status', 'owner', 'none'] as const
export type Group = (typeof GROUPS)[number]

export const emptyConfig = (): ListViewConfig => ({ mode: '', filters: {}, sort: '', columns: '', group: '' })

/** The same config, ignoring the order of the values and the defaults. */
export function sameConfig(a: ListViewConfig, b: ListViewConfig): boolean {
  const keys = new Set([...Object.keys(a.filters), ...Object.keys(b.filters)])
  for (const k of keys) {
    const x = [...(a.filters[k] ?? [])].sort().join('\u0000')
    const y = [...(b.filters[k] ?? [])].sort().join('\u0000')
    if (x !== y) return false
  }
  return (
    (a.mode || 'kanban') === (b.mode || 'kanban') &&
    (a.sort || DEFAULT_SORT) === (b.sort || DEFAULT_SORT) &&
    (a.columns || 'status') === (b.columns || 'status') &&
    (a.group || 'program') === (b.group || 'program')
  )
}

export const modeOf = (c: ListViewConfig): Mode => c.mode || 'kanban'
