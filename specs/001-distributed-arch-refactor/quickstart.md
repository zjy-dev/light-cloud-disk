# Quickstart: Distributed Architecture Refactoring

**Feature**: `001-distributed-arch-refactor`

## Prerequisites

- Go 1.25+
- MySQL 8.0+ (or SQLite for development)
- Redis 7+
- Consul 1.19+
- pnpm 10+ / Node.js 24+

## Post-Migration Setup

### 1. Update environment

```bash
# Copy the updated example file
cp .env.example .env

# Key changes from previous versions:
# - STORAGE_MODE removed (always local disk)
# - KAFKA_BROKERS removed (goroutine MQ only)
# - SEAWEEDFS_* vars removed
# - DB_DRIVER defaults to mysql (set to sqlite for dev)
# - New: ERASURE_DATA_SHARDS (default: 4)
# - New: ERASURE_PARITY_SHARDS (default: 2)
# - New: ERASURE_MIN_FILE_SIZE (default: 1048576 = 1MB)
```

### 2. Start infrastructure + services

```bash
# Start infra (Consul + Redis + MySQL)
make infra-up

# Run services
make run-user     # gRPC :9001
make run-file     # gRPC :9002
make run-gateway  # HTTP :8080

# Frontend
cd frontend && pnpm dev
```

### 3. Container deployment

```bash
# Build images (no more images-all, no worker image)
make images

# Start all services
make up
```

### 4. Verify concurrent upload safety

```bash
# Upload the same file from two terminals simultaneously:
# Terminal 1:
curl -X POST http://localhost:8080/api/v1/file/check-upload \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"file_md5":"abc123","file_size":10485760,"total_chunks":2}'

# Terminal 2 (same file):
curl -X POST http://localhost:8080/api/v1/file/check-upload \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"file_md5":"abc123","file_size":10485760,"total_chunks":2}'

# Both should succeed; second one shows uploaded_chunks from first.
```

### 5. Verify erasure coding

```bash
# Upload a file > 1MB, then check shard files:
ls /app/store/abc123.shard.*
# Should see 6 files: .shard.0 through .shard.5

# Delete 2 shard files to simulate failure:
rm /app/store/abc123.shard.3 /app/store/abc123.shard.5

# Download should still succeed (reconstruction):
curl http://localhost:8080/api/v1/file/stream/$FILE_ID \
  -H "Authorization: Bearer $TOKEN" -o recovered.bin
```

## Migration from Dual-Mode (v6 → v7)

If you were running Mode B (SeaweedFS + Kafka), you MUST:

1. **Export files from SeaweedFS** to local disk or OSS before upgrading.
2. **Stop file-worker** containers (no longer part of the system).
3. **Update `file_stores` table**: Change any `storage_type = 'seaweedfs'` rows to `'local'` (if files copied to local) or `'oss'` (if migrated to OSS).
4. **Run the upgrade**: New binary will auto-migrate schema (add `upload_status` column, `erasure_shards` table).
