import type { JSONContent } from '@tiptap/vue-3'
import { onBeforeUnmount, ref, shallowRef } from 'vue'
import { ApiError } from '@/api/client'
import { savePage, type PageOwner, type SavedPage } from '@/api/pages'
import { pageForSave } from '@/features/richtext/pageExtensions'
import type { MentionRefs, RichDoc } from '@/types/work'

/** Time without typing before the page saves itself (RF-31). */
export const AUTOSAVE_DELAY_MS = 2000
/** Limit of the page's text, the description's of spec 004 (RF-36). */
export const MAX_PAGE_CHARS = 50_000
const MAX_RETRY_MS = 30_000

/**
 * saved: nothing pending · dirty: changes waiting for the delay · saving ·
 * conflict: someone else changed the page, autosave stops (RF-34) ·
 * error: the last save failed (retried when it was the network, RF-35) ·
 * too_long: over the limit, nothing is sent (RF-36).
 */
export type SaveStatus = 'saved' | 'dirty' | 'saving' | 'conflict' | 'error' | 'too_long'

/** Characters of text in a page, as the API counts its plain text. */
export function textLength(doc: JSONContent | null | undefined): number {
  if (!doc) return 0
  return (doc.text?.length ?? 0) + (doc.content ?? []).reduce((n, c) => n + textLength(c), 0)
}

/** Plain text of a page, for "copy my text" after a conflict (RF-34). */
export function plainText(doc: JSONContent | null | undefined): string {
  if (!doc) return ''
  if (doc.type === 'text') return doc.text ?? ''
  const inner = (doc.content ?? []).map(plainText).join(doc.type === 'doc' ? '\n' : '')
  return ['paragraph', 'heading', 'codeBlock', 'detailsSummary'].includes(doc.type ?? '') ? inner + '\n' : inner
}

export interface AutosaveOptions {
  /** Called after each save, to update the owner wherever it is cached. */
  onSaved?: (saved: SavedPage) => void
}

/** The autosave of a page (RF-31 to RF-36). Only the page version is compared (RF-59). */
export function usePageAutosave(owner: () => PageOwner, initialVersion: number, initialRefs: MentionRefs = {}, options: AutosaveOptions = {}) {
  const status = ref<SaveStatus>('saved')
  const errorCode = ref<string | null>(null)
  const pageVersion = ref(initialVersion)
  const refs = shallowRef<MentionRefs>(initialRefs)
  let pending: RichDoc | undefined // the page to send, undefined when nothing is pending
  let lastDoc: JSONContent | null = null
  let timer: ReturnType<typeof setTimeout> | null = null
  let retryMs = AUTOSAVE_DELAY_MS
  let inFlight: Promise<void> | null = null

  const clear = () => {
    if (timer) clearTimeout(timer)
    timer = null
  }
  const schedule = (ms: number) => {
    clear()
    timer = setTimeout(() => void save(), ms)
  }

  /** The editor changed: save once the user stops typing for the delay. */
  function change(doc: JSONContent) {
    if (status.value === 'conflict') return
    lastDoc = doc
    if (textLength(doc) > MAX_PAGE_CHARS) {
      clear()
      pending = undefined
      status.value = 'too_long'
      return
    }
    pending = pageForSave(doc)
    status.value = 'dirty'
    retryMs = AUTOSAVE_DELAY_MS
    schedule(AUTOSAVE_DELAY_MS)
  }

  async function send(keepalive: boolean) {
    if (pending === undefined) return
    const page = pending
    pending = undefined
    status.value = 'saving'
    try {
      const saved = await savePage(owner(), page, pageVersion.value, { keepalive })
      pageVersion.value = saved.page_version
      refs.value = saved.refs
      errorCode.value = null
      options.onSaved?.(saved)
      if (pending !== undefined) schedule(AUTOSAVE_DELAY_MS) // typed while saving
      else status.value = 'saved'
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'unknown'
      errorCode.value = code
      if (code === 'page_conflict') {
        status.value = 'conflict'
        return
      }
      pending ??= page // keep it on screen and in line (RF-35)
      status.value = 'error'
      const retryable = code === 'network_error' || (e instanceof ApiError && e.status >= 500)
      if (retryable) {
        schedule(retryMs)
        retryMs = Math.min(retryMs * 2, MAX_RETRY_MS)
      }
    }
  }

  function save(keepalive = false): Promise<void> {
    clear()
    if (inFlight) return inFlight.then(() => save(keepalive))
    inFlight = send(keepalive).finally(() => (inFlight = null))
    return inFlight
  }

  /** Saves what is pending now, e.g. before closing the panel or leaving (RF-32). */
  async function flush() {
    if (status.value === 'conflict' || status.value === 'too_long') return
    if (pending !== undefined || inFlight) await save()
  }

  /** After a conflict: take the version someone else saved and resume. */
  function adopt(version: number, newRefs: MentionRefs = {}) {
    clear()
    pending = undefined
    lastDoc = null
    pageVersion.value = version
    refs.value = newRefs
    errorCode.value = null
    status.value = 'saved'
  }

  const hasChanges = () => pending !== undefined || status.value === 'saving' || status.value === 'conflict' || status.value === 'too_long'

  // Closing the tab: send with keepalive and ask before leaving (RF-32, ING-8).
  const onPageHide = () => {
    if (pending !== undefined && status.value !== 'conflict') void save(true)
  }
  const onBeforeUnload = (e: BeforeUnloadEvent) => {
    if (hasChanges()) e.preventDefault()
  }
  window.addEventListener('pagehide', onPageHide)
  window.addEventListener('beforeunload', onBeforeUnload)
  onBeforeUnmount(() => {
    window.removeEventListener('pagehide', onPageHide)
    window.removeEventListener('beforeunload', onBeforeUnload)
    if (pending !== undefined && status.value !== 'conflict') void save()
    clear()
  })

  return {
    status,
    errorCode,
    pageVersion,
    refs,
    change,
    flush,
    adopt,
    hasChanges,
    /** The text the user wrote, kept for copying after a conflict. */
    myText: () => plainText(lastDoc),
  }
}
