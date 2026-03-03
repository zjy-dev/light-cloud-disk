import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/file', () => ({
  fileApi: {
    checkUpload: vi.fn(),
    uploadChunk: vi.fn(),
    mergeChunks: vi.fn(),
  },
}))

import { fileApi } from '@/api/file'

describe('useUpload', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.resetModules()
  })

  it('exports tasks and isUploading', async () => {
    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, isUploading } = useUpload()

    expect(tasks.value).toEqual([])
    expect(isUploading.value).toBe(false)
  })

  it('uploadFile adds task and handles fast upload', async () => {
    vi.mocked(fileApi.checkUpload).mockResolvedValue({
      data: { canFastUpload: true, uploadedChunks: [] },
    } as never)
    vi.mocked(fileApi.mergeChunks).mockResolvedValue({
      data: { success: true, file: { id: 1 } },
    } as never)

    // Mock crypto.subtle.digest
    const mockDigest = vi.fn().mockResolvedValue(new ArrayBuffer(32))
    Object.defineProperty(globalThis, 'crypto', {
      value: {
        subtle: { digest: mockDigest },
        randomUUID: () => 'test-uuid-123',
      },
      writable: true,
    })

    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, uploadFile } = useUpload()

    const onComplete = vi.fn()
    const file = new File(['hello world'], 'test.txt', { type: 'text/plain' })
    await uploadFile(file, 0, onComplete)

    expect(tasks.value.length).toBeGreaterThanOrEqual(1)
    const task = tasks.value.find((t) => t.fileName === 'test.txt')
    expect(task).toBeDefined()
    expect(task!.status).toBe('done')
    expect(task!.progress).toBe(100)
    expect(onComplete).toHaveBeenCalled()
    expect(fileApi.mergeChunks).toHaveBeenCalled()
  })

  it('uploadFile handles chunked upload', async () => {
    vi.mocked(fileApi.checkUpload).mockResolvedValue({
      data: { canFastUpload: false, uploadedChunks: [] },
    } as never)
    vi.mocked(fileApi.uploadChunk).mockResolvedValue({
      data: { success: true, chunkIndex: 0 },
    } as never)
    vi.mocked(fileApi.mergeChunks).mockResolvedValue({
      data: { success: true, file: { id: 2 } },
    } as never)

    const mockDigest = vi.fn().mockResolvedValue(new ArrayBuffer(32))
    Object.defineProperty(globalThis, 'crypto', {
      value: {
        subtle: { digest: mockDigest },
        randomUUID: () => 'test-uuid-456',
      },
      writable: true,
    })

    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, uploadFile } = useUpload()

    const file = new File(['small file content'], 'small.txt', { type: 'text/plain' })
    await uploadFile(file, 0)

    const task = tasks.value.find((t) => t.fileName === 'small.txt')
    expect(task).toBeDefined()
    expect(task!.status).toBe('done')
    expect(fileApi.uploadChunk).toHaveBeenCalled()
    expect(fileApi.mergeChunks).toHaveBeenCalled()
  })

  it('uploadFile handles errors', async () => {
    vi.mocked(fileApi.checkUpload).mockRejectedValue(new Error('network error'))

    const mockDigest = vi.fn().mockResolvedValue(new ArrayBuffer(32))
    Object.defineProperty(globalThis, 'crypto', {
      value: {
        subtle: { digest: mockDigest },
        randomUUID: () => 'test-uuid-789',
      },
      writable: true,
    })

    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, uploadFile } = useUpload()

    const file = new File(['data'], 'fail.txt', { type: 'text/plain' })
    await uploadFile(file, 0)

    const task = tasks.value.find((t) => t.fileName === 'fail.txt')
    expect(task).toBeDefined()
    expect(task!.status).toBe('error')
    expect(task!.error).toBe('network error')
  })

  it('removeTask removes a task by ID', async () => {
    Object.defineProperty(globalThis, 'crypto', {
      value: {
        subtle: { digest: vi.fn().mockResolvedValue(new ArrayBuffer(32)) },
        randomUUID: () => 'removable-task',
      },
      writable: true,
    })

    vi.mocked(fileApi.checkUpload).mockResolvedValue({
      data: { canFastUpload: true, uploadedChunks: [] },
    } as never)
    vi.mocked(fileApi.mergeChunks).mockResolvedValue({
      data: { success: true, file: { id: 1 } },
    } as never)

    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, uploadFile, removeTask } = useUpload()

    const file = new File(['x'], 'remove-me.txt', { type: 'text/plain' })
    await uploadFile(file, 0)

    const tasksBefore = tasks.value.length
    removeTask('removable-task')
    expect(tasks.value.length).toBe(tasksBefore - 1)
  })

  it('clearCompleted removes done and error tasks', async () => {
    Object.defineProperty(globalThis, 'crypto', {
      value: {
        subtle: { digest: vi.fn().mockResolvedValue(new ArrayBuffer(32)) },
        randomUUID: () => 'clear-test-' + Math.random(),
      },
      writable: true,
    })

    vi.mocked(fileApi.checkUpload).mockResolvedValue({
      data: { canFastUpload: true, uploadedChunks: [] },
    } as never)
    vi.mocked(fileApi.mergeChunks).mockResolvedValue({
      data: { success: true, file: { id: 1 } },
    } as never)

    const { useUpload } = await import('@/composables/useUpload')
    const { tasks, uploadFile, clearCompleted } = useUpload()

    await uploadFile(new File(['a'], 'a.txt'), 0)

    const beforeCount = tasks.value.length
    clearCompleted()
    expect(tasks.value.length).toBeLessThan(beforeCount)
  })
})
