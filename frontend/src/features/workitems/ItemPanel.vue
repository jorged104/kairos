<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import MeetingSheet from '@/features/meetings/MeetingSheet.vue'
import WorkItemSheet from './WorkItemSheet.vue'

// The work item open in the side panel, kept in the URL (?item=KAI-12) so it
// survives a reload and the back button closes it (spec 005, RF-37). It takes
// half the screen; its left edge drags it wider or narrower (RF-65 to RF-67).
// The table, the kanban and the timeline share it (spec 008, RF-5).
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const openItem = computed(() => (typeof route.query.item === 'string' ? route.query.item : null))
// A meeting opens in the same panel (spec 018, RF-41).
const openMeeting = computed(() => (!openItem.value && typeof route.query.meeting === 'string' ? route.query.meeting : null))
function closePanel() {
  const { item: _item, meeting: _meeting, ...query } = route.query
  void router.replace({ query })
}
// Only while it is open: null is half again (double click on the edge).
const panelWidth = ref<number | null>(null)
const MIN_PANEL_PX = 360
const clampWidth = (px: number) => Math.round(Math.min(Math.max(px, MIN_PANEL_PX), window.innerWidth - 120))
function startResize(e: PointerEvent) {
  e.preventDefault()
  const move = (ev: PointerEvent) => (panelWidth.value = clampWidth(window.innerWidth - ev.clientX))
  const stop = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stop)
    document.body.style.removeProperty('user-select')
  }
  document.body.style.userSelect = 'none' // no text selected while dragging
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop)
}
function resizeWithKeys(e: KeyboardEvent) {
  const step = e.key === 'ArrowLeft' ? 32 : e.key === 'ArrowRight' ? -32 : 0
  if (!step) return
  e.preventDefault()
  panelWidth.value = clampWidth((panelWidth.value ?? window.innerWidth / 2) + step)
}
</script>

<template>
  <Transition name="panel">
    <aside
      v-if="openItem || openMeeting"
      class="fixed inset-y-0 right-0 z-30 w-full overflow-y-auto border-l bg-background p-4 shadow-2xl sm:w-(--panel-w) sm:p-6"
      :style="{ '--panel-w': panelWidth === null ? '50vw' : `${panelWidth}px` }"
      :aria-label="openItem ?? t('meetings.title')"
      data-testid="item-panel"
    >
      <div
        role="separator"
        aria-orientation="vertical"
        tabindex="0"
        :aria-label="t('page.resizePanel')"
        class="group fixed inset-y-0 z-40 hidden w-3 -translate-x-1/2 cursor-col-resize touch-none outline-none sm:block"
        :style="{ right: 'var(--panel-w)' }"
        data-testid="panel-resize"
        @pointerdown="startResize"
        @keydown="resizeWithKeys"
        @dblclick="panelWidth = null"
      >
        <span class="mx-auto block h-full w-0.5 bg-transparent transition-colors group-hover:bg-primary/40 group-focus-visible:bg-primary" />
      </div>
      <Transition name="sheet" mode="out-in">
        <WorkItemSheet v-if="openItem" :key="openItem" :wikey="openItem" panel @close="closePanel" />
        <MeetingSheet v-else-if="openMeeting" :key="openMeeting" :id="openMeeting" panel @close="closePanel" />
      </Transition>
    </aside>
  </Transition>
</template>
