import { ref, computed } from 'vue'
import type { AxiosError } from 'axios'
import SparkMD5 from 'spark-md5'
import { fileApi } from '@/api/file'

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB per upload chunk
const HASH_CHUNK_SIZE = 2 * 1024 * 1024 // 2MB per hash chunk for smoother UI

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

  /**
   * Compute file MD5 in 2MB chunks with spark-md5
   * Chunked reads prevent UI freeze on large files and improve progress updates
   * onProgress receives values in [0, 1]
   */
  async function computeMd5(file: File, onProgress?: (pct: number) => void): Promise<string> {
    const spark = new SparkMD5.ArrayBuffer()
    const totalChunks = Math.ceil(file.size / HASH_CHUNK_SIZE)

    for (let i = 0; i < totalChunks; i++) {
      const start = i * HASH_CHUNK_SIZE
      const slice = file.slice(start, Math.min(start + HASH_CHUNK_SIZE, file.size))
      const buffer = await slice.arrayBuffer()
      spark.append(buffer)
      onProgress?.((i + 1) / totalChunks)
      // Yield to event loop between chunks so Vue can refresh UI
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    }

    return spark.end()
  }

  /**
   * Direct upload flow (Mode A default): upload chunks to gateway, then merge.
   */
  async function directUpload(
    task: UploadTask,
    parentId: number,
    fileMd5: string,
    totalChunks: number,
    uploadedChunks: number[],
    onComplete?: () => void,
  ) {
    task.status = 'uploading'
    const uploadedSet = new Set(uploadedChunks)

    let uploaded = uploadedSet.size
    for (let i = 0; i < totalChunks; i++) {
      if (uploadedSet.has(i)) continue

      const start = i * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, task.file.size)
      const chunkBlob = task.file.slice(start, end)

      await fileApi.uploadChunk({
        fileMd5,
        chunkIndex: i,
        chunkSize: end - start,
        chunkFile: chunkBlob,
      })

      uploaded++
      task.progress = 15 + Math.round((uploaded / totalChunks) * 70)
    }

    // Merge chunks
    task.status = 'merging'
    task.progress = 90
    await fileApi.mergeChunks({
      parentId,
      fileName: task.file.name,
      fileMd5,
      fileSize: task.file.size,
      totalChunks,
    })

    task.progress = 100
    task.status = 'done'
    onComplete?.()
  }

  /**
   * Presigned upload flow: browser PUTs each part directly to S3-compatible
   * storage via presigned URLs, then tells the backend to complete the multipart.
   */
  async function presignedUpload(
    task: UploadTask,
    parentId: number,
    fileMd5: string,
    onComplete?: () => void,
  ) {
    const totalParts = Math.ceil(task.file.size / CHUNK_SIZE)

    // Init session (also handles resume: returns pending + completed parts)
    const { data: initReply } = await fileApi.initPresignedUpload({
      parentId,
      fileName: task.file.name,
      fileMd5,
      fileSize: task.file.size,
      totalParts,
    })

    // Fast-upload dedup hit
    const rawInit = initReply as Record<string, unknown>
    const canFast = !!(rawInit['can_fast_upload'] ?? rawInit['canFastUpload'])
    if (canFast) {
      task.progress = 100
      task.status = 'done'
      onComplete?.()
      return
    }

    const sessionId = ((rawInit['session_id'] ?? rawInit['sessionId']) as string) || ''
    const partSize = Number(rawInit['part_size'] ?? rawInit['partSize']) || CHUNK_SIZE

    // completed_parts / pending_parts may arrive in snake_case
    type RawPart = Record<string, unknown>
    const rawCompleted = ((rawInit['completed_parts'] ?? rawInit['completedParts']) as RawPart[] | undefined) ?? []
    const rawPending = ((rawInit['pending_parts'] ?? rawInit['pendingParts']) as RawPart[] | undefined) ?? []

    const completedSet = new Set(
      rawCompleted.map((p) => Number(p['part_number'] ?? p['partNumber'])),
    )
    const pendingParts = rawPending.map((p) => ({
      partNumber: Number(p['part_number'] ?? p['partNumber']),
      uploadUrl: (p['upload_url'] ?? p['uploadUrl']) as string,
    }))

    task.status = 'uploading'
    let uploaded = completedSet.size

    try {
      for (const part of pendingParts) {
        if (completedSet.has(part.partNumber)) continue

        // S3 part numbers are 1-based; slice accordingly
        const start = (part.partNumber - 1) * partSize
        const end = Math.min(start + partSize, task.file.size)
        const blob = task.file.slice(start, end)

        // PUT directly to presigned URL (no auth header — the URL is self-authenticating)
        const resp = await fetch(part.uploadUrl, {
          method: 'PUT',
          body: blob,
          headers: { 'Content-Type': 'application/octet-stream' },
        })

        if (!resp.ok) {
          throw new Error(`Presigned PUT failed for part ${part.partNumber}: ${resp.status}`)
        }

        const etag = resp.headers.get('ETag') ?? ''

        // Report the uploaded part to backend
        await fileApi.reportUploadedPart({
          sessionId,
          partNumber: part.partNumber,
          etag,
          size: end - start,
        })

        uploaded++
        task.progress = 15 + Math.round((uploaded / totalParts) * 70)
      }

      // Complete the multipart upload
      task.status = 'merging'
      task.progress = 90
      await fileApi.completePresignedUpload(sessionId)

      task.progress = 100
      task.status = 'done'
      onComplete?.()
    } catch (err) {
      // Abort the S3 multipart upload to avoid leaked parts
      if (sessionId) {
        try {
          await fileApi.abortPresignedUpload(sessionId)
        } catch { /* best-effort cleanup */ }
      }
      throw err // re-throw so outer catch sets task.status = 'error'
    }
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
      // Compute MD5 in chunks and map hashing progress to 0-15
      const fileMd5 = await computeMd5(file, (pct) => {
        task.progress = Math.round(pct * 15)
      })
      const totalChunks = Math.ceil(file.size / CHUNK_SIZE)

      // Check if instant upload is available + decide upload mode
      const { data: checkResult } = await fileApi.checkUpload({
        fileMd5,
        fileSize: file.size,
        totalChunks,
      })

      // Backend proto uses snake_case JSON keys; TS types use camelCase.
      // Access both to handle either serialization format.
      const rawResult = checkResult as Record<string, unknown>
      const canFastUpload = !!(rawResult['can_fast_upload'] ?? rawResult['canFastUpload'])

      if (canFastUpload) {
        // Instant upload hit, go straight to merge
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

      // Branch based on upload mode.
      const uploadMode = (rawResult['upload_mode'] ?? rawResult['uploadMode'] ?? 'direct') as string
      const uploadedChunks = (rawResult['uploaded_chunks'] ?? rawResult['uploadedChunks'] ?? []) as number[]
      if (uploadMode === 'presigned') {
        await presignedUpload(task, parentId, fileMd5, onComplete)
      } else {
        await directUpload(task, parentId, fileMd5, totalChunks, uploadedChunks, onComplete)
      }
    } catch (err: unknown) {
      task.status = 'error'
      const axiosErr = err as AxiosError<{ disk_full?: boolean; error?: string }>
      if (axiosErr.response?.data?.error) {
        task.error = axiosErr.response.data.error
      } else {
        task.error = err instanceof Error ? err.message : 'Upload failed'
      }
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
