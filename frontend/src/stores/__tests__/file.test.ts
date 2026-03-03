import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useFileStore } from '@/stores/file'

vi.mock('@/api/file', () => ({
  fileApi: {
    listFiles: vi.fn(),
    searchFiles: vi.fn(),
    createFolder: vi.fn(),
    renameFile: vi.fn(),
    deleteFiles: vi.fn(),
    moveFiles: vi.fn(),
    getDownloadURL: vi.fn(),
  },
}))

import { fileApi } from '@/api/file'

describe('file store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('initializes with default state', () => {
    const store = useFileStore()
    expect(store.files).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.currentParentId).toBe(0)
    expect(store.breadcrumb).toEqual([{ id: 0, name: 'My Files' }])
    expect(store.selectedIds.size).toBe(0)
    expect(store.viewMode).toBe('grid')
    expect(store.sortField).toBe('name')
    expect(store.sortOrder).toBe('asc')
    expect(store.searchKeyword).toBe('')
  })

  it('fetchFiles loads files from API', async () => {
    const mockFiles = [
      { id: 1, name: 'docs', isFolder: true, size: 0, fileMd5: '', parentId: 0, path: '/docs', createdAt: 1000, updatedAt: 1000 },
      { id: 2, name: 'readme.txt', isFolder: false, size: 1024, fileMd5: 'abc', parentId: 0, path: '/readme.txt', createdAt: 1000, updatedAt: 1000 },
    ]
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: mockFiles, total: 2 },
    } as never)

    const store = useFileStore()
    await store.fetchFiles(0)

    expect(store.files).toEqual(mockFiles)
    expect(store.total).toBe(2)
    expect(store.loading).toBe(false)
    expect(fileApi.listFiles).toHaveBeenCalledWith({ parentId: 0 })
  })

  it('fetchFiles uses currentParentId when no arg provided', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.currentParentId = 42
    await store.fetchFiles()

    expect(fileApi.listFiles).toHaveBeenCalledWith({ parentId: 42 })
  })

  it('fetchFiles clears selection', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.selectedIds = new Set([1, 2, 3])
    await store.fetchFiles(0)

    expect(store.selectedIds.size).toBe(0)
  })

  it('sortedFiles sorts folders first, then by field', () => {
    const store = useFileStore()
    store.files = [
      { id: 1, name: 'banana.txt', isFolder: false, size: 200, fileMd5: '', parentId: 0, path: '', createdAt: 1000, updatedAt: 2000 },
      { id: 2, name: 'alpha', isFolder: true, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 1000, updatedAt: 1000 },
      { id: 3, name: 'apple.txt', isFolder: false, size: 100, fileMd5: '', parentId: 0, path: '', createdAt: 1000, updatedAt: 3000 },
    ]

    // Default sort: name asc
    const sorted = store.sortedFiles
    expect(sorted[0]!.name).toBe('alpha') // folder first
    expect(sorted[1]!.name).toBe('apple.txt')
    expect(sorted[2]!.name).toBe('banana.txt')
  })

  it('sortedFiles respects sort order', () => {
    const store = useFileStore()
    store.files = [
      { id: 1, name: 'a.txt', isFolder: false, size: 100, fileMd5: '', parentId: 0, path: '', createdAt: 1000, updatedAt: 1000 },
      { id: 2, name: 'b.txt', isFolder: false, size: 200, fileMd5: '', parentId: 0, path: '', createdAt: 1000, updatedAt: 1000 },
    ]
    store.sortField = 'size'
    store.sortOrder = 'desc'

    const sorted = store.sortedFiles
    expect(sorted[0]!.name).toBe('b.txt') // 200
    expect(sorted[1]!.name).toBe('a.txt') // 100
  })

  it('search calls searchFiles API', async () => {
    const mockFiles = [
      { id: 1, name: 'found.txt', isFolder: false, size: 50, fileMd5: 'x', parentId: 0, path: '', createdAt: 1000, updatedAt: 1000 },
    ]
    vi.mocked(fileApi.searchFiles).mockResolvedValue({
      data: { files: mockFiles, total: 1 },
    } as never)

    const store = useFileStore()
    await store.search('found')

    expect(fileApi.searchFiles).toHaveBeenCalledWith({ keyword: 'found' })
    expect(store.files).toEqual(mockFiles)
    expect(store.searchKeyword).toBe('found')
  })

  it('search with empty keyword falls back to fetchFiles', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.searchKeyword = 'old'
    await store.search('  ')

    expect(store.searchKeyword).toBe('')
    expect(fileApi.listFiles).toHaveBeenCalled()
  })

  it('navigateToFolder updates breadcrumb and fetches files', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.navigateToFolder(5, 'Documents')

    expect(store.breadcrumb).toEqual([
      { id: 0, name: 'My Files' },
      { id: 5, name: 'Documents' },
    ])
    expect(store.searchKeyword).toBe('')
    expect(fileApi.listFiles).toHaveBeenCalledWith({ parentId: 5 })
  })

  it('navigateToFolder truncates breadcrumb when navigating back', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.breadcrumb = [
      { id: 0, name: 'My Files' },
      { id: 5, name: 'Documents' },
      { id: 10, name: 'Work' },
    ]

    store.navigateToFolder(5, 'Documents')

    expect(store.breadcrumb).toEqual([
      { id: 0, name: 'My Files' },
      { id: 5, name: 'Documents' },
    ])
  })

  it('navigateUp pops breadcrumb and fetches parent', async () => {
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.breadcrumb = [
      { id: 0, name: 'My Files' },
      { id: 5, name: 'Documents' },
    ]

    store.navigateUp()

    expect(store.breadcrumb).toEqual([{ id: 0, name: 'My Files' }])
    expect(fileApi.listFiles).toHaveBeenCalledWith({ parentId: 0 })
  })

  it('navigateUp does nothing at root', () => {
    const store = useFileStore()
    store.navigateUp()
    expect(store.breadcrumb).toEqual([{ id: 0, name: 'My Files' }])
  })

  it('createFolder calls API and refreshes files', async () => {
    vi.mocked(fileApi.createFolder).mockResolvedValue({
      data: { success: true, folder: { id: 99 } },
    } as never)
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.currentParentId = 5
    await store.createFolder('New Folder')

    expect(fileApi.createFolder).toHaveBeenCalledWith({ parentId: 5, name: 'New Folder' })
    expect(fileApi.listFiles).toHaveBeenCalled()
  })

  it('renameFile calls API and refreshes files', async () => {
    vi.mocked(fileApi.renameFile).mockResolvedValue({
      data: { success: true },
    } as never)
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    await store.renameFile(1, 'renamed.txt')

    expect(fileApi.renameFile).toHaveBeenCalledWith({ fileId: 1, newName: 'renamed.txt' })
  })

  it('deleteSelected sends selected IDs and refreshes', async () => {
    vi.mocked(fileApi.deleteFiles).mockResolvedValue({
      data: { success: true },
    } as never)
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    store.selectedIds = new Set([1, 3, 5])
    await store.deleteSelected()

    expect(fileApi.deleteFiles).toHaveBeenCalledWith(expect.arrayContaining([1, 3, 5]))
  })

  it('deleteSelected does nothing when no selection', async () => {
    const store = useFileStore()
    await store.deleteSelected()
    expect(fileApi.deleteFiles).not.toHaveBeenCalled()
  })

  it('moveFiles calls API and refreshes', async () => {
    vi.mocked(fileApi.moveFiles).mockResolvedValue({
      data: { success: true },
    } as never)
    vi.mocked(fileApi.listFiles).mockResolvedValue({
      data: { files: [], total: 0 },
    } as never)

    const store = useFileStore()
    await store.moveFiles([1, 2], 10)

    expect(fileApi.moveFiles).toHaveBeenCalledWith({ fileIds: [1, 2], targetFolderId: 10 })
  })

  it('downloadFile opens URL in new tab', async () => {
    vi.mocked(fileApi.getDownloadURL).mockResolvedValue({
      data: { downloadUrl: 'https://cdn.example.com/file.zip', fileName: 'file.zip' },
    } as never)
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    const store = useFileStore()
    await store.downloadFile(7)

    expect(fileApi.getDownloadURL).toHaveBeenCalledWith(7)
    expect(openSpy).toHaveBeenCalledWith('https://cdn.example.com/file.zip', '_blank')
    openSpy.mockRestore()
  })

  it('toggleSelect adds and removes IDs', () => {
    const store = useFileStore()

    store.toggleSelect(1)
    expect(store.selectedIds.has(1)).toBe(true)

    store.toggleSelect(1)
    expect(store.selectedIds.has(1)).toBe(false)
  })

  it('selectAll toggles all files', () => {
    const store = useFileStore()
    store.files = [
      { id: 1, name: 'a', isFolder: false, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 0, updatedAt: 0 },
      { id: 2, name: 'b', isFolder: false, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 0, updatedAt: 0 },
    ]

    store.selectAll()
    expect(store.selectedIds.size).toBe(2)

    store.selectAll()
    expect(store.selectedIds.size).toBe(0)
  })

  it('clearSelection empties selection', () => {
    const store = useFileStore()
    store.selectedIds = new Set([1, 2, 3])

    store.clearSelection()
    expect(store.selectedIds.size).toBe(0)
  })

  it('selectedFiles returns files matching selection', () => {
    const store = useFileStore()
    store.files = [
      { id: 1, name: 'a', isFolder: false, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 0, updatedAt: 0 },
      { id: 2, name: 'b', isFolder: false, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 0, updatedAt: 0 },
      { id: 3, name: 'c', isFolder: false, size: 0, fileMd5: '', parentId: 0, path: '', createdAt: 0, updatedAt: 0 },
    ]
    store.selectedIds = new Set([1, 3])

    expect(store.selectedFiles.map((f) => f.id)).toEqual([1, 3])
  })
})
