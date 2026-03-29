<!--
  Sync Impact Report
  ==================
  Version change: 2.0.0 → 2.1.0
  Modified principles:
    - II. Updated storage/MQ baseline to scattered local chunks + OSS recovery + Kafka-primary async flow
    - III. Updated interface list to reflect Kafka primary producer and chunk-recovery erasure usage
    - IV. Updated test count baseline
    - V. Updated config file references
  Added sections: None
  Removed sections: None
  Templates requiring updates:
    - .specify/templates/plan-template.md ✅ reviewed (no changes needed)
    - .specify/templates/spec-template.md ✅ reviewed (no changes needed)
    - .specify/templates/tasks-template.md ✅ reviewed (no changes needed)
  Follow-up TODOs: None
-->

# Light Cloud Disk Constitution

## Core Principles

### I. Clean Architecture Enforcement

Every Kratos microservice (user, file) MUST follow the four-layer
Clean Architecture: **Service → Biz → Data → Server**.

- **Biz layer** defines all repository, client, and producer interfaces.
  Business logic MUST NOT import any concrete data-layer package.
- **Data layer** implements Biz interfaces using GORM, Redis, gRPC,
  Kafka, or object-storage SDKs — the Biz layer remains unaware of
  the specific implementation.
- **Service layer** handles gRPC request/response mapping and parameter
  validation only; it MUST NOT contain business logic.
- **Wire** MUST be used for compile-time dependency injection in every
  Kratos service. Gateway does not use Wire because it has no
  Biz/Data layers.

**Rationale**: Dependency inversion enables isolated unit testing of
business logic with mocks and allows infrastructure components to be
swapped without touching the Biz layer (e.g., MySQL ↔ SQLite,
Kafka ↔ goroutine channel).

### II. Single-Mode with DB Flexibility

The system uses a single storage architecture: **scattered local chunks as
primary storage + LRU eviction to Alibaba Cloud OSS + Kafka as the primary
async queue**.

- **Database flexibility**: MySQL (default) or SQLite, controlled by
  `DB_DRIVER` environment variable. No code-level branching beyond
  GORM dialect selection.
- **Storage**: Local disk is always the hot tier. Direct uploads are stored as
  scattered chunks across file-service instances; each chunk also produces
  Reed-Solomon recovery shards written to OSS so healthy instances can rebuild
  failed chunks on demand.
- **Message queue**: Kafka is the primary production queue for cold migration
  and thumbnail events. When `KAFKA_BROKERS` is unset, local development may
  fall back to the in-process goroutine queue.

Code MUST NOT use compile-time flags or build tags to differentiate
database drivers (except `integration` tests).

**Rationale**: A single scattered-storage mode simplifies deployment,
eliminates gateway upload bottlenecks, and keeps failure recovery centered on
chunk metadata plus OSS recovery shards. DB flexibility is retained because
SQLite is useful for development and personal use.

### III. Interface-Driven Cross-Service Communication

All cross-service calls MUST go through interfaces defined in the
Biz layer.

- File Service → User Service: `biz.UserClient` interface,
  implemented via gRPC in `data/user_client.go`.
- File Service → MQ: `biz.MessageProducer` interface, implemented
  by `data/mq_kafka.go` in production and `data/mq_goroutine.go` as the local
  development fallback.
- File Service → Storage: `biz.CloudStorage` interface, implemented
  by OSS (alibabacloud-oss-go-sdk-v2) with noop fallback.
- File Service → Erasure Coding: `biz.ErasureEncoder` interface,
  implemented by `data/erasure.go` (klauspost/reedsolomon) for chunk recovery
  shards and legacy local_ec reconstruction.

Direct service-to-service calls or shared database access are
PROHIBITED.

**Rationale**: Interface isolation decouples microservices, enables
full mock-based unit testing, and makes it safe to swap transports
(gRPC → HTTP, Kafka → NATS) without touching business logic.

### IV. Test-First Discipline

- Every new feature MUST include unit tests for the Biz layer using
  mocked interfaces before merging.
- Handler and middleware layers MUST have unit tests with mocked gRPC
  clients.
- Data layer changes that interact with real infrastructure MUST have
  integration tests guarded by the `integration` build tag.
- Frontend composables, stores, and router MUST have Vitest unit
  tests.
- CI MUST run `go test -v -race -coverprofile=coverage.out ./...`
  and `pnpm test:run` on every push and PR.

Target coverage is not mandated as a hard gate, but test count MUST
NOT decrease for existing modules without documented justification.

**Rationale**: The project's 101+ backend tests and 50+ frontend
tests are a quality baseline. Test-first prevents regressions and
documents expected behavior.

### V. Configuration Hygiene

- **Sensitive values** (DB passwords, JWT secrets, OSS keys) MUST
  be read from environment variables and `.env` files. They MUST
  NOT appear in YAML config files, source code, or version control.
- **Non-sensitive values** (ports, timeouts, feature flags) MUST be
  stored in service-specific `configs/config.yaml` files with `${ENV_VAR}`
  placeholder substitution for any values that vary by environment.
- `.env.example` MUST be maintained as the canonical template.

**Rationale**: Separating sensitive and non-sensitive config prevents
accidental credential leaks while keeping YAML configs human-readable
and version-controlled.

### VI. Vendor-Locked Offline Build

- `vendor/` MUST be checked into the repository and kept in sync
  via `go mod vendor`.
- The unified `Dockerfile` MUST use `-mod=vendor` and `--network host`
  to achieve zero-network builds.
- `.dockerignore` MUST NOT exclude `vendor/`.
- CGO MUST be enabled (`CGO_ENABLED=1`) for SQLite support; the
  builder stage MUST use `golang:1.25` (debian) which ships gcc
  out of the box.

**Rationale**: Vendor-locked builds guarantee reproducibility across
CI, local development, and container builds. Podman's default
bridge network may lack internet access, making offline builds
essential.

### VII. Documentation as Deliverable

- `README.md` MUST be updated whenever a feature is added, modified,
  or removed. It serves dual purpose: user-facing guide and
  interview reference.
- `docs/` MUST contain one markdown file per major feature explaining
  the design decisions, implementation details, and interview talking
  points.
- `AGENTS.md` MUST be the single source of truth for AI agent and
  developer onboarding. It MUST be updated whenever the architecture,
  service list, or test count changes.

**Rationale**: This project doubles as a portfolio piece and interview
artifact. Complete documentation is not optional — it is a core
deliverable that demonstrates engineering maturity.

## Technology Constraints

- **Backend language**: Go (latest stable). Framework: Kratos v2.
  API gateway: Gin. No compatibility shims for older Go versions.
- **Frontend**: Vue 3 (Composition API) + TypeScript + Tailwind CSS v4.
  Package manager: pnpm. Build tool: Vite.
- **Container runtime**: Podman is the primary citizen. Docker
  Compose compatibility MUST be maintained. Makefile MUST auto-detect
  `podman` / `docker`.
- **CI/CD**: GitHub Actions. Go version MUST be read via
  `go-version-file: go.mod` with `GOTOOLCHAIN=local`. Container
  images MUST be pushed to GHCR.
- **Python** (if needed): use `uv` for virtual environment management.
- **Node.js** (if not present): use `fnm` to install latest LTS.
- **Monorepo structure**: Backend Go code at repo root, frontend at
  `frontend/`. Do NOT restructure the Go module path.

## Development Workflow

1. **Feature branch** → implement → unit tests pass → update
   `docs/` and `README.md` → PR.
2. **PR checks**: backend tests (`go test ./...`), frontend tests
   (`pnpm test:run`), Compose smoke test (Gateway + Frontend health),
   binary build, GHCR image push (on merge to main/v2).
3. **Release**: push `v*` tag → CI cross-compiles amd64/arm64
   binaries → GitHub Release.
4. **Local dev flow**:
   - `make infra-up` → `make run-user` → `make run-file` →
     `make run-gateway` → `cd frontend && pnpm dev`
5. **Container flow**:
   - `make images && make up`
6. **Code generation**: `make api` (proto) → `make conf` (config proto) →
   `make wire` (DI) after any proto or provider changes.

## Governance

- This constitution supersedes ad-hoc conventions. All PRs and code
  reviews MUST verify compliance with the principles above.
- **Amendment procedure**: propose changes via PR to this file +
  AGENTS.md. Amendments MUST include a Sync Impact Report (HTML
  comment at top) and update the version below.
- **Versioning policy**: MAJOR for principle removals/redefinitions,
  MINOR for new principles or material expansions, PATCH for
  wording/typo fixes.
- **Compliance review**: every feature PR MUST reference the relevant
  principle(s) it satisfies or justify deviations.

**Version**: 2.1.0 | **Ratified**: 2026-03-28 | **Last Amended**: 2026-07-22
