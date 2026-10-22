<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createAttribute, listAttributes, setAttributeRetired, updateAttribute, type AttributeInput } from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Attribute, AttributeType } from '@/types/api'

const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const TYPES: AttributeType[] = ['text', 'number', 'date', 'boolean', 'select']

const { data: attributes } = useQuery({ queryKey: ['attributes'], queryFn: listAttributes })

// Options are edited as one value per line. Options removed from the list
// are kept retired by the API for whoever already has them (RF-107).
interface AttrForm {
  name: string
  type: AttributeType
  required: boolean
  options: string
}
const blank = (): AttrForm => ({ name: '', type: 'text', required: false, options: '' })
const toInput = (f: AttrForm): AttributeInput => ({
  name: f.name,
  type: f.type,
  required: f.required,
  options: f.type === 'select'
    ? [...new Set(f.options.split('\n').map((o) => o.trim()).filter(Boolean))].map((value) => ({ value, retired: false }))
    : [],
})

const error = ref<unknown>(null)
function mutation<A>(fn: (arg: A) => Promise<unknown>) {
  return useMutation({
    mutationFn: fn,
    onMutate: () => (error.value = null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['attributes'] }),
    onError: (e) => (error.value = e),
  })
}

const form = ref<AttrForm>(blank())
const create = mutation(() => createAttribute(toInput(form.value)))
async function submitNew() {
  await create.mutateAsync(undefined).catch(() => undefined)
  if (!error.value) form.value = blank()
}

const editing = ref<string | null>(null)
const edit = ref<AttrForm>(blank())
function startEdit(a: Attribute) {
  editing.value = a.id
  edit.value = {
    name: a.name,
    type: a.type,
    required: a.required,
    options: a.options.filter((o) => !o.retired).map((o) => o.value).join('\n'),
  }
}
const update = mutation(() => updateAttribute(editing.value as string, toInput(edit.value)))
async function submitEdit() {
  await update.mutateAsync(undefined).catch(() => undefined)
  if (!error.value) editing.value = null
}
const retire = mutation((a: Attribute) => setAttributeRetired(a.id, !a.retired_at))
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="font-heading text-xl font-semibold">{{ t('attributes.title') }}</h1>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <Card>
      <CardHeader>
        <CardTitle>{{ t('attributes.new') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <form class="grid gap-3 sm:grid-cols-4" @submit.prevent="submitNew">
          <div class="flex flex-col gap-1.5">
            <Label for="attr-name">{{ t('attributes.name') }}</Label>
            <Input id="attr-name" v-model="form.name" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="attr-type">{{ t('attributes.type') }}</Label>
            <NativeSelect id="attr-type" v-model="form.type">
              <option v-for="ty in TYPES" :key="ty" :value="ty">{{ t(`attributes.types.${ty}`) }}</option>
            </NativeSelect>
          </div>
          <label class="flex items-end gap-2 pb-1.5 text-sm">
            <input v-model="form.required" type="checkbox" />
            {{ t('attributes.required') }}
          </label>
          <div class="flex items-end">
            <Button type="submit" :disabled="create.isPending.value">{{ t('common.create') }}</Button>
          </div>
          <div v-if="form.type === 'select'" class="flex flex-col gap-1.5 sm:col-span-4">
            <Label for="attr-options">{{ t('attributes.options') }}</Label>
            <textarea id="attr-options" v-model="form.options" rows="3" class="rounded-lg border border-input bg-transparent p-2 text-sm" />
            <p class="text-xs text-muted-foreground">{{ t('attributes.optionsHint') }}</p>
          </div>
        </form>
      </CardContent>
    </Card>

    <ul class="flex flex-col gap-2">
      <li v-for="a in attributes ?? []" :key="a.id" class="rounded-lg border bg-background p-3" data-testid="attribute-row">
        <form v-if="editing === a.id" class="grid gap-2 sm:grid-cols-4" @submit.prevent="submitEdit">
          <Input v-model="edit.name" :aria-label="t('attributes.name')" required />
          <NativeSelect v-model="edit.type" :aria-label="t('attributes.type')">
            <option v-for="ty in TYPES" :key="ty" :value="ty">{{ t(`attributes.types.${ty}`) }}</option>
          </NativeSelect>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="edit.required" type="checkbox" />
            {{ t('attributes.required') }}
          </label>
          <div class="flex gap-2">
            <Button type="submit" size="sm">{{ t('common.save') }}</Button>
            <Button type="button" size="sm" variant="ghost" @click="editing = null">{{ t('common.cancel') }}</Button>
          </div>
          <div v-if="edit.type === 'select'" class="flex flex-col gap-1.5 sm:col-span-4">
            <textarea v-model="edit.options" rows="3" :aria-label="t('attributes.options')" class="rounded-lg border border-input bg-transparent p-2 text-sm" />
            <p class="text-xs text-muted-foreground">{{ t('attributes.optionsHint') }}</p>
          </div>
        </form>
        <div v-else class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ a.name }}</span>
          <Badge variant="outline">{{ t(`attributes.types.${a.type}`) }}</Badge>
          <Badge v-if="a.required" variant="outline">{{ t('attributes.required') }}</Badge>
          <Badge v-if="a.retired_at" variant="secondary">{{ t('attributes.retired') }}</Badge>
          <span v-if="a.type === 'select'" class="text-xs text-muted-foreground">
            {{ a.options.map((o) => (o.retired ? t('attributes.retiredOption', { value: o.value }) : o.value)).join(', ') }}
          </span>
          <div class="ml-auto flex gap-1">
            <Button v-if="!a.retired_at" size="sm" variant="ghost" @click="startEdit(a)">{{ t('common.edit') }}</Button>
            <Button size="sm" variant="ghost" @click="retire.mutate(a)">
              {{ a.retired_at ? t('attributes.restore') : t('attributes.retire') }}
            </Button>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>
