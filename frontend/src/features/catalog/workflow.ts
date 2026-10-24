import type { Role } from '@/types/api'
import type { Transition } from '@/types/work'

/**
 * The workflow grid keeps one value per "from>to" cell: "" (not allowed),
 * "all" (every role) or a role. The API stores one row per allowed pair and
 * role, so a pair with several roles shows the first one.
 */
export function cellsFrom(transitions: Transition[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const tr of transitions) {
    const key = `${tr.from_status_id}>${tr.to_status_id}`
    if (!out[key] || tr.role === null) out[key] = tr.role ?? 'all'
  }
  return out
}

export function transitionsFrom(cells: Record<string, string>): Transition[] {
  const out: Transition[] = []
  for (const [key, value] of Object.entries(cells)) {
    if (!value) continue
    const [from, to] = key.split('>')
    out.push({ from_status_id: from, to_status_id: to, role: value === 'all' ? null : (value as Role) })
  }
  return out
}
