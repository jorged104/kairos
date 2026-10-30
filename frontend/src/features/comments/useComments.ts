import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { listComments, type CommentList, type CommentResult } from '@/api/comments'

/**
 * The comments of a work item, shared by the list and the page's
 * highlights. Every change answers with the whole list, which replaces the
 * cached one (RF-43).
 */
export function useComments(workItemId: () => string) {
  const qc = useQueryClient()
  const key = computed(() => ['comments', workItemId()])
  const { data, error } = useQuery({
    queryKey: key,
    queryFn: () => listComments(workItemId()),
    enabled: computed(() => !!workItemId()),
  })
  function apply(r: CommentResult) {
    qc.setQueryData<CommentList>(key.value, r.comments)
    return r
  }
  // Resolved threads don't highlight their text (RF-49).
  const openAnchors = computed(() =>
    (data.value?.threads ?? []).filter((th) => th.anchored && !th.resolved_at && !th.anchor_deleted).map((th) => th.id),
  )
  return { list: data, error, apply, openAnchors }
}

/** Takes the reader to the commented text in the page (RF-48). */
export function showAnchor(threadId: string) {
  const el = document.querySelector<HTMLElement>(`.page-comment[data-comment="${CSS.escape(threadId)}"]`)
  if (!el) return false
  el.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
  el.classList.add('page-comment-flash')
  setTimeout(() => el.classList.remove('page-comment-flash'), 1500)
  return true
}
