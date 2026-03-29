# Data Model: 分块打散存储

## 表变更

### 新增: `chunk_records` 表

存储每个文件分块的物理位置信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键 |
| file_md5 | VARCHAR(32) | 文件 MD5 |
| file_size | BIGINT | 文件总大小 |
| chunk_index | INT | 分块索引 (0-based) |
| chunk_size | BIGINT | 分块大小 (bytes) |
| instance_id | VARCHAR(128) | 存储该分块的文件服务实例标识 (host:port) |
| store_path | VARCHAR(512) | 本地磁盘路径 |
| checksum | VARCHAR(64) | 分块 MD5/SHA256 |
| storage_type | VARCHAR(16) | "local" 或 "oss" |
| created_at | DATETIME | 创建时间 |

**索引**: UNIQUE (file_md5, file_size, chunk_index)

### 修改: `file_stores` 表

| 字段变更 | 说明 |
|----------|------|
| 新增 `total_chunks` INT | 文件总分块数 |
| `store_path` | 不再存储合并文件路径，改为空或 OSS key (冷存储迁移后) |
| `storage_type` | 新增值 "scattered" 表示打散存储 |

### 修改: `erasure_shards` 表

| 字段变更 | 说明 |
|----------|------|
| 新增 `chunk_record_id` BIGINT | 关联到 chunk_records（纠删码按分块编码） |
| `file_store_id` | 保留，用于整文件级别的纠删码（向后兼容） |
| 新增 `instance_id` VARCHAR(128) | 校验分片所在实例 |

## 移除的逻辑

- `MergeChunkData` repo 方法：不再需要将分块合并为完整文件
- 临时分块目录 (`FILE_TMP_DIR`) 不再作为"临时"概念——分块直接存储到 `FILE_STORE_DIR` 下的按实例组织的目录

## 新增 Repo 接口方法

```go
// chunk_records CRUD
CreateChunkRecord(ctx context.Context, record *ChunkRecord) error
FindChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) ([]*ChunkRecord, error)
FindChunkRecordByIndex(ctx context.Context, fileMD5 string, fileSize int64, index int32) (*ChunkRecord, error)
CountChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) (int32, error)
DeleteChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) error

// 更新 chunk 的存储位置 (冷热迁移时)
UpdateChunkStorageLocation(ctx context.Context, id int64, storageType, newPath string) error
```

## 新增 Biz 实体

```go
type ChunkRecord struct {
    ID          int64
    FileMD5     string
    FileSize    int64
    ChunkIndex  int32
    ChunkSize   int64
    InstanceID  string
    StorePath   string
    Checksum    string
    StorageType string
    CreatedAt   time.Time
}

type UploadPlan struct {
    FileMD5     string
    FileSize    int64
    TotalChunks int32
    ChunkSize   int64
    Assignments []ChunkAssignment
    Token       string // JWT token for direct upload auth
}

type ChunkAssignment struct {
    ChunkIndex int32
    TargetAddr string // file-service HTTP address
    UploadURL  string // full URL: http://{addr}/api/v1/chunks/{md5}/{index}
}

type DownloadPlan struct {
    FileMD5     string
    FileSize    int64
    TotalChunks int32
    Chunks      []ChunkLocation
}

type ChunkLocation struct {
    ChunkIndex  int32
    ChunkSize   int64
    DownloadURL string // file-service HTTP address
    Checksum    string
}
```
