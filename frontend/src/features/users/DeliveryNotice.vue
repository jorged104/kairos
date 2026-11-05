<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Delivery } from '@/api/accounts'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// What happened to an invitation or a link: sent by mail, or the link to pass
// on when the mail is off or failed (spec 019, RF-13, RF-21, RF-33).
defineProps<{ delivery: Delivery; what: 'invitation' | 'link' }>()
const { t } = useI18n()
const copied = ref(false)
async function copy(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    copied.value = true
  } catch {
    // Without clipboard access the link stays selectable in the field.
  }
}
</script>

<template>
  <Alert v-if="delivery.sent" role="status" data-testid="delivery-sent">
    <AlertDescription>{{ t(`accounts.sent.${what}`) }}</AlertDescription>
  </Alert>
  <div v-else-if="delivery.url" class="flex flex-col gap-2 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-100" role="status" data-testid="delivery-link">
    <p>{{ delivery.kind === 'login' ? t('accounts.notSent.login') : t(`accounts.notSent.${what}`) }}</p>
    <div class="flex gap-2">
      <Input :model-value="delivery.url" readonly class="bg-background font-mono text-xs" data-testid="delivery-url" @focus="($event.target as HTMLInputElement).select()" />
      <Button type="button" size="sm" variant="outline" data-testid="delivery-copy" @click="copy(delivery.url!)">
        <Check v-if="copied" aria-hidden="true" />
        <Copy v-else aria-hidden="true" />
        {{ copied ? t('accounts.copied') : t('accounts.copy') }}
      </Button>
    </div>
  </div>
</template>
