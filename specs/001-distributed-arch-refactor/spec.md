# Feature Specification: Distributed Architecture Refactoring

**Feature Branch**: `001-distributed-arch-refactor`
**Created**: 2026-03-28
**Status**: Draft
**Input**: User description: "分布式架构重构: 1) 并发上传分布式锁(分块级别) + MySQL唯一约束 + uploading状态; 2) 一致性哈希多实例部署; 3) 文件冗余备份; 4) 删除双架构模式,仅使用本地磁盘+MySQL(保留SQLite适配代码)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Concurrent Upload Safety (Priority: P1)

Two users (or one user from two clients) upload the **same large file** (identical MD5 + size) at the same time. Today, without coordination, both processes create duplicate `file_stores` records. After this change, the system MUST guarantee that exactly **one** physical copy is stored, regardless of concurrency, and both users see a successful upload.

Furthermore, if both uploads start around the same time, each client MUST be able to upload **different chunk ranges** of the same file in parallel (cooperative chunking), so the file completes faster instead of one client wasting work.

**Why this priority**: Data corruption and duplicate storage are the most critical correctness issues. The interviewer specifically flagged this as the single biggest architectural gap.

**Independent Test**: Can be fully tested by launching two concurrent upload sessions for the same file and verifying: (a) only one `file_stores` row exists, (b) all chunks are present, (c) both users receive the completed file reference, (d) no deadlocks or partial states remain.

**Acceptance Scenarios**:

1. **Given** User A and User B both initiate `CheckUpload` for the same file (identical MD5 + size), **When** both proceed to upload chunks concurrently, **Then** chunks are distributed across both clients; each chunk is saved exactly once; no duplicate chunk writes occur.
2. **Given** User A has uploaded chunks 1-3 of a 6-chunk file, **When** User B starts uploading the same file, **Then** User B's `CheckUpload` returns chunks 1-3 as already uploaded; User B only uploads chunks 4-6.
3. **Given** both users have uploaded all chunks, **When** `MergeChunks` is called by either user, **Then** exactly one `file_stores` record is created (enforced by unique constraint); the second `MergeChunks` call increments `ref_count` instead of creating a duplicate.
4. **Given** a merge is in progress for a file, **When** a second merge request arrives for the same MD5, **Then** the second request waits (or retries) until the first merge completes, then reuses the result via deduplication.
5. **Given** an upload is in progress (file_stores status is "uploading"), **When** a new user tries to upload the same file, **Then** they join the existing upload session by contributing missing chunks.

---

### User Story 2 - Remove Dual Architecture Mode (Priority: P2)

The system currently supports two deployment modes: Mode A (local disk + SQLite + goroutine MQ) and Mode B (SeaweedFS + MySQL + Kafka). After this change, the system MUST use **local disk only** as primary storage with **MySQL as the default database**, while preserving all SQLite adapter code and configuration for lightweight deployments.

All SeaweedFS references, presigned upload to SeaweedFS, and Kafka-specific infrastructure MUST be removed. The cold migration path to Alibaba Cloud OSS MUST be retained (using goroutine MQ as the async transport).

**Why this priority**: The dual-mode code path adds substantial complexity to every feature. Simplifying to a single architecture reduces the maintenance burden and makes the remaining features (distributed locking, redundancy) easier to implement correctly.

**Independent Test**: Can be tested by deploying the system with MySQL + local disk, uploading/downloading files, verifying cold migration to OSS still works, and confirming that no SeaweedFS/Kafka code paths are reachable.

**Acceptance Scenarios**:

1. **Given** the system is deployed, **When** a file is uploaded, **Then** chunks are saved to local disk and merged to `FILE_STORE_DIR`; no SeaweedFS or S3 API is invoked.
2. **Given** local disk usage exceeds the configured threshold, **When** LRU eviction triggers, **Then** cold files are migrated to Alibaba Cloud OSS via goroutine MQ (not Kafka).
3. **Given** the `DB_DRIVER` environment variable is set to `sqlite`, **When** the service starts, **Then** it uses SQLite; all existing SQLite-related code and tests continue to function.
4. **Given** FreqReview: all SeaweedFS, Kafka, and Mode B code paths have been removed, **When** `go build ./...` is run, **Then** compilation succeeds with no SeaweedFS/Kafka imports remaining.
5. **Given** the Docker Compose file has been updated, **When** `make up` is run, **Then** only Consul, Redis, MySQL, user-service, file-service, gateway, and frontend containers start; no SeaweedFS, Kafka, or file-worker containers exist.

---

### User Story 3 - Consistent Hashing for Multi-Instance File Service (Priority: P3)

The Gateway already implements a consistent hash router (FNV32a, 150 virtual nodes) for routing upload requests by file MD5 to a specific File Service instance. This story enhances the hash router to support the **distributed lock coordination** from Story 1, and ensures all file-related operations (not just uploads) are correctly routed to maintain chunk locality.

**Why this priority**: Multi-instance routing is a prerequisite for horizontal scaling but is already partially implemented. The main gap is ensuring that the hash router integrates with the new distributed locking and that chunk operations remain co-located.

**Independent Test**: Can be tested by running 3 file-service instances behind the gateway, uploading a file, and verifying all chunks are routed to the same instance. Then, killing that instance and verifying the hash ring rebalances so existing uploads can be resumed on a different node (leveraging the distributed lock from Story 1).

**Acceptance Scenarios**:

1. **Given** 3 file-service instances are running, **When** a file upload is initiated, **Then** all chunk uploads for that file are routed to the same instance (determined by file MD5 hash).
2. **Given** one file-service instance goes down, **When** the Consul health check detects the failure, **Then** the hash ring rebalances within 15 seconds; new upload requests for affected files are routed to the next available instance.
3. **Given** a file-service instance rejoins the cluster, **When** the hash ring updates, **Then** subsequent requests for keys originally assigned to that instance resume routing to it.
4. **Given** the consistent hash router receives a `MergeChunks` request, **When** the chunks are distributed across multiple instances (due to failover), **Then** the merge operation collects chunks from all instances and produces a correct merged file.

---

### User Story 4 - File Redundancy Backup (Priority: P4)

The system MUST provide a mechanism for file redundancy so that data is not lost if the primary storage disk fails. The redundancy strategy uses **erasure coding** at the application level: each stored file is split into data + parity shards, allowing recovery even if some shards are lost.

**Why this priority**: Data durability is a long-term concern. While critical, it can be implemented after the architecture is simplified (Story 2) and concurrent upload safety is established (Story 1).

**Independent Test**: Can be tested by storing a file, artificially deleting some shards from disk, and verifying the file can still be read back correctly from the remaining shards.

**Acceptance Scenarios**:

1. **Given** a file has been stored, **When** the system writes the file to disk, **Then** it is encoded into N data shards + M parity shards (configurable, default 4+2) and stored as separate shard files.
2. **Given** a file with 4 data + 2 parity shards, **When** up to 2 shard files are corrupted or deleted, **Then** the system detects the missing shards and reconstructs the file from the remaining shards transparently during download.
3. **Given** erasure coding is enabled, **When** the effective storage overhead is calculated, **Then** it is approximately 1.5x the original file size (with 4+2 configuration).
4. **Given** a small file (less than 1 MB), **When** it is stored, **Then** erasure coding is NOT applied (overhead not justified); the file is stored as a single copy.

---

### Edge Cases

- What happens when the distributed lock service (Redis) is temporarily unreachable during concurrent uploads? → Uploads MUST fail gracefully with a retryable error rather than proceeding without locking.
- What happens when a file-service instance crashes mid-merge? → The uploading status and chunk data MUST be preserved so another instance can retry the merge.
- What happens when disk space is exhausted during merge? → The system MUST return a clear "disk full" error and clean up partial merge artifacts.
- What happens to existing data when migrating from dual-mode to single-mode? → A migration guide MUST be provided; existing SeaweedFS data MUST be manually migrated to local disk or OSS before upgrading.
- What happens when erasure coding shards span a disk failure that exceeds parity capacity? → The system MUST detect unrecoverable corruption and return an explicit error rather than serving corrupted data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST acquire a distributed lock (per file MD5) before writing any chunk to prevent duplicate concurrent writes.
- **FR-002**: System MUST support chunk-level lock granularity, allowing two clients uploading the same file to write to different chunks concurrently.
- **FR-003**: The `file_stores` table MUST enforce a `UNIQUE` constraint on `(file_md5, size)` to prevent duplicate physical storage at the database level.
- **FR-004**: The `file_stores` table MUST include an `upload_status` column with values: `uploading`, `completed`, `failed`. The `MergeChunks` operation MUST transition the status atomically from `uploading` to `completed`.
- **FR-005**: System MUST remove all SeaweedFS-related code, configuration, and dependencies (aws-sdk-go-v2 S3 client, SeaweedFS container, S3-mode code paths).
- **FR-006**: System MUST remove all Kafka-related code, configuration, and dependencies (segmentio/kafka-go, Kafka container, file-worker process).
- **FR-007**: System MUST retain the goroutine-based message queue as the sole async transport for cold migration to OSS.
- **FR-008**: System MUST retain all SQLite adapter code, drivers, and configuration (DB_DRIVER=sqlite support).
- **FR-009**: System MUST default to MySQL as the primary database when DB_DRIVER is unset.
- **FR-010**: The presigned upload flow MUST be simplified to only support OSS direct upload (no SeaweedFS presigning).
- **FR-011**: The consistent hash router MUST route all operations for a given file MD5 (check-upload, save-chunk, merge-chunks) to the same file-service instance.
- **FR-012**: The consistent hash router MUST gracefully handle instance failures by rebalancing the hash ring and allowing chunk uploads to be resumed on a different instance.
- **FR-013**: System MUST implement erasure coding for files larger than a configurable minimum size (default: 1 MB), producing N data + M parity shards.
- **FR-014**: System MUST transparently reconstruct files from available shards during download when some shards are missing or corrupted.
- **FR-015**: The Docker Compose configuration MUST be updated to remove SeaweedFS, Kafka, and file-worker services.
- **FR-016**: All existing unit tests MUST be updated to reflect the single-architecture mode; no test MUST reference removed code paths.
- **FR-017**: System MUST update README.md, AGENTS.md, and all docs/ to reflect the architectural changes.

### Key Entities

- **FileStore**: Represents a unique physical file. Now includes `upload_status` (uploading/completed/failed) and has a unique constraint on `(file_md5, size)`. When erasure coding is enabled, references shard metadata.
- **ChunkLock**: Logical concept — a distributed lock per `(file_md5, chunk_index)` in Redis, ensuring only one writer per chunk at any time.
- **FileLock**: Logical concept — a distributed lock per `file_md5` in Redis, serializing merge operations.
- **ErasureShard**: A shard file produced by erasure coding. Each original file maps to N data shards + M parity shards. Shards are individual files on disk with a naming convention like `{file_md5}.shard.{index}`.
- **UploadSession**: Existing entity, but simplified — now only targets OSS for presigned uploads (no SeaweedFS target option).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Two concurrent uploads of the same 100 MB file complete successfully with exactly one physical copy stored, zero duplicate records, and zero data corruption — verified by 10 repeated trials.
- **SC-002**: Cooperative chunking reduces total upload time for a 1 GB file uploaded by 2 clients to less than 60% of the single-client upload time.
- **SC-003**: The system builds and passes all tests (`go test ./...`) with zero SeaweedFS/Kafka imports after dual-mode removal.
- **SC-004**: Multi-instance deployment (3 file-service instances) handles file uploads correctly with all chunks landing on the consistent-hash-designated instance.
- **SC-005**: Erasure-coded files (4+2 shards) survive the deletion of any 2 shards and are reconstructed correctly during download.
- **SC-006**: System handles 50 concurrent file uploads (different files) across 3 file-service instances without deadlocks or upload failures.
- **SC-007**: Cold migration from local disk to OSS continues to function correctly via goroutine MQ after Kafka removal.

## Assumptions

- Redis is assumed to be available and reachable for distributed locking; if Redis is down, upload operations will fail with a retryable error.
- MySQL is the default database for production deployments; SQLite is retained for development and single-machine deployments.
- Existing users migrating from dual-mode (Mode B) MUST manually move data from SeaweedFS to local disk or OSS before upgrading. The system will not provide automatic data migration.
- The erasure coding library will be a pure Go implementation (e.g., klauspost/reedsolomon) to avoid CGO dependencies beyond SQLite.
- The consistent hash router's 15-second Consul watch interval is acceptable latency for detecting instance failures.
- Frontend changes are minimal — the upload flow already handles both direct and presigned modes; removing SeaweedFS presigning only simplifies the backend response, not the frontend logic.
