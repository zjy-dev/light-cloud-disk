package biz

import (
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
	StorageLocal     = "local"     // files kept on local disk (Mode A)
	StorageSeaweedFS = "seaweedfs" // files in SeaweedFS S3 (Mode B)
	StorageOSS       = "oss"       // cold-tier files in Alibaba Cloud OSS

	ModeLocal = "local" // small-server mode: local disk as primary
	ModeS3    = "s3"    // large-server mode: SeaweedFS as primary
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
	StorageType    string // "seaweedfs" or "oss"
	RefCount       int32
	LastAccessedAt time.Time
	CreatedAt      time.Time
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
	StorageTarget string // "seaweedfs" or "oss"
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

// StorageConfig holds storage tier configuration from conf.proto.
// Mode decides the primary storage backend:
//   - "local": local disk is primary, OSS is the eviction target
//   - "s3":    SeaweedFS is primary, OSS is the eviction target
type StorageConfig struct {
	Mode            string // "local" or "s3"
	PrimaryMaxBytes int64  // capacity of the primary storage (local disk or SeaweedFS)
	ThresholdPct    int32  // eviction trigger, e.g. 80 means 80 %
	EvictTargetPct  int32  // evict down to this % of threshold (default 90)
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

	// Share operations
	CreateShare(ctx context.Context, share *Share) error
	FindShareByID(ctx context.Context, id string) (*Share, error)
	DeleteExpiredShares(ctx context.Context) error

	// Chunk upload operations (Redis + local disk)
	GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error)
	SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error
	MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error)
	SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error
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

// ObjectStorage abstracts SeaweedFS or any S3-compatible object storage
type ObjectStorage interface {
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

// MessageProducer sends async messages to Kafka
type MessageProducer interface {
	SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
	SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
	Close() error
}

// FileUsecase contains core file business logic
type FileUsecase struct {
	repo       FileRepo
	userClient UserClient
	mq         MessageProducer
	objStore   ObjectStorage
	cloudStore CloudStorage
	storageCfg *StorageConfig
	storeDir   string // local file store directory (from Upload.StoreDir)
	log        *log.Helper
}

func NewFileUsecase(
	repo FileRepo,
	userClient UserClient,
	mq MessageProducer,
	objStore ObjectStorage,
	cloudStore CloudStorage,
	storageCfg *StorageConfig,
	storeDir string,
	logger log.Logger,
) *FileUsecase {
	return &FileUsecase{
		repo:       repo,
		userClient: userClient,
		mq:         mq,
		objStore:   objStore,
		cloudStore: cloudStore,
		storageCfg: storageCfg,
		storeDir:   storeDir,
		log:        log.NewHelper(logger),
	}
}

// primaryDiskType returns the Redis counter key for the primary storage tier.
func (uc *FileUsecase) primaryDiskType() string {
	if uc.storageCfg.Mode == ModeS3 {
		return "seaweedfs"
	}
	return "local"
}

// CheckUpload verifies instant/resumable upload and primary-storage disk availability.
// Returns: canFastUpload, uploadedChunks, diskFull, uploadMode, error
// uploadMode is "direct" (chunks through backend) or "presigned" (client uploads to S3/OSS directly).
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, bool, string, error) {
	// 1) Check instant upload by MD5 deduplication
	store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if err == nil && store != nil {
		return true, nil, false, "direct", nil // instant-upload hit
	}

	// 2) Determine upload mode
	// Mode B (s3): always presigned (data goes directly to SeaweedFS/OSS, not through backend)
	if uc.storageCfg.Mode == ModeS3 {
		return false, nil, false, "presigned", nil
	}

	// Mode A (local): check primary disk availability
	used, _ := uc.repo.GetDiskUsage(ctx, uc.primaryDiskType())
	if used+fileSize > uc.storageCfg.PrimaryMaxBytes {
		// Disk full → switch to presigned OSS upload
		return false, nil, true, "presigned", nil
	}

	// 3) Return uploaded chunks for resumable direct upload
	uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
	if err != nil {
		return false, nil, false, "direct", err
	}
	return false, uploadedChunks, false, "direct", nil
}

// SaveChunk writes a single chunk to local temp storage
func (uc *FileUsecase) SaveChunk(ctx context.Context, fileMD5 string, chunkIndex int32, chunkSize int64, chunkData []byte) error {
	if int64(len(chunkData)) != chunkSize {
		return errors.New("chunk size mismatch")
	}
	if err := uc.repo.SaveChunkData(ctx, fileMD5, chunkIndex, chunkData); err != nil {
		return err
	}
	// Track local disk usage counter
	_ = uc.repo.IncrDiskUsage(ctx, "local", chunkSize)

	chunk := &ChunkInfo{
		FileMD5:    fileMD5,
		ChunkIndex: chunkIndex,
		ChunkSize:  chunkSize,
		Uploaded:   true,
	}
	return uc.repo.SaveChunkInfo(ctx, chunk)
}

// MergeChunks merges chunks, uploads to the appropriate storage, and cleans temp files.
func (uc *FileUsecase) MergeChunks(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64, totalChunks int32) (*File, error) {
	// 1) Re-check deduplication to reuse existing storage
	store, _ := uc.repo.FindStoreByMD5(ctx, fileMD5)
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

	// 2) Merge chunks into a local file
	mergedPath, err := uc.repo.MergeChunkData(ctx, fileMD5, fileName, totalChunks)
	if err != nil {
		return nil, err
	}

	// 3) Choose storage target based on mode
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fileMD5 + ext
	storageType, storePath := uc.uploadMergedFile(ctx, mergedPath, objectKey, fileSize)

	// 4) Remove local merged file only if uploaded to remote storage
	if storageType != StorageLocal {
		_ = os.Remove(mergedPath)
		_ = uc.repo.IncrDiskUsage(ctx, "local", -fileSize)
	}

	// 5) Create file_stores record
	if err := uc.repo.CreateStore(ctx, &FileStore{
		FileMD5:     fileMD5,
		Size:        fileSize,
		StorePath:   storePath,
		StorageType: storageType,
		RefCount:    1,
	}); err != nil {
		return nil, err
	}

	// 6) Create files record
	file := &File{
		UserID: userID, ParentID: parentID, Name: fileName,
		FileMD5: fileMD5, Size: fileSize, IsFolder: false,
		Path: storePath,
	}
	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	// 7) Cleanup
	if err := uc.repo.ClearChunkInfo(ctx, fileMD5); err != nil {
		uc.log.Warnf("Could not clear chunk metadata for %s: %v", fileMD5, err)
	}
	if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
		uc.log.Warnf("Could not update storage usage for user %d: %v", userID, err)
	}

	// 8) Check primary storage threshold and trigger LRU eviction if needed
	primUsed, _ := uc.repo.GetDiskUsage(ctx, uc.primaryDiskType())
	uc.maybeEvictToCloud(ctx, primUsed)

	// 9) Send async thumbnail task for media files
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
// falling back to OSS when the primary tier is full.
func (uc *FileUsecase) uploadMergedFile(ctx context.Context, mergedPath, objectKey string, fileSize int64) (storageType, storePath string) {
	storePath = objectKey

	if uc.storageCfg.Mode == ModeS3 {
		// Mode B: SeaweedFS is primary, OSS is fallback
		primUsed, _ := uc.repo.GetDiskUsage(ctx, "seaweedfs")
		if primUsed+fileSize <= uc.storageCfg.PrimaryMaxBytes {
			localFile, err := os.Open(mergedPath)
			if err == nil {
				defer localFile.Close()
				if err := uc.objStore.Put(ctx, objectKey, localFile, fileSize); err == nil {
					_ = uc.repo.IncrDiskUsage(ctx, "seaweedfs", fileSize)
					return StorageSeaweedFS, storePath
				} else {
					uc.log.Warnf("SeaweedFS upload failed for %s, fallback to OSS: %v", objectKey, err)
				}
			} else {
				uc.log.Warnf("Open merged file failed for %s: %v", objectKey, err)
			}
		}
		// Fallback to OSS
		return uc.uploadToOSS(ctx, mergedPath, objectKey, fileSize, storePath)
	}

	// Mode A: local disk is primary, OSS is fallback
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

	// 3) New session: choose storage target
	storageTarget := uc.choosePresignedTarget(ctx, fileSize)

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fileMD5 + ext

	// Init S3 multipart upload
	var s3UploadID string
	var initErr error
	if storageTarget == StorageSeaweedFS {
		s3UploadID, initErr = uc.objStore.InitMultipartUpload(ctx, objectKey)
	} else {
		s3UploadID, initErr = uc.cloudStore.InitMultipartUpload(ctx, objectKey)
	}
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

	// Complete multipart upload on the storage backend
	if session.StorageTarget == StorageSeaweedFS {
		err = uc.objStore.CompleteMultipartUpload(ctx, session.ObjectKey, session.S3UploadID, s3Parts)
	} else {
		err = uc.cloudStore.CompleteMultipartUpload(ctx, session.ObjectKey, session.S3UploadID, s3Parts)
	}
	if err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	// Create file_stores record
	if createErr := uc.repo.CreateStore(ctx, &FileStore{
		FileMD5:     session.FileMD5,
		Size:        session.FileSize,
		StorePath:   session.ObjectKey,
		StorageType: session.StorageTarget,
		RefCount:    1,
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

	// Update disk usage counter
	_ = uc.repo.IncrDiskUsage(ctx, session.StorageTarget, session.FileSize)

	// Check eviction threshold
	primUsed, _ := uc.repo.GetDiskUsage(ctx, uc.primaryDiskType())
	uc.maybeEvictToCloud(ctx, primUsed)

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

// choosePresignedTarget decides which storage backend to use for presigned uploads.
func (uc *FileUsecase) choosePresignedTarget(ctx context.Context, fileSize int64) string {
	if uc.storageCfg.Mode == ModeS3 {
		// Mode B: SeaweedFS is primary, fallback to OSS if full
		used, _ := uc.repo.GetDiskUsage(ctx, "seaweedfs")
		if used+fileSize <= uc.storageCfg.PrimaryMaxBytes {
			return StorageSeaweedFS
		}
		return StorageOSS
	}
	// Mode A: local disk is primary, presigned goes to OSS
	return StorageOSS
}

// generatePresignedURLs creates presigned upload URLs for parts not in completedSet.
func (uc *FileUsecase) generatePresignedURLs(ctx context.Context, session *UploadSession, completedSet map[int32]bool) ([]PresignedPart, error) {
	var pending []PresignedPart
	for i := int32(1); i <= session.TotalParts; i++ {
		if completedSet[i] {
			continue
		}
		var url string
		var err error
		if session.StorageTarget == StorageSeaweedFS {
			url, err = uc.objStore.PresignUploadPart(ctx, session.ObjectKey, session.S3UploadID, i, presignedURLExpiry)
		} else {
			url, err = uc.cloudStore.PresignUploadPart(ctx, session.ObjectKey, session.S3UploadID, i, presignedURLExpiry)
		}
		if err != nil {
			return nil, fmt.Errorf("presign part %d: %w", i, err)
		}
		pending = append(pending, PresignedPart{PartNumber: i, UploadURL: url})
	}
	return pending, nil
}

// abortS3Upload calls abort on the storage backend, logging errors.
func (uc *FileUsecase) abortS3Upload(ctx context.Context, session *UploadSession) {
	var err error
	if session.StorageTarget == StorageSeaweedFS {
		err = uc.objStore.AbortMultipartUpload(ctx, session.ObjectKey, session.S3UploadID)
	} else {
		err = uc.cloudStore.AbortMultipartUpload(ctx, session.ObjectKey, session.S3UploadID)
	}
	if err != nil {
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

	// Determine which storage type holds the primary files
	primaryType := StorageLocal
	if uc.storageCfg.Mode == ModeS3 {
		primaryType = StorageSeaweedFS
	}

	stores, err := uc.repo.FindLRUStores(ctx, primaryType, 100)
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
	pType := uc.primaryDiskType()
	primaryUsed, _ = uc.repo.GetDiskUsage(ctx, pType)
	return primaryUsed, uc.storageCfg.PrimaryMaxBytes, pType, nil
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
	case StorageLocal:
		downloadURL = "local://" + store.StorePath
	case StorageOSS:
		downloadURL, err = uc.cloudStore.PresignGetURL(ctx, store.StorePath, time.Hour)
	default: // seaweedfs
		downloadURL, err = uc.objStore.PresignGetURL(ctx, store.StorePath, time.Hour)
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

	if store.StorageType != StorageLocal {
		return nil, "", 0, "", errors.New("file is not stored locally")
	}

	// Refresh LRU ordering
	_ = uc.repo.UpdateLastAccessed(ctx, file.FileMD5)

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
