export interface UserInfo {
  id: number
  username: string
  nickname: string
  email: string
  avatar: string
  storageUsed: number
  storageLimit: number
  createdAt: number
}

export interface LoginReply {
  token: string
  expireAt: number
  user: UserInfo
}

export interface RegisterReply {
  userId: number
  username: string
}

export interface FileInfo {
  id: number
  name: string
  fileMd5: string
  size: number
  isFolder: boolean
  parentId: number
  path: string
  createdAt: number
  updatedAt: number
}

export interface TrashFileInfo {
  id: number
  name: string
  size: number
  isFolder: boolean
  deletedAt: number
  expireAt: number
}

export interface ChunkAssignment {
  chunkIndex: number
  targetAddr: string
  uploadUrl: string
}

export interface UploadPlan {
  totalChunks: number
  chunkSize: number
  assignments: ChunkAssignment[]
}

export interface CheckUploadReply {
  canFastUpload: boolean
  uploadedChunks: number[]
  diskFull?: boolean
  uploadMode?: 'direct' | 'presigned'
  uploadStatus?: string
  uploadPlan?: UploadPlan
}

export interface CompleteUploadReply {
  success: boolean
  file: FileInfo
}

export interface ChunkLocation {
  chunkIndex: number
  chunkSize: number
  downloadUrl: string
  checksum: string
  backupUrls?: string[]
}

export interface DownloadPlanReply {
  fileName: string
  fileMd5: string
  fileSize: number
  totalChunks: number
  chunks: ChunkLocation[]
}

// ---- Presigned multipart upload ----

export interface PresignedPartInfo {
  partNumber: number
  uploadUrl: string
}

export interface UploadedPartInfo {
  partNumber: number
  etag: string
}

export interface InitPresignedUploadReply {
  sessionId: string
  pendingParts: PresignedPartInfo[]
  completedParts: UploadedPartInfo[]
  storageTarget: string
  partSize: number
  canFastUpload: boolean
  file?: FileInfo
}

export interface ReportUploadedPartReply {
  success: boolean
}

export interface CompletePresignedUploadReply {
  success: boolean
  file: FileInfo
}

export interface AbortPresignedUploadReply {
  success: boolean
}

export interface CreateShareReply {
  shareId: string
  shareUrl: string
  password: string
  expireAt: number
}

export interface GetShareReply {
  file: FileInfo
  downloadUrl: string
}

export interface ListFilesReply {
  files: FileInfo[]
  total: number
}

export interface ListTrashReply {
  files: TrashFileInfo[]
  total: number
}

export interface SearchFilesReply {
  files: FileInfo[]
  total: number
}

export interface GetDownloadURLReply {
  downloadUrl: string
  fileName: string
}

export interface DiskUsageReply {
  primaryUsedBytes: number
  primaryMaxBytes: number
  primaryType: string
}

export type ViewMode = 'grid' | 'list'
export type SortField = 'name' | 'size' | 'updatedAt'
export type SortOrder = 'asc' | 'desc'
