package biz

import (
"context"
"errors"
"io"
"mime"
"os"
"path/filepath"
"strings"
"time"

"github.com/go-kratos/kratos/v2/log"
)

// Storage type constants.
const (
StorageSeaweedFS = "seaweedfs"
StorageOSS       = "oss"
)

var (
ErrFileNotFound    = errors.New("file not found")
ErrFolderNotFound  = errors.New("folder not found")
ErrShareNotFound   = errors.New("share not found")
ErrShareExpired    = errors.New("share expired")
ErrInvalidPassword = errors.New("invalid share password")
ErrStorageExceeded = errors.New("storage limit exceeded")
ErrDiskFull        = errors.New("local disk full, please retry later")
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

// StorageConfig holds storage tier configuration (from conf.proto).
type StorageConfig struct {
	LocalMaxBytes          int64
	SeaweedFSMaxBytes      int64
	SeaweedFSThresholdPct  int32 // e.g. 80 means 80%
}

// FileRepo is the data-access interface for file metadata + chunk operations.
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

	// FileStore CRUD
	FindStoreByMD5(ctx context.Context, md5 string) (*FileStore, error)
	CreateStore(ctx context.Context, store *FileStore) error
	IncrStoreRefCount(ctx context.Context, md5 string) error
	DecrStoreRefCount(ctx context.Context, md5 string) error
	UpdateStorageLocation(ctx context.Context, fileMD5 string, storageType string, newPath string) error
	UpdateLastAccessed(ctx context.Context, fileMD5 string) error
	FindLRUStores(ctx context.Context, storageType string, limit int) ([]*FileStore, error)
	SumSizeByStorageType(ctx context.Context, storageType string) (int64, error)

	// Share
	CreateShare(ctx context.Context, share *Share) error
	FindShareByID(ctx context.Context, id string) (*Share, error)
	DeleteExpiredShares(ctx context.Context) error

	// Chunk upload (Redis + local disk)
	GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error)
	SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error
	MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error)
	SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error
	ClearChunkInfo(ctx context.Context, fileMD5 string) error

	// Disk usage (Redis atomic counters)
	GetDiskUsage(ctx context.Context, diskType string) (int64, error)
	IncrDiskUsage(ctx context.Context, diskType string, delta int64) error
}

// UserClient is the cross-service gRPC interface for user operations.
type UserClient interface {
	UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error
}

// ObjectStorage is the interface for SeaweedFS (or any S3-compatible storage).
type ObjectStorage interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// CloudStorage is the interface for cloud object storage (Alibaba Cloud OSS, etc.).
type CloudStorage interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// CloudMigrateMessage is sent to Kafka to trigger async cloud migration.
type CloudMigrateMessage struct {
	FileMD5   string `json:"file_md5"`
	SourceKey string `json:"source_key"`
	FileSize  int64  `json:"file_size"`
}

// ThumbnailMessage triggers async thumbnail generation.
type ThumbnailMessage struct {
	FileID   int64  `json:"file_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
}

// MessageProducer sends async messages to Kafka.
type MessageProducer interface {
	SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
	SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
	Close() error
}

// FileUsecase is the core business logic for file operations.
type FileUsecase struct {
	repo       FileRepo
	userClient UserClient
	mq         MessageProducer
	objStore   ObjectStorage  // SeaweedFS
	cloudStore CloudStorage   // OSS
	storageCfg *StorageConfig
	log        *log.Helper
}

func NewFileUsecase(
repo FileRepo,
userClient UserClient,
mq MessageProducer,
objStore ObjectStorage,
cloudStore CloudStorage,
storageCfg *StorageConfig,
logger log.Logger,
) *FileUsecase {
	return &FileUsecase{
		repo:       repo,
		userClient: userClient,
		mq:         mq,
		objStore:   objStore,
		cloudStore: cloudStore,
		storageCfg: storageCfg,
		log:        log.NewHelper(logger),
	}
}

// CheckUpload checks fast upload (dedup) and resume upload, also checks local disk availability.
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, bool, error) {
	// 1. Check fast upload (dedup by MD5)
	store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if err == nil && store != nil {
		return true, nil, false, nil // fast upload
	}

	// 2. Check local disk availability
	localUsed, _ := uc.repo.GetDiskUsage(ctx, "local")
	if localUsed+fileSize > uc.storageCfg.LocalMaxBytes {
		return false, nil, true, nil // disk full
	}

	// 3. Return uploaded chunks for resume
	uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
	if err != nil {
		return false, nil, false, err
	}
	return false, uploadedChunks, false, nil
}

// SaveChunk saves a single chunk to local disk.
func (uc *FileUsecase) SaveChunk(ctx context.Context, fileMD5 string, chunkIndex int32, chunkSize int64, chunkData []byte) error {
	if int64(len(chunkData)) != chunkSize {
		return errors.New("chunk size mismatch")
	}
	if err := uc.repo.SaveChunkData(ctx, fileMD5, chunkIndex, chunkData); err != nil {
		return err
	}
	// Track local disk usage
	_ = uc.repo.IncrDiskUsage(ctx, "local", chunkSize)

	chunk := &ChunkInfo{
		FileMD5:    fileMD5,
		ChunkIndex: chunkIndex,
		ChunkSize:  chunkSize,
		Uploaded:   true,
	}
	return uc.repo.SaveChunkInfo(ctx, chunk)
}

// MergeChunks merges chunks, uploads to SeaweedFS (or OSS fallback), cleans local disk.
func (uc *FileUsecase) MergeChunks(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64, totalChunks int32) (*File, error) {
	// 1. Check dedup (fast upload via existing store)
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
			uc.log.Warnf("failed to update storage used for user %d: %v", userID, err)
		}
		return createdFile, nil
	}

	// 2. Merge chunks to local disk
	mergedPath, err := uc.repo.MergeChunkData(ctx, fileMD5, fileName, totalChunks)
	if err != nil {
		return nil, err
	}

	// 3. Determine storage target: SeaweedFS or direct OSS
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	objectKey := fileMD5 + ext
	storageType := StorageSeaweedFS
	storePath := objectKey

	swfUsed, _ := uc.repo.GetDiskUsage(ctx, "seaweedfs")
	if swfUsed+fileSize <= uc.storageCfg.SeaweedFSMaxBytes {
		// Upload to SeaweedFS
		localFile, err := os.Open(mergedPath)
		if err != nil {
			return nil, err
		}
		defer localFile.Close()

		if err := uc.objStore.Put(ctx, objectKey, localFile, fileSize); err != nil {
			return nil, err
		}
		_ = uc.repo.IncrDiskUsage(ctx, "seaweedfs", fileSize)
	} else {
		// SeaweedFS full -> upload directly to OSS
		localFile, err := os.Open(mergedPath)
		if err != nil {
			return nil, err
		}
		defer localFile.Close()

		if err := uc.cloudStore.Put(ctx, objectKey, localFile, fileSize); err != nil {
			return nil, err
		}
		storageType = StorageOSS
	}

	// 4. Remove local merged file and track disk usage
	_ = os.Remove(mergedPath)
	_ = uc.repo.IncrDiskUsage(ctx, "local", -fileSize)

	// 5. Create file_stores record
	if err := uc.repo.CreateStore(ctx, &FileStore{
		FileMD5:     fileMD5,
		Size:        fileSize,
		StorePath:   storePath,
		StorageType: storageType,
		RefCount:    1,
	}); err != nil {
		return nil, err
	}

	// 6. Create file record
	file := &File{
		UserID: userID, ParentID: parentID, Name: fileName,
		FileMD5: fileMD5, Size: fileSize, IsFolder: false,
		Path: storePath,
	}
	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	// 7. Cleanup
	if err := uc.repo.ClearChunkInfo(ctx, fileMD5); err != nil {
		uc.log.Warnf("failed to clear chunk info for %s: %v", fileMD5, err)
	}
	if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
		uc.log.Warnf("failed to update storage used for user %d: %v", userID, err)
	}

	// 8. Check SeaweedFS threshold -> trigger LRU eviction
	if storageType == StorageSeaweedFS {
		uc.maybeEvictToCloud(ctx, swfUsed+fileSize)
	}

	// 9. Send thumbnail message for media files
	if uc.mq != nil && isMediaFile(fileName) {
		thumbMsg := &ThumbnailMessage{
			FileID:   createdFile.ID,
			FilePath: storePath,
			FileType: detectMIME(fileName),
		}
		if err := uc.mq.SendThumbnailMessage(ctx, thumbMsg); err != nil {
			uc.log.Warnf("failed to send thumbnail message for file %d: %v", createdFile.ID, err)
		}
	}

	return createdFile, nil
}

// maybeEvictToCloud checks if SeaweedFS usage exceeds the threshold and triggers LRU cloud migration.
func (uc *FileUsecase) maybeEvictToCloud(ctx context.Context, currentUsed int64) {
	threshold := uc.storageCfg.SeaweedFSMaxBytes * int64(uc.storageCfg.SeaweedFSThresholdPct) / 100
	if currentUsed <= threshold {
		return
	}

	// Evict to 90% of threshold
	target := threshold * 90 / 100
	toFree := currentUsed - target

	stores, err := uc.repo.FindLRUStores(ctx, StorageSeaweedFS, 100)
	if err != nil {
		uc.log.Warnf("failed to find LRU stores for eviction: %v", err)
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
				uc.log.Warnf("failed to send cloud migrate message for %s: %v", s.FileMD5, err)
				continue
			}
		}
		freed += s.Size
	}
	if freed > 0 {
		uc.log.Infof("triggered LRU eviction: %d bytes enqueued for cloud migration", freed)
	}
}

// GetDiskUsage returns current disk usage information.
func (uc *FileUsecase) GetDiskUsage(ctx context.Context) (localUsed, localMax, swfUsed, swfMax int64, err error) {
	localUsed, _ = uc.repo.GetDiskUsage(ctx, "local")
	swfUsed, _ = uc.repo.GetDiskUsage(ctx, "seaweedfs")
	return localUsed, uc.storageCfg.LocalMaxBytes, swfUsed, uc.storageCfg.SeaweedFSMaxBytes, nil
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

// GetDownloadURL returns a presigned download URL based on storage type.
func (uc *FileUsecase) GetDownloadURL(ctx context.Context, userID, fileID int64) (string, string, error) {
	file, err := uc.repo.FindByID(ctx, fileID)
	if err != nil || file.UserID != userID || file.IsFolder {
		return "", "", ErrFileNotFound
	}

	store, err := uc.repo.FindStoreByMD5(ctx, file.FileMD5)
	if err != nil {
		return "", "", ErrFileNotFound
	}

	// Refresh last accessed time for LRU
	_ = uc.repo.UpdateLastAccessed(ctx, file.FileMD5)

	var downloadURL string
	switch store.StorageType {
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
