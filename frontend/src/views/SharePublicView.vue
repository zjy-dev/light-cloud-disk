<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { fileApi } from '@/api/file'
import type { FileInfo } from '@/types'
import { useTheme } from '@/composables/useTheme'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'
import {
  Cloud,
  Download,
  FileText,
  Lock,
  AlertCircle,
  Loader2,
} from 'lucide-vue-next'

useTheme()
const route = useRoute()

const file = ref<FileInfo | null>(null)
const downloadUrl = ref('')
const loading = ref(false)
const error = ref('')
const needPassword = ref(false)
const password = ref('')

onMounted(() => loadShare())

async function loadShare(pwd?: string) {
  loading.value = true
  error.value = ''
  try {
    const shareId = route.params.shareId as string
    const { data } = await fileApi.getShare(shareId, pwd ?? password.value)
    file.value = data.file
    downloadUrl.value = data.downloadUrl
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err) {
      const axiosErr = err as { response?: { status?: number; data?: { error?: string } } }
      if (axiosErr.response?.status === 403 || axiosErr.response?.data?.error?.includes('password')) {
        needPassword.value = true
      } else {
        error.value = axiosErr.response?.data?.error ?? 'Share not found or expired'
      }
    } else {
      error.value = 'Failed to load share'
    }
  } finally {
    loading.value = false
  }
}

function submitPassword() {
  needPassword.value = false
  loadShare(password.value)
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface p-4 relative">
    <div class="absolute top-5 right-5">
      <ThemeToggle />
    </div>

    <div class="w-full max-w-[420px]">
      <!-- Logo -->
      <div class="flex flex-col items-center mb-8">
        <div
          class="flex items-center justify-center w-12 h-12 rounded-2xl
                 bg-accent text-accent-text mb-3"
        >
          <Cloud class="w-6 h-6" />
        </div>
        <h1 class="font-display font-semibold text-xl text-text-primary">
          Shared File
        </h1>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex justify-center py-12">
        <Loader2 class="w-8 h-8 text-accent animate-spin" />
      </div>

      <!-- Password form -->
      <div
        v-else-if="needPassword"
        class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-7"
      >
        <div class="flex flex-col items-center mb-5">
          <Lock class="w-8 h-8 text-text-tertiary mb-2" />
          <p class="text-sm text-text-secondary text-center">
            This file is password protected
          </p>
        </div>
        <form @submit.prevent="submitPassword" class="space-y-4">
          <input
            v-model="password"
            type="password"
            autofocus
            class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                   text-text-primary text-sm placeholder:text-text-tertiary
                   focus:border-accent focus:ring-2 focus:ring-accent-soft
                   outline-none transition-all"
            placeholder="Enter password"
          />
          <button
            type="submit"
            class="w-full h-10 rounded-xl bg-accent text-accent-text text-sm font-medium
                   hover:bg-accent-hover transition-colors"
          >
            Access File
          </button>
        </form>
      </div>

      <!-- Error -->
      <div
        v-else-if="error"
        class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-7 text-center"
      >
        <AlertCircle class="w-10 h-10 text-danger mx-auto mb-3" />
        <p class="text-sm text-text-secondary">{{ error }}</p>
      </div>

      <!-- File info -->
      <div
        v-else-if="file"
        class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-7"
      >
        <div class="flex flex-col items-center mb-6">
          <div class="w-14 h-14 rounded-2xl bg-accent-soft flex items-center justify-center mb-3">
            <FileText class="w-7 h-7 text-accent" />
          </div>
          <h2 class="font-display font-medium text-text-primary text-center break-all">
            {{ file.name }}
          </h2>
          <span class="text-sm text-text-tertiary mt-1">
            {{ formatSize(file.size) }}
          </span>
        </div>

        <a
          :href="downloadUrl"
          target="_blank"
          class="flex items-center justify-center gap-2 w-full h-10 rounded-xl
                 bg-accent text-accent-text text-sm font-medium
                 hover:bg-accent-hover transition-colors"
        >
          <Download class="w-4 h-4" />
          Download
        </a>
      </div>
    </div>
  </div>
</template>
