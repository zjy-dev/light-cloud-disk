<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useFileStore } from '@/stores/file'
import type { FileInfo } from '@/types'
import FileBreadcrumb from '@/components/file/FileBreadcrumb.vue'
import FileToolbar from '@/components/file/FileToolbar.vue'
import FileItem from '@/components/file/FileItem.vue'
import UploadProgress from '@/components/file/UploadProgress.vue'
import ShareDialog from '@/components/file/ShareDialog.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import { FolderOpen } from 'lucide-vue-next'

const fileStore = useFileStore()

const shareFileId = ref<number | null>(null)
const showShare = ref(false)
const showRename = ref(false)
const renameTarget = ref<FileInfo | null>(null)
const newName = ref('')

onMounted(() => {
  fileStore.fetchFiles(0)
})

function onShare(fileId: number) {
  shareFileId.value = fileId
  showShare.value = true
}

function onRename(file: FileInfo) {
  renameTarget.value = file
  newName.value = file.name
  showRename.value = true
}

async function confirmRename() {
  if (!renameTarget.value || !newName.value.trim()) return
  await fileStore.renameFile(renameTarget.value.id, newName.value.trim())
  showRename.value = false
}

function onDownload(file: FileInfo) {
  fileStore.downloadFile(file.id)
}

function onDelete(file: FileInfo) {
  fileStore.selectedIds = new Set([file.id])
  fileStore.deleteSelected()
}

function onShareFile(file: FileInfo) {
  shareFileId.value = file.id
  showShare.value = true
}
</script>

<template>
  <div class="space-y-5">
    <FileBreadcrumb />
    <FileToolbar @share="onShare" />

    <!-- Loading -->
    <div v-if="fileStore.loading" class="flex justify-center py-20">
      <div class="w-8 h-8 border-2 border-accent/20 border-t-accent rounded-full animate-spin" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="fileStore.sortedFiles.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <div class="w-16 h-16 rounded-2xl bg-surface-hover flex items-center justify-center mb-4">
        <FolderOpen class="w-8 h-8 text-text-tertiary" />
      </div>
      <h3 class="font-display font-medium text-text-primary mb-1">
        {{ fileStore.searchKeyword ? 'No results found' : 'No files yet' }}
      </h3>
      <p class="text-sm text-text-tertiary max-w-xs">
        {{
          fileStore.searchKeyword
            ? `No files matching "${fileStore.searchKeyword}"`
            : 'Upload files or create a folder to get started'
        }}
      </p>
    </div>

    <!-- Grid view -->
    <div
      v-else-if="fileStore.viewMode === 'grid'"
      class="grid gap-2"
      style="grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));"
    >
      <FileItem
        v-for="file in fileStore.sortedFiles"
        :key="file.id"
        :file="file"
        view-mode="grid"
        @select="fileStore.toggleSelect"
        @rename="onRename"
        @download="onDownload"
        @share="onShareFile"
        @delete="onDelete"
      />
    </div>

    <!-- List view -->
    <div v-else class="space-y-0.5">
      <!-- List header -->
      <div class="flex items-center gap-3 px-3 h-8 text-xs text-text-tertiary font-medium">
        <div class="w-5" />
        <div class="w-5" />
        <div class="flex-1">Name</div>
        <div class="w-20 text-right">Size</div>
        <div class="w-28 text-right">Modified</div>
        <div class="w-7" />
      </div>
      <FileItem
        v-for="file in fileStore.sortedFiles"
        :key="file.id"
        :file="file"
        view-mode="list"
        @select="fileStore.toggleSelect"
        @rename="onRename"
        @download="onDownload"
        @share="onShareFile"
        @delete="onDelete"
      />
    </div>

    <!-- Upload progress panel -->
    <UploadProgress />

    <!-- Share dialog -->
    <ShareDialog
      :open="showShare"
      :file-id="shareFileId"
      @close="showShare = false"
    />

    <!-- Rename dialog -->
    <BaseModal :open="showRename" title="Rename" size="sm" @close="showRename = false">
      <form @submit.prevent="confirmRename" class="space-y-4">
        <input
          v-model="newName"
          type="text"
          autofocus
          class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                 text-text-primary text-sm placeholder:text-text-tertiary
                 focus:border-accent focus:ring-2 focus:ring-accent-soft
                 outline-none transition-all"
        />
        <div class="flex justify-end gap-2">
          <button
            type="button"
            class="h-9 px-4 rounded-xl border border-border text-text-secondary text-sm
                   hover:bg-surface-hover transition-colors"
            @click="showRename = false"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="h-9 px-4 rounded-xl bg-accent text-accent-text text-sm font-medium
                   hover:bg-accent-hover transition-colors"
          >
            Rename
          </button>
        </div>
      </form>
    </BaseModal>
  </div>
</template>
