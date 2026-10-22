import type { Team } from '@/types/api'

/** The team and all its subteams, at any depth. */
export function descendants(teams: Team[], rootId: string): Set<string> {
  const out = new Set<string>([rootId])
  let added = true
  while (added) {
    added = false
    for (const t of teams) {
      if (t.parent_id && out.has(t.parent_id) && !out.has(t.id)) {
        out.add(t.id)
        added = true
      }
    }
  }
  return out
}

/** Teams a user leads plus their subteams: where a lead may act. */
export function ledScope(teams: Team[], userId: string): Set<string> {
  const out = new Set<string>()
  for (const t of teams) {
    if (!t.archived_at && t.leaders.some((l) => l.user_id === userId)) {
      for (const id of descendants(teams, t.id)) out.add(id)
    }
  }
  return out
}

/** Teams in tree order with their depth, for indented lists. */
export function flatten(teams: Team[]): { team: Team; depth: number }[] {
  const byParent = new Map<string | null, Team[]>()
  for (const t of teams) {
    const key = t.parent_id && teams.some((p) => p.id === t.parent_id) ? t.parent_id : null
    byParent.set(key, [...(byParent.get(key) ?? []), t])
  }
  const out: { team: Team; depth: number }[] = []
  const walk = (parent: string | null, depth: number) => {
    for (const t of (byParent.get(parent) ?? []).sort((a, b) => a.name.localeCompare(b.name))) {
      out.push({ team: t, depth })
      walk(t.id, depth + 1)
    }
  }
  walk(null, 0)
  return out
}
