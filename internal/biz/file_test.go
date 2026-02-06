package biz

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockFileRepo struct {
	mock.Mock
}

func (m *MockFileRepo) Create(ctx context.Context, file *File) (*File, error) {
	args := m.Called(ctx, file)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*File), args.Error(1)
}

func (m *MockFileRepo) FindByID(ctx context.Context, id int64) (*File, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*File), args.Error(1)
}

func (m *MockFileRepo) FindByUserAndParent(ctx context.Context, userID, parentID int64, page, pageSize int32) ([]*File, int64, error) {
	args := m.Called(ctx, userID, parentID, page, pageSize)
	return args.Get(0).([]*File), args.Get(1).(int64), args.Error(2)
}

func (m *MockFileRepo) Update(ctx context.Context, file *File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockFileRepo) SoftDelete(ctx context.Context, ids []int64) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockFileRepo) Restore(ctx context.Context, ids []int64) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockFileRepo) PermanentDelete(ctx context.Context, ids []int64) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockFileRepo) FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	return args.Get(0).([]*File), args.Get(1).(int64), args.Error(2)
}

func (m *MockFileRepo) Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error) {
	args := m.Called(ctx, userID, keyword, page, pageSize)
	return args.Get(0).([]*File), args.Get(1).(int64), args.Error(2)
}

func (m *MockFileRepo) FindStoreByMD5(ctx context.Context, md5 string) (*FileStore, error) {
	args := m.Called(ctx, md5)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FileStore), args.Error(1)
}

func (m *MockFileRepo) CreateStore(ctx context.Context, store *FileStore) error {
	args := m.Called(ctx, store)
	return args.Error(0)
}

func (m *MockFileRepo) IncrStoreRefCount(ctx context.Context, md5 string) error {
	args := m.Called(ctx, md5)
	return args.Error(0)
}

func (m *MockFileRepo) DecrStoreRefCount(ctx context.Context, md5 string) error {
	args := m.Called(ctx, md5)
	return args.Error(0)
}

func (m *MockFileRepo) CreateShare(ctx context.Context, share *Share) error {
	args := m.Called(ctx, share)
	return args.Error(0)
}

func (m *MockFileRepo) FindShareByID(ctx context.Context, id string) (*Share, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Share), args.Error(1)
}

func (m *MockFileRepo) DeleteExpiredShares(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockFileRepo) GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error) {
	args := m.Called(ctx, fileMD5)
	return args.Get(0).([]int32), args.Error(1)
}

func (m *MockFileRepo) SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error {
	args := m.Called(ctx, chunk)
	return args.Error(0)
}

func (m *MockFileRepo) ClearChunkInfo(ctx context.Context, fileMD5 string) error {
	args := m.Called(ctx, fileMD5)
	return args.Error(0)
}

func TestFileUsecase_CheckUpload(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("秒传 - 文件已存在", func(t *testing.T) {
		existingStore := &FileStore{
			ID:        1,
			FileMD5:   "abc123",
			Size:      1024,
			StorePath: "/store/abc123/file.txt",
		}
		mockFileRepo.On("FindStoreByMD5", ctx, "abc123").Return(existingStore, nil).Once()

		canFastUpload, chunks, err := uc.CheckUpload(ctx, "abc123", 1024, 1)

		assert.NoError(t, err)
		assert.True(t, canFastUpload)
		assert.Nil(t, chunks)
	})

	t.Run("断点续传 - 部分分块已上传", func(t *testing.T) {
		mockFileRepo.On("FindStoreByMD5", ctx, "def456").Return(nil, ErrFileNotFound).Once()
		mockFileRepo.On("GetUploadedChunks", ctx, "def456").Return([]int32{1, 2, 3}, nil).Once()

		canFastUpload, chunks, err := uc.CheckUpload(ctx, "def456", 10240, 5)

		assert.NoError(t, err)
		assert.False(t, canFastUpload)
		assert.Equal(t, []int32{1, 2, 3}, chunks)
	})

	t.Run("全新上传", func(t *testing.T) {
		mockFileRepo.On("FindStoreByMD5", ctx, "ghi789").Return(nil, ErrFileNotFound).Once()
		mockFileRepo.On("GetUploadedChunks", ctx, "ghi789").Return([]int32{}, nil).Once()

		canFastUpload, chunks, err := uc.CheckUpload(ctx, "ghi789", 10240, 5)

		assert.NoError(t, err)
		assert.False(t, canFastUpload)
		assert.Empty(t, chunks)
	})
}

func TestFileUsecase_CreateFolder(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("创建文件夹成功", func(t *testing.T) {
		mockFileRepo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
			ID:       1,
			UserID:   1,
			ParentID: 0,
			Name:     "新建文件夹",
			IsFolder: true,
		}, nil).Once()

		folder, err := uc.CreateFolder(ctx, 1, 0, "新建文件夹")

		assert.NoError(t, err)
		assert.NotNil(t, folder)
		assert.Equal(t, "新建文件夹", folder.Name)
		assert.True(t, folder.IsFolder)
	})
}

func TestFileUsecase_ListFiles(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("获取文件列表", func(t *testing.T) {
		expectedFiles := []*File{
			{ID: 1, Name: "文件夹1", IsFolder: true},
			{ID: 2, Name: "文件1.txt", IsFolder: false, Size: 1024},
		}
		mockFileRepo.On("FindByUserAndParent", ctx, int64(1), int64(0), int32(1), int32(20)).
			Return(expectedFiles, int64(2), nil).Once()

		files, total, err := uc.ListFiles(ctx, 1, 0, 1, 20)

		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, files, 2)
		assert.True(t, files[0].IsFolder)
	})
}

func TestFileUsecase_SearchFiles(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("搜索文件", func(t *testing.T) {
		expectedFiles := []*File{
			{ID: 1, Name: "测试文档.docx", Size: 2048},
			{ID: 2, Name: "测试图片.png", Size: 4096},
		}
		mockFileRepo.On("Search", ctx, int64(1), "测试", int32(1), int32(20)).
			Return(expectedFiles, int64(2), nil).Once()

		files, total, err := uc.SearchFiles(ctx, 1, "测试", 1, 20)

		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, files, 2)
	})
}

func TestFileUsecase_CreateShare(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("创建分享链接", func(t *testing.T) {
		mockFileRepo.On("CreateShare", ctx, mock.AnythingOfType("*biz.Share")).Return(nil).Once()

		share, err := uc.CreateShare(ctx, 1, 1, 7, "1234")

		assert.NoError(t, err)
		assert.NotNil(t, share)
		assert.Equal(t, "1234", share.Password)
		assert.NotNil(t, share.ExpireAt)
	})

	t.Run("创建永久分享", func(t *testing.T) {
		mockFileRepo.On("CreateShare", ctx, mock.AnythingOfType("*biz.Share")).Return(nil).Once()

		share, err := uc.CreateShare(ctx, 1, 1, 0, "")

		assert.NoError(t, err)
		assert.NotNil(t, share)
		assert.Empty(t, share.Password)
		assert.Nil(t, share.ExpireAt)
	})
}

func TestFileUsecase_GetShare(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("获取分享成功", func(t *testing.T) {
		share := &Share{
			ID:       "abc123",
			UserID:   1,
			FileID:   1,
			Password: "",
		}
		file := &File{
			ID:   1,
			Name: "分享的文件.txt",
			Size: 1024,
		}
		mockFileRepo.On("FindShareByID", ctx, "abc123").Return(share, nil).Once()
		mockFileRepo.On("FindByID", ctx, int64(1)).Return(file, nil).Once()

		result, err := uc.GetShare(ctx, "abc123", "")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "分享的文件.txt", result.Name)
	})

	t.Run("分享已过期", func(t *testing.T) {
		expiredTime := time.Now().Add(-24 * time.Hour)
		share := &Share{
			ID:       "expired123",
			ExpireAt: &expiredTime,
		}
		mockFileRepo.On("FindShareByID", ctx, "expired123").Return(share, nil).Once()

		result, err := uc.GetShare(ctx, "expired123", "")

		assert.Error(t, err)
		assert.Equal(t, ErrShareExpired, err)
		assert.Nil(t, result)
	})

	t.Run("密码错误", func(t *testing.T) {
		share := &Share{
			ID:       "pwd123",
			Password: "correct",
		}
		mockFileRepo.On("FindShareByID", ctx, "pwd123").Return(share, nil).Once()

		result, err := uc.GetShare(ctx, "pwd123", "wrong")

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidPassword, err)
		assert.Nil(t, result)
	})
}

func TestFileUsecase_DeleteAndRestore(t *testing.T) {
	mockFileRepo := new(MockFileRepo)
	mockUserRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewFileUsecase(mockFileRepo, mockUserRepo, logger)

	ctx := context.Background()

	t.Run("删除文件到回收站", func(t *testing.T) {
		mockFileRepo.On("SoftDelete", ctx, []int64{1, 2, 3}).Return(nil).Once()

		err := uc.DeleteFiles(ctx, 1, []int64{1, 2, 3})

		assert.NoError(t, err)
	})

	t.Run("恢复文件", func(t *testing.T) {
		mockFileRepo.On("Restore", ctx, []int64{1, 2}).Return(nil).Once()

		err := uc.RestoreFiles(ctx, 1, []int64{1, 2})

		assert.NoError(t, err)
	})

	t.Run("彻底删除", func(t *testing.T) {
		mockFileRepo.On("PermanentDelete", ctx, []int64{1}).Return(nil).Once()

		err := uc.PermanentDelete(ctx, 1, []int64{1})

		assert.NoError(t, err)
	})
}
