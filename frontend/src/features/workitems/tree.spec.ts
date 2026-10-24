import { describe, expect, it } from 'vitest'
import type { WorkNode } from '@/types/work'
import { allParents, childrenOf, visibleRows } from './tree'

function node(key: string, parent: string | null, position = 0): WorkNode {
  return {
    id: key, parent_id: parent, key, title: key, type_id: 't', status_id: 's', priority_id: 'p', assignee: null,
    derived_done_ratio: 0, derived_estimated_hours: 0, derived_remaining_hours: 0, depth: 0, position, due_date: null,
  }
}

const nodes = [
  node('K-3', null),
  node('K-1', null),
  node('K-2', 'K-1'),
  node('K-10', 'K-1'),
  node('K-4', 'K-2'),
  node('K-5', 'missing'),
]

describe('árbol de work items', () => {
  it('ordena por posición y número, y trata como raíz a quien no tiene padre en la lista', () => {
    const children = childrenOf(nodes)
    expect(children.get(null)?.map((n) => n.key)).toEqual(['K-1', 'K-3', 'K-5'])
    expect(children.get('K-1')?.map((n) => n.key)).toEqual(['K-2', 'K-10'])
  })

  it('solo pinta los hijos de lo expandido', () => {
    const children = childrenOf(nodes)
    expect(visibleRows(children, new Set()).map((r) => r.node.key)).toEqual(['K-1', 'K-3', 'K-5'])
    const rows = visibleRows(children, new Set(['K-1']))
    expect(rows.map((r) => [r.node.key, r.level, r.hasChildren])).toEqual([
      ['K-1', 0, true],
      ['K-2', 1, true],
      ['K-10', 1, false],
      ['K-3', 0, false],
      ['K-5', 0, false],
    ])
    expect(visibleRows(children, allParents(children))).toHaveLength(6)
  })

  it('arma 5 000 nodos sin recorrerlos más de una vez por nivel', () => {
    const big: WorkNode[] = []
    for (let c = 0; c < 500; c++) {
      for (let d = 0; d < 10; d++) big.push(node(`B-${c * 10 + d + 1}`, d === 0 ? null : `B-${c * 10 + d}`))
    }
    const started = performance.now()
    const children = childrenOf(big)
    const rows = visibleRows(children, allParents(children))
    expect(rows).toHaveLength(5000)
    expect(performance.now() - started).toBeLessThan(500)
  })
})
