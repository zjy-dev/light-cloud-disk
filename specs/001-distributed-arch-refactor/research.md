# Research: Distributed Architecture Refactoring

**Feature**: `001-distributed-arch-refactor`
**Date**: 2026-03-28

## R1: Distributed Locking Strategy

**Decision**: Use Redis `SET NX EX` (SetNX in go-redis/v8) for distributed locks at two levels:
1. **Chunk-level lock**: Key `lock:chunk:{fileMD5}:{chunkIndex}`, TTL 60s. Allows cooperative concurrent chunk uploads.
2. **Merge-level lock**: Key `lock:merge:{fileMD5}`, TTL 300s. Serializes merge operations for the same file.

**Rationale**:
- The project already uses go-redis/v8 and has a Redis dependency. Adding a separate Redlock library (e.g., go-redsync) would add dependency complexity for a single-Redis deployment.
- `SetNX` with TTL guarantees mutual exclusion with automatic expiry (no zombie locks).
- For a single Redis node (which this project uses), `SET NX EX` is sufficient. Redlock is only needed for multi-master Redis setups.

**Alternatives considered**:
1. **go-redsync/redsync** (Redlock algorithm): Overkill for single-Redis deployment; adds 3 extra dependencies.
2. **MySQL advisory locks (GET_LOCK)**: Works but ties lock lifetime to DB connections; harder to implement TTL; not portable to SQLite.
3. **File-system locks (flock)**: Only works on single-instance deployment; defeats the purpose.

**Lock release pattern**: Use Lua script for safe unlock (only release if value matches owner UUID), preventing accidental release by a different process.

```lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
```

## R2: Chunk Record Migration from JSON Blob to Redis SET

**Decision**: Migrate chunk tracking from a single Redis JSON key (`upload:{md5}:chunks → [0,1,2,3]`) to Redis SET (`upload:{md5}:chunks` using SADD/SISMEMBER).

**Rationale**:
- Current implementation: `GetUploadedChunks` reads the entire JSON array, `SaveChunkInfo` reads + appends + writes the full array. This is NOT concurrent-safe (two writers can overwrite each other).
- Redis SET with `SADD` is atomic per member. Multiple processes adding different chunk indices never interfere.
- `SISMEMBER` checks if a specific chunk exists in O(1).
- `SCARD` returns total count for completion check.

**Alternatives considered**:
1. **Redis bitmap**: Better memory efficiency for large chunk counts, but harder to iterate and less readable for debugging.
2. **MySQL table per chunk**: More durable but introduces DB round-trip per chunk write; too slow for 5MB chunk granularity.

## R3: Erasure Coding Library

**Decision**: Use `github.com/klauspost/reedsolomon` (pure Go, no CGO).

**Rationale**:
- Most popular Go erasure coding library (6k+ stars), actively maintained by the author of many Go standard library optimizations.
- Pure Go — no additional CGO dependencies (the project already has CGO for SQLite, but keeping erasure coding CGO-free simplifies things).
- Supports streaming API (`NewStream`) for large files without loading everything into memory.
- Default configuration: **4 data shards + 2 parity shards** (can tolerate loss of any 2 shards, 1.5x storage overhead).

**Implementation pattern**:
1. After merge, if file > 1MB: encode into 6 shard files stored as `{md5}.shard.{0..5}`.
2. On download: check if shard files exist → if any missing, reconstruct with remaining shards → serve.
3. `file_stores.storage_type` gains new value: `"local_ec"` (local + erasure coded).
4. Shard metadata stored in new `erasure_shards` table.

**Alternatives considered**:
1. **Simple replication (3 copies)**: 3x storage overhead vs 1.5x; wasteful for a cost-conscious self-hosted project.
2. **RAID-level redundancy**: Requires OS/hardware support; not portable across deployments.
3. **ZFS/Btrfs RAID-Z**: Filesystem-level solution; not all deployments run these filesystems.

## R4: Dual-Mode Removal Scope

**Decision**: Remove the following, preserve the rest:

| Remove | Preserve |
|--------|----------|
| `data/seaweedfs.go` (ObjectStorage impl) | `data/oss.go` (CloudStorage impl) |
| `data/kafka.go` (kafkaProducer impl) | `data/mq_goroutine.go` (goroutineMQ impl) |
| `app/file/cmd/worker/` (entire directory) | SQLite driver + DB_DRIVER config |
| `biz.ObjectStorage` interface | `biz.CloudStorage` interface |
| `StorageSeaweedFS` constant + all mode branches | `StorageLocal` + `StorageOSS` constants |
| `ModeS3` constant + all mode branches | `StorageConfig.Mode` field (simplified) |
| SeaweedFS env vars + config | OSS env vars + config |
| Kafka env vars + config | goroutine MQ (now sole MQ) |
| `docker-compose.yml` services: mysql (s3 profile), kafka, seaweedfs, file-worker | mysql service moved out of s3 profile → always available |
| `.env.s3` file | `.env.local` (renamed to `.env.example`) |
| `segmentio/kafka-go` go.mod dep | All other deps |
| `aws-sdk-go-v2` go.mod deps | `alibabacloud-oss-go-sdk-v2` |
| Presigned multipart to SeaweedFS | Presigned multipart to OSS |
| `make images-all`, `make run-worker` | All other Makefile targets |

**Key: `biz.ObjectStorage` vs `biz.CloudStorage`**:
- `ObjectStorage` was for SeaweedFS (warm tier) → **remove entirely**.
- `CloudStorage` was for OSS (cold tier) → **keep**, but also use for presigned uploads when disk is full.
- After removal, `FileUsecase` no longer needs `objStore ObjectStorage` field.

## R5: MySQL as Default with UNIQUE Constraint Impact

**Decision**: Change `FileStorePO.FileMD5` from `uniqueIndex` to a composite unique index `UNIQUE(file_md5, size)`.

**Current state**: `FileMD5` already has a `uniqueIndex` tag in GORM. This is STRICTER than what we need (two different-sized files could theoretically share an MD5 — astronomically unlikely but possible).

**Post-change**: The UNIQUE constraint on `(file_md5, size)` serves as a database-level safety net for the application-level distributed lock. If the lock fails (Redis crash between lock acquire and DB insert), the DB constraint prevents duplicates.

**SQLite compatibility**: Both MySQL and SQLite support composite UNIQUE constraints via GORM tags.

## R6: Upload Status State Machine

**Decision**: Add `UploadStatus` column to `file_stores` with these transitions:

```
     ┌──────────────────────────────────────┐
     │         CheckUpload / MergeStart     │
     ▼                                      │
 (not exists) ──CreateStore──→ uploading ───┘
                                  │
                    MergeChunks   │
                    success       │    failure
                      │          │        │
                      ▼          │        ▼
                  completed      │     failed
                      │          │
                      └──────────┘
```

- `uploading`: File store record created, chunks being uploaded/merged.
- `completed`: Merge finished, file is ready for download.
- `failed`: Merge or storage write failed; row can be cleaned up or retried.

When a second user calls `CheckUpload` and finds a `file_stores` record with `upload_status=uploading`, they join the existing chunk upload rather than creating a new one.
