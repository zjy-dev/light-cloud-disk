# gRPC Contract Changes: Distributed Architecture Refactoring

**Feature**: `001-distributed-arch-refactor`

## file/v1/file.proto Changes

### Modified RPCs

#### CheckUpload (Response change)

The response gains an `upload_status` field to indicate if another client is already uploading.

```diff
 message CheckUploadReply {
   bool can_fast_upload = 1;
   repeated int32 uploaded_chunks = 2;
   bool disk_full = 3;
   string upload_mode = 4;
+  string upload_status = 5;  // "none" | "uploading" | "completed"
 }
```

#### MergeChunks (Behavior change)

No proto change. Behavior changes:
- MUST acquire `lock:merge:{fileMD5}` before proceeding.
- If `file_stores` already has `upload_status=completed` for this MD5, skip merge and increment ref_count.
- If `upload_status=uploading`, wait for lock then re-check.

#### SaveChunk (Behavior change)

No proto change. Behavior changes:
- MUST acquire `lock:chunk:{fileMD5}:{chunkIndex}` before writing.
- Uses SADD instead of JSON array for chunk tracking.

### Removed RPCs (SeaweedFS presigned upload simplification)

No RPCs are removed. The 4 presigned upload RPCs remain but are simplified internally:
- `InitPresignedUpload`: `storage_target` always returns `oss` (never `seaweedfs`).
- `CompletePresignedUpload`: Only invokes `cloudStore` (OSS), never `objStore` (removed).

### New message types

```protobuf
// ErasureShardInfo represents a single shard for file redundancy display
message ErasureShardInfo {
  int32 shard_index = 1;
  bool is_parity = 2;
  bool is_healthy = 3;  // false if shard file is missing/corrupted
}
```

### GetDownloadURL (Behavior change)

No proto change. Behavior changes:
- `storage_type = "local_ec"`: System reconstructs file from shards transparently, then serves via streaming.
- `storage_type = "seaweedfs"`: **Removed** — returns error if encountered (stale data from pre-migration).

## HTTP API Contract Changes (Gateway)

### No new endpoints.

### Removed behavior

| Endpoint | Change |
|----------|--------|
| All presigned upload endpoints | `storage_target` field in responses is always `oss` |
| `GET /api/v1/file/download/:file_id` | No longer returns SeaweedFS presigned URLs |

### Response format changes

`POST /api/v1/file/check-upload` response now includes:

```json
{
  "can_fast_upload": false,
  "uploaded_chunks": [0, 1, 2],
  "disk_full": false,
  "upload_mode": "direct",
  "upload_status": "uploading"
}
```

The `upload_status` field tells the client:
- `"none"` — no existing upload for this file, proceed normally.
- `"uploading"` — another client is uploading this file; contribute missing chunks.
- `"completed"` — file already exists (equivalent to `can_fast_upload=true` but via status path).
