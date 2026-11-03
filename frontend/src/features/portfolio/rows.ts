import type { PortfolioRow } from '@/api/portfolio'
import type { Level } from '@/api/schedule'

// Filtering and sorting the rows of the portfolio (spec 010, RF-32 to RF-36).

export type SortKey = 'name' | 'plan' | 'real' | 'gap'
export interface Sort {
  key: SortKey
  dir: 'asc' | 'desc'
}
/** The most behind first while nothing else is chosen (RF-36). */
export const DEFAULT_SORT: Sort = { key: 'gap', dir: 'asc' }

/** A level of the traffic light, or «Sin plazo». */
export type LevelFilter = Level | 'none'
export const levelOf = (r: PortfolioRow): LevelFilter => (r.figures.has_plan && r.figures.level ? r.figures.level : 'none')

export function sortRows(rows: PortfolioRow[], sort: Sort): PortfolioRow[] {
  const value = (r: PortfolioRow) => (sort.key === 'name' ? r.name : sort.key === 'real' ? r.figures.real : r.figures.has_plan ? r.figures[sort.key] : null)
  const sign = sort.dir === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => {
    const va = value(a)
    const vb = value(b)
    // Without a plan, always last.
    if (va === null || vb === null) return va === vb ? a.name.localeCompare(b.name) : va === null ? 1 : -1
    const c = typeof va === 'string' ? va.localeCompare(vb as string) : va - (vb as number)
    return c === 0 ? a.name.localeCompare(b.name) : sign * c
  })
}

export const filterRows = (rows: PortfolioRow[], levels: LevelFilter[]) => (levels.length ? rows.filter((r) => levels.includes(levelOf(r))) : rows)
