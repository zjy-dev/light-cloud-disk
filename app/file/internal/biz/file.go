package biz

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrFolderNotFound  = errors.New("folder not found")
	ErrShareNotFound   = errors.New("share not found")
	ErrShareExpired    = errors.New("share expired")
	ErrInvalidPassword = errors.New("invalid share password")
	ErrStorageExceeded = errors.New("storage limit exceeded")
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
	ID        int64
	FileMD5   string
	Size      int64
	StorePath string
	RefCount  int32
	CreatedAt time.Time
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

type FileRepo interface {
	// File operations
	Create(ctx context.Context, file *File) (*File, error)
	FindByID(ctx context.Context, id int64) (*File, error)
	FindByUserAndParent(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error)
	Update(ctx context.Context, file *File) error
	SoftDelete(ctx context.Context, userID int64, ids []int64) error
	Restore(ctx context.Context, userID int64, ids []int64) error
	PermanentDelete(ctx context.Context, userID int64, ids []int64) error
	FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error)
	Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error)

	// File store
	FindStoreByMD5(ctx context.Context, md5 string) (*FileStore, error)
	CreateStore(ctx context.Context, store *FileStore) error
	IncrStoreRefCount(ctx context.Context, md5 string) error
	DecrStoreRefCount(ctx context.Context, md5 string) error

	// Share
	CreateShare(ctx context.Context, share *Share) error
	FindShareByID(ctx context.Context, id string) (*Share, error)
	DeleteExpiredShares(ctx context.Context) error

	// Chunk upload (Redis)
	GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error)
	SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error
	MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error)
	SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error
	ClearChunkInfo(ctx context.Context, fileMD5 string) error
}

// UserClient is the interface for calling user-service via gRPC.
// This replaces the direct UserRepo dependency from the old monolith.
type UserClient interface {
	UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error
}

type FileUsecase struct {
	repo       FileRepo
	userClient UserClient
	log        *log.Helper
}

func NewFileUsecase(repo FileRepo, userClient UserClient, logger log.Logger) *FileUsecase {
	return &FileUsecase{
		repo:       repo,
		userClient: userClient,
		log:        log.NewHelper(logger),
	}
}

func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, error) {
	store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if err == nil && store != nil {
		return true, nil, nil
	}

	uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
	if err != nil {
		return false, nil, err
	}

	return false, uploadedChunks, nil
}

func (uc *FileUsecase) SaveChunk(ctx context.Context, fileMD5 string, chunkIndex int32, chunkSize int64, chunkData []byte) error {
	if int64(len(chunkData)) != chunkSize {
		return errors.New("chunk size mismatch")
	}

	if err := uc.repo.SaveChunkData(ctx, fileMD5, chunkIndex, chunkData); err != nil {
		return err
	}

	chunk := &ChunkInfo{
		FileMD5:    fileMD5,
		ChunkIndex: chunkIndex,
		ChunkSize:  chunkSize,
		Uploaded:   true,
	}
	return uc.repo.SaveChunkInfo(ctx, chunk)
}

func (uc *FileUsecase) MergeChunks(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64, totalChunks int32) (*File, error) {
	store, _ := uc.repo.FindStoreByMD5(ctx, fileMD5)
	storePath := ""

	if store != nil {
		if err := uc.repo.IncrStoreRefCount(ctx, fileMD5); err != nil {
			return nil, err
		}
		storePath = store.StorePath
	} else {
		mergedPath, err := uc.repo.MergeChunkData(ctx, fileMD5, fileName, totalChunks)
		if err != nil {
			return nil, err
		}
		storePath = mergedPath

		if err := uc.repo.CreateStore(ctx, &FileStore{
			FileMD5:   fileMD5,
			Size:      fileSize,
			StorePath: mergedPath,
			RefCount:  1,
		}); err != nil {
			return nil, err
		}
	}

	file := &File{
		UserID:   userID,
		ParentID: parentID,
		Name:     fileName,
		FileMD5:  fileMD5,
		Size:     fileSize,
		IsFolder: false,
		Path:     storePath,
	}

	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	if err := uc.repo.ClearChunkInfo(ctx, fileMD5); err != nil {
		uc.log.Warnf("failed to clear chunk info for %s: %v", fileMD5, err)
	}

	// Call user-service via gRPC to update storage usage
	if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
		uc.log.Warnf("failed to update storage used for user %d: %v", userID, err)
	}

	return createdFile, nil
}

func (uc *FileUsecase) ListFiles(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error) {
	return uc.repo.FindByUserAndParent(ctx, userID, parentID, page, pageSize)
}

func (uc *FileUsecase) CreateFolder(ctx context.Context, userID, parentID int64, name string) (*File, error) {
	folder := &File{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		IsFolder: true,
	}
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

func (uc *FileUsecase) GetDownloadURL(ctx context.Context, userID, fileID int64) (string, string, error) {
	file, err := uc.repo.FindByID(ctx, fileID)
	if err != nil || file.UserID != userID || file.IsFolder {
		return "", "", ErrFileNotFound
	}

	store, err := uc.repo.FindStoreByMD5(ctx, file.FileMD5)
	if err != nil {
		return "", "", ErrFileNotFound
	}

	downloadURL := store.StorePath
	if prefix := strings.TrimSpace(os.Getenv("DOWNLOAD_URL_PREFIX")); prefix != "" {
		downloadURL = strings.TrimRight(prefix, "/") + "/" + filepath.Base(store.StorePath)
	}

	return downloadURL, file.Name, nil
}

func (uc *FileUsecase) CreateShare(ctx context.Context, userID, fileID int64, expireDays int32, password string) (*Share, error) {
	share := &Share{
		UserID:   userID,
		FileID:   fileID,
		Password: password,
	}

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
