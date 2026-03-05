import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { fileApi } from '@/api/file'
import type { FileInfo, ViewMode, SortField, SortOrder } from '@/types'

export const useFileStore = defineStore('file', () => {
  const files = ref<FileInfo[]>([])
  const total = ref(0)
  const loading = ref(false)
  const currentParentId = ref(0)
  const breadcrumb = ref<{ id: number; name: string }[]>([{ id: 0, name: 'My Files' }])
  const selectedIds = ref<Set<number>>(new Set())
  const viewMode = ref<ViewMode>('grid')
  const sortField = ref<SortField>('name')
  const sortOrder = ref<SortOrder>('asc')
  const searchKeyword = ref('')

  const selectedFiles = computed(() =>
    files.value.filter((f) => selectedIds.value.has(f.id)),
  )

  const sortedFiles = computed(() => {
    const sorted = [...files.value]
    sorted.sort((a, b) => {
      // Folders always come first
      if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1

      let cmp = 0
      switch (sortField.value) {
        case 'name':
          cmp = a.name.localeCompare(b.name)
          break
        case 'size':
          cmp = a.size - b.size
          break
        case 'updatedAt':
          cmp = a.updatedAt - b.updatedAt
          break
      }
      return sortOrder.value === 'asc' ? cmp : -cmp
    })
    return sorted
  })

  async function fetchFiles(parentId?: number) {
    loading.value = true
    selectedIds.value.clear()
    try {
      const pid = parentId ?? currentParentId.value
      const { data } = await fileApi.listFiles({ parentId: pid })
      files.value = data.files ?? []
      total.value = data.total
      currentParentId.value = pid
    } finally {
      loading.value = false
    }
  }

  async function search(keyword: string) {
    if (!keyword.trim()) {
      searchKeyword.value = ''
      await fetchFiles()
      return
    }
    loading.value = true
    selectedIds.value.clear()
    searchKeyword.value = keyword
    try {
      const { data } = await fileApi.searchFiles({ keyword })
      files.value = data.files ?? []
      total.value = data.total
    } finally {
      loading.value = false
    }
  }

  function navigateToFolder(folderId: number, folderName: string) {
    const idx = breadcrumb.value.findIndex((b) => b.id === folderId)
    if (idx >= 0) {
      breadcrumb.value = breadcrumb.value.slice(0, idx + 1)
    } else {
      breadcrumb.value.push({ id: folderId, name: folderName })
    }
    searchKeyword.value = ''
    fetchFiles(folderId)
  }

  function navigateUp() {
    if (breadcrumb.value.length > 1) {
      breadcrumb.value.pop()
      const parent = breadcrumb.value[breadcrumb.value.length - 1]
      if (parent) fetchFiles(parent.id)
    }
  }

  async function createFolder(name: string) {
    await fileApi.createFolder({ parentId: currentParentId.value, name })
    await fetchFiles()
  }

  async function renameFile(fileId: number, newName: string) {
    await fileApi.renameFile({ fileId, newName })
    await fetchFiles()
  }

  async function deleteSelected() {
    const ids = Array.from(selectedIds.value)
    if (ids.length === 0) return
    await fileApi.deleteFiles(ids)
    await fetchFiles()
  }

  async function moveFiles(fileIds: number[], targetFolderId: number) {
    await fileApi.moveFiles({ fileIds, targetFolderId })
    await fetchFiles()
  }

  async function downloadFile(fileId: number) {
    const { data } = await fileApi.getDownloadURL(fileId)
    const url = data.downloadUrl

    // Relative URLs (local-mode stream) require auth header, so fetch as blob
    if (url.startsWith('/')) {
      const { data: blob } = await fileApi.downloadBlob(url)
      const blobUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = blobUrl
      a.download = data.fileName || 'download'
      a.click()
      URL.revokeObjectURL(blobUrl)
    } else {
      // External presigned URL (S3 / OSS) - open directly
      window.open(url, '_blank')
    }
  }

  function toggleSelect(id: number) {
    if (selectedIds.value.has(id)) {
      selectedIds.value.delete(id)
    } else {
      selectedIds.value.add(id)
    }
    // Reassign to trigger reactivity
    selectedIds.value = new Set(selectedIds.value)
  }

  function selectAll() {
    if (selectedIds.value.size === files.value.length) {
      selectedIds.value.clear()
    } else {
      selectedIds.value = new Set(files.value.map((f) => f.id))
    }
    selectedIds.value = new Set(selectedIds.value)
  }

  function clearSelection() {
    selectedIds.value = new Set()
  }

  return {
    files,
    total,
    loading,
    currentParentId,
    breadcrumb,
    selectedIds,
    selectedFiles,
    viewMode,
    sortField,
    sortOrder,
    sortedFiles,
    searchKeyword,
    fetchFiles,
    search,
    navigateToFolder,
    navigateUp,
    createFolder,
    renameFile,
    deleteSelected,
    moveFiles,
    downloadFile,
    toggleSelect,
    selectAll,
    clearSelection,
  }
})
