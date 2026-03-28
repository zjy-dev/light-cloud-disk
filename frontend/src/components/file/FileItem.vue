<script setup lang="ts">
import { ref, computed } from 'vue'
import type { FileInfo } from '@/types'
import { useFileStore } from '@/stores/file'
import {
  FolderClosed,
  FileText,
  FileImage,
  FileVideo,
  FileAudio,
  FileArchive,
  FileCode,
  File,
  MoreHorizontal,
  Pencil,
  Download,
  Share2,
  Trash2,
} from 'lucide-vue-next'

const props = defineProps<{
  file: FileInfo
  viewMode: 'grid' | 'list'
}>()

const emit = defineEmits<{
  open: [file: FileInfo]
  select: [id: number]
  rename: [file: FileInfo]
  download: [file: FileInfo]
  share: [file: FileInfo]
  delete: [file: FileInfo]
}>()

const fileStore = useFileStore()
const showMenu = ref(false)

const isSelected = computed(() => fileStore.selectedIds.has(props.file.id))

const fileIcon = computed(() => {
  if (props.file.isFolder) return FolderClosed
  const ext = props.file.name.split('.').pop()?.toLowerCase() ?? ''
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp'].includes(ext)) return FileImage
  if (['mp4', 'avi', 'mov', 'mkv', 'webm'].includes(ext)) return FileVideo
  if (['mp3', 'wav', 'flac', 'aac', 'ogg'].includes(ext)) return FileAudio
  if (['zip', 'rar', '7z', 'tar', 'gz', 'bz2'].includes(ext)) return FileArchive
  if (['js', 'ts', 'py', 'go', 'rs', 'java', 'c', 'cpp', 'h', 'vue', 'jsx', 'tsx', 'css', 'html', 'json', 'yaml', 'yml', 'xml', 'sql', 'sh', 'md'].includes(ext)) return FileCode
  if (['txt', 'doc', 'docx', 'pdf', 'xls', 'xlsx', 'ppt', 'pptx', 'csv'].includes(ext)) return FileText
  return File
})

const iconColor = computed(() => {
  if (props.file.isFolder) return 'text-accent'
  const ext = props.file.name.split('.').pop()?.toLowerCase() ?? ''
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext)) return 'text-success'
  if (['mp4', 'avi', 'mov', 'mkv', 'webm'].includes(ext)) return 'text-warning'
  if (['mp3', 'wav', 'flac', 'aac'].includes(ext)) return 'text-[#a855f7]'
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'text-warning'
  return 'text-text-tertiary'
})

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

function onDoubleClick() {
  if (props.file.isFolder) {
    fileStore.navigateToFolder(props.file.id, props.file.name)
  } else {
    emit('download', props.file)
  }
}

function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  showMenu.value = !showMenu.value
}

function closeMenu() {
  showMenu.value = false
}
</script>

<template>
  <!-- Grid view -->
  <div
    v-if="viewMode === 'grid'"
    class="group relative flex flex-col items-center p-4 rounded-xl border
           cursor-pointer select-none transition-all duration-150"
    :class="
      isSelected
        ? 'border-accent bg-accent-soft'
        : 'border-transparent hover:border-border-subtle hover:bg-surface-elevated hover:shadow-card'
    "
    @click.exact="emit('select', file.id)"
    @dblclick="onDoubleClick"
    @contextmenu="onContextMenu"
  >
    <!-- Checkbox -->
    <div
      class="absolute top-2 left-2 w-5 h-5 rounded-md border flex items-center justify-center
             transition-all duration-150"
      :class="
        isSelected
          ? 'border-accent bg-accent'
          : 'border-border opacity-0 group-hover:opacity-100 bg-surface-elevated'
      "
    >
      <svg
        v-if="isSelected"
        class="w-3 h-3 text-accent-text"
        viewBox="0 0 12 12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <polyline points="2,6 5,9 10,3" />
      </svg>
    </div>

    <!-- Menu button -->
    <button
      class="absolute top-2 right-2 p-1 rounded-md text-text-tertiary
             hover:text-text-secondary hover:bg-surface-hover
             opacity-0 group-hover:opacity-100 transition-all z-10"
      @click.stop="showMenu = !showMenu"
    >
      <MoreHorizontal class="w-4 h-4" />
    </button>

    <!-- Context menu -->
    <div
      v-if="showMenu"
      class="absolute top-9 right-2 z-20 w-40 py-1 rounded-xl bg-surface-elevated
             border border-border-subtle shadow-dropdown"
      @mouseleave="closeMenu"
    >
      <button
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
               hover:bg-surface-hover hover:text-text-primary transition-colors"
        @click.stop="emit('rename', file); closeMenu()"
      >
        <Pencil class="w-3 h-3" /> Rename
      </button>
      <button
        v-if="!file.isFolder"
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
               hover:bg-surface-hover hover:text-text-primary transition-colors"
        @click.stop="emit('download', file); closeMenu()"
      >
        <Download class="w-3 h-3" /> Download
      </button>
      <button
        v-if="!file.isFolder"
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
               hover:bg-surface-hover hover:text-text-primary transition-colors"
        @click.stop="emit('share', file); closeMenu()"
      >
        <Share2 class="w-3 h-3" /> Share
      </button>
      <div class="my-1 border-t border-border-subtle" />
      <button
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-danger
               hover:bg-danger-soft transition-colors"
        @click.stop="emit('delete', file); closeMenu()"
      >
        <Trash2 class="w-3 h-3" /> Delete
      </button>
    </div>

    <!-- Icon -->
    <component
      :is="fileIcon"
      :class="[iconColor, file.isFolder ? 'w-12 h-12 mb-1' : 'w-10 h-10 mb-2']"
      :fill="file.isFolder ? 'currentColor' : 'none'"
      :stroke-width="file.isFolder ? 1.5 : 2"
    />

    <!-- Name -->
    <span
      class="text-xs text-text-primary font-medium text-center truncate w-full leading-tight"
    >
      {{ file.name }}
    </span>

    <!-- Size -->
    <span class="text-[10px] text-text-tertiary mt-1">
      {{ file.isFolder ? 'Folder' : formatSize(file.size) }}
    </span>
  </div>

  <!-- List view -->
  <div
    v-else
    class="group flex items-center gap-3 px-3 h-11 rounded-lg cursor-pointer
           select-none transition-all duration-150"
    :class="
      isSelected
        ? 'bg-accent-soft'
        : 'hover:bg-surface-hover'
    "
    @click.exact="emit('select', file.id)"
    @dblclick="onDoubleClick"
    @contextmenu="onContextMenu"
  >
    <!-- Checkbox -->
    <div
      class="w-5 h-5 rounded-md border flex items-center justify-center shrink-0
             transition-all duration-150"
      :class="
        isSelected
          ? 'border-accent bg-accent'
          : 'border-border opacity-0 group-hover:opacity-100 bg-surface-elevated'
      "
    >
      <svg
        v-if="isSelected"
        class="w-3 h-3 text-accent-text"
        viewBox="0 0 12 12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <polyline points="2,6 5,9 10,3" />
      </svg>
    </div>

    <!-- Icon -->
    <component
      :is="fileIcon"
      class="w-5 h-5 shrink-0"
      :class="iconColor"
    />

    <!-- Name -->
    <span class="flex-1 text-sm text-text-primary truncate">
      {{ file.name }}
    </span>

    <!-- Size -->
    <span class="text-xs text-text-tertiary w-20 text-right shrink-0">
      {{ file.isFolder ? '--' : formatSize(file.size) }}
    </span>

    <!-- Date -->
    <span class="text-xs text-text-tertiary w-28 text-right shrink-0">
      {{ formatDate(file.updatedAt) }}
    </span>

    <!-- Actions -->
    <div class="relative shrink-0">
      <button
        class="p-1 rounded-md text-text-tertiary hover:text-text-secondary hover:bg-surface-hover
               opacity-0 group-hover:opacity-100 transition-all"
        @click.stop="showMenu = !showMenu"
      >
        <MoreHorizontal class="w-4 h-4" />
      </button>

      <div
        v-if="showMenu"
        class="absolute top-7 right-0 z-20 w-40 py-1 rounded-xl bg-surface-elevated
               border border-border-subtle shadow-dropdown"
        @mouseleave="closeMenu"
      >
        <button
          class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
                 hover:bg-surface-hover hover:text-text-primary transition-colors"
          @click.stop="emit('rename', file); closeMenu()"
        >
          <Pencil class="w-3 h-3" /> Rename
        </button>
        <button
          v-if="!file.isFolder"
          class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
                 hover:bg-surface-hover hover:text-text-primary transition-colors"
          @click.stop="emit('download', file); closeMenu()"
        >
          <Download class="w-3 h-3" /> Download
        </button>
        <button
          v-if="!file.isFolder"
          class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-text-secondary
                 hover:bg-surface-hover hover:text-text-primary transition-colors"
          @click.stop="emit('share', file); closeMenu()"
        >
          <Share2 class="w-3 h-3" /> Share
        </button>
        <div class="my-1 border-t border-border-subtle" />
        <button
          class="flex items-center gap-2 w-full px-3 py-1.5 text-xs text-danger
                 hover:bg-danger-soft transition-colors"
          @click.stop="emit('delete', file); closeMenu()"
        >
          <Trash2 class="w-3 h-3" /> Delete
        </button>
      </div>
    </div>
  </div>
</template>
