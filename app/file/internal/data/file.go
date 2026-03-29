package data

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

type FilePO struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`
	UserID    int64          `gorm:"index;not null"`
	ParentID  int64          `gorm:"index;default:0"`
	Name      string         `gorm:"size:256;not null"`
	FileMD5   string         `gorm:"size:32;index"`
	Size      int64          `gorm:"default:0"`
	IsFolder  bool           `gorm:"default:false"`
	Path      string         `gorm:"size:1024"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (FilePO) TableName() string {
	return "files"
}

type FileStorePO struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	FileMD5        string    `gorm:"uniqueIndex:idx_md5_size;size:32;not null"`
	Size           int64     `gorm:"uniqueIndex:idx_md5_size;not null"`
	StorePath      string    `gorm:"size:512;not null"`
	StorageType    string    `gorm:"size:16;not null;default:local"`
	UploadStatus   string    `gorm:"size:16;not null;default:uploading"`
	RefCount       int32     `gorm:"default:1"`
	LastAccessedAt time.Time `gorm:"autoUpdateTime"`
	CreatedAt      time.Time
}

func (FileStorePO) TableName() string {
	return "file_stores"
}

// ErasureShardPO stores individual erasure-coded shard metadata.
type ErasureShardPO struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	FileStoreID int64  `gorm:"index;not null"`
	ShardIndex  int32  `gorm:"not null"`
	ShardPath   string `gorm:"size:512;not null"`
	ShardSize   int64  `gorm:"not null"`
	CreatedAt   time.Time
}

func (ErasureShardPO) TableName() string {
	return "erasure_shards"
}

type SharePO struct {
	ID        string `gorm:"primaryKey;size:32"`
	UserID    int64  `gorm:"index;not null"`
	FileID    int64  `gorm:"not null"`
	Password  string `gorm:"size:16"`
	ExpireAt  *time.Time
	ViewCount int64 `gorm:"default:0"`
	CreatedAt time.Time
}

func (SharePO) TableName() string {
	return "shares"
}

// UploadSessionPO persists presigned multipart upload sessions.
type UploadSessionPO struct {
	ID            string `gorm:"primaryKey;size:36"`
	UserID        int64  `gorm:"index;not null"`
	ParentID      int64  `gorm:"default:0"`
	FileName      string `gorm:"size:256;not null"`
	FileMD5       string `gorm:"size:32;index;not null"`
	FileSize      int64  `gorm:"not null"`
	TotalParts    int32  `gorm:"not null"`
	PartSize      int64  `gorm:"not null"`
	StorageTarget string `gorm:"size:16;not null"`
	ObjectKey     string `gorm:"size:512;not null"`
	S3UploadID    string `gorm:"size:256;not null"`
	Status        string `gorm:"size:16;not null;default:uploading;index"`
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

func (UploadSessionPO) TableName() string {
	return "upload_sessions"
}

// UploadPartPO records individual completed parts within a session.
type UploadPartPO struct {
	SessionID  string `gorm:"primaryKey;size:36"`
	PartNumber int32  `gorm:"primaryKey"`
	ETag       string `gorm:"size:128;not null"`
	Size       int64  `gorm:"not null"`
	UploadedAt time.Time
}

func (UploadPartPO) TableName() string {
	return "upload_parts"
}

type fileRepo struct {
	data *Data
	log  *log.Helper
}

func NewFileRepo(data *Data, logger log.Logger) biz.FileRepo {
	return &fileRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *fileRepo) Create(ctx context.Context, file *biz.File) (*biz.File, error) {
	po := &FilePO{
		UserID:   file.UserID,
		ParentID: file.ParentID,
		Name:     file.Name,
		FileMD5:  file.FileMD5,
		Size:     file.Size,
		IsFolder: file.IsFolder,
		Path:     file.Path,
	}

	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}

	file.ID = po.ID
	file.CreatedAt = po.CreatedAt
	return file, nil
}

func (r *fileRepo) FindByID(ctx context.Context, id int64) (*biz.File, error) {
	var po FilePO
	if err := r.data.db.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, err
	}
	return r.poToDomain(&po), nil
}

func (r *fileRepo) FindByUserAndParent(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*biz.File, int64, error) {
	var pos []FilePO
	var total int64

	db := r.data.db.WithContext(ctx).Model(&FilePO{}).Where("user_id = ? AND parent_id = ?", userID, parentID)
	db.Count(&total)

	offset := (page - 1) * pageSize
	if err := db.Order("is_folder DESC, created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	files := make([]*biz.File, len(pos))
	for i, po := range pos {
		files[i] = r.poToDomain(&po)
	}
	return files, total, nil
}

func (r *fileRepo) Update(ctx context.Context, file *biz.File) error {
	return r.data.db.WithContext(ctx).Model(&FilePO{}).Where("id = ?", file.ID).Updates(map[string]interface{}{
		"name":      file.Name,
		"parent_id": file.ParentID,
	}).Error
}

func (r *fileRepo) SoftDelete(ctx context.Context, userID int64, ids []int64) error {
	return r.data.db.WithContext(ctx).Where("user_id = ? AND id IN ?", userID, ids).Delete(&FilePO{}).Error
}

func (r *fileRepo) Restore(ctx context.Context, userID int64, ids []int64) error {
	return r.data.db.WithContext(ctx).Unscoped().Model(&FilePO{}).
		Where("user_id = ? AND id IN ?", userID, ids).
		Update("deleted_at", nil).Error
}

func (r *fileRepo) PermanentDelete(ctx context.Context, userID int64, ids []int64) error {
	return r.data.db.WithContext(ctx).Unscoped().Where("user_id = ? AND id IN ?", userID, ids).Delete(&FilePO{}).Error
}

func (r *fileRepo) FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*biz.File, int64, error) {
	var pos []FilePO
	var total int64

	db := r.data.db.WithContext(ctx).Unscoped().Model(&FilePO{}).Where("user_id = ? AND deleted_at IS NOT NULL", userID)
	db.Count(&total)

	offset := (page - 1) * pageSize
	if err := db.Offset(int(offset)).Limit(int(pageSize)).Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	files := make([]*biz.File, len(pos))
	for i, po := range pos {
		files[i] = r.poToDomain(&po)
	}
	return files, total, nil
}

func (r *fileRepo) Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*biz.File, int64, error) {
	var pos []FilePO
	var total int64

	db := r.data.db.WithContext(ctx).Model(&FilePO{}).Where("user_id = ? AND name LIKE ?", userID, "%"+keyword+"%")
	db.Count(&total)

	offset := (page - 1) * pageSize
	if err := db.Offset(int(offset)).Limit(int(pageSize)).Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	files := make([]*biz.File, len(pos))
	for i, po := range pos {
		files[i] = r.poToDomain(&po)
	}
	return files, total, nil
}

func (r *fileRepo) FindStoreByMD5(ctx context.Context, md5 string) (*biz.FileStore, error) {
	var po FileStorePO
	if err := r.data.db.WithContext(ctx).Where("file_md5 = ?", md5).First(&po).Error; err != nil {
		return nil, err
	}
	return &biz.FileStore{
		ID:             po.ID,
		FileMD5:        po.FileMD5,
		Size:           po.Size,
		StorePath:      po.StorePath,
		StorageType:    po.StorageType,
		UploadStatus:   po.UploadStatus,
		RefCount:       po.RefCount,
		LastAccessedAt: po.LastAccessedAt,
	}, nil
}

func (r *fileRepo) CreateStore(ctx context.Context, store *biz.FileStore) error {
	po := &FileStorePO{
		FileMD5:     store.FileMD5,
		Size:        store.Size,
		StorePath:   store.StorePath,
		StorageType: store.StorageType,
		RefCount:    1,
	}
	return r.data.db.WithContext(ctx).Create(po).Error
}

func (r *fileRepo) IncrStoreRefCount(ctx context.Context, md5 string) error {
	return r.data.db.WithContext(ctx).Model(&FileStorePO{}).Where("file_md5 = ?", md5).
		UpdateColumn("ref_count", gorm.Expr("ref_count + 1")).Error
}

func (r *fileRepo) DecrStoreRefCount(ctx context.Context, md5 string) error {
	return r.data.db.WithContext(ctx).Model(&FileStorePO{}).Where("file_md5 = ?", md5).
		UpdateColumn("ref_count", gorm.Expr("ref_count - 1")).Error
}

func (r *fileRepo) CreateShare(ctx context.Context, share *biz.Share) error {
	share.ID = uuid.New().String()[:8]
	po := &SharePO{
		ID:       share.ID,
		UserID:   share.UserID,
		FileID:   share.FileID,
		Password: share.Password,
		ExpireAt: share.ExpireAt,
	}
	return r.data.db.WithContext(ctx).Create(po).Error
}

func (r *fileRepo) FindShareByID(ctx context.Context, id string) (*biz.Share, error) {
	var po SharePO
	if err := r.data.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &biz.Share{
		ID:        po.ID,
		UserID:    po.UserID,
		FileID:    po.FileID,
		Password:  po.Password,
		ExpireAt:  po.ExpireAt,
		ViewCount: po.ViewCount,
		CreatedAt: po.CreatedAt,
	}, nil
}

func (r *fileRepo) DeleteExpiredShares(ctx context.Context) error {
	return r.data.db.WithContext(ctx).Where("expire_at IS NOT NULL AND expire_at < ?", time.Now()).Delete(&SharePO{}).Error
}

// Redis operations for chunk upload — SET-based (SADD/SMEMBERS)
func (r *fileRepo) GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error) {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	members, err := r.data.redis.SMembers(ctx, key).Result()
	if err != nil {
		return []int32{}, nil
	}
	chunks := make([]int32, 0, len(members))
	for _, m := range members {
		v, _ := strconv.Atoi(m)
		chunks = append(chunks, int32(v))
	}
	return chunks, nil
}

func (r *fileRepo) AddUploadedChunk(ctx context.Context, fileMD5 string, chunkIndex int32) error {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	if err := r.data.redis.SAdd(ctx, key, chunkIndex).Err(); err != nil {
		return err
	}
	return r.data.redis.Expire(ctx, key, 24*time.Hour).Err()
}

func (r *fileRepo) IsChunkUploaded(ctx context.Context, fileMD5 string, chunkIndex int32) (bool, error) {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	return r.data.redis.SIsMember(ctx, key, chunkIndex).Result()
}

func (r *fileRepo) CountUploadedChunks(ctx context.Context, fileMD5 string) (int32, error) {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	n, err := r.data.redis.SCard(ctx, key).Result()
	return int32(n), err
}

// AcquireChunkLock acquires a per-chunk distributed lock via Redis SETNX.
func (r *fileRepo) AcquireChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("lock:chunk:%s:%d", fileMD5, chunkIndex)
	return r.data.redis.SetNX(ctx, key, "1", ttl).Result()
}

// ReleaseChunkLock releases the per-chunk lock.
func (r *fileRepo) ReleaseChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32) error {
	key := fmt.Sprintf("lock:chunk:%s:%d", fileMD5, chunkIndex)
	return r.data.redis.Del(ctx, key).Err()
}

// AcquireMergeLock acquires a distributed merge lock via Redis SETNX.
func (r *fileRepo) AcquireMergeLock(ctx context.Context, fileMD5 string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("lock:merge:%s", fileMD5)
	return r.data.redis.SetNX(ctx, key, "1", ttl).Result()
}

// ReleaseMergeLock releases the merge lock.
func (r *fileRepo) ReleaseMergeLock(ctx context.Context, fileMD5 string) error {
	key := fmt.Sprintf("lock:merge:%s", fileMD5)
	return r.data.redis.Del(ctx, key).Err()
}

// CreateStoreWithStatus creates a store record with an initial status.
// Returns the created store. If a UNIQUE constraint violation occurs (dup md5+size),
// it returns the existing record and no error.
func (r *fileRepo) CreateStoreWithStatus(ctx context.Context, store *biz.FileStore) (*biz.FileStore, error) {
	po := &FileStorePO{
		FileMD5:      store.FileMD5,
		Size:         store.Size,
		StorePath:    store.StorePath,
		StorageType:  store.StorageType,
		UploadStatus: store.UploadStatus,
		RefCount:     1,
	}
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		// Check for UNIQUE constraint violation — another instance already created the record
		var existing FileStorePO
		if findErr := r.data.db.WithContext(ctx).Where("file_md5 = ? AND size = ?", store.FileMD5, store.Size).First(&existing).Error; findErr == nil {
			return &biz.FileStore{
				ID:           existing.ID,
				FileMD5:      existing.FileMD5,
				Size:         existing.Size,
				StorePath:    existing.StorePath,
				StorageType:  existing.StorageType,
				UploadStatus: existing.UploadStatus,
				RefCount:     existing.RefCount,
			}, nil
		}
		return nil, err
	}
	store.ID = po.ID
	return store, nil
}

// UpdateStoreStatus atomically updates the upload status of a store.
func (r *fileRepo) UpdateStoreStatus(ctx context.Context, id int64, status string) error {
	return r.data.db.WithContext(ctx).Model(&FileStorePO{}).Where("id = ?", id).Update("upload_status", status).Error
}

// FindStoreByMD5AndStatus finds a store by MD5 and upload status.
func (r *fileRepo) FindStoreByMD5AndStatus(ctx context.Context, md5, status string) (*biz.FileStore, error) {
	var po FileStorePO
	if err := r.data.db.WithContext(ctx).Where("file_md5 = ? AND upload_status = ?", md5, status).First(&po).Error; err != nil {
		return nil, err
	}
	return &biz.FileStore{
		ID:           po.ID,
		FileMD5:      po.FileMD5,
		Size:         po.Size,
		StorePath:    po.StorePath,
		StorageType:  po.StorageType,
		UploadStatus: po.UploadStatus,
		RefCount:     po.RefCount,
	}, nil
}

// CreateErasureShard persists a single erasure shard record.
func (r *fileRepo) CreateErasureShard(ctx context.Context, shard *biz.ErasureShard) error {
	po := &ErasureShardPO{
		FileStoreID: shard.FileStoreID,
		ShardIndex:  shard.ShardIndex,
		ShardPath:   shard.ShardPath,
		ShardSize:   shard.ShardSize,
	}
	return r.data.db.WithContext(ctx).Create(po).Error
}

// FindErasureShards returns all shards for a given file_store_id.
func (r *fileRepo) FindErasureShards(ctx context.Context, fileStoreID int64) ([]*biz.ErasureShard, error) {
	var pos []ErasureShardPO
	if err := r.data.db.WithContext(ctx).Where("file_store_id = ?", fileStoreID).Order("shard_index ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	shards := make([]*biz.ErasureShard, len(pos))
	for i, po := range pos {
		shards[i] = &biz.ErasureShard{
			ID:          po.ID,
			FileStoreID: po.FileStoreID,
			ShardIndex:  po.ShardIndex,
			ShardPath:   po.ShardPath,
			ShardSize:   po.ShardSize,
		}
	}
	return shards, nil
}

func chunkRootDir() string {
	if v := os.Getenv("FILE_TMP_DIR"); v != "" {
		return v
	}
	return "/tmp/light-cloud-disk/chunks"
}

func storeRootDir() string {
	if v := os.Getenv("FILE_STORE_DIR"); v != "" {
		return v
	}
	return "./store"
}

func (r *fileRepo) SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error {
	_ = ctx
	chunkDir := filepath.Join(chunkRootDir(), fileMD5)
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return err
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%06d.part", chunkIndex))
	return os.WriteFile(chunkPath, data, 0o644)
}

func (r *fileRepo) MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error) {
	_ = ctx

	chunkDir := filepath.Join(chunkRootDir(), fileMD5)
	if err := os.MkdirAll(storeRootDir(), 0o755); err != nil {
		return "", err
	}

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	storePath := filepath.Join(storeRootDir(), fileMD5+ext)

	out, err := os.Create(storePath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	hasher := md5.New()
	multiWriter := io.MultiWriter(out, hasher)

	for i := int32(0); i < totalChunks; i++ {
		partPath := filepath.Join(chunkDir, fmt.Sprintf("%06d.part", i))
		part, err := os.Open(partPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(multiWriter, part); err != nil {
			part.Close()
			return "", err
		}
		part.Close()
	}

	actualMD5 := hex.EncodeToString(hasher.Sum(nil))
	if actualMD5 != fileMD5 {
		_ = os.Remove(storePath)
		return "", fmt.Errorf("merged file MD5 mismatch: expected %s, got %s", fileMD5, actualMD5)
	}

	if err := os.RemoveAll(chunkDir); err != nil {
		r.log.Warnf("Could not remove chunk directory %s: %v", chunkDir, err)
	}

	return storePath, nil
}

func (r *fileRepo) ClearChunkInfo(ctx context.Context, fileMD5 string) error {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	return r.data.redis.Del(ctx, key).Err()
}

func (r *fileRepo) UpdateStorageLocation(ctx context.Context, fileMD5 string, storageType string, newPath string) error {
	return r.data.db.WithContext(ctx).Model(&FileStorePO{}).
		Where("file_md5 = ?", fileMD5).
		Updates(map[string]interface{}{
			"storage_type": storageType,
			"store_path":   newPath,
		}).Error
}

func (r *fileRepo) UpdateLastAccessed(ctx context.Context, fileMD5 string) error {
	return r.data.db.WithContext(ctx).Model(&FileStorePO{}).
		Where("file_md5 = ?", fileMD5).
		Update("last_accessed_at", time.Now()).Error
}

func (r *fileRepo) FindLRUStores(ctx context.Context, storageType string, limit int) ([]*biz.FileStore, error) {
	var pos []FileStorePO
	if err := r.data.db.WithContext(ctx).
		Where("storage_type = ?", storageType).
		Order("last_accessed_at ASC").
		Limit(limit).
		Find(&pos).Error; err != nil {
		return nil, err
	}
	stores := make([]*biz.FileStore, len(pos))
	for i, po := range pos {
		stores[i] = &biz.FileStore{
			ID:             po.ID,
			FileMD5:        po.FileMD5,
			Size:           po.Size,
			StorePath:      po.StorePath,
			StorageType:    po.StorageType,
			UploadStatus:   po.UploadStatus,
			RefCount:       po.RefCount,
			LastAccessedAt: po.LastAccessedAt,
		}
	}
	return stores, nil
}

func (r *fileRepo) SumSizeByStorageType(ctx context.Context, storageType string) (int64, error) {
	var total int64
	if err := r.data.db.WithContext(ctx).Model(&FileStorePO{}).
		Where("storage_type = ?", storageType).
		Select("COALESCE(SUM(size), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *fileRepo) GetDiskUsage(ctx context.Context, diskType string) (int64, error) {
	key := fmt.Sprintf("disk_usage:%s", diskType)
	val, err := r.data.redis.Get(ctx, key).Int64()
	if err != nil {
		return 0, nil // key not found → 0
	}
	return val, nil
}

func (r *fileRepo) IncrDiskUsage(ctx context.Context, diskType string, delta int64) error {
	key := fmt.Sprintf("disk_usage:%s", diskType)
	return r.data.redis.IncrBy(ctx, key, delta).Err()
}

func (r *fileRepo) poToDomain(po *FilePO) *biz.File {
	file := &biz.File{
		ID:        po.ID,
		UserID:    po.UserID,
		ParentID:  po.ParentID,
		Name:      po.Name,
		FileMD5:   po.FileMD5,
		Size:      po.Size,
		IsFolder:  po.IsFolder,
		Path:      po.Path,
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}
	if po.DeletedAt.Valid {
		file.DeletedAt = &po.DeletedAt.Time
	}
	return file
}

// ---------------------------------------------------------------------------
// Upload Session operations
// ---------------------------------------------------------------------------

func (r *fileRepo) CreateUploadSession(ctx context.Context, session *biz.UploadSession) error {
	po := &UploadSessionPO{
		ID:            session.ID,
		UserID:        session.UserID,
		ParentID:      session.ParentID,
		FileName:      session.FileName,
		FileMD5:       session.FileMD5,
		FileSize:      session.FileSize,
		TotalParts:    session.TotalParts,
		PartSize:      session.PartSize,
		StorageTarget: session.StorageTarget,
		ObjectKey:     session.ObjectKey,
		S3UploadID:    session.S3UploadID,
		Status:        session.Status,
		ExpiresAt:     session.ExpiresAt,
	}
	return r.data.db.WithContext(ctx).Create(po).Error
}

func (r *fileRepo) FindUploadSession(ctx context.Context, userID int64, fileMD5 string) (*biz.UploadSession, error) {
	var po UploadSessionPO
	if err := r.data.db.WithContext(ctx).
		Where("user_id = ? AND file_md5 = ? AND status = ?", userID, fileMD5, "uploading").
		Order("created_at DESC").
		First(&po).Error; err != nil {
		return nil, err
	}
	return r.sessionToDomain(&po), nil
}

func (r *fileRepo) FindUploadSessionByID(ctx context.Context, sessionID string) (*biz.UploadSession, error) {
	var po UploadSessionPO
	if err := r.data.db.WithContext(ctx).First(&po, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	return r.sessionToDomain(&po), nil
}

func (r *fileRepo) UpdateUploadSessionStatus(ctx context.Context, sessionID, status string) error {
	return r.data.db.WithContext(ctx).Model(&UploadSessionPO{}).
		Where("id = ?", sessionID).
		Update("status", status).Error
}

func (r *fileRepo) SaveUploadPart(ctx context.Context, sessionID string, partNumber int32, etag string, size int64) error {
	po := &UploadPartPO{
		SessionID:  sessionID,
		PartNumber: partNumber,
		ETag:       etag,
		Size:       size,
		UploadedAt: time.Now(),
	}
	// Upsert: if part already exists (retry), overwrite it
	return r.data.db.WithContext(ctx).Save(po).Error
}

func (r *fileRepo) FindUploadedParts(ctx context.Context, sessionID string) ([]biz.UploadedPart, error) {
	var pos []UploadPartPO
	if err := r.data.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("part_number ASC").
		Find(&pos).Error; err != nil {
		return nil, err
	}
	parts := make([]biz.UploadedPart, len(pos))
	for i, po := range pos {
		parts[i] = biz.UploadedPart{
			SessionID:  po.SessionID,
			PartNumber: po.PartNumber,
			ETag:       po.ETag,
			Size:       po.Size,
			UploadedAt: po.UploadedAt,
		}
	}
	return parts, nil
}

func (r *fileRepo) DeleteUploadSession(ctx context.Context, sessionID string) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&UploadPartPO{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", sessionID).Delete(&UploadSessionPO{}).Error
	})
}

func (r *fileRepo) sessionToDomain(po *UploadSessionPO) *biz.UploadSession {
	return &biz.UploadSession{
		ID:            po.ID,
		UserID:        po.UserID,
		ParentID:      po.ParentID,
		FileName:      po.FileName,
		FileMD5:       po.FileMD5,
		FileSize:      po.FileSize,
		TotalParts:    po.TotalParts,
		PartSize:      po.PartSize,
		StorageTarget: po.StorageTarget,
		ObjectKey:     po.ObjectKey,
		S3UploadID:    po.S3UploadID,
		Status:        po.Status,
		CreatedAt:     po.CreatedAt,
		ExpiresAt:     po.ExpiresAt,
	}
}
