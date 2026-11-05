import { ref } from 'vue'

// The one realtime connection of the tab (/ws, spec 003 RF-47): it brings
// the notifications of spec 017 and reconnects on its own, waiting longer
// each time, from 1 s up to 30 s (RF-36).

export type RealtimeMessage = { type: string; [key: string]: unknown }
type Listener = (m: RealtimeMessage) => void

const listeners = new Set<Listener>()
/** Whether the connection is open; without it the count is polled (RF-38). */
export const realtimeConnected = ref(false)

let socket: WebSocket | null = null
let retry: ReturnType<typeof setTimeout> | null = null
let delay = 1000
let running = false

/** Listens to every message; the returned function stops listening. */
export function onRealtime(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

function emit(m: RealtimeMessage) {
  for (const fn of listeners) fn(m)
}

function open() {
  if (!running) return
  const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`
  const ws = new WebSocket(url)
  socket = ws
  ws.onopen = () => {
    delay = 1000
    realtimeConnected.value = true
    // What happened while it was down: everyone reads again (RF-37).
    emit({ type: 'connected' })
  }
  ws.onmessage = (e) => {
    try {
      const m = JSON.parse(String(e.data)) as RealtimeMessage
      if (m && typeof m.type === 'string') emit(m)
    } catch {
      // Not ours: ignored.
    }
  }
  ws.onclose = () => {
    realtimeConnected.value = false
    if (socket === ws) socket = null
    if (!running) return
    retry = setTimeout(open, delay)
    delay = Math.min(delay * 2, 30_000)
  }
}

/** Opens the connection for the signed-in person. */
export function startRealtime() {
  if (running || typeof WebSocket === 'undefined') return
  running = true
  delay = 1000
  open()
}

/** Closes it, e.g. on sign out. */
export function stopRealtime() {
  running = false
  if (retry) clearTimeout(retry)
  retry = null
  socket?.close()
  socket = null
  realtimeConnected.value = false
}
