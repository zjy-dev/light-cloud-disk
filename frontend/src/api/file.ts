import client from './client'
import type {
  CheckUploadReply,
  CreateShareReply,
  DiskUsageReply,
  GetDownloadURLReply,
  GetShareReply,
  ListFilesReply,
  ListTrashReply,
  MergeChunksReply,
  SearchFilesReply,
  UploadChunkReply,
  InitPresignedUploadReply,
  ReportUploadedPartReply,
  CompletePresignedUploadReply,
  AbortPresignedUploadReply,
} from '@/types'

export const fileApi = {
  listFiles(params: { parentId?: number; page?: number; pageSize?: number }) {
    return client.get<ListFilesReply>('/files', {
      params: {
        parent_id: params.parentId ?? 0,
        page: params.page ?? 1,
        page_size: params.pageSize ?? 50,
      },
    })
  },

  searchFiles(params: { keyword: string; page?: number; pageSize?: number }) {
    return client.get<SearchFilesReply>('/files/search', {
      params: {
        keyword: params.keyword,
        page: params.page ?? 1,
        page_size: params.pageSize ?? 50,
      },
    })
  },

  createFolder(data: { parentId: number; name: string }) {
    return client.post<{ success: boolean; folder: { id: number } }>('/file/folder', {
      parent_id: data.parentId,
      name: data.name,
    })
  },

  renameFile(data: { fileId: number; newName: string }) {
    return client.put<{ success: boolean }>('/file/rename', {
      file_id: data.fileId,
      new_name: data.newName,
    })
  },

  deleteFiles(fileIds: number[]) {
    return client.delete<{ success: boolean }>('/files', {
      data: { file_ids: fileIds },
    })
  },

  moveFiles(data: { fileIds: number[]; targetFolderId: number }) {
    return client.put<{ success: boolean }>('/file/move', {
      file_ids: data.fileIds,
      target_folder_id: data.targetFolderId,
    })
  },

  getDownloadURL(fileId: number) {
    return client.get<GetDownloadURLReply>(`/file/download/${fileId}`)
  },

  // Upload APIs
  checkUpload(data: { fileMd5: string; fileSize: number; totalChunks: number }) {
    return client.post<CheckUploadReply>('/file/check-upload', {
      file_md5: data.fileMd5,
      file_size: data.fileSize,
      total_chunks: data.totalChunks,
    })
  },

  uploadChunk(data: { fileMd5: string; chunkIndex: number; chunkSize: number; chunkFile: Blob }) {
    const formData = new FormData()
    formData.append('file_md5', data.fileMd5)
    formData.append('chunk_index', String(data.chunkIndex))
    formData.append('chunk_size', String(data.chunkSize))
    formData.append('chunk_file', data.chunkFile)

    return client.post<UploadChunkReply>('/file/upload-chunk', formData)
  },

  mergeChunks(data: {
    parentId: number
    fileName: string
    fileMd5: string
    fileSize: number
    totalChunks: number
  }) {
    return client.post<MergeChunksReply>('/file/merge-chunks', {
      parent_id: data.parentId,
      file_name: data.fileName,
      file_md5: data.fileMd5,
      file_size: data.fileSize,
      total_chunks: data.totalChunks,
    })
  },

  // Trash APIs
  listTrash(params?: { page?: number; pageSize?: number }) {
    return client.get<ListTrashReply>('/trash', {
      params: {
        page: params?.page ?? 1,
        page_size: params?.pageSize ?? 50,
      },
    })
  },

  restoreFiles(fileIds: number[]) {
    return client.post<{ success: boolean }>('/trash/restore', {
      file_ids: fileIds,
    })
  },

  permanentDelete(fileIds: number[]) {
    return client.delete<{ success: boolean }>('/trash', {
      data: { file_ids: fileIds },
    })
  },

  // Share APIs
  createShare(data: { fileId: number; expireDays: number; password?: string }) {
    return client.post<CreateShareReply>('/share', {
      file_id: data.fileId,
      expire_days: data.expireDays,
      password: data.password ?? '',
    })
  },

  getShare(shareId: string, password?: string) {
    return client.get<GetShareReply>(`/share/${shareId}`, {
      params: password ? { password } : undefined,
    })
  },

  // Download a file as a blob (for local-mode stream URLs that need auth)
  downloadBlob(url: string) {
    return client.get<Blob>(url, { responseType: 'blob' })
  },

  // Presigned multipart upload APIs
  initPresignedUpload(data: {
    parentId: number
    fileName: string
    fileMd5: string
    fileSize: number
    totalParts: number
  }) {
    return client.post<InitPresignedUploadReply>('/file/presigned-upload', {
      parent_id: data.parentId,
      file_name: data.fileName,
      file_md5: data.fileMd5,
      file_size: data.fileSize,
      total_parts: data.totalParts,
    })
  },

  reportUploadedPart(data: { sessionId: string; partNumber: number; etag: string; size: number }) {
    return client.post<ReportUploadedPartReply>('/file/presigned-upload/part', {
      session_id: data.sessionId,
      part_number: data.partNumber,
      etag: data.etag,
      size: data.size,
    })
  },

  completePresignedUpload(sessionId: string) {
    return client.post<CompletePresignedUploadReply>('/file/presigned-upload/complete', {
      session_id: sessionId,
    })
  },

  abortPresignedUpload(sessionId: string) {
    return client.post<AbortPresignedUploadReply>('/file/presigned-upload/abort', {
      session_id: sessionId,
    })
  },

  // Storage usage
  getDiskUsage() {
    return client.get<DiskUsageReply>('/disk-usage')
  },
}
