import type { WorkNode } from '@/types/work'

export interface TreeRow {
  node: WorkNode
  level: number
  hasChildren: boolean
  expanded: boolean
}

/** Children of each work item (null = roots), in position and number order. */
export function childrenOf(nodes: WorkNode[]): Map<string | null, WorkNode[]> {
  const ids = new Set(nodes.map((n) => n.id))
  const out = new Map<string | null, WorkNode[]>()
  for (const n of nodes) {
    // A parent missing from the list (e.g. a filtered view) makes it a root.
    const key = n.parent_id && ids.has(n.parent_id) ? n.parent_id : null
    const list = out.get(key)
    if (list) list.push(n)
    else out.set(key, [n])
  }
  const order = (a: WorkNode, b: WorkNode) => a.position - b.position || seq(a) - seq(b)
  for (const list of out.values()) list.sort(order)
  return out
}

function seq(n: WorkNode): number {
  return Number(n.key.slice(n.key.lastIndexOf('-') + 1))
}

/**
 * The rows to paint: only the children of expanded work items, so a project
 * with thousands of items renders what the user opened (RF-130).
 */
export function visibleRows(children: Map<string | null, WorkNode[]>, expanded: Set<string>): TreeRow[] {
  const out: TreeRow[] = []
  const walk = (parent: string | null, level: number) => {
    for (const node of children.get(parent) ?? []) {
      const hasChildren = (children.get(node.id)?.length ?? 0) > 0
      const open = hasChildren && expanded.has(node.id)
      out.push({ node, level, hasChildren, expanded: open })
      if (open) walk(node.id, level + 1)
    }
  }
  walk(null, 0)
  return out
}

/** Every id with children, to expand the whole tree. */
export function allParents(children: Map<string | null, WorkNode[]>): Set<string> {
  return new Set([...children.keys()].filter((k): k is string => k !== null))
}
