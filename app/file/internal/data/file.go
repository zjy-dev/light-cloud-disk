package data

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	FileMD5        string    `gorm:"uniqueIndex;size:32;not null"`
	Size           int64     `gorm:"not null"`
	StorePath      string    `gorm:"size:512;not null"`
	StorageType    string    `gorm:"size:16;not null;default:seaweedfs"`
	RefCount       int32     `gorm:"default:1"`
	LastAccessedAt time.Time `gorm:"autoUpdateTime"`
	CreatedAt      time.Time
}

func (FileStorePO) TableName() string {
	return "file_stores"
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

// Redis operations for chunk upload
func (r *fileRepo) GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error) {
	key := fmt.Sprintf("upload:%s:chunks", fileMD5)
	data, err := r.data.redis.Get(ctx, key).Bytes()
	if err != nil {
		return []int32{}, nil
	}

	var chunks []int32
	json.Unmarshal(data, &chunks)
	sort.Slice(chunks, func(i, j int) bool { return chunks[i] < chunks[j] })
	return chunks, nil
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
	return "/tmp/light-cloud-disk/store"
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

func (r *fileRepo) SaveChunkInfo(ctx context.Context, chunk *biz.ChunkInfo) error {
	key := fmt.Sprintf("upload:%s:chunks", chunk.FileMD5)

	chunks, _ := r.GetUploadedChunks(ctx, chunk.FileMD5)
	chunkSet := make(map[int32]struct{}, len(chunks)+1)
	for _, v := range chunks {
		chunkSet[v] = struct{}{}
	}
	chunkSet[chunk.ChunkIndex] = struct{}{}

	merged := make([]int32, 0, len(chunkSet))
	for v := range chunkSet {
		merged = append(merged, v)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })

	data, _ := json.Marshal(merged)
	return r.data.redis.Set(ctx, key, data, 24*time.Hour).Err()
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
