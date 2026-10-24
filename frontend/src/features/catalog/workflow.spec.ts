import { describe, expect, it } from 'vitest'
import { cellsFrom, transitionsFrom } from './workflow'

describe('rejilla del flujo de estados', () => {
  it('pasa de transiciones a celdas y vuelve', () => {
    const transitions = [
      { from_status_id: 'a', to_status_id: 'b', role: null },
      { from_status_id: 'b', to_status_id: 'c', role: 'lead' as const },
    ]
    const cells = cellsFrom(transitions)
    expect(cells).toEqual({ 'a>b': 'all', 'b>c': 'lead' })
    expect(transitionsFrom({ ...cells, 'c>a': '' })).toEqual(transitions)
  })

  it('una transición para todos los roles gana a una por rol', () => {
    const cells = cellsFrom([
      { from_status_id: 'a', to_status_id: 'b', role: 'member' },
      { from_status_id: 'a', to_status_id: 'b', role: null },
    ])
    expect(cells['a>b']).toBe('all')
  })
})
