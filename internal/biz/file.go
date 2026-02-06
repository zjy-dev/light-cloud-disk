package biz

import (
	"context"
	"errors"
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
	// 文件操作
	Create(ctx context.Context, file *File) (*File, error)
	FindByID(ctx context.Context, id int64) (*File, error)
	FindByUserAndParent(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error)
	Update(ctx context.Context, file *File) error
	SoftDelete(ctx context.Context, ids []int64) error
	Restore(ctx context.Context, ids []int64) error
	PermanentDelete(ctx context.Context, ids []int64) error
	FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error)
	Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error)

	// 文件存储
	FindStoreByMD5(ctx context.Context, md5 string) (*FileStore, error)
	CreateStore(ctx context.Context, store *FileStore) error
	IncrStoreRefCount(ctx context.Context, md5 string) error
	DecrStoreRefCount(ctx context.Context, md5 string) error

	// 分享
	CreateShare(ctx context.Context, share *Share) error
	FindShareByID(ctx context.Context, id string) (*Share, error)
	DeleteExpiredShares(ctx context.Context) error

	// 分块上传
	GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error)
	SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error
	ClearChunkInfo(ctx context.Context, fileMD5 string) error
}

type FileUsecase struct {
	repo     FileRepo
	userRepo UserRepo
	log      *log.Helper
}

func NewFileUsecase(repo FileRepo, userRepo UserRepo, logger log.Logger) *FileUsecase {
	return &FileUsecase{
		repo:     repo,
		userRepo: userRepo,
		log:      log.NewHelper(logger),
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

func (uc *FileUsecase) SaveChunk(ctx context.Context, fileMD5 string, chunkIndex int32, chunkSize int64) error {
	chunk := &ChunkInfo{
		FileMD5:    fileMD5,
		ChunkIndex: chunkIndex,
		ChunkSize:  chunkSize,
		Uploaded:   true,
	}
	return uc.repo.SaveChunkInfo(ctx, chunk)
}

func (uc *FileUsecase) MergeChunks(ctx context.Context, userID, parentID int64, fileName, fileMD5 string, fileSize int64) (*File, error) {
	store, _ := uc.repo.FindStoreByMD5(ctx, fileMD5)
	if store != nil {
		uc.repo.IncrStoreRefCount(ctx, fileMD5)
	}

	file := &File{
		UserID:   userID,
		ParentID: parentID,
		Name:     fileName,
		FileMD5:  fileMD5,
		Size:     fileSize,
		IsFolder: false,
	}

	createdFile, err := uc.repo.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	uc.repo.ClearChunkInfo(ctx, fileMD5)
	uc.userRepo.UpdateStorageUsed(ctx, userID, fileSize)

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
	return uc.repo.SoftDelete(ctx, fileIDs)
}

func (uc *FileUsecase) MoveFiles(ctx context.Context, userID int64, fileIDs []int64, targetFolderID int64) error {
	for _, id := range fileIDs {
		file, err := uc.repo.FindByID(ctx, id)
		if err != nil {
			continue
		}
		file.ParentID = targetFolderID
		uc.repo.Update(ctx, file)
	}
	return nil
}

func (uc *FileUsecase) ListTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error) {
	return uc.repo.FindTrash(ctx, userID, page, pageSize)
}

func (uc *FileUsecase) RestoreFiles(ctx context.Context, userID int64, fileIDs []int64) error {
	return uc.repo.Restore(ctx, fileIDs)
}

func (uc *FileUsecase) PermanentDelete(ctx context.Context, userID int64, fileIDs []int64) error {
	return uc.repo.PermanentDelete(ctx, fileIDs)
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
