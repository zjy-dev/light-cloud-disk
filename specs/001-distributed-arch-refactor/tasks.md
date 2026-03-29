# Tasks: Distributed Architecture Refactoring

**Input**: Design documents from `/specs/001-distributed-arch-refactor/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Test tasks included (Constitution Principle IV: Test-First Discipline requires unit tests for all new biz-layer logic).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Backend Go**: `app/file/internal/`, `app/gateway/internal/`, `api/file/v1/`
- **Frontend**: `frontend/src/`
- **Config**: project root (`docker-compose.yml`, `Makefile`, `.env.*`, `Dockerfile`)
- **Docs**: `docs/`, `README.md`, `AGENTS.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new dependency, update proto, and prepare shared data model changes that all stories depend on.

- [X] T001 Add `github.com/klauspost/reedsolomon` dependency to `go.mod` and run `go mod vendor`
- [X] T002 Update `api/file/v1/file.proto`: add `upload_status` field to `CheckUploadReply`, add `ErasureShardInfo` message
- [X] T003 Regenerate proto code: run `make api` to produce `api/file/v1/file.pb.go` and `api/file/v1/file_grpc.pb.go`
- [X] T004 Update `app/file/internal/conf/conf.proto`: remove SeaweedFS/Kafka config blocks, add `ErasureCoding` config (data_shards, parity_shards, min_file_size)
- [X] T005 Regenerate config proto: run `make conf` to produce `app/file/internal/conf/conf.pb.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core schema and interface changes that MUST be complete before ANY user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Modify `FileStorePO` in `app/file/internal/data/file.go`: add `UploadStatus` field, change `FileMD5` from `uniqueIndex` to composite unique `(file_md5, size)`, change `StorageType` default from `seaweedfs` to `local`
- [X] T007 Add `ErasureShardPO` model in `app/file/internal/data/file.go`: define struct with `file_store_id`, `shard_index`, `shard_path`, `shard_size`, `is_parity`, `checksum`, composite unique `(file_store_id, shard_index)`
- [X] T008 Update auto-migration in `app/file/internal/data/data.go`: add `ErasureShardPO` to `db.AutoMigrate()` call
- [X] T009 Add new `FileRepo` interface methods in `app/file/internal/biz/file.go`: `AcquireChunkLock`, `ReleaseChunkLock`, `AcquireMergeLock`, `ReleaseMergeLock`, `AddUploadedChunk`, `IsChunkUploaded`, `CountUploadedChunks`, `CreateStoreWithStatus`, `UpdateStoreStatus`, `FindStoreByMD5AndStatus`, `CreateErasureShard`, `FindErasureShards`
- [X] T010 Remove `SaveChunkInfo` and `GetUploadedChunks` from `FileRepo` interface in `app/file/internal/biz/file.go` (replaced by new SET-based methods)
- [X] T011 Add `UploadStatus` field to `FileStore` biz entity and `ErasureShard` biz entity in `app/file/internal/biz/file.go`
- [X] T012 Implement new `FileRepo` Redis methods in `app/file/internal/data/file.go`: `AcquireChunkLock` (SetNX + TTL 60s), `ReleaseChunkLock` (Lua script), `AcquireMergeLock` (SetNX + TTL 300s), `ReleaseMergeLock` (Lua script)
- [X] T013 Implement new `FileRepo` Redis SET methods in `app/file/internal/data/file.go`: replace JSON-based `SaveChunkInfo`/`GetUploadedChunks` with `AddUploadedChunk` (SADD), `IsChunkUploaded` (SISMEMBER), `CountUploadedChunks` (SCARD), update `GetUploadedChunks` to use SMEMBERS
- [X] T014 Implement new `FileRepo` GORM methods in `app/file/internal/data/file.go`: `CreateStoreWithStatus`, `UpdateStoreStatus`, `FindStoreByMD5AndStatus`, `CreateErasureShard`, `FindErasureShards`

**Checkpoint**: Foundation ready — biz interfaces defined, data layer implements new methods. User story implementation can now begin.

---

## Phase 3: User Story 2 — Remove Dual Architecture Mode (Priority: P2) 🎯 MVP

**Goal**: Strip all SeaweedFS, Kafka, Mode B code. Simplify to local disk + MySQL (default) + goroutine MQ + OSS cold migration. This is ordered FIRST because all subsequent stories (US1, US3, US4) are easier to implement on the simplified codebase.

**Independent Test**: `go build ./...` succeeds with zero SeaweedFS/Kafka imports; `go test ./...` passes; uploading and downloading files works on local disk; cold migration to OSS via goroutine MQ works.

### Implementation for User Story 2

- [X] T015 [US2] Remove `biz.ObjectStorage` interface and all SeaweedFS-related constants (`StorageSeaweedFS`, `ModeS3`) from `app/file/internal/biz/file.go`
- [X] T016 [US2] Remove `objStore ObjectStorage` field from `FileUsecase` struct and `NewFileUsecase` constructor in `app/file/internal/biz/file.go`
- [X] T017 [US2] Simplify `StorageConfig` in `app/file/internal/biz/file.go`: remove `Mode` field branching, `primaryDiskType()` now always returns `"local"`
- [X] T018 [US2] Simplify `CheckUpload` in `app/file/internal/biz/file.go`: remove `ModeS3` presigned branch, always use direct upload mode (presigned only when disk full → OSS)
- [X] T019 [US2] Simplify `uploadMergedFile` in `app/file/internal/biz/file.go`: remove SeaweedFS branch, keep only local disk + OSS fallback
- [X] T020 [US2] Simplify `choosePresignedTarget` in `app/file/internal/biz/file.go`: always return `StorageOSS` (remove SeaweedFS option)
- [X] T021 [US2] Simplify presigned upload methods in `app/file/internal/biz/file.go`: remove all `session.StorageTarget == StorageSeaweedFS` branches from `InitPresignedUpload`, `CompletePresignedUpload`, `AbortPresignedUpload`, `generatePresignedURLs`, `abortS3Upload`
- [X] T022 [US2] Simplify `GetDownloadURL` in `app/file/internal/biz/file.go`: remove `StorageSeaweedFS` case from switch
- [X] T023 [US2] Simplify `maybeEvictToCloud` in `app/file/internal/biz/file.go`: remove SeaweedFS primary type branch
- [X] T024 [US2] Delete `app/file/internal/data/seaweedfs.go`
- [X] T025 [US2] Delete `app/file/internal/data/kafka.go`
- [X] T026 [US2] Update `ProviderSet` in `app/file/internal/data/data.go`: remove `NewSeaweedFSClient` from wire set
- [X] T027 [US2] Simplify `NewMessageProducer` in `app/file/internal/data/mq_goroutine.go`: always create goroutine MQ (remove Kafka broker detection logic if any)
- [X] T028 [US2] Delete `app/file/cmd/worker/` directory (entire Kafka consumer process)
- [X] T029 [US2] Update `app/file/cmd/main.go`: remove any worker-related references
- [X] T030 [US2] Update `app/file/cmd/wire.go`: remove SeaweedFS and ObjectStorage providers
- [X] T031 [US2] Regenerate Wire: run `make wire` to produce `app/file/cmd/wire_gen.go`
- [X] T032 [US2] Remove `aws-sdk-go-v2` and `segmentio/kafka-go` from `go.mod`, run `go mod tidy && go mod vendor`
- [X] T033 [US2] Update `app/file/internal/service/file.go`: remove any SeaweedFS-specific response logic
- [X] T034 [US2] Update `app/gateway/internal/handler/file.go`: remove SeaweedFS presigned path handling
- [X] T035 [US2] Update `app/file/internal/data/file.go`: change `FileStorePO.StorageType` default from `"seaweedfs"` to `"local"`
- [X] T036 [US2] Update unit tests in `app/file/internal/biz/file_test.go`: remove all mock expectations for `ObjectStorage` interface, remove SeaweedFS/ModeS3 test branches
- [X] T037 [US2] Update unit tests in `app/gateway/internal/handler/handler_test.go`: remove SeaweedFS presigned test cases
- [X] T038 [US2] Verify build: run `go build ./...` and confirm zero SeaweedFS/Kafka imports
- [X] T039 [US2] Verify tests: run `go test ./...` and confirm all pass

**Checkpoint**: Single-architecture mode complete. All SeaweedFS/Kafka code removed. Build and tests pass.

---

## Phase 4: User Story 1 — Concurrent Upload Safety (Priority: P1)

**Goal**: Add distributed locking at chunk-level and merge-level, upload_status tracking, MySQL UNIQUE constraint enforcement, and cooperative chunking so duplicate uploads are prevented and concurrent clients can collaborate.

**Independent Test**: Launch 2 concurrent upload sessions for the same file; verify exactly 1 `file_stores` row, all chunks present, both users get successful upload, no deadlocks.

### Tests for User Story 1

- [X] T040 [P] [US1] Write unit tests for `AcquireChunkLock`/`ReleaseChunkLock` in `app/file/internal/biz/file_test.go`: test lock acquisition, contention, TTL expiry, safe release with owner ID
- [X] T041 [P] [US1] Write unit tests for `AcquireMergeLock`/`ReleaseMergeLock` in `app/file/internal/biz/file_test.go`: test merge serialization, lock wait/retry, dedup after lock acquired
- [X] T042 [P] [US1] Write unit tests for `CheckUpload` with `upload_status` in `app/file/internal/biz/file_test.go`: test joining existing upload, returning uploaded chunks via SET methods
- [X] T043 [P] [US1] Write unit tests for `SaveChunk` with distributed lock in `app/file/internal/biz/file_test.go`: test lock acquire before write, lock release after write, lock contention error
- [X] T044 [P] [US1] Write unit tests for `MergeChunks` with merge lock + dedup in `app/file/internal/biz/file_test.go`: test lock acquisition, status transition uploading→completed, second merge reuses result

### Implementation for User Story 1

- [X] T045 [US1] Modify `CheckUpload` in `app/file/internal/biz/file.go`: add `FindStoreByMD5AndStatus` check for `uploading`/`completed` states, return `upload_status` field, use `GetUploadedChunks` (SMEMBERS) for joined uploads
- [X] T046 [US1] Modify `SaveChunk` in `app/file/internal/biz/file.go`: acquire chunk lock before write (`AcquireChunkLock`), check `IsChunkUploaded` to skip duplicates, use `AddUploadedChunk` (SADD), release lock on completion
- [X] T047 [US1] Modify `MergeChunks` in `app/file/internal/biz/file.go`: acquire merge lock (`AcquireMergeLock`), re-check dedup with `FindStoreByMD5AndStatus(completed)`, create store with `upload_status=uploading` via `CreateStoreWithStatus`, transition to `completed` after merge, release lock
- [X] T048 [US1] Add `CreateStoreWithStatus` logic in `MergeChunks`: handle MySQL UNIQUE constraint violation gracefully (catch duplicate key error → increment ref_count instead)
- [X] T049 [US1] Update `app/file/internal/service/file.go`: pass `upload_status` in `CheckUploadReply` response
- [X] T050 [US1] Update `app/gateway/internal/handler/file.go`: forward `upload_status` field in CheckUpload HTTP response
- [X] T051 [US1] Update frontend types in `frontend/src/types/index.ts`: add `upload_status` field to `CheckUploadResponse` type
- [X] T052 [US1] Update `frontend/src/composables/useUpload.ts`: handle `upload_status === "uploading"` to show cooperative upload state to user
- [X] T053 [US1] Verify tests: run `go test ./app/file/internal/biz/...` and confirm new lock/dedup tests pass

**Checkpoint**: Concurrent upload safety complete. Two clients uploading the same file are coordinated via distributed locks, exactly one `file_stores` record is created.

---

## Phase 5: User Story 3 — Consistent Hashing for Multi-Instance File Service (Priority: P3)

**Goal**: Enhance the existing hash router to route all file operations (not just uploads) by MD5, handle instance failover gracefully with the distributed locks from US1.

**Independent Test**: Run 3 file-service instances, upload a file, verify chunks routed to same instance. Kill instance, verify ring rebalances and upload resumes on another node.

### Tests for User Story 3

- [X] T054 [P] [US3] Write unit tests for enhanced `hashRouter.pick()` in `app/gateway/internal/client/client_test.go`: test consistent routing by MD5, ring rebalance on node removal/addition

### Implementation for User Story 3

- [X] T055 [US3] Enhance `FileClientByKey` usage in `app/gateway/internal/handler/file.go`: ensure all file-MD5-keyed operations (check-upload, save-chunk, merge-chunks, download) use `FileClientByKey(fileMD5)` instead of default `File` client
- [X] T056 [US3] Add health check integration in `app/gateway/internal/client/client.go`: log ring rebalance events, add metric for active instance count
- [X] T057 [US3] Add `MergeChunks` cross-instance chunk collection in `app/file/internal/biz/file.go`: when chunks are on local disk and a merge request arrives at a different instance (post-failover), check if all chunk files exist locally, if not return a clear error guiding retry
- [X] T058 [US3] Verify multi-instance routing: run `go test ./app/gateway/...` and confirm hash router tests pass

**Checkpoint**: Consistent hashing routes all MD5-keyed operations to the same instance. Failover triggers rebalance, distributed locks from US1 prevent data corruption during transitions.

---

## Phase 6: User Story 4 — File Redundancy Backup (Priority: P4)

**Goal**: Implement Reed-Solomon erasure coding (4+2 shards by default) for files > 1 MB, with transparent reconstruction on download.

**Independent Test**: Store a file > 1 MB, verify 6 shard files on disk. Delete 2 shards, download the file and verify it is reconstructed correctly.

### Tests for User Story 4

- [ ] T059 [P] [US4] Write unit tests for `encodeWithErasure` in `app/file/internal/biz/file_test.go`: test shard generation for files > 1 MB, test skipping for files < 1 MB
- [ ] T060 [P] [US4] Write unit tests for `reconstructFromShards` in `app/file/internal/biz/file_test.go`: test reconstruction with missing shards, test failure when too many shards missing

### Implementation for User Story 4

- [ ] T061 [P] [US4] Create `app/file/internal/data/erasure.go`: implement Reed-Solomon encode/decode using `klauspost/reedsolomon`, streaming API for large files, shard file I/O (`{md5}.shard.{index}`)
- [ ] T062 [US4] Add erasure coding config to `FileUsecase` in `app/file/internal/biz/file.go`: add `ErasureConfig` struct (DataShards, ParityShards, MinFileSize), read from conf.proto config
- [ ] T063 [US4] Implement `encodeWithErasure` in `app/file/internal/biz/file.go`: after merge, if file > MinFileSize, encode into shards, save shard metadata via `CreateErasureShard`, set `storage_type = "local_ec"`
- [ ] T064 [US4] Integrate erasure encoding into `MergeChunks` in `app/file/internal/biz/file.go`: call `encodeWithErasure` after successful merge and before setting status to `completed`
- [ ] T065 [US4] Implement `reconstructFromShards` in `app/file/internal/biz/file.go`: check shard integrity, reconstruct from available shards if any are missing, return `io.ReadCloser` for streaming
- [ ] T066 [US4] Update `OpenLocalFile` in `app/file/internal/biz/file.go`: when `storage_type == "local_ec"`, use `reconstructFromShards` instead of direct file open
- [ ] T067 [US4] Update `GetDownloadURL` in `app/file/internal/biz/file.go`: handle `"local_ec"` storage type (return `local://` URL so gateway streams via shard reconstruction)
- [ ] T068 [US4] Add erasure config env vars to `app/file/cmd/main.go`: read `ERASURE_DATA_SHARDS`, `ERASURE_PARITY_SHARDS`, `ERASURE_MIN_FILE_SIZE` from environment
- [ ] T069 [US4] Verify tests: run `go test ./app/file/internal/biz/...` and confirm erasure coding tests pass

**Checkpoint**: Erasure coding operational. Files > 1 MB are split into 4+2 shards. Loss of up to 2 shards is recoverable transparently during download.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Infrastructure, documentation, CI/CD updates that affect the entire project.

### Infrastructure

- [ ] T070 [P] Update `docker-compose.yml`: remove SeaweedFS, Kafka, file-worker services; move MySQL out of s3 profile to default; remove s3 profile entirely
- [ ] T071 [P] Update `Makefile`: remove `run-worker`, `image-worker`, `images-all` targets; add `ERASURE_*` env var defaults
- [ ] T072 [P] Update `Dockerfile`: remove `worker` build path from `SERVICE` build-arg options
- [ ] T073 [P] Update `.env.example`: remove SeaweedFS/Kafka vars, add `ERASURE_DATA_SHARDS=4`, `ERASURE_PARITY_SHARDS=2`, `ERASURE_MIN_FILE_SIZE=1048576`
- [ ] T074 [P] Update `.env.local`: align with new single-mode defaults
- [ ] T075 Delete `.env.s3` file
- [ ] T076 [P] Update `.github/workflows/ci.yml`: remove worker image build step, remove `--profile s3` compose test
- [ ] T077 [P] Update `.github/workflows/release.yml`: remove worker binary from release artifacts

### Documentation

- [X] T078 [P] Create `docs/distributed-locking.md`: document chunk-level and merge-level Redis locking strategy, Lua unlock script, cooperative chunking
- [X] T079 [P] Create `docs/erasure-coding.md`: document Reed-Solomon 4+2 configuration, shard naming, reconstruction flow, storage overhead
- [X] T080 [P] Update `docs/architecture.md`: remove dual-mode architecture, add distributed locking and erasure coding to architecture diagram
- [X] T081 [P] Update `docs/message-queue.md`: remove Kafka sections, document goroutine MQ as sole implementation
- [X] T082 [P] Update `docs/containerization.md`: remove worker/kafka/seaweedfs container docs
- [X] T083 [P] Update `docs/presigned-upload.md`: document OSS-only presigned upload flow
- [X] T084 [P] Update `docs/cold-hot-storage.md`: remove SeaweedFS tier, update to local → OSS only
- [X] T085 Delete or rename `docs/dual-mode-storage.md` to `docs/storage.md` reflecting single-mode
- [X] T086 Update `README.md`: remove dual-mode references, add distributed locking and erasure coding feature descriptions, update architecture diagram, update service list
- [X] T087 Update `AGENTS.md`: update architecture section, service list, environment variables table, test counts, remove all dual-mode references

### Constitution Amendment

- [X] T088 Update `.specify/memory/constitution.md`: rewrite Principle II from "Dual-Mode Portability" to "Single-Mode with DB Flexibility" (local disk + MySQL default, SQLite optional); bump version to 2.0.0

### Final Validation

- [X] T089 Run full backend test suite: `go test -v -race ./...` — all tests must pass
- [X] T090 Run frontend test suite: `cd frontend && pnpm test:run` — all tests must pass
- [X] T091 Verify build: `go build ./...` for all services (user, file, gateway)
- [X] T092 Run quickstart.md validation scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **US2 (Phase 3)**: Depends on Phase 2 — **execute FIRST** because all other stories are easier on the simplified codebase
- **US1 (Phase 4)**: Depends on Phase 2 + Phase 3 (cleaner to implement after dual-mode removal)
- **US3 (Phase 5)**: Depends on Phase 2 + Phase 4 (needs distributed locks from US1 for failover safety)
- **US4 (Phase 6)**: Depends on Phase 2 + Phase 3 (needs simplified storage layer)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

```
Phase 1 (Setup)
     │
     ▼
Phase 2 (Foundational)
     │
     ▼
Phase 3 (US2: Remove Dual Mode) ← Execute first to simplify codebase
     │
     ├──────────┐
     ▼          ▼
Phase 4 (US1)  Phase 6 (US4) ← Can run in parallel
     │
     ▼
Phase 5 (US3) ← Depends on US1 for distributed lock integration
     │
     ▼
Phase 7 (Polish)
```

### Within Each User Story

- Tests → Models/Interfaces → Biz logic → Service layer → Gateway/Frontend
- Core implementation before integration
- Story complete → run tests → proceed to next

### Parallel Opportunities

**Phase 2**: T012 and T013 and T014 can all run in parallel (different methods in same file, but logically independent)

**Phase 3 (US2)**: T024/T025 (deletions) can run in parallel; T015-T023 (biz layer simplification) are sequential within the file

**Phase 4 (US1)**: T040-T044 (tests) can all run in parallel; T051/T052 (frontend) can run in parallel with backend work

**Phase 6 (US4)**: T059/T060 (tests) can run in parallel with T061 (erasure data layer)

**Phase 7**: T070-T077 (infra) all run in parallel; T078-T087 (docs) all run in parallel

---

## Parallel Example: User Story 1

```bash
# Launch all tests for US1 together:
Task T040: "Unit tests for AcquireChunkLock/ReleaseChunkLock in biz/file_test.go"
Task T041: "Unit tests for AcquireMergeLock/ReleaseMergeLock in biz/file_test.go"
Task T042: "Unit tests for CheckUpload with upload_status in biz/file_test.go"
Task T043: "Unit tests for SaveChunk with distributed lock in biz/file_test.go"
Task T044: "Unit tests for MergeChunks with merge lock in biz/file_test.go"

# Launch frontend tasks in parallel with backend:
Task T051: "Update frontend types in frontend/src/types/index.ts"
Task T052: "Update useUpload composable in frontend/src/composables/useUpload.ts"
```

---

## Implementation Strategy

### MVP First (US2 → US1)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: Foundational (T006-T014)
3. Complete Phase 3: US2 — Remove Dual Mode (T015-T039)
4. **STOP and VALIDATE**: Build passes, tests pass, single-mode works
5. Complete Phase 4: US1 — Concurrent Upload Safety (T040-T053)
6. **STOP and VALIDATE**: Concurrent upload test scenario passes

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US2 (Remove dual-mode) → Test → **First milestone: simplified architecture**
3. US1 (Distributed locking) → Test → **Second milestone: concurrent safety**
4. US3 (Consistent hashing) → Test → **Third milestone: multi-instance ready**
5. US4 (Erasure coding) → Test → **Fourth milestone: data redundancy**
6. Polish → **Final milestone: docs, CI, constitution updated**

### Parallel Team Strategy

With 2 developers after US2 is complete:
- **Developer A**: US1 (distributed locking) → US3 (consistent hashing)
- **Developer B**: US4 (erasure coding) → Polish (docs/infra)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US2 is ordered BEFORE US1 intentionally — removing dual-mode first simplifies all subsequent work
- US3 depends on US1 because consistent hashing failover relies on distributed locks
- US4 is independent of US1/US3 and can be parallelized
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
