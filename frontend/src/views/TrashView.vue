<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { fileApi } from '@/api/file'
import type { TrashFileInfo } from '@/types'
import { Trash2, RotateCcw, AlertTriangle, Clock } from 'lucide-vue-next'

const files = ref<TrashFileInfo[]>([])
const loading = ref(false)
const selectedIds = ref<Set<number>>(new Set())

const selectedCount = computed(() => selectedIds.value.size)

onMounted(loadTrash)

async function loadTrash() {
  loading.value = true
  try {
    const { data } = await fileApi.listTrash()
    files.value = data.files ?? []
  } finally {
    loading.value = false
  }
}

function toggleSelect(id: number) {
  if (selectedIds.value.has(id)) {
    selectedIds.value.delete(id)
  } else {
    selectedIds.value.add(id)
  }
  selectedIds.value = new Set(selectedIds.value)
}

function selectAll() {
  if (selectedIds.value.size === files.value.length) {
    selectedIds.value = new Set()
  } else {
    selectedIds.value = new Set(files.value.map((f) => f.id))
  }
}

async function restoreSelected() {
  const ids = Array.from(selectedIds.value)
  if (ids.length === 0) return
  await fileApi.restoreFiles(ids)
  selectedIds.value = new Set()
  await loadTrash()
}

async function deleteSelected() {
  const ids = Array.from(selectedIds.value)
  if (ids.length === 0) return
  if (!confirm('Permanently delete these files? This cannot be undone.')) return
  await fileApi.permanentDelete(ids)
  selectedIds.value = new Set()
  await loadTrash()
}

async function emptyTrash() {
  if (!confirm('Empty the trash? All files will be permanently deleted.')) return
  const ids = files.value.map((f) => f.id)
  await fileApi.permanentDelete(ids)
  files.value = []
  selectedIds.value = new Set()
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '--'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '--'
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function daysUntilExpire(expireAt: number): string {
  if (!expireAt) return 'Never'
  const days = Math.ceil((expireAt * 1000 - Date.now()) / (1000 * 60 * 60 * 24))
  if (days <= 0) return 'Expiring'
  return `${days}d left`
}
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="font-display font-semibold text-xl text-text-primary">Trash</h1>
        <p class="text-sm text-text-tertiary mt-0.5">
          Files will be permanently deleted after 30 days
        </p>
      </div>
      <div class="flex items-center gap-2">
        <template v-if="selectedCount > 0">
          <span class="text-xs text-text-tertiary">{{ selectedCount }} selected</span>
          <button
            class="flex items-center gap-1.5 h-8 px-3 rounded-lg text-sm
                   text-text-secondary hover:bg-surface-hover transition-all"
            @click="restoreSelected"
          >
            <RotateCcw class="w-3.5 h-3.5" />
            Restore
          </button>
          <button
            class="flex items-center gap-1.5 h-8 px-3 rounded-lg text-sm
                   text-danger hover:bg-danger-soft transition-all"
            @click="deleteSelected"
          >
            <Trash2 class="w-3.5 h-3.5" />
            Delete forever
          </button>
        </template>
        <button
          v-if="files.length > 0"
          class="flex items-center gap-1.5 h-8 px-3 rounded-lg text-sm
                 text-danger hover:bg-danger-soft transition-all"
          @click="emptyTrash"
        >
          <AlertTriangle class="w-3.5 h-3.5" />
          Empty trash
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-20">
      <div class="w-8 h-8 border-2 border-accent/20 border-t-accent rounded-full animate-spin" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="files.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <div class="w-16 h-16 rounded-2xl bg-surface-hover flex items-center justify-center mb-4">
        <Trash2 class="w-8 h-8 text-text-tertiary" />
      </div>
      <h3 class="font-display font-medium text-text-primary mb-1">Trash is empty</h3>
      <p class="text-sm text-text-tertiary">Deleted files will appear here</p>
    </div>

    <!-- File list -->
    <div v-else class="space-y-0.5">
      <div class="flex items-center gap-3 px-3 h-8 text-xs text-text-tertiary font-medium">
        <button
          class="w-5 h-5 rounded-md border flex items-center justify-center transition-all"
          :class="selectedIds.size === files.length ? 'border-accent bg-accent' : 'border-border'"
          @click="selectAll"
        >
          <svg
            v-if="selectedIds.size === files.length"
            class="w-3 h-3 text-accent-text"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="2,6 5,9 10,3" />
          </svg>
        </button>
        <div class="flex-1">Name</div>
        <div class="w-20 text-right">Size</div>
        <div class="w-28 text-right">Deleted</div>
        <div class="w-20 text-right">Expires</div>
      </div>

      <div
        v-for="file in files"
        :key="file.id"
        class="group flex items-center gap-3 px-3 h-11 rounded-lg cursor-pointer
               transition-all duration-150"
        :class="selectedIds.has(file.id) ? 'bg-accent-soft' : 'hover:bg-surface-hover'"
        @click="toggleSelect(file.id)"
      >
        <div
          class="w-5 h-5 rounded-md border flex items-center justify-center shrink-0 transition-all"
          :class="selectedIds.has(file.id) ? 'border-accent bg-accent' : 'border-border opacity-0 group-hover:opacity-100'"
        >
          <svg
            v-if="selectedIds.has(file.id)"
            class="w-3 h-3 text-accent-text"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="2,6 5,9 10,3" />
          </svg>
        </div>

        <span class="flex-1 text-sm text-text-primary truncate">
          {{ file.name }}
        </span>

        <span class="text-xs text-text-tertiary w-20 text-right shrink-0">
          {{ file.isFolder ? '--' : formatSize(file.size) }}
        </span>

        <span class="text-xs text-text-tertiary w-28 text-right shrink-0">
          {{ formatDate(file.deletedAt) }}
        </span>

        <span class="flex items-center gap-1 text-xs text-text-tertiary w-20 justify-end shrink-0">
          <Clock class="w-3 h-3" />
          {{ daysUntilExpire(file.expireAt) }}
        </span>
      </div>
    </div>
  </div>
</template>
