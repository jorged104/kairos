<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import type { SavedPage } from '@/api/pages'
import { getProject } from '@/api/work'
import PageSection from '@/features/pages/PageSection.vue'
import type { Project } from '@/types/work'

// The project's page, saved as it is written (spec 005, RF-43), in its own
// tab (spec 008, RF-1).
const props = defineProps<{ project: Project }>()
const qc = useQueryClient()

const writable = computed(() => props.project.can.manage && props.project.lifecycle === 'activo')

// An autosave of the page updates the cached project, versions included.
function onPageSaved(saved: SavedPage) {
  qc.setQueryData<Project>(['project', props.project.id], (old) =>
    old && { ...old, description: saved.page, page_version: saved.page_version, version: saved.version, refs: saved.refs },
  )
}
async function reloadPage() {
  const fresh = await getProject(props.project.id)
  qc.setQueryData(['project', props.project.id], fresh)
  return fresh
}
</script>

<template>
  <section class="rounded-lg border bg-background p-4">
    <PageSection
      :key="project.id"
      :owner="{ kind: 'projects', id: project.id }"
      :doc="project.description"
      :page-version="project.page_version"
      :refs="project.refs"
      :editable="writable"
      :reload="reloadPage"
      @saved="onPageSaved"
    />
  </section>
</template>
