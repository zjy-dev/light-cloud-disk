import { ref, computed } from 'vue'
import type { AxiosError } from 'axios'
import SparkMD5 from 'spark-md5'
import { fileApi } from '@/api/file'

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB per upload chunk
const HASH_CHUNK_SIZE = 2 * 1024 * 1024 // 2MB per hash chunk for smoother UI
const MAX_CONCURRENT_UPLOADS = 4 // parallel chunk uploads

export interface UploadTask {
  id: string
  file: File
  fileName: string
  progress: number
  status: 'pending' | 'hashing' | 'uploading' | 'completing' | 'done' | 'error'
  error?: string
}

const tasks = ref<UploadTask[]>([])

export function useUpload() {
  const isUploading = computed(() => tasks.value.some((t) => ['hashing', 'uploading', 'completing'].includes(t.status)))

  /**
   * Compute file MD5 in 2MB chunks with spark-md5
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
      await new Promise<void>((resolve) => setTimeout(resolve, 0))
    }

    return spark.end()
  }

  /**
   * Direct scattered upload: upload each chunk directly to the assigned
   * file-service instance via HTTP PUT, bypassing the gateway.
   */
  async function directUpload(
    task: UploadTask,
    parentId: number,
    fileMd5: string,
    totalChunks: number,
    uploadPlanAssignments: { chunkIndex: number; uploadUrl: string }[],
    uploadedChunks: number[],
    onComplete?: () => void,
  ) {
    task.status = 'uploading'
    const uploadedSet = new Set(uploadedChunks)
    const token = localStorage.getItem('token') ?? ''

    // Filter to only chunks that haven't been uploaded yet
    const pending = uploadPlanAssignments.filter((a) => !uploadedSet.has(a.chunkIndex))

    let uploaded = uploadedSet.size

    // Upload chunks in parallel with concurrency limit
    const queue = [...pending]
    const inflight: Promise<void>[] = []

    async function uploadOne(assignment: { chunkIndex: number; uploadUrl: string }) {
      const start = assignment.chunkIndex * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, task.file.size)
      const blob = task.file.slice(start, end)
      const buffer = await blob.arrayBuffer()

      const resp = await fetch(assignment.uploadUrl, {
        method: 'PUT',
        body: new Uint8Array(buffer),
        headers: {
          'Content-Type': 'application/octet-stream',
          'X-File-Size': String(task.file.size),
          Authorization: `Bearer ${token}`,
        },
      })

      if (!resp.ok) {
        throw new Error(`Chunk ${assignment.chunkIndex} upload failed: ${resp.status}`)
      }

      uploaded++
      task.progress = 15 + Math.round((uploaded / totalChunks) * 70)
    }

    for (const assignment of queue) {
      const p = uploadOne(assignment).then(() => {
        inflight.splice(inflight.indexOf(p), 1)
      })
      inflight.push(p)
      if (inflight.length >= MAX_CONCURRENT_UPLOADS) {
        await Promise.race(inflight)
      }
    }
    await Promise.all(inflight)

    // Complete the upload (no merging — scattered storage)
    task.status = 'completing'
    task.progress = 90
    await fileApi.completeUpload({
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

    const { data: initReply } = await fileApi.initPresignedUpload({
      parentId,
      fileName: task.file.name,
      fileMd5,
      fileSize: task.file.size,
      totalParts,
    })

    if (initReply.canFastUpload) {
      task.progress = 100
      task.status = 'done'
      onComplete?.()
      return
    }

    const sessionId = initReply.sessionId ?? ''
    const partSize = initReply.partSize || CHUNK_SIZE

    const completedSet = new Set(
      (initReply.completedParts ?? []).map((p: { partNumber: number }) => p.partNumber),
    )
    const pendingParts = (initReply.pendingParts ?? []).map((p: { partNumber: number; uploadUrl: string }) => ({
      partNumber: p.partNumber,
      uploadUrl: p.uploadUrl,
    }))

    task.status = 'uploading'
    let uploaded = completedSet.size

    try {
      for (const part of pendingParts) {
        if (completedSet.has(part.partNumber)) continue

        const start = (part.partNumber - 1) * partSize
        const end = Math.min(start + partSize, task.file.size)
        const blob = task.file.slice(start, end)

        const resp = await fetch(part.uploadUrl, {
          method: 'PUT',
          body: blob,
          headers: { 'Content-Type': 'application/octet-stream' },
        })

        if (!resp.ok) {
          throw new Error(`Presigned PUT failed for part ${part.partNumber}: ${resp.status}`)
        }

        const etag = resp.headers.get('ETag') ?? ''

        await fileApi.reportUploadedPart({
          sessionId,
          partNumber: part.partNumber,
          etag,
          size: end - start,
        })

        uploaded++
        task.progress = 15 + Math.round((uploaded / totalParts) * 70)
      }

      task.status = 'completing'
      task.progress = 90
      await fileApi.completePresignedUpload(sessionId)

      task.progress = 100
      task.status = 'done'
      onComplete?.()
    } catch (err) {
      if (sessionId) {
        try {
          await fileApi.abortPresignedUpload(sessionId)
        } catch { /* best-effort cleanup */ }
      }
      throw err
    }
  }

  async function uploadFile(file: File, parentId: number, onComplete?: () => void) {
    const taskId = crypto.randomUUID()
    const taskData: UploadTask = {
      id: taskId,
      file,
      fileName: file.name,
      progress: 0,
      status: 'hashing',
    }
    tasks.value.push(taskData)
    const task = tasks.value[tasks.value.length - 1]!

    try {
      const fileMd5 = await computeMd5(file, (pct) => {
        task.progress = Math.round(pct * 15)
      })
      const totalChunks = Math.ceil(file.size / CHUNK_SIZE)

      const { data: checkResult } = await fileApi.checkUpload({
        fileMd5,
        fileSize: file.size,
        totalChunks,
      })

      if (checkResult.canFastUpload) {
        // Instant dedup — just create the file record
        task.status = 'completing'
        task.progress = 90
        await fileApi.completeUpload({
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

      const uploadMode = checkResult.uploadMode ?? 'direct'
      const uploadedChunks = checkResult.uploadedChunks ?? []

      if (uploadMode === 'presigned') {
        await presignedUpload(task, parentId, fileMd5, onComplete)
      } else {
        // Build upload assignments from the plan, or fall back to empty
        const assignments = (checkResult.uploadPlan?.assignments ?? []).map((a) => ({
          chunkIndex: a.chunkIndex,
          uploadUrl: a.uploadUrl,
        }))
        await directUpload(task, parentId, fileMd5, totalChunks, assignments, uploadedChunks, onComplete)
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
