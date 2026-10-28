export type CellKind = 'text' | 'number' | 'date' | 'select' | 'boolean' | 'color'

export interface CellOption {
  value: string
  label: string
  disabled?: boolean
}
