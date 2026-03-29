# Data Model: Distributed Architecture Refactoring

**Feature**: `001-distributed-arch-refactor`
**Date**: 2026-03-28

## Entity: FileStore (Modified)

Represents a unique physical file (keyed by content hash).

| Field | Type | Constraints | Change |
|-------|------|-------------|--------|
| id | int64 | PK, auto-increment | - |
| file_md5 | string(32) | NOT NULL | Remove old `uniqueIndex`, add composite unique |
| size | int64 | NOT NULL | Part of new composite unique |
| store_path | string(512) | NOT NULL | - |
| storage_type | string(16) | NOT NULL, default: `local` | Remove `seaweedfs` value; add `local_ec` |
| upload_status | string(16) | NOT NULL, default: `uploading` | **NEW** |
| ref_count | int32 | default: 1 | - |
| last_accessed_at | time | auto-update | - |
| created_at | time | auto-create | - |

**Composite UNIQUE**: `(file_md5, size)` — prevents duplicate physical storage at DB level.

**Valid `storage_type` values**: `local`, `local_ec`, `oss`

**Valid `upload_status` values**: `uploading`, `completed`, `failed`

**State transitions**:
- `CreateStore` → sets `upload_status = uploading`
- `MergeChunks` success → atomically updates `upload_status = completed`
- `MergeChunks` failure → updates `upload_status = failed`
- `CheckUpload` finding `upload_status = completed` → treat as instant-upload hit
- `CheckUpload` finding `upload_status = uploading` → join existing upload (return uploaded chunks)

---

## Entity: ErasureShard (New)

Represents a single shard file produced by Reed-Solomon erasure coding.

| Field | Type | Constraints |
|-------|------|-------------|
| id | int64 | PK, auto-increment |
| file_store_id | int64 | FK to file_stores.id, NOT NULL, INDEX |
| shard_index | int32 | NOT NULL (0-based: 0..N+M-1) |
| shard_path | string(512) | NOT NULL |
| shard_size | int64 | NOT NULL |
| is_parity | bool | NOT NULL (false for data shards, true for parity) |
| checksum | string(32) | MD5 of individual shard for integrity check |
| created_at | time | auto-create |

**Composite UNIQUE**: `(file_store_id, shard_index)`

**Relationship**: One FileStore → Many ErasureShards (when `storage_type = local_ec`)

---

## Entity: UploadSession (Modified)

Simplified for OSS-only presigned uploads.

| Field | Type | Constraints | Change |
|-------|------|-------------|--------|
| id | string(36) | PK (UUID) | - |
| user_id | int64 | INDEX, NOT NULL | - |
| parent_id | int64 | default: 0 | - |
| file_name | string(256) | NOT NULL | - |
| file_md5 | string(32) | INDEX, NOT NULL | - |
| file_size | int64 | NOT NULL | - |
| total_parts | int32 | NOT NULL | - |
| part_size | int64 | NOT NULL | - |
| storage_target | string(16) | NOT NULL | **Always `oss`** now |
| object_key | string(512) | NOT NULL | - |
| s3_upload_id | string(256) | NOT NULL | Now refers to OSS upload ID |
| status | string(16) | NOT NULL, INDEX | - |
| created_at | time | - | - |
| expires_at | time | - | - |

---

## Redis Keys (Modified)

### Chunk Tracking (CHANGED from JSON to SET)

| Key | Type | TTL | Description |
|-----|------|-----|-------------|
| `upload:{fileMD5}:chunks` | SET | 24h | Members are chunk indices (int32). Use SADD to add, SISMEMBER to check, SCARD to count. |

### Distributed Locks (NEW)

| Key | Type | TTL | Description |
|-----|------|-----|-------------|
| `lock:chunk:{fileMD5}:{chunkIndex}` | STRING (UUID) | 60s | Chunk-level write lock. Value = owner UUID for safe release. |
| `lock:merge:{fileMD5}` | STRING (UUID) | 300s | Merge-level lock. Only one merge operation per fileMD5.  |

### Disk Usage (UNCHANGED)

| Key | Type | TTL | Description |
|-----|------|-----|-------------|
| `disk_usage:local` | INT64 | none | Atomic counter for local disk usage |

**Removed key**: `disk_usage:seaweedfs` (no longer tracked after SeaweedFS removal)

---

## Interface Changes (Biz Layer)

### Removed Interfaces
- `biz.ObjectStorage` — was SeaweedFS S3 abstraction

### Modified Interfaces

**`biz.FileRepo`** additions:
```
AcquireChunkLock(ctx, fileMD5, chunkIndex, ownerID) (bool, error)
ReleaseChunkLock(ctx, fileMD5, chunkIndex, ownerID) error
AcquireMergeLock(ctx, fileMD5, ownerID) (bool, error)
ReleaseMergeLock(ctx, fileMD5, ownerID) error
AddUploadedChunk(ctx, fileMD5, chunkIndex) error        // SADD
IsChunkUploaded(ctx, fileMD5, chunkIndex) (bool, error)  // SISMEMBER
CountUploadedChunks(ctx, fileMD5) (int64, error)         // SCARD
CreateStoreWithStatus(ctx, store *FileStore) error       // Creates with upload_status
UpdateStoreStatus(ctx, fileMD5, status) error            // Atomically updates status
FindStoreByMD5AndStatus(ctx, md5, status) (*FileStore, error)
CreateErasureShard(ctx, shard *ErasureShard) error
FindErasureShards(ctx, fileStoreID) ([]*ErasureShard, error)
```

**`biz.FileRepo`** removals:
```
SaveChunkInfo  → replaced by AddUploadedChunk (SADD)
GetUploadedChunks → replaced by individual SISMEMBER + SMEMBERS
```

### Removed from FileUsecase
- `objStore ObjectStorage` field
- `storageCfg.Mode` field (always local now)
- `uploadMergedFile` SeaweedFS branch
- `choosePresignedTarget` SeaweedFS branch

### New in FileUsecase
- `erasureEncoder` configuration (data shards, parity shards, min file size)
- `encodeWithErasure(ctx, mergedPath, fileMD5) error`
- `reconstructFromShards(ctx, fileMD5) (io.ReadCloser, error)`
