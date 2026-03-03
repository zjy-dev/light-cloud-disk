<script setup lang="ts">
import { ref } from 'vue'
import { fileApi } from '@/api/file'
import BaseModal from '@/components/ui/BaseModal.vue'
import { Copy, Check } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  fileId: number | null
}>()

const emit = defineEmits<{
  close: []
}>()

const expireDays = ref(7)
const password = ref('')
const loading = ref(false)
const result = ref<{ shareUrl: string; password: string; expireAt: number } | null>(null)
const copied = ref(false)

async function createShare() {
  if (!props.fileId) return
  loading.value = true
  try {
    const { data } = await fileApi.createShare({
      fileId: props.fileId,
      expireDays: expireDays.value,
      password: password.value,
    })
    result.value = {
      shareUrl: data.shareUrl || `${window.location.origin}/share/${data.shareId}`,
      password: data.password,
      expireAt: data.expireAt,
    }
  } finally {
    loading.value = false
  }
}

async function copyLink() {
  if (!result.value) return
  let text = result.value.shareUrl
  if (result.value.password) {
    text += `\nPassword: ${result.value.password}`
  }
  await navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function onClose() {
  result.value = null
  password.value = ''
  expireDays.value = 7
  emit('close')
}
</script>

<template>
  <BaseModal :open="open" title="Share File" size="sm" @close="onClose">
    <template v-if="!result">
      <form @submit.prevent="createShare" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-text-secondary mb-1.5">
            Expire after
          </label>
          <select
            v-model="expireDays"
            class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                   text-text-primary text-sm
                   focus:border-accent focus:ring-2 focus:ring-accent-soft
                   outline-none transition-all"
          >
            <option :value="1">1 day</option>
            <option :value="7">7 days</option>
            <option :value="30">30 days</option>
            <option :value="0">Never</option>
          </select>
        </div>

        <div>
          <label class="block text-sm font-medium text-text-secondary mb-1.5">
            Password (optional)
          </label>
          <input
            v-model="password"
            type="text"
            class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                   text-text-primary text-sm placeholder:text-text-tertiary
                   focus:border-accent focus:ring-2 focus:ring-accent-soft
                   outline-none transition-all"
            placeholder="Leave empty for no password"
          />
        </div>

        <div class="flex justify-end gap-2 pt-1">
          <button
            type="button"
            class="h-9 px-4 rounded-xl border border-border text-text-secondary text-sm
                   hover:bg-surface-hover transition-colors"
            @click="onClose"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="loading"
            class="h-9 px-4 rounded-xl bg-accent text-accent-text text-sm font-medium
                   hover:bg-accent-hover disabled:opacity-50 transition-colors"
          >
            {{ loading ? 'Creating...' : 'Create Link' }}
          </button>
        </div>
      </form>
    </template>

    <template v-else>
      <div class="space-y-4">
        <div class="p-3 rounded-xl bg-success-soft border border-success/20">
          <p class="text-sm text-success font-medium">Link created successfully!</p>
        </div>

        <div>
          <label class="block text-xs text-text-tertiary mb-1">Share link</label>
          <div
            class="flex items-center gap-2 p-2.5 rounded-xl bg-surface border border-border text-sm"
          >
            <span class="flex-1 text-text-primary truncate">{{ result.shareUrl }}</span>
            <button
              class="shrink-0 p-1.5 rounded-lg hover:bg-surface-hover text-text-secondary
                     transition-colors"
              @click="copyLink"
            >
              <component :is="copied ? Check : Copy" class="w-4 h-4" />
            </button>
          </div>
        </div>

        <div v-if="result.password">
          <label class="block text-xs text-text-tertiary mb-1">Password</label>
          <div class="p-2.5 rounded-xl bg-surface border border-border text-sm text-text-primary font-mono">
            {{ result.password }}
          </div>
        </div>

        <button
          class="w-full h-9 rounded-xl bg-accent text-accent-text text-sm font-medium
                 hover:bg-accent-hover transition-colors"
          @click="onClose"
        >
          Done
        </button>
      </div>
    </template>
  </BaseModal>
</template>
