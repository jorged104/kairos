import { describe, expect, it } from 'vitest'
import type { Team } from '@/types/api'
import { descendants, flatten, ledScope } from './tree'

const team = (id: string, parent: string | null, extra: Partial<Team> = {}): Team => ({
  id,
  name: id,
  parent_id: parent,
  external_ref: null,
  archived_at: null,
  leaders: [],
  member_count: 0,
  ...extra,
})

const teams = [
  team('bi', null, { leaders: [{ user_id: 'lead', name: 'Lead' }] }),
  team('bi-rep', 'bi'),
  team('bi-rep-2', 'bi-rep'),
  team('dev', null),
]

describe('árbol de equipos', () => {
  it('incluye los subequipos a cualquier profundidad', () => {
    expect([...descendants(teams, 'bi')].sort()).toEqual(['bi', 'bi-rep', 'bi-rep-2'])
  })

  it('el ámbito de un líder hereda los subequipos', () => {
    expect([...ledScope(teams, 'lead')].sort()).toEqual(['bi', 'bi-rep', 'bi-rep-2'])
    expect(ledScope(teams, 'nadie').size).toBe(0)
  })

  it('aplana en orden de árbol con su profundidad', () => {
    expect(flatten(teams).map(({ team, depth }) => `${depth}:${team.id}`)).toEqual([
      '0:bi',
      '1:bi-rep',
      '2:bi-rep-2',
      '0:dev',
    ])
  })
})
