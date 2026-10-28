// Spreadsheet-like keyboard moves between the editable cells of a table.
// Editable cells carry `data-grid-cell` and are focusable.

export type GridKey = 'ArrowUp' | 'ArrowDown' | 'ArrowLeft' | 'ArrowRight'

const CELL = '[data-grid-cell]'

/** The editable cell the key moves to from `cell`, or null at an edge. */
export function neighbor(cell: HTMLElement, key: GridKey): HTMLElement | null {
  const row = cell.closest('tr')
  if (!row) return null
  if (key === 'ArrowLeft' || key === 'ArrowRight') {
    const cells = [...row.querySelectorAll<HTMLElement>(CELL)]
    return cells[cells.indexOf(cell) + (key === 'ArrowRight' ? 1 : -1)] ?? null
  }
  // Up and down keep the column; rows without an editable cell there are skipped.
  const column = (cell as HTMLTableCellElement).cellIndex
  let next = key === 'ArrowDown' ? row.nextElementSibling : row.previousElementSibling
  while (next) {
    const target = (next as HTMLTableRowElement).cells?.[column]
    if (target?.matches(CELL)) return target
    next = key === 'ArrowDown' ? next.nextElementSibling : next.previousElementSibling
  }
  return null
}

/** Keydown handler for a table: arrows move the focus while no cell is being edited. */
export function onGridKeydown(event: KeyboardEvent) {
  const target = event.target as HTMLElement
  if (!target.matches(CELL)) return
  if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) return
  const next = neighbor(target, event.key as GridKey)
  if (!next) return
  event.preventDefault()
  next.focus()
}
