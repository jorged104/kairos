<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import NativeSelect from '@/components/NativeSelect.vue'
import { Input } from '@/components/ui/input'
import type { Attribute, AttributeValue } from '@/types/api'

const props = defineProps<{ attribute: Attribute; id: string; disabled?: boolean }>()
const model = defineModel<AttributeValue>({ default: null })
const { t } = useI18n()

// Inputs work with strings; the API gets numbers and booleans typed.
const text = computed({
  get: () => (model.value === null || model.value === undefined ? '' : String(model.value)),
  set: (v: string) => {
    if (v === '') model.value = null
    else if (props.attribute.type === 'number') model.value = Number(v)
    else if (props.attribute.type === 'boolean') model.value = v === 'true'
    else model.value = v
  },
})

// A retired option stays visible for whoever already has it (RF-107).
const options = computed(() =>
  props.attribute.options.filter((o) => !o.retired || o.value === model.value),
)
</script>

<template>
  <NativeSelect v-if="attribute.type === 'select'" :id="id" v-model="text" :disabled="disabled">
    <option value="">{{ t('attributes.empty') }}</option>
    <option v-for="o in options" :key="o.value" :value="o.value">
      {{ o.retired ? t('attributes.retiredOption', { value: o.value }) : o.value }}
    </option>
  </NativeSelect>
  <NativeSelect v-else-if="attribute.type === 'boolean'" :id="id" v-model="text" :disabled="disabled">
    <option value="">{{ t('attributes.empty') }}</option>
    <option value="true">{{ t('common.yes') }}</option>
    <option value="false">{{ t('common.no') }}</option>
  </NativeSelect>
  <Input
    v-else
    :id="id"
    v-model="text"
    :type="attribute.type === 'number' ? 'number' : attribute.type === 'date' ? 'date' : 'text'"
    :disabled="disabled"
    :required="attribute.required"
  />
</template>
