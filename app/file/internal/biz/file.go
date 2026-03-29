package biz

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Storage type constants
const (
	StorageLocal   = "local"    // files kept on local disk
	StorageLocalEC = "local_ec" // files stored with erasure coding shards on local disk
	StorageOSS     = "oss"      // cold-tier files in Alibaba Cloud OSS
)

var (
	ErrFileNotFound     = errors.New("file not found")
	ErrFolderNotFound   = errors.New("folder not found")
	ErrShareNotFound    = errors.New("share not found")
	ErrShareExpired     = errors.New("share expired")
	ErrInvalidPassword  = errors.New("invalid share password")
	ErrStorageExceeded  = errors.New("storage limit exceeded")
	ErrDiskFull         = errors.New("local disk full, please retry later")
	ErrSessionNotFound  = errors.New("upload session not found")
	ErrSessionCompleted = errors.New("upload session already completed")
	ErrIncompleteUpload = errors.New("not all parts uploaded yet")
	ErrUnauthorized     = errors.New("not authorized to access this session")
	ErrChunkLocked      = errors.New("chunk is locked by another writer")
	ErrMergeLocked      = errors.New("merge is locked by another process")
)

type File struct {
	ID        int64
	UserID    int64
	ParentID  int64
	Name      string
	FileMD5   string
	Size      int64
	IsFolder  bool
	Path      string
	StorePath string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FileStore struct {
	ID             int64
	FileMD5        string
	Size           int64
	StorePath      string
	StorageType    string // "local", "local_ec", or "oss"
	UploadStatus   string // "uploading" or "completed"
	RefCount       int32
	LastAccessedAt time.Time
	CreatedAt      time.Time
}

type ErasureShard struct {
	ID          int64
	FileStoreID int64
	ShardIndex  int32
	ShardPath   string
	ShardSize   int64
	IsParity    bool
	Checksum    string
}

type Share struct {
	ID        string
	UserID    int64
	FileID    int64
	Password  string
	ExpireAt  *time.Time
	ViewCount int64
	CreatedAt time.Time
}

type ChunkInfo struct {
	FileMD5    string
	ChunkIndex int32
	ChunkSize  int64
	Uploaded   bool
}

// UploadSession persists a presigned multipart upload session for cross-device resume.
type UploadSession struct {
	ID            string // UUID
	UserID        int64
	ParentID      int64
	FileName      string
	FileMD5       string
	FileSize      int64
	TotalParts    int32
	PartSize      int64  // bytes per part (last may be smaller)
	StorageTarget string // "oss"
	ObjectKey     string // S3/OSS key
	S3UploadID    string // from InitMultipartUpload
	Status        string // "uploading", "completed", "aborted"
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// UploadedPart records a successfully uploaded part within a session.
type UploadedPart struct {
	SessionID  string
	PartNumber int32
	ETag       string
	Size       int64
	UploadedAt time.Time
}

// StorageConfig holds storage tier configuration.
// Local disk is the primary storage, OSS is the eviction target.
type StorageConfig struct {
	PrimaryMaxBytes int64 // capacity of the local disk primary storage
	ThresholdPct    int32 // eviction trigger, e.g. 80 means 80 %
	EvictTargetPct  int32 // evict down to this % of threshold (default 90)
}

// ErasureConfig holds erasure coding parameters.
type ErasureConfig struct {
	DataShards   int   // number of data shards (default 4)
	ParityShards int   // number of parity shards (default 2)
	MinFileSize  int64 // minimum file size for erasure coding (default 1 MB)
}

// FileRepo is the data-access interface for metadata and chunk operations
type FileRepo interface {
	Create(ctx context.Context, file *File) (*File, error)
	FindByID(ctx context.Context, id int64) (*File, error)
	FindByUserAndParent(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error)
	Update(ctx context.Context, file *File) error
	SoftDelete(ctx context.Context, userID int64, ids []int64) error
	Restore(ctx context.Context, userID int64, ids []int64) error
	PermanentDelete(ctx context.Context, userID int64, ids []int64) error
	FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error)
	Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error)

	// FileStore CRUD operations
	FindStoreByMD5(ctx context.Context, md5 string) (*FileStore, error)
	CreateStore(ctx context.Context, store *FileStore) error
	IncrStoreRefCount(ctx context.Context, md5 string) error
	DecrStoreRefCount(ctx context.Context, md5 string) error
	UpdateStorageLocation(ctx context.Context, fileMD5 string, storageType string, newPath string) error
	UpdateLastAccessed(ctx context.Context, fileMD5 string) error
	FindLRUStores(ctx context.Context, storageType string, limit int) ([]*FileStore, error)
	SumSizeByStorageType(ctx context.Context, storageType string) (int64, error)

	// FileStore status operations (concurrent upload safety)
	CreateStoreWithStatus(ctx context.Context, store *FileStore) (*FileStore, error)
	UpdateStoreStatus(ctx context.Context, id int64, status string) error
	FindStoreByMD5AndStatus(ctx context.Context, md5, status string) (*FileStore, error)

	// Share operations
	CreateShare(ctx context.Context, share *Share) error
	FindShareByID(ctx context.Context, id string) (*Share, error)
	DeleteExpiredShares(ctx context.Context) error

	// Distributed lock operations (Redis)
	AcquireChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32, ttl time.Duration) (bool, error)
	ReleaseChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32) error
	AcquireMergeLock(ctx context.Context, fileMD5 string, ttl time.Duration) (bool, error)
	ReleaseMergeLock(ctx context.Context, fileMD5 string) error

	// Chunk upload operations (Redis SET + local disk)
	AddUploadedChunk(ctx context.Context, fileMD5 string, chunkIndex int32) error
	IsChunkUploaded(ctx context.Context, fileMD5 string, chunkIndex int32) (bool, error)
	CountUploadedChunks(ctx context.Context, fileMD5 string) (int32, error)
	GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error)
	SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error
	MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error)
	ClearChunkInfo(ctx context.Context, fileMD5 string) error

	// Disk usage operations (Redis atomic counters)
	GetDiskUsage(ctx context.Context, diskType string) (int64, error)
	IncrDiskUsage(ctx context.Context, diskType string, delta int64) error

	// Upload session operations (for presigned multipart uploads)
	CreateUploadSession(ctx context.Context, session *UploadSession) error
	FindUploadSession(ctx context.Context, userID int64, fileMD5 string) (*UploadSession, error)
	FindUploadSessionByID(ctx context.Context, sessionID string) (*UploadSession, error)
	UpdateUploadSessionStatus(ctx context.Context, sessionID, status string) error
	SaveUploadPart(ctx context.Context, sessionID string, partNumber int32, etag string, size int64) error
	FindUploadedParts(ctx context.Context, sessionID string) ([]UploadedPart, error)
	DeleteUploadSession(ctx context.Context, sessionID string) error

	// Erasure coding shard operations
	CreateErasureShard(ctx context.Context, shard *ErasureShard) error
	FindErasureShards(ctx context.Context, fileStoreID int64) ([]*ErasureShard, error)
}

// UserClient is the cross-service gRPC interface for user operations
type UserClient interface {
	UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error
}

// CompletedPart represents a completed part of a multipart upload
type CompletedPart struct {
	PartNumber int32
	ETag       string
}

// CloudStorage abstracts cloud object storage such as Alibaba Cloud OSS
type CloudStorage interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)

	// Multipart upload (for presigned uploads)
	InitMultipartUpload(ctx context.Context, key string) (uploadID string, err error)
	PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (string, error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}

// CloudMigrateMessage is sent to Kafka to trigger async cold migration
type CloudMigrateMessage struct {
	FileMD5   string `json:"file_md5"`
	SourceKey string `json:"source_key"`
	FileSize  int64  `json:"file_size"`
}

// ThumbnailMessage triggers async thumbnail generation
type ThumbnailMessage struct {
	FileID   int64  `json:"file_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
}

// MessageProducer sends async messages for cloud migration
type MessageProducer interface {
	SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
	SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
	Close() error
}

// ErasureEncoder abstracts Reed-Solomon encode/decode operations for testability.
type ErasureEncoder interface {
	Encode(filePath, storeDir, fileMD5 string, dataShards, parityShards int) (shardPaths []string, err error)
	Reconstruct(shardPaths []string, dataShards, parityShards int) ([]byte, error)
	ShardChecksum(path string) (string, error)
}

// FileUsecase contains core file business logic
type FileUsecase struct {
	repo       FileRepo
	userClient UserClient
	mq         MessageProducer
	cloudStore CloudStorage
	storageCfg *StorageConfig
	erasureCfg *ErasureConfig
	erasureEnc ErasureEncoder
	storeDir   string // local file store directory (from Upload.StoreDir)
	log        *log.Helper
}

func NewFileUsecase(
	repo FileRepo,
	userClient UserClient,
	mq MessageProducer,
	cloudStore CloudStorage,
	storageCfg *StorageConfig,
	erasureCfg *ErasureConfig,
	erasureEnc ErasureEncoder,
	storeDir string,
	logger log.Logger,
) *FileUsecase {
	if erasureCfg == nil {
		erasureCfg = &ErasureConfig{DataShards: 4, ParityShards: 2, MinFileSize: 1 << 20}
	}
	return &FileUsecase{
		repo:       repo,
		userClient: userClient,
		mq:         mq,
		cloudStore: cloudStore,
		storageCfg: storageCfg,
		erasureCfg: erasureCfg,
		erasureEnc: erasureEnc,
		storeDir:   storeDir,
		log:        log.NewHelper(logger),
	}
}

// CheckUpload verifies instant/resumable upload and primary-storage disk availability.
// Returns: canFastUpload, uploadedChunks, diskFull, uploadMode, uploadStatus, error
// uploadMode is "direct" (chunks through backend) or "presigned" (client uploads to OSS directly).
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, bool, string, string, error) {
	// 1) Check instant upload by MD5 deduplication
	store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if err == nil && store != nil {
		return true, nil, false, "direct", store.UploadStatus, nil // instant-upload hit
	}

	// 1b) Check for in-progress upload (join cooperative upload)
	store, err = uc.repo.FindStoreByMD5AndStatus(ctx, fileMD5, "uploading")
	if err == nil && store != nil {
		// Another client is uploading this file — join and return uploaded chunks
		uploadedChunks, _ := uc.repo.GetUploadedChunks(ctx, fileMD5)
		return false, uploadedChunks, false, "direct", "uploading", nil
	}

	// 2) Check primary disk availability
	used, _ := uc.repo.GetDiskUsage(ctx, "local")
	if used+fileSize > uc.storageCfg.PrimaryMaxBytes {
		// Disk full → switch to presigned OSS upload
		return false, nil, true, "presigned", "", nil
	}

	// 3) Return uploaded chunks for resumable direct upload
	uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
	if err != nil {
		return false, nil, false, "direct", "", err
	}
	return false, uploadedChunks, false, "direct", "", nil
}

// SaveChunk writes a single chunk to local temp storage with distributed locking
func (uc *FileUsecase) SaveChunk(ctx context.Context, fileMD5 string, chunkIndex int32, chunkSize int64, chunkData []byte) error {
	if int64(len(chunkData)) != chunkSize {
		return errors.New("chunk size mismatch")
	}

	// Acquire chunk-level distributed lock (30s TTL)
	acquired, err := uc.repo.AcquireChunkLock(ctx, fileMD5, chunkIndex, 30*time.Second)
	if err != nil {
		return fmt.Errorf("acquire chunk lock: %w", err)
	}
	if !acquired {
		return ErrChunkLocked
	}
	defer func() {
		_ = uc.repo.ReleaseChunkLock(ctx, fileMD5, chunkIndex)
	}()

	// Check if chunk already uploaded (idempotent skip)
	uploaded, _ := uc.repo.IsChunkUploaded(ctx, fileMD5, chunkIndex)
	if uploaded {
		return nil // already uploaded by another client
	}

	if err := uc.repo.SaveChunkData(ctx, fileMD5, chunkIndex, chunkData); err != nil {
		return err
	}
	// Track local disk usage counter
	_ = uc.repo.IncrDiskUsage(ctx, "local", chunkSize)

	return uc.repo.AddUploadedChunk(ctx, fileMD5, chunkIndex)
}

// MergeChunks merges chunks, uploads to the appropriate storage, and cleans temp files.
func (uc *FileUsecase) MergeChunks(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64, totalChunks int32) (*File, error) {
	// 1) Re-check deduplication to reuse existing storage
	store, _ := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if store != nil && store.UploadStatus == "completed" {
		if err := uc.repo.IncrStoreRefCount(ctx, fileMD5); err != nil {
			return nil, err
		}
		file := &File{
			UserID: userID, ParentID: parentID, Name: fileName,
			FileMD5: fileMD5, Size: fileSize, IsFolder: false,
			Path: store.StorePath,
		}
		createdFile, err := uc.repo.Create(ctx, file)
		if err != nil {
			return nil, err
		}
		_ = uc.repo.ClearChunkInfo(ctx, fileMD5)
		if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
			uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
		}
		return createdFile, nil
	}

	// 2) Acquire merge lock (distributed across instances, 2 min TTL)
	acquired, err := uc.repo.AcquireMergeLock(ctx, fileMD5, 2*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("acquire merge lock: %w", err)
	}
	if !acquired {
		return nil, ErrMergeLocked
	}
	defer func() {
		_ = uc.repo.ReleaseMergeLock(ctx, fileMD5)
	}()

	// 3) Re-check dedup after acquiring lock (another instance may have completed)
	store, _ = uc.repo.FindStoreByMD5AndStatus(ctx, fileMD5, "completed")
	if store != nil {
		if err := uc.repo.IncrStoreRefCount(ctx, fileMD5); err != nil {
			return nil, err
		}
		file := &File{
			UserID: userID, ParentID: parentID, Name: fileName,
			FileMD5: fileMD5, Size: fileSize, IsFolder: false,
			Path: store.StorePath,
		}
		createdFile, err := uc.repo.Create(ctx, file)
		if err != nil {
			return nil, err
		}
		_ = uc.repo.ClearChunkInfo(ctx, fileMD5)
		if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
			uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
		}
		return createdFile, nil
	}

	// 4) Create store with uploading status (MySQL UNIQUE constraint handles races)
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fileMD5 + ext

	createdStore, err := uc.repo.CreateStoreWithStatus(ctx, &FileStore{
		FileMD5:      fileMD5,
		Size:         fileSize,
		StorePath:    objectKey,
		StorageType:  StorageLocal,
		UploadStatus: "uploading",
		RefCount:     1,
	})
	if err != nil {
		return nil, err
	}
	// UNIQUE constraint race: another instance may have created & completed it
	if createdStore.UploadStatus == "completed" || createdStore.UploadStatus == "done" {
		if err := uc.repo.IncrStoreRefCount(ctx, fileMD5); err != nil {
			return nil, err
		}
		file := &File{
			UserID: userID, ParentID: parentID, Name: fileName,
			FileMD5: fileMD5, Size: fileSize, IsFolder: false,
			Path: createdStore.StorePath,
		}
		createdFile, createErr := uc.repo.Create(ctx, file)
		if createErr != nil {
			return nil, createErr
		}
		_ = uc.repo.ClearChunkInfo(ctx, fileMD5)
		if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
			uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
		}
		return createdFile, nil
	}

	// 5) Verify all chunks are present locally (cross-instance failover guard)
	uploaded, _ := uc.repo.CountUploadedChunks(ctx, fileMD5)
	if uploaded < totalChunks {
		return nil, fmt.Errorf("incomplete chunks: have %d of %d — retry after chunks are re-uploaded to this instance", uploaded, totalChunks)
	}

	// 6) Merge chunks into a local file
	mergedPath, err := uc.repo.MergeChunkData(ctx, fileMD5, fileName, totalChunks)
	if err != nil {
		return nil, err
	}

	// 7) Choose storage target: local disk, OSS fallback if disk full
	storageType, storePath := uc.uploadMergedFile(ctx, mergedPath, objectKey, fileSize)

	// 8) Remove local merged file only if uploaded to remote storage
	if storageType != StorageLocal {
		_ = os.Remove(mergedPath)
		_ = uc.repo.IncrDiskUsage(ctx, "local", -fileSize)
	}

	// 8.5) Apply erasure coding to local files above size threshold
	if storageType == StorageLocal {
		localPath := filepath.Join(uc.storeDir, objectKey)
		if uc.encodeWithErasure(ctx, localPath, fileMD5, fileSize, createdStore.ID) {
			storageType = StorageLocalEC
		}
	}

	// 9) Update store status to completed with final storage info
	_ = uc.repo.UpdateStorageLocation(ctx, fileMD5, storageType, storePath)
	_ = uc.repo.UpdateStoreStatus(ctx, createdStore.ID, "completed")

	// 10) Create files record
	file := &File{
		UserID: userID, ParentID: parentID, Name: fileName,
		FileMD5: fileMD5, Size: fileSize, IsFolder: false,
		Path: storePath,
	}
	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	// 11) Cleanup
	if err := uc.repo.ClearChunkInfo(ctx, fileMD5); err != nil {
		uc.log.Warnf("Could not clear chunk metadata for %s: %v", fileMD5, err)
	}
	if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
		uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
	}

	// 12) Check primary storage threshold and trigger LRU eviction if needed
	primUsed, _ := uc.repo.GetDiskUsage(ctx, "local")
	uc.maybeEvictToCloud(ctx, primUsed)

	// 13) Send async thumbnail task for media files
	if uc.mq != nil && isMediaFile(fileName) {
		thumbMsg := &ThumbnailMessage{
			FileID:   createdFile.ID,
			FilePath: storePath,
			FileType: detectMIME(fileName),
		}
		if err := uc.mq.SendThumbnailMessage(ctx, thumbMsg); err != nil {
			uc.log.Warnf("Could not enqueue thumbnail task for file %d: %v", createdFile.ID, err)
		}
	}

	return createdFile, nil
}

// uploadMergedFile puts the merged file into the appropriate storage backend,
// falling back to OSS when the local disk is full.
func (uc *FileUsecase) uploadMergedFile(ctx context.Context, mergedPath, objectKey string, fileSize int64) (storageType, storePath string) {
	storePath = objectKey

	// Local disk is primary, OSS is fallback
	primUsed, _ := uc.repo.GetDiskUsage(ctx, "local")
	if primUsed+fileSize <= uc.storageCfg.PrimaryMaxBytes {
		// Keep file on local disk — copy/link to storeDir if not already there
		dest := filepath.Join(uc.storeDir, objectKey)
		if dest != mergedPath {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err == nil {
				// Try hard link first (cheap), fall back to copy
				if err := os.Link(mergedPath, dest); err != nil {
					uc.copyFile(mergedPath, dest)
				}
			}
		}
		_ = uc.repo.IncrDiskUsage(ctx, "local", fileSize)
		return StorageLocal, storePath
	}
	// Local disk full at merge time, upload to OSS directly
	return uc.uploadToOSS(ctx, mergedPath, objectKey, fileSize, storePath)
}

// uploadToOSS uploads the merged file to cloud (OSS) storage.
// Returns StorageOSS on success, or ("", "") and logs error on failure.
func (uc *FileUsecase) uploadToOSS(ctx context.Context, mergedPath, objectKey string, fileSize int64, storePath string) (string, string) {
	localFile, err := os.Open(mergedPath)
	if err != nil {
		uc.log.Errorf("uploadToOSS: failed to open %s: %v", mergedPath, err)
		return StorageLocal, storePath // keep local as fallback
	}
	defer localFile.Close()
	if err := uc.cloudStore.Put(ctx, objectKey, localFile, fileSize); err != nil {
		uc.log.Errorf("uploadToOSS: failed to upload %s: %v", objectKey, err)
		return StorageLocal, storePath // keep local as fallback
	}
	return StorageOSS, storePath
}

// copyFile copies src to dst by reading and writing.
func (uc *FileUsecase) copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		uc.log.Errorf("copyFile: open %s: %v", src, err)
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		uc.log.Errorf("copyFile: create %s: %v", dst, err)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		uc.log.Errorf("copyFile: copy %s -> %s: %v", src, dst, err)
	}
}

// encodeWithErasure applies Reed-Solomon encoding to a locally stored file
// and persists shard metadata. It deletes the original merged file after
// successful encoding.
// Returns true if encoding was performed, false otherwise.
func (uc *FileUsecase) encodeWithErasure(ctx context.Context, mergedPath, fileMD5 string, fileSize int64, storeID int64) bool {
	if uc.erasureEnc == nil || fileSize < uc.erasureCfg.MinFileSize {
		return false
	}

	shardPaths, err := uc.erasureEnc.Encode(mergedPath, uc.storeDir, fileMD5, uc.erasureCfg.DataShards, uc.erasureCfg.ParityShards)
	if err != nil {
		uc.log.Warnf("erasure encode failed for %s, keeping original: %v", fileMD5, err)
		return false
	}

	total := uc.erasureCfg.DataShards + uc.erasureCfg.ParityShards
	for i, sp := range shardPaths {
		checksum, _ := uc.erasureEnc.ShardChecksum(sp)
		info, _ := os.Stat(sp)
		var size int64
		if info != nil {
			size = info.Size()
		}
		shard := &ErasureShard{
			FileStoreID: storeID,
			ShardIndex:  int32(i),
			ShardPath:   filepath.Base(sp),
			ShardSize:   size,
			IsParity:    i >= uc.erasureCfg.DataShards,
			Checksum:    checksum,
		}
		if err := uc.repo.CreateErasureShard(ctx, shard); err != nil {
			uc.log.Errorf("failed to save erasure shard %d for %s: %v", i, fileMD5, err)
			// Clean up shard files on partial failure
			for _, p := range shardPaths {
				_ = os.Remove(p)
			}
			return false
		}
		_ = i
		_ = total
	}

	// Remove original merged file — data is now in shards
	_ = os.Remove(mergedPath)
	return true
}

// reconstructFromShards reads erasure shards from disk and reconstructs the
// original file data. Returns an io.ReadCloser over the reconstructed bytes.
func (uc *FileUsecase) reconstructFromShards(ctx context.Context, fileStoreID int64) (io.ReadCloser, int64, error) {
	shards, err := uc.repo.FindErasureShards(ctx, fileStoreID)
	if err != nil || len(shards) == 0 {
		return nil, 0, fmt.Errorf("no erasure shards found for store %d", fileStoreID)
	}

	total := int32(uc.erasureCfg.DataShards + uc.erasureCfg.ParityShards)
	paths := make([]string, total)
	for _, s := range shards {
		if s.ShardIndex < total {
			paths[s.ShardIndex] = filepath.Join(uc.storeDir, s.ShardPath)
		}
	}

	data, err := uc.erasureEnc.Reconstruct(paths, uc.erasureCfg.DataShards, uc.erasureCfg.ParityShards)
	if err != nil {
		return nil, 0, fmt.Errorf("reconstruct store %d: %w", fileStoreID, err)
	}

	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

// ---------------------------------------------------------------------------
// Presigned Multipart Upload
// ---------------------------------------------------------------------------

const (
	defaultPartSize     int64 = 5 * 1024 * 1024 // 5 MB
	presignedURLExpiry        = 2 * time.Hour
	uploadSessionExpiry       = 24 * time.Hour
)

// PresignedUploadResult holds the response for InitPresignedUpload.
type PresignedUploadResult struct {
	SessionID      string
	PendingParts   []PresignedPart
	CompletedParts []UploadedPart
	StorageTarget  string
	PartSize       int64
	CanFastUpload  bool
	File           *File // non-nil only when CanFastUpload=true
}

// PresignedPart carries a part number and its presigned upload URL.
type PresignedPart struct {
	PartNumber int32
	UploadURL  string
}

// InitPresignedUpload creates (or resumes) a presigned multipart upload session.
func (uc *FileUsecase) InitPresignedUpload(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64, totalParts int32) (*PresignedUploadResult, error) {
	// 1) Fast-upload dedup check
	store, _ := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if store != nil {
		// Reuse existing storage, create a files record referencing it
		if err := uc.repo.IncrStoreRefCount(ctx, fileMD5); err != nil {
			return nil, err
		}
		file := &File{
			UserID: userID, ParentID: parentID, Name: fileName,
			FileMD5: fileMD5, Size: fileSize, IsFolder: false,
			Path: store.StorePath,
		}
		createdFile, err := uc.repo.Create(ctx, file)
		if err != nil {
			return nil, err
		}
		if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
			uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
		}
		return &PresignedUploadResult{CanFastUpload: true, File: createdFile}, nil
	}

	// 2) Try to resume an existing session
	session, _ := uc.repo.FindUploadSession(ctx, userID, fileMD5)
	if session != nil && session.ExpiresAt.After(time.Now()) {
		// Session alive → refresh presigned URLs for pending parts
		completedParts, _ := uc.repo.FindUploadedParts(ctx, session.ID)
		completedSet := make(map[int32]bool, len(completedParts))
		for _, p := range completedParts {
			completedSet[p.PartNumber] = true
		}
		pendingParts, err := uc.generatePresignedURLs(ctx, session, completedSet)
		if err != nil {
			return nil, err
		}
		return &PresignedUploadResult{
			SessionID:      session.ID,
			PendingParts:   pendingParts,
			CompletedParts: completedParts,
			StorageTarget:  session.StorageTarget,
			PartSize:       session.PartSize,
		}, nil
	}

	// Abort stale session if exists
	if session != nil {
		uc.abortS3Upload(ctx, session)
		_ = uc.repo.DeleteUploadSession(ctx, session.ID)
	}

	// 3) New session: presigned always goes to OSS
	storageTarget := StorageOSS

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fileMD5 + ext

	// Init S3 multipart upload on OSS
	s3UploadID, initErr := uc.cloudStore.InitMultipartUpload(ctx, objectKey)
	if initErr != nil {
		return nil, fmt.Errorf("init multipart upload: %w", initErr)
	}

	partSize := defaultPartSize
	// Server computes totalParts from fileSize, ignoring client-supplied value
	computedParts := int32((fileSize + partSize - 1) / partSize)
	sessionID := generateUUID()
	newSession := &UploadSession{
		ID:            sessionID,
		UserID:        userID,
		ParentID:      parentID,
		FileName:      fileName,
		FileMD5:       fileMD5,
		FileSize:      fileSize,
		TotalParts:    computedParts,
		PartSize:      partSize,
		StorageTarget: storageTarget,
		ObjectKey:     objectKey,
		S3UploadID:    s3UploadID,
		Status:        "uploading",
		ExpiresAt:     time.Now().Add(uploadSessionExpiry),
	}
	if err := uc.repo.CreateUploadSession(ctx, newSession); err != nil {
		return nil, err
	}

	pendingParts, err := uc.generatePresignedURLs(ctx, newSession, nil)
	if err != nil {
		return nil, err
	}

	return &PresignedUploadResult{
		SessionID:     sessionID,
		PendingParts:  pendingParts,
		StorageTarget: storageTarget,
		PartSize:      partSize,
	}, nil
}

// ReportUploadedPart records that the client has finished uploading one part.
func (uc *FileUsecase) ReportUploadedPart(ctx context.Context, userID int64, sessionID string, partNumber int32, etag string, size int64) error {
	session, err := uc.repo.FindUploadSessionByID(ctx, sessionID)
	if err != nil {
		return ErrSessionNotFound
	}
	if session.UserID != userID {
		return ErrUnauthorized
	}
	if session.Status != "uploading" {
		return ErrSessionCompleted
	}
	return uc.repo.SaveUploadPart(ctx, sessionID, partNumber, etag, size)
}

// CompletePresignedUpload finishes the multipart upload and creates file records.
func (uc *FileUsecase) CompletePresignedUpload(ctx context.Context, userID int64, sessionID string) (*File, error) {
	session, err := uc.repo.FindUploadSessionByID(ctx, sessionID)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	if session.UserID != userID {
		return nil, ErrUnauthorized
	}
	if session.Status != "uploading" {
		return nil, ErrSessionCompleted
	}

	parts, err := uc.repo.FindUploadedParts(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	if int32(len(parts)) < session.TotalParts {
		return nil, ErrIncompleteUpload
	}

	// Build completed parts list for S3
	s3Parts := make([]CompletedPart, len(parts))
	for i, p := range parts {
		s3Parts[i] = CompletedPart{PartNumber: p.PartNumber, ETag: p.ETag}
	}

	// Complete multipart upload on OSS
	if err := uc.cloudStore.CompleteMultipartUpload(ctx, session.ObjectKey, session.S3UploadID, s3Parts); err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	// Create file_stores record
	if createErr := uc.repo.CreateStore(ctx, &FileStore{
		FileMD5:      session.FileMD5,
		Size:         session.FileSize,
		StorePath:    session.ObjectKey,
		StorageType:  StorageOSS,
		UploadStatus: "completed",
		RefCount:     1,
	}); createErr != nil {
		return nil, createErr
	}

	// Create files record
	file := &File{
		UserID: session.UserID, ParentID: session.ParentID, Name: session.FileName,
		FileMD5: session.FileMD5, Size: session.FileSize, IsFolder: false,
		Path: session.ObjectKey,
	}
	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	// Mark session completed
	_ = uc.repo.UpdateUploadSessionStatus(ctx, session.ID, "completed")

	// Update user storage quota
	if err := uc.userClient.UpdateStorageUsed(ctx, session.UserID, session.FileSize); err != nil {
		uc.log.Warnf("Could not update storage usage for user %d: %v", session.UserID, err)
	}

	return createdFile, nil
}

// AbortPresignedUpload cancels the multipart upload and cleans up the session.
func (uc *FileUsecase) AbortPresignedUpload(ctx context.Context, userID int64, sessionID string) error {
	session, err := uc.repo.FindUploadSessionByID(ctx, sessionID)
	if err != nil {
		return ErrSessionNotFound
	}
	if session.UserID != userID {
		return ErrUnauthorized
	}
	if session.Status != "uploading" {
		return nil // already completed or aborted
	}
	uc.abortS3Upload(ctx, session)
	_ = uc.repo.UpdateUploadSessionStatus(ctx, session.ID, "aborted")
	return nil
}

// generatePresignedURLs creates presigned upload URLs for parts not in completedSet.
func (uc *FileUsecase) generatePresignedURLs(ctx context.Context, session *UploadSession, completedSet map[int32]bool) ([]PresignedPart, error) {
	var pending []PresignedPart
	for i := int32(1); i <= session.TotalParts; i++ {
		if completedSet[i] {
			continue
		}
		url, err := uc.cloudStore.PresignUploadPart(ctx, session.ObjectKey, session.S3UploadID, i, presignedURLExpiry)
		if err != nil {
			return nil, fmt.Errorf("presign part %d: %w", i, err)
		}
		pending = append(pending, PresignedPart{PartNumber: i, UploadURL: url})
	}
	return pending, nil
}

// abortS3Upload calls abort on OSS, logging errors.
func (uc *FileUsecase) abortS3Upload(ctx context.Context, session *UploadSession) {
	if err := uc.cloudStore.AbortMultipartUpload(ctx, session.ObjectKey, session.S3UploadID); err != nil {
		uc.log.Warnf("Failed to abort multipart upload %s: %v", session.S3UploadID, err)
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(cryptorand.Reader, b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// maybeEvictToCloud triggers LRU cold migration when primary storage exceeds threshold.
func (uc *FileUsecase) maybeEvictToCloud(ctx context.Context, currentUsed int64) {
	threshold := uc.storageCfg.PrimaryMaxBytes * int64(uc.storageCfg.ThresholdPct) / 100
	if currentUsed <= threshold {
		return
	}

	// Evict down to EvictTargetPct of threshold to reduce oscillation
	evictPct := uc.storageCfg.EvictTargetPct
	if evictPct <= 0 {
		evictPct = 90
	}
	target := threshold * int64(evictPct) / 100
	toFree := currentUsed - target

	stores, err := uc.repo.FindLRUStores(ctx, StorageLocal, 100)
	if err != nil {
		uc.log.Warnf("Could not load LRU candidates for eviction: %v", err)
		return
	}

	var freed int64
	for _, s := range stores {
		if freed >= toFree {
			break
		}
		if uc.mq != nil {
			msg := &CloudMigrateMessage{
				FileMD5:   s.FileMD5,
				SourceKey: s.StorePath,
				FileSize:  s.Size,
			}
			if err := uc.mq.SendCloudMigrateMessage(ctx, msg); err != nil {
				uc.log.Warnf("Could not enqueue cloud-migration task for %s: %v", s.FileMD5, err)
				continue
			}
		}
		freed += s.Size
	}
	if freed > 0 {
		uc.log.Infof("LRU eviction triggered: queued %d bytes for cloud migration", freed)
	}
}

// GetDiskUsage returns current usage of primary storage and its capacity.
func (uc *FileUsecase) GetDiskUsage(ctx context.Context) (primaryUsed, primaryMax int64, primaryType string, err error) {
	primaryUsed, _ = uc.repo.GetDiskUsage(ctx, "local")
	return primaryUsed, uc.storageCfg.PrimaryMaxBytes, "local", nil
}

func (uc *FileUsecase) ListFiles(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error) {
	return uc.repo.FindByUserAndParent(ctx, userID, parentID, page, pageSize)
}

func (uc *FileUsecase) CreateFolder(ctx context.Context, userID, parentID int64, name string) (*File, error) {
	folder := &File{UserID: userID, ParentID: parentID, Name: name, IsFolder: true}
	return uc.repo.Create(ctx, folder)
}

func (uc *FileUsecase) RenameFile(ctx context.Context, userID, fileID int64, newName string) error {
	file, err := uc.repo.FindByID(ctx, fileID)
	if err != nil {
		return ErrFileNotFound
	}
	if file.UserID != userID {
		return ErrFileNotFound
	}
	file.Name = newName
	return uc.repo.Update(ctx, file)
}

func (uc *FileUsecase) DeleteFiles(ctx context.Context, userID int64, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}
	for _, id := range fileIDs {
		file, err := uc.repo.FindByID(ctx, id)
		if err != nil || file.UserID != userID {
			return ErrFileNotFound
		}
	}
	return uc.repo.SoftDelete(ctx, userID, fileIDs)
}

func (uc *FileUsecase) MoveFiles(ctx context.Context, userID int64, fileIDs []int64, targetFolderID int64) error {
	if targetFolderID != 0 {
		target, err := uc.repo.FindByID(ctx, targetFolderID)
		if err != nil || target.UserID != userID || !target.IsFolder {
			return ErrFolderNotFound
		}
	}
	for _, id := range fileIDs {
		file, err := uc.repo.FindByID(ctx, id)
		if err != nil {
			return ErrFileNotFound
		}
		if file.UserID != userID {
			return ErrFileNotFound
		}
		file.ParentID = targetFolderID
		if err := uc.repo.Update(ctx, file); err != nil {
			return err
		}
	}
	return nil
}

func (uc *FileUsecase) ListTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error) {
	return uc.repo.FindTrash(ctx, userID, page, pageSize)
}

func (uc *FileUsecase) RestoreFiles(ctx context.Context, userID int64, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}
	return uc.repo.Restore(ctx, userID, fileIDs)
}

func (uc *FileUsecase) PermanentDelete(ctx context.Context, userID int64, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}
	return uc.repo.PermanentDelete(ctx, userID, fileIDs)
}

// GetDownloadURL generates a presigned URL based on storage type.
// For local storage, it returns "local://<store_path>" so the gateway can
// proxy the file content via the streaming RPC.
func (uc *FileUsecase) GetDownloadURL(ctx context.Context, userID, fileID int64) (string, string, error) {
	file, err := uc.repo.FindByID(ctx, fileID)
	if err != nil || file.UserID != userID || file.IsFolder {
		return "", "", ErrFileNotFound
	}

	store, err := uc.repo.FindStoreByMD5(ctx, file.FileMD5)
	if err != nil {
		return "", "", ErrFileNotFound
	}

	// Refresh last-access time for LRU ordering
	_ = uc.repo.UpdateLastAccessed(ctx, file.FileMD5)

	var downloadURL string
	switch store.StorageType {
	case StorageLocal, StorageLocalEC:
		downloadURL = "local://" + store.StorePath
	case StorageOSS:
		downloadURL, err = uc.cloudStore.PresignGetURL(ctx, store.StorePath, time.Hour)
	default:
		return "", "", fmt.Errorf("unsupported storage type: %s", store.StorageType)
	}
	if err != nil {
		return "", "", err
	}
	return downloadURL, file.Name, nil
}

func (uc *FileUsecase) CreateShare(ctx context.Context, userID, fileID int64, expireDays int32, password string) (*Share, error) {
	share := &Share{UserID: userID, FileID: fileID, Password: password}
	if expireDays > 0 {
		expireAt := time.Now().AddDate(0, 0, int(expireDays))
		share.ExpireAt = &expireAt
	}
	err := uc.repo.CreateShare(ctx, share)
	if err != nil {
		return nil, err
	}
	return share, nil
}

func (uc *FileUsecase) GetShare(ctx context.Context, shareID, password string) (*File, error) {
	share, err := uc.repo.FindShareByID(ctx, shareID)
	if err != nil {
		return nil, ErrShareNotFound
	}
	if share.ExpireAt != nil && share.ExpireAt.Before(time.Now()) {
		return nil, ErrShareExpired
	}
	if share.Password != "" && share.Password != password {
		return nil, ErrInvalidPassword
	}
	return uc.repo.FindByID(ctx, share.FileID)
}

func (uc *FileUsecase) SearchFiles(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error) {
	return uc.repo.Search(ctx, userID, keyword, page, pageSize)
}

// OpenLocalFile opens a locally stored file for streaming download.
// Returns the file handle, name, size, and MIME type.
func (uc *FileUsecase) OpenLocalFile(ctx context.Context, userID, fileID int64) (io.ReadCloser, string, int64, string, error) {
	file, err := uc.repo.FindByID(ctx, fileID)
	if err != nil || file.UserID != userID || file.IsFolder {
		return nil, "", 0, "", ErrFileNotFound
	}

	store, err := uc.repo.FindStoreByMD5(ctx, file.FileMD5)
	if err != nil {
		return nil, "", 0, "", ErrFileNotFound
	}

	if store.StorageType != StorageLocal && store.StorageType != StorageLocalEC {
		return nil, "", 0, "", errors.New("file is not stored locally")
	}

	// Refresh LRU ordering
	_ = uc.repo.UpdateLastAccessed(ctx, file.FileMD5)

	if store.StorageType == StorageLocalEC {
		rc, size, err := uc.reconstructFromShards(ctx, store.ID)
		if err != nil {
			return nil, "", 0, "", err
		}
		return rc, file.Name, size, detectMIME(file.Name), nil
	}

	localPath := filepath.Join(uc.storeDir, store.StorePath)
	f, err := os.Open(localPath)
	if err != nil {
		return nil, "", 0, "", err
	}
	return f, file.Name, file.Size, detectMIME(file.Name), nil
}

var mediaExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".webp": true, ".svg": true, ".ico": true,
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
	".webm": true, ".flv": true, ".wmv": true,
}

func isMediaFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return mediaExtensions[ext]
}

func detectMIME(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
