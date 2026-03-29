# Implementation Plan: Distributed Architecture Refactoring

**Branch**: `001-distributed-arch-refactor` | **Date**: 2026-03-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-distributed-arch-refactor/spec.md`

## Summary

Refactor the cloud disk system based on interviewer feedback to address four architectural gaps: (1) add distributed locking at chunk-level granularity for concurrent upload safety, with MySQL UNIQUE constraints and upload_status tracking; (2) remove the dual-mode architecture (SeaweedFS + Kafka + Mode B), simplifying to local disk + MySQL + goroutine MQ while preserving SQLite and OSS cold migration; (3) enhance the consistent hash router for multi-instance coordination with distributed locks; (4) implement Reed-Solomon erasure coding for file redundancy backup.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Kratos v2.9.2, Gin v1.12, GORM v1.30, go-redis/v8, klauspost/reedsolomon (NEW), alibabacloud-oss-go-sdk-v2
**Storage**: MySQL 8.0 (default) / SQLite (dev), Redis 7, local disk, Alibaba Cloud OSS
**Testing**: `go test` (101+ unit tests, testify/mock), Vitest (50+ frontend tests)
**Target Platform**: Linux server (amd64/arm64), containerized via Podman/Docker
**Project Type**: Microservices web application (Monorepo)
**Performance Goals**: 50 concurrent file uploads across 3 instances; cooperative chunking < 60% single-client time
**Constraints**: Vendor-locked offline build; CGO_ENABLED=1 for SQLite; single Redis node
**Scale/Scope**: Personal/small-team cloud storage; ~100 concurrent users

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clean Architecture | ✅ PASS | Distributed locking added as `FileRepo` interface methods (data layer implements via Redis). Erasure coding logic in biz layer, shard storage in data layer. |
| II. Dual-Mode Portability | ⚠️ INTENTIONAL VIOLATION | This feature **removes** Mode B (SeaweedFS + Kafka). Constitution Principle II will need amendment after this feature ships. SQLite compatibility is preserved. |
| III. Interface-Driven Communication | ✅ PASS | `ObjectStorage` interface removed (SeaweedFS gone). `CloudStorage` interface retained for OSS. New lock/shard methods added to `FileRepo` interface. |
| IV. Test-First Discipline | ✅ PASS | All new biz-layer logic will have unit tests with mocks. Existing tests updated to remove SeaweedFS/Kafka paths. Test count MUST NOT decrease. |
| V. Configuration Hygiene | ✅ PASS | New env vars (ERASURE_*) follow existing pattern. SeaweedFS/Kafka env vars removed. |
| VI. Vendor-Locked Offline Build | ✅ PASS | New dependency (reedsolomon) vendored. Removed deps (kafka-go, aws-sdk-go-v2) cleaned from vendor/. |
| VII. Documentation as Deliverable | ✅ PASS | README, AGENTS.md, docs/ all updated. New `docs/distributed-locking.md` and `docs/erasure-coding.md` added. |

> **Constitution Amendment Required**: After this feature ships, Principle II ("Dual-Mode Portability") MUST be rewritten to reflect the single-mode architecture (local disk + MySQL default, SQLite optional).

## Project Structure

### Documentation (this feature)

```text
specs/001-distributed-arch-refactor/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research decisions
├── data-model.md        # Phase 1 data model
├── quickstart.md        # Phase 1 quickstart guide
├── contracts/
│   └── grpc-http-changes.md  # Contract changes
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
# Backend (Go, project root)
app/file/internal/
├── biz/
│   ├── file.go              # MODIFY: remove ObjectStorage/ModeS3, add locking, erasure, upload_status
│   └── file_test.go         # MODIFY: update tests, add new concurrent/erasure tests
├── data/
│   ├── data.go              # MODIFY: remove NewSeaweedFSClient from ProviderSet
│   ├── file.go              # MODIFY: add lock/shard/status methods, migrate chunks to SET
│   ├── seaweedfs.go         # DELETE
│   ├── kafka.go             # DELETE
│   ├── mq_goroutine.go      # KEEP (sole MQ implementation)
│   ├── oss.go               # KEEP
│   └── erasure.go           # NEW: Reed-Solomon encode/decode + shard CRUD
├── service/
│   └── file.go              # MODIFY: update gRPC responses
├── server/
│   └── grpc.go              # KEEP
└── conf/
    ├── conf.proto           # MODIFY: remove SeaweedFS/Kafka config, add erasure config
    └── conf.pb.go           # REGENERATE

app/file/cmd/
├── main.go                  # MODIFY: remove worker references
├── wire.go                  # MODIFY: remove SeaweedFS/Kafka providers
├── wire_gen.go              # REGENERATE
└── worker/                  # DELETE entire directory

app/gateway/internal/
├── client/
│   └── client.go            # MODIFY: hash router improvements for failover
└── handler/
    └── file.go              # MODIFY: remove SeaweedFS presigned paths

# Config & Infrastructure
docker-compose.yml           # MODIFY: remove kafka, seaweedfs, file-worker; promote mysql
Makefile                     # MODIFY: remove worker targets, images-all
Dockerfile                   # MODIFY: remove worker build path
.env.example                 # MODIFY: remove SeaweedFS/Kafka vars, add ERASURE_*
.env.local                   # MODIFY: update
.env.s3                      # DELETE
go.mod                       # MODIFY: remove kafka-go, aws-sdk-go-v2
go.sum                       # REGENERATE

# Proto
api/file/v1/file.proto       # MODIFY: add upload_status to CheckUploadReply

# Documentation
README.md                    # MODIFY: remove dual-mode refs, add locking/erasure docs
AGENTS.md                    # MODIFY: update architecture, service list, test counts
docs/architecture.md         # MODIFY
docs/dual-mode-storage.md    # DELETE (or rename to docs/storage.md)
docs/distributed-locking.md  # NEW
docs/erasure-coding.md       # NEW
docs/message-queue.md        # MODIFY: remove Kafka sections
docs/containerization.md     # MODIFY: remove worker/kafka/seaweedfs
docs/presigned-upload.md     # MODIFY: OSS-only
docs/cold-hot-storage.md     # MODIFY: remove SeaweedFS tier

# CI/CD
.github/workflows/ci.yml    # MODIFY: remove worker image build
.github/workflows/release.yml # MODIFY: remove worker binary

# Frontend (minimal changes)
frontend/src/composables/useUpload.ts  # MODIFY: remove seaweedfs presigned path (if any)
frontend/src/types/index.ts            # MODIFY: add upload_status to types
```

**Structure Decision**: Existing Monorepo structure preserved. Backend Go at root, frontend at `frontend/`. The primary changes are deletions (SeaweedFS, Kafka, worker) and modifications (biz/data layers) within the existing `app/file/` service.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| Constitution Principle II removal | Interviewer-driven: dual arch adds complexity without value for the target use case (personal cloud) | Keeping dual-mode means every new feature (locking, erasure) must be implemented twice for two storage backends |
| klauspost/reedsolomon new dependency | Erasure coding requires matrix math (Galois field arithmetic); not feasible to reimplement | Writing custom Reed-Solomon is error-prone and unmaintainable |
| Redis SET migration (chunks) | Current JSON blob approach is not concurrent-safe | Fixing the JSON approach with optimistic locking adds complexity without atomic guarantees |
