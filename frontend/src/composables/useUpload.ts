import { ref, computed } from 'vue'
import { fileApi } from '@/api/file'

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB

export interface UploadTask {
  id: string
  file: File
  fileName: string
  progress: number
  status: 'pending' | 'hashing' | 'uploading' | 'merging' | 'done' | 'error'
  error?: string
}

const tasks = ref<UploadTask[]>([])

export function useUpload() {
  const isUploading = computed(() => tasks.value.some((t) => ['hashing', 'uploading', 'merging'].includes(t.status)))

  async function computeMd5(file: File): Promise<string> {
    const buffer = await file.arrayBuffer()
    const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
    const hashArray = Array.from(new Uint8Array(hashBuffer))
    return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('')
  }

  function arrayBufferToBase64(buffer: ArrayBuffer): string {
    const bytes = new Uint8Array(buffer)
    let binary = ''
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]!)
    }
    return btoa(binary)
  }

  async function uploadFile(file: File, parentId: number, onComplete?: () => void) {
    const taskId = crypto.randomUUID()
    const task: UploadTask = {
      id: taskId,
      file,
      fileName: file.name,
      progress: 0,
      status: 'hashing',
    }
    tasks.value.push(task)

    try {
      // Compute hash
      const fileMd5 = await computeMd5(file)
      const totalChunks = Math.ceil(file.size / CHUNK_SIZE)

      // Check for instant upload
      const { data: checkResult } = await fileApi.checkUpload({
        fileMd5,
        fileSize: file.size,
        totalChunks,
      })

      if (checkResult.canFastUpload) {
        // Instant upload - merge directly
        task.status = 'merging'
        task.progress = 90
        await fileApi.mergeChunks({
          parentId,
          fileName: file.name,
          fileMd5,
          fileSize: file.size,
          totalChunks,
        })
        task.progress = 100
        task.status = 'done'
        onComplete?.()
        return
      }

      // Upload chunks
      task.status = 'uploading'
      const uploadedSet = new Set(checkResult.uploadedChunks ?? [])

      let uploaded = uploadedSet.size
      for (let i = 0; i < totalChunks; i++) {
        if (uploadedSet.has(i)) continue

        const start = i * CHUNK_SIZE
        const end = Math.min(start + CHUNK_SIZE, file.size)
        const chunkBlob = file.slice(start, end)
        const chunkBuffer = await chunkBlob.arrayBuffer()
        const chunkData = arrayBufferToBase64(chunkBuffer)

        await fileApi.uploadChunk({
          fileMd5,
          chunkIndex: i,
          chunkSize: end - start,
          chunkData,
        })

        uploaded++
        task.progress = Math.round((uploaded / totalChunks) * 85)
      }

      // Merge chunks
      task.status = 'merging'
      task.progress = 90
      await fileApi.mergeChunks({
        parentId,
        fileName: file.name,
        fileMd5,
        fileSize: file.size,
        totalChunks,
      })

      task.progress = 100
      task.status = 'done'
      onComplete?.()
    } catch (err: unknown) {
      task.status = 'error'
      task.error = err instanceof Error ? err.message : 'Upload failed'
    }
  }

  function removeTask(taskId: string) {
    const idx = tasks.value.findIndex((t) => t.id === taskId)
    if (idx >= 0) tasks.value.splice(idx, 1)
  }

  function clearCompleted() {
    tasks.value = tasks.value.filter((t) => t.status !== 'done' && t.status !== 'error')
  }

  return {
    tasks,
    isUploading,
    uploadFile,
    removeTask,
    clearCompleted,
  }
}
