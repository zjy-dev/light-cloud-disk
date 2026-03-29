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

  async function fetchChunkWithFallback(chunk: { chunkIndex: number; downloadUrl: string; backupUrls?: string[] }, token: string) {
    const urls = [chunk.downloadUrl, ...(chunk.backupUrls ?? [])]
    let lastError: Error | undefined

    for (const url of urls) {
      const resp = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (resp.ok) {
        return resp.arrayBuffer()
      }
      lastError = new Error(`Chunk ${chunk.chunkIndex} download failed: ${resp.status}`)
    }

    throw lastError ?? new Error(`Chunk ${chunk.chunkIndex} download failed`)
  }

  async function downloadFile(fileId: number) {
    const { data: plan } = await fileApi.getDownloadPlan(fileId)

    if (plan.totalChunks === 1 && plan.chunks.length === 1) {
      const chunk = plan.chunks[0]
      const url = chunk.downloadUrl
      const isDirectChunkURL = url.includes('/api/v1/chunks/')
      if ((url.startsWith('http://') || url.startsWith('https://')) && !isDirectChunkURL && !(chunk.backupUrls?.length)) {
        window.open(url, '_blank')
      } else {
        const token = localStorage.getItem('token') ?? ''
        const buffer = await fetchChunkWithFallback(chunk, token)
        const blob = new Blob([buffer], { type: 'application/octet-stream' })
        const blobUrl = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = blobUrl
        a.download = plan.fileName || 'download'
        a.click()
        URL.revokeObjectURL(blobUrl)
      }
      return
    }

    // Multi-chunk scattered file: parallel download + reassemble
    const token = localStorage.getItem('token') ?? ''
    const chunks = plan.chunks.sort((a, b) => a.chunkIndex - b.chunkIndex)
    const buffers: ArrayBuffer[] = new Array(plan.totalChunks)
    const inflight: Promise<void>[] = []

    async function fetchOne(chunk: { chunkIndex: number; downloadUrl: string; backupUrls?: string[] }) {
      buffers[chunk.chunkIndex] = await fetchChunkWithFallback(chunk, token)
    }

    for (const chunk of chunks) {
      const p = fetchOne(chunk).then(() => {
        inflight.splice(inflight.indexOf(p), 1)
      })
      inflight.push(p)
      if (inflight.length >= 4) {
        await Promise.race(inflight)
      }
    }
    await Promise.all(inflight)

    const blob = new Blob(buffers, { type: 'application/octet-stream' })
    const blobUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = plan.fileName || 'download'
    a.click()
    URL.revokeObjectURL(blobUrl)
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
