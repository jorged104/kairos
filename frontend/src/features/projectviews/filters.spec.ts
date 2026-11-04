import { describe, expect, it } from 'vitest'
import type { BoardProject } from '@/api/projectBoard'
import { emptyConfig, filter, sameConfig, sort } from './filters'

const project = (id: string, extra: Partial<BoardProject> = {}): BoardProject => ({
  id, identifier: id.toUpperCase(), name: `Proyecto ${id}`, status_id: 's1', lifecycle: 'activo', health: 'en_curso',
  team: { id: 't1', name: 'Equipo' }, owner: { id: 'u1', name: 'Ana' }, is_program: false, program: null, program_ids: [],
  start_date: null, end_date: null, base_start: null, base_end: null, version: 1, can_manage: true,
  figures: { has_plan: false, plan: 0, real: 0, gap: 0 }, milestones: [], ...extra,
})
const fig = (plan: number, real: number, level: 'late' | 'on_track' | 'ahead') => ({ has_plan: true, plan, real, gap: real - plan, level })
const ctx = { me: 'u2', today: '2026-05-20' }
const ids = (rows: BoardProject[]) => rows.map((r) => r.id)

describe('filtros de proyectos (spec 012)', () => {
  const rows = [
    project('a', { owner: { id: 'u2', name: 'Yo' }, figures: fig(50, 30, 'late'), end_date: '2026-05-10', program: { id: 'p2', name: 'Sub' }, program_ids: ['p2', 'p1'] }),
    project('b', { status_id: 's2', figures: fig(50, 60, 'ahead'), end_date: '2026-06-15', health: 'en_riesgo' }),
    project('c', { end_date: '2026-09-01', team: { id: 't2', name: 'Otro' } }),
    project('d', { lifecycle: 'cerrado', end_date: '2026-05-01' }),
  ]

  it('combina valores con «o» y campos con «y» (RF-62 a RF-65)', () => {
    expect(ids(filter(rows, { status_id: ['s1'] }, ctx))).toEqual(['a', 'c', 'd'])
    expect(ids(filter(rows, { level: ['late', 'none'] }, ctx))).toEqual(['a', 'c', 'd'])
    expect(ids(filter(rows, { level: ['none'], team_id: ['t2'] }, ctx))).toEqual(['c'])
    expect(ids(filter(rows, { health: ['en_riesgo'] }, ctx))).toEqual(['b'])
    expect(ids(filter(rows, { q: ['proyecto C'] }, ctx))).toEqual(['c'])
  })

  it('resuelve «Yo», el programa a cualquier profundidad y las fechas (RF-63, RF-64, RF-67)', () => {
    expect(ids(filter(rows, { owner_id: ['me'] }, ctx))).toEqual(['a'])
    expect(ids(filter(rows, { program_id: ['p1'] }, ctx))).toEqual(['a'])
    expect(ids(filter(rows, { program_id: ['none'] }, ctx))).toEqual(['b', 'c', 'd'])
    // Overdue only while active.
    expect(ids(filter(rows, { end: ['overdue'] }, ctx))).toEqual(['a'])
    expect(ids(filter(rows, { end: ['month'] }, ctx))).toEqual(['a', 'd'])
    expect(ids(filter(rows, { end: ['quarter'] }, ctx))).toEqual(['a', 'b', 'd'])
    expect(ids(filter(rows, { end_from: ['2026-05-05'], end_to: ['2026-06-30'] }, ctx))).toEqual(['a', 'b'])
  })

  it('no filtra por los valores que ya no están disponibles (RF-78)', () => {
    expect(ids(filter(rows, { status_id: ['gone'] }, ctx, (_, v) => v !== 'gone'))).toEqual(['a', 'b', 'c', 'd'])
  })

  it('ordena por brecha con los más atrasados primero y los sin plan al final (RF-61, RF-68)', () => {
    expect(ids(sort(rows, ''))).toEqual(['a', 'b', 'c', 'd'])
    expect(ids(sort(rows, '-gap'))).toEqual(['b', 'a', 'c', 'd'])
    expect(ids(sort(rows, 'end'))).toEqual(['d', 'a', 'b', 'c'])
    expect(ids(sort(rows, 'status', ['s2', 's1']))[0]).toBe('b')
  })

  it('compara configuraciones con sus valores por defecto', () => {
    expect(sameConfig(emptyConfig(), { mode: 'kanban', filters: {}, sort: 'gap', columns: 'status', group: 'program' })).toBe(true)
    expect(sameConfig({ ...emptyConfig(), filters: { level: ['late', 'at_risk'] } }, { ...emptyConfig(), filters: { level: ['at_risk', 'late'] } })).toBe(true)
    expect(sameConfig(emptyConfig(), { ...emptyConfig(), mode: 'table' })).toBe(false)
  })
})
