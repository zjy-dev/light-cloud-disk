<script setup lang="ts">
import { ref } from 'vue'
import { useFileStore } from '@/stores/file'
import { useUpload } from '@/composables/useUpload'
import BaseModal from '@/components/ui/BaseModal.vue'
import {
  Upload,
  FolderPlus,
  Trash2,
  Download,
  Share2,
  Grid3x3,
  List,
  ArrowUpDown,
} from 'lucide-vue-next'

const fileStore = useFileStore()
const { uploadFile } = useUpload()

const emit = defineEmits<{
  share: [fileId: number]
}>()

const showNewFolder = ref(false)
const newFolderName = ref('')
const fileInput = ref<HTMLInputElement>()

function onUploadClick() {
  fileInput.value?.click()
}

function onFilesSelected(e: Event) {
  const target = e.target as HTMLInputElement
  const files = target.files
  if (!files) return

  for (const file of files) {
    uploadFile(file, fileStore.currentParentId, () => fileStore.fetchFiles())
  }

  target.value = ''
}

function createFolder() {
  if (!newFolderName.value.trim()) return
  fileStore.createFolder(newFolderName.value.trim())
  newFolderName.value = ''
  showNewFolder.value = false
}

function onShareClick() {
  const selected = fileStore.selectedFiles
  if (selected.length === 1 && selected[0] && !selected[0].isFolder) {
    emit('share', selected[0].id)
  }
}

async function downloadSelected() {
  for (const file of fileStore.selectedFiles) {
    if (!file.isFolder) {
      await fileStore.downloadFile(file.id)
    }
  }
}

function toggleSort() {
  if (fileStore.sortField === 'name') {
    fileStore.sortField = 'updatedAt'
  } else if (fileStore.sortField === 'updatedAt') {
    fileStore.sortField = 'size'
  } else {
    fileStore.sortField = 'name'
  }
}
</script>

<template>
  <div class="flex items-center gap-2 flex-wrap">
    <!-- Primary actions -->
    <button
      class="flex items-center gap-2 h-9 px-4 rounded-xl bg-accent text-accent-text
             text-sm font-medium hover:bg-accent-hover transition-colors"
      @click="onUploadClick"
    >
      <Upload class="w-4 h-4" />
      Upload
    </button>
    <input
      ref="fileInput"
      type="file"
      multiple
      class="hidden"
      @change="onFilesSelected"
    />

    <button
      class="flex items-center gap-2 h-9 px-3 rounded-xl border border-border
             text-text-secondary text-sm hover:bg-surface-hover
             hover:text-text-primary transition-all"
      @click="showNewFolder = true"
    >
      <FolderPlus class="w-4 h-4" />
      New Folder
    </button>

    <!-- Selection actions -->
    <template v-if="fileStore.selectedIds.size > 0">
      <div class="w-px h-5 bg-border mx-1" />

      <span class="text-xs text-text-tertiary">
        {{ fileStore.selectedIds.size }} selected
      </span>

      <button
        class="flex items-center gap-1.5 h-8 px-2.5 rounded-lg
               text-text-secondary text-xs hover:bg-surface-hover transition-all"
        @click="downloadSelected"
      >
        <Download class="w-3.5 h-3.5" />
        Download
      </button>

      <button
        v-if="fileStore.selectedFiles.length === 1 && fileStore.selectedFiles[0] && !fileStore.selectedFiles[0].isFolder"
        class="flex items-center gap-1.5 h-8 px-2.5 rounded-lg
               text-text-secondary text-xs hover:bg-surface-hover transition-all"
        @click="onShareClick"
      >
        <Share2 class="w-3.5 h-3.5" />
        Share
      </button>

      <button
        class="flex items-center gap-1.5 h-8 px-2.5 rounded-lg
               text-danger text-xs hover:bg-danger-soft transition-all"
        @click="fileStore.deleteSelected()"
      >
        <Trash2 class="w-3.5 h-3.5" />
        Delete
      </button>
    </template>

    <!-- Spacer -->
    <div class="flex-1" />

    <!-- View controls -->
    <button
      class="flex items-center gap-1.5 h-8 px-2 rounded-lg
             text-text-tertiary text-xs hover:text-text-secondary hover:bg-surface-hover
             transition-all"
      @click="toggleSort"
    >
      <ArrowUpDown class="w-3.5 h-3.5" />
      {{ fileStore.sortField === 'name' ? 'Name' : fileStore.sortField === 'size' ? 'Size' : 'Date' }}
    </button>

    <div class="flex items-center bg-surface-hover rounded-lg p-0.5">
      <button
        class="p-1.5 rounded-md transition-all"
        :class="fileStore.viewMode === 'grid' ? 'bg-surface-elevated shadow-card text-text-primary' : 'text-text-tertiary hover:text-text-secondary'"
        @click="fileStore.viewMode = 'grid'"
      >
        <Grid3x3 class="w-4 h-4" />
      </button>
      <button
        class="p-1.5 rounded-md transition-all"
        :class="fileStore.viewMode === 'list' ? 'bg-surface-elevated shadow-card text-text-primary' : 'text-text-tertiary hover:text-text-secondary'"
        @click="fileStore.viewMode = 'list'"
      >
        <List class="w-4 h-4" />
      </button>
    </div>

    <!-- New Folder Modal -->
    <BaseModal :open="showNewFolder" title="New Folder" size="sm" @close="showNewFolder = false">
      <form @submit.prevent="createFolder" class="space-y-4">
        <input
          v-model="newFolderName"
          type="text"
          autofocus
          class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                 text-text-primary text-sm placeholder:text-text-tertiary
                 focus:border-accent focus:ring-2 focus:ring-accent-soft
                 outline-none transition-all"
          placeholder="Folder name"
        />
        <div class="flex justify-end gap-2">
          <button
            type="button"
            class="h-9 px-4 rounded-xl border border-border text-text-secondary text-sm
                   hover:bg-surface-hover transition-colors"
            @click="showNewFolder = false"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="h-9 px-4 rounded-xl bg-accent text-accent-text text-sm font-medium
                   hover:bg-accent-hover transition-colors"
          >
            Create
          </button>
        </div>
      </form>
    </BaseModal>
  </div>
</template>
