// A meeting's date and time is an instant; the person sees and writes it in
// their own time zone (spec 018, RF-7).

const pad = (n: number) => String(n).padStart(2, '0')

/** The value of a datetime-local input for an instant. */
export function toLocalInput(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** The instant, with its zone, of a datetime-local value. */
export function fromLocalInput(local: string): string {
  return new Date(local).toISOString()
}
