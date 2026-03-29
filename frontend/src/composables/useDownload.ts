import { ref } from 'vue'
import { fileApi } from '@/api/file'

const MAX_CONCURRENT_DOWNLOADS = 4

export interface DownloadTask {
  fileId: number
  fileName: string
  progress: number
  status: 'downloading' | 'done' | 'error'
  error?: string
}

const tasks = ref<DownloadTask[]>([])

export function useDownload() {
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

  /**
   * Download a scattered file: fetch download plan → parallel GET each chunk
   * → reassemble in order → trigger browser download.
   */
  async function downloadFile(fileId: number) {
    // First try legacy single-file download (for non-scattered or OSS files)
    const { data: planReply } = await fileApi.getDownloadPlan(fileId)

    const task: DownloadTask = {
      fileId,
      fileName: planReply.fileName,
      progress: 0,
      status: 'downloading',
    }
    tasks.value.push(task)
    const reactiveTask = tasks.value[tasks.value.length - 1]!

    try {
      const totalChunks = planReply.totalChunks
      const chunks = planReply.chunks.sort((a, b) => a.chunkIndex - b.chunkIndex)
      const buffers: ArrayBuffer[] = new Array(totalChunks)
      let downloaded = 0
      const token = localStorage.getItem('token') ?? ''

      // Download chunks in parallel with concurrency limit
      const queue = [...chunks]
      const inflight: Promise<void>[] = []

      async function fetchOne(chunk: { chunkIndex: number; downloadUrl: string; backupUrls?: string[] }) {
        buffers[chunk.chunkIndex] = await fetchChunkWithFallback(chunk, token)
        downloaded++
        reactiveTask.progress = Math.round((downloaded / totalChunks) * 90)
      }

      for (const chunk of queue) {
        const p = fetchOne(chunk).then(() => {
          inflight.splice(inflight.indexOf(p), 1)
        })
        inflight.push(p)
        if (inflight.length >= MAX_CONCURRENT_DOWNLOADS) {
          await Promise.race(inflight)
        }
      }
      await Promise.all(inflight)

      // Reassemble and trigger download
      reactiveTask.progress = 95
      const blob = new Blob(buffers, { type: 'application/octet-stream' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = planReply.fileName
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      reactiveTask.progress = 100
      reactiveTask.status = 'done'
    } catch (err) {
      reactiveTask.status = 'error'
      reactiveTask.error = err instanceof Error ? err.message : 'Download failed'
    }
  }

  function removeTask(fileId: number) {
    const idx = tasks.value.findIndex((t) => t.fileId === fileId)
    if (idx >= 0) tasks.value.splice(idx, 1)
  }

  return {
    tasks,
    downloadFile,
    removeTask,
  }
}
