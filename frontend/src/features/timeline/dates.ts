// Calendar days as whole numbers, so the timeline never meets time zones.

const DAY = 86400000

/** Days since 1970-01-01 of a YYYY-MM-DD date. */
export const toDay = (iso: string): number => Math.round(Date.UTC(+iso.slice(0, 4), +iso.slice(5, 7) - 1, +iso.slice(8, 10)) / DAY)

export const fromDay = (day: number): string => new Date(day * DAY).toISOString().slice(0, 10)

export const today = (): number => {
  const d = new Date()
  return Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / DAY)
}

export type Scale = 'weeks' | 'months' | 'quarters'

/** Pixels per day of each scale (RF-73). */
export const PX: Record<Scale, number> = { weeks: 28, months: 7, quarters: 2.5 }

/** The ticks of the header between two days: its label and position. */
export function ticks(from: number, to: number, scale: Scale, locale: string): { day: number; label: string }[] {
  const out: { day: number; label: string }[] = []
  const d = new Date(from * DAY)
  if (scale === 'weeks') {
    d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7)) // Monday
    const fmt = new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short', timeZone: 'UTC' })
    for (let x = d.getTime(); x / DAY <= to; x += 7 * DAY) out.push({ day: Math.round(x / DAY), label: fmt.format(new Date(x)) })
    return out
  }
  const step = scale === 'months' ? 1 : 3
  d.setUTCDate(1)
  d.setUTCMonth(d.getUTCMonth() - (d.getUTCMonth() % step))
  const fmt = new Intl.DateTimeFormat(locale, { month: 'short', year: 'numeric', timeZone: 'UTC' })
  while (d.getTime() / DAY <= to) {
    const label = scale === 'months' ? fmt.format(d) : `T${Math.floor(d.getUTCMonth() / 3) + 1} ${d.getUTCFullYear()}`
    out.push({ day: Math.round(d.getTime() / DAY), label })
    d.setUTCMonth(d.getUTCMonth() + step)
  }
  return out
}

export type DragMode = 'move' | 'start' | 'end'

/**
 * The dates after dragging a bar by some days (RF-75, RF-76); null when the
 * end would fall before the start (RF-77).
 */
export function dragged(start: number, end: number, mode: DragMode, delta: number): { start: number; end: number } | null {
  const next = mode === 'move' ? { start: start + delta, end: end + delta } : mode === 'start' ? { start: start + delta, end } : { start, end: end + delta }
  return next.end < next.start ? null : next
}
