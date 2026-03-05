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

// Storage type constants
const (
	StorageLocal     = "local"     // files kept on local disk (Mode A)
	StorageSeaweedFS = "seaweedfs" // files in SeaweedFS S3 (Mode B)
	StorageOSS       = "oss"       // cold-tier files in Alibaba Cloud OSS

	ModeLocal = "local" // small-server mode: local disk as primary
	ModeS3    = "s3"    // large-server mode: SeaweedFS as primary
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
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, bool, error) {
	// 1) Check instant upload by MD5 deduplication
	store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if err == nil && store != nil {
		return true, nil, false, nil // instant-upload hit
	}

	// 2) Check primary storage availability
	used, _ := uc.repo.GetDiskUsage(ctx, uc.primaryDiskType())
	if used+fileSize > uc.storageCfg.PrimaryMaxBytes {
		return false, nil, true, nil // primary storage full
	}

	// 3) Return uploaded chunks for resumable upload
	uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
	if err != nil {
		return false, nil, false, err
	}
	return false, uploadedChunks, false, nil
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
				}
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
