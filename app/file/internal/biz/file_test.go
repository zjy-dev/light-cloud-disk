package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileRepo is a testify mock for FileRepo.
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
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*File), args.Get(1).(int64), args.Error(2)
}

func (m *MockFileRepo) Update(ctx context.Context, file *File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockFileRepo) SoftDelete(ctx context.Context, userID int64, ids []int64) error {
	args := m.Called(ctx, userID, ids)
	return args.Error(0)
}

func (m *MockFileRepo) Restore(ctx context.Context, userID int64, ids []int64) error {
	args := m.Called(ctx, userID, ids)
	return args.Error(0)
}

func (m *MockFileRepo) PermanentDelete(ctx context.Context, userID int64, ids []int64) error {
	args := m.Called(ctx, userID, ids)
	return args.Error(0)
}

func (m *MockFileRepo) FindTrash(ctx context.Context, userID int64, page, pageSize int32) ([]*File, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*File), args.Get(1).(int64), args.Error(2)
}

func (m *MockFileRepo) Search(ctx context.Context, userID int64, keyword string, page, pageSize int32) ([]*File, int64, error) {
	args := m.Called(ctx, userID, keyword, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]int32), args.Error(1)
}

func (m *MockFileRepo) SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error {
	args := m.Called(ctx, fileMD5, chunkIndex, data)
	return args.Error(0)
}

func (m *MockFileRepo) MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error) {
	args := m.Called(ctx, fileMD5, fileName, totalChunks)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.String(0), args.Error(1)
}

func (m *MockFileRepo) SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error {
	args := m.Called(ctx, chunk)
	return args.Error(0)
}

func (m *MockFileRepo) ClearChunkInfo(ctx context.Context, fileMD5 string) error {
	args := m.Called(ctx, fileMD5)
	return args.Error(0)
}

// MockUserClient is a testify mock for UserClient.
type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error {
	args := m.Called(ctx, userID, delta)
	return args.Error(0)
}

func newTestFileUsecase(repo *MockFileRepo, userClient *MockUserClient) *FileUsecase {
	return NewFileUsecase(repo, userClient, log.DefaultLogger)
}

// --- CheckUpload tests ---

func TestCheckUpload_FastUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5:   "abc123",
		Size:      1024,
		StorePath: "/store/abc123",
	}, nil)

	canFast, chunks, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.True(t, canFast)
	assert.Nil(t, chunks)
	repo.AssertExpectations(t)
}

func TestCheckUpload_ResumeUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32{0, 1, 3}, nil)

	canFast, chunks, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Equal(t, []int32{0, 1, 3}, chunks)
	repo.AssertExpectations(t)
}

func TestCheckUpload_NewUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32(nil), nil)

	canFast, chunks, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Nil(t, chunks)
	repo.AssertExpectations(t)
}

// --- SaveChunk tests ---

func TestSaveChunk_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("SaveChunkInfo", ctx, mock.MatchedBy(func(c *ChunkInfo) bool {
		return c.FileMD5 == "abc123" && c.ChunkIndex == 2 && c.ChunkSize == int64(len([]byte("chunk-data"))) && c.Uploaded
	})).Return(nil)
	repo.On("SaveChunkData", ctx, "abc123", int32(2), []byte("chunk-data")).Return(nil)

	err := uc.SaveChunk(ctx, "abc123", 2, int64(len([]byte("chunk-data"))), []byte("chunk-data"))

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// --- MergeChunks tests ---

func TestMergeChunks_NewStore(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("MergeChunkData", ctx, "abc123", "file.zip", int32(2)).Return("/tmp/store/abc123.zip", nil)
	repo.On("CreateStore", ctx, mock.MatchedBy(func(s *FileStore) bool {
		return s.FileMD5 == "abc123" && s.Size == 2048 && s.StorePath == "/tmp/store/abc123.zip"
	})).Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID:      1,
		UserID:  100,
		Name:    "file.zip",
		FileMD5: "abc123",
		Size:    2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), file.ID)
	assert.Equal(t, "file.zip", file.Name)
	repo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestMergeChunks_ExistingStore(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5:   "abc123",
		StorePath: "/tmp/store/abc123.zip",
		RefCount:  1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID:      2,
		UserID:  100,
		Name:    "file.zip",
		FileMD5: "abc123",
		Size:    2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), file.ID)
	repo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestMergeChunks_UpdateStorageFailsGracefully(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("MergeChunkData", ctx, "abc123", "file.zip", int32(2)).Return("/tmp/store/abc123.zip", nil)
	repo.On("CreateStore", ctx, mock.AnythingOfType("*biz.FileStore")).Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 3, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(errors.New("user-service unavailable"))

	// Should still succeed - storage update failure is non-fatal
	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.NotNil(t, file)
	repo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

// --- ListFiles tests ---

func TestListFiles_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	expected := []*File{
		{ID: 1, Name: "doc.pdf"},
		{ID: 2, Name: "photos", IsFolder: true},
	}
	repo.On("FindByUserAndParent", ctx, int64(1), int64(0), int32(1), int32(20)).Return(expected, int64(2), nil)

	files, total, err := uc.ListFiles(ctx, 1, 0, 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, files, 2)
	repo.AssertExpectations(t)
}

// --- CreateFolder tests ---

func TestCreateFolder_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("Create", ctx, mock.MatchedBy(func(f *File) bool {
		return f.UserID == 1 && f.ParentID == 0 && f.Name == "Documents" && f.IsFolder
	})).Return(&File{
		ID: 10, UserID: 1, ParentID: 0, Name: "Documents", IsFolder: true,
	}, nil)

	folder, err := uc.CreateFolder(ctx, 1, 0, "Documents")

	assert.NoError(t, err)
	assert.Equal(t, "Documents", folder.Name)
	assert.True(t, folder.IsFolder)
	repo.AssertExpectations(t)
}

// --- RenameFile tests ---

func TestRenameFile_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	existing := &File{ID: 1, UserID: 100, Name: "old.txt"}
	repo.On("FindByID", ctx, int64(1)).Return(existing, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(f *File) bool {
		return f.Name == "new.txt"
	})).Return(nil)

	err := uc.RenameFile(ctx, 100, 1, "new.txt")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRenameFile_WrongUser(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	existing := &File{ID: 1, UserID: 100, Name: "old.txt"}
	repo.On("FindByID", ctx, int64(1)).Return(existing, nil)

	err := uc.RenameFile(ctx, 999, 1, "new.txt") // wrong user

	assert.ErrorIs(t, err, ErrFileNotFound)
	repo.AssertExpectations(t)
}

func TestRenameFile_NotFound(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindByID", ctx, int64(999)).Return(nil, errors.New("not found"))

	err := uc.RenameFile(ctx, 100, 999, "new.txt")

	assert.ErrorIs(t, err, ErrFileNotFound)
	repo.AssertExpectations(t)
}

// --- DeleteFiles tests ---

func TestDeleteFiles_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindByID", ctx, int64(1)).Return(&File{ID: 1, UserID: 100}, nil)
	repo.On("FindByID", ctx, int64(2)).Return(&File{ID: 2, UserID: 100}, nil)
	repo.On("FindByID", ctx, int64(3)).Return(&File{ID: 3, UserID: 100}, nil)
	repo.On("SoftDelete", ctx, int64(100), []int64{1, 2, 3}).Return(nil)

	err := uc.DeleteFiles(ctx, 100, []int64{1, 2, 3})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// --- MoveFiles tests ---

func TestMoveFiles_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	file1 := &File{ID: 1, UserID: 100, ParentID: 0}
	file2 := &File{ID: 2, UserID: 100, ParentID: 0}
	repo.On("FindByID", ctx, int64(10)).Return(&File{ID: 10, UserID: 100, IsFolder: true}, nil)
	repo.On("FindByID", ctx, int64(1)).Return(file1, nil)
	repo.On("FindByID", ctx, int64(2)).Return(file2, nil)
	repo.On("Update", ctx, mock.AnythingOfType("*biz.File")).Return(nil).Twice()

	err := uc.MoveFiles(ctx, 100, []int64{1, 2}, 10)

	assert.NoError(t, err)
	assert.Equal(t, int64(10), file1.ParentID)
	assert.Equal(t, int64(10), file2.ParentID)
	repo.AssertExpectations(t)
}

// --- Trash tests ---

func TestListTrash_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	deletedAt := time.Now()
	trashFiles := []*File{
		{ID: 1, Name: "deleted.txt", DeletedAt: &deletedAt},
	}
	repo.On("FindTrash", ctx, int64(1), int32(1), int32(20)).Return(trashFiles, int64(1), nil)

	files, total, err := uc.ListTrash(ctx, 1, 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, files, 1)
	repo.AssertExpectations(t)
}

func TestRestoreFiles_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("Restore", ctx, int64(100), []int64{1, 2}).Return(nil)

	err := uc.RestoreFiles(ctx, 100, []int64{1, 2})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPermanentDelete_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("PermanentDelete", ctx, int64(100), []int64{1}).Return(nil)

	err := uc.PermanentDelete(ctx, 100, []int64{1})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// --- Share tests ---

func TestCreateShare_WithExpiry(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("CreateShare", ctx, mock.MatchedBy(func(s *Share) bool {
		return s.UserID == 100 && s.FileID == 1 && s.Password == "secret" && s.ExpireAt != nil
	})).Return(nil)

	share, err := uc.CreateShare(ctx, 100, 1, 7, "secret")

	assert.NoError(t, err)
	assert.NotNil(t, share.ExpireAt)
	assert.Equal(t, "secret", share.Password)
	repo.AssertExpectations(t)
}

func TestCreateShare_NoExpiry(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("CreateShare", ctx, mock.MatchedBy(func(s *Share) bool {
		return s.ExpireAt == nil
	})).Return(nil)

	share, err := uc.CreateShare(ctx, 100, 1, 0, "")

	assert.NoError(t, err)
	assert.Nil(t, share.ExpireAt)
	repo.AssertExpectations(t)
}

func TestGetShare_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	share := &Share{ID: "share123", UserID: 100, FileID: 1}
	repo.On("FindShareByID", ctx, "share123").Return(share, nil)
	repo.On("FindByID", ctx, int64(1)).Return(&File{ID: 1, Name: "shared.txt"}, nil)

	file, err := uc.GetShare(ctx, "share123", "")

	assert.NoError(t, err)
	assert.Equal(t, "shared.txt", file.Name)
	repo.AssertExpectations(t)
}

func TestGetShare_NotFound(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindShareByID", ctx, "unknown").Return(nil, errors.New("not found"))

	file, err := uc.GetShare(ctx, "unknown", "")

	assert.Nil(t, file)
	assert.ErrorIs(t, err, ErrShareNotFound)
	repo.AssertExpectations(t)
}

func TestGetShare_Expired(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	past := time.Now().Add(-24 * time.Hour)
	share := &Share{ID: "share123", FileID: 1, ExpireAt: &past}
	repo.On("FindShareByID", ctx, "share123").Return(share, nil)

	file, err := uc.GetShare(ctx, "share123", "")

	assert.Nil(t, file)
	assert.ErrorIs(t, err, ErrShareExpired)
	repo.AssertExpectations(t)
}

func TestGetShare_WrongPassword(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	share := &Share{ID: "share123", FileID: 1, Password: "correct"}
	repo.On("FindShareByID", ctx, "share123").Return(share, nil)

	file, err := uc.GetShare(ctx, "share123", "wrong")

	assert.Nil(t, file)
	assert.ErrorIs(t, err, ErrInvalidPassword)
	repo.AssertExpectations(t)
}

func TestGetShare_CorrectPassword(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	share := &Share{ID: "share123", FileID: 1, Password: "secret"}
	repo.On("FindShareByID", ctx, "share123").Return(share, nil)
	repo.On("FindByID", ctx, int64(1)).Return(&File{ID: 1, Name: "protected.pdf"}, nil)

	file, err := uc.GetShare(ctx, "share123", "secret")

	assert.NoError(t, err)
	assert.Equal(t, "protected.pdf", file.Name)
	repo.AssertExpectations(t)
}

// --- SearchFiles tests ---

func TestSearchFiles_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	results := []*File{
		{ID: 1, Name: "report.pdf"},
		{ID: 2, Name: "report-v2.pdf"},
	}
	repo.On("Search", ctx, int64(1), "report", int32(1), int32(20)).Return(results, int64(2), nil)

	files, total, err := uc.SearchFiles(ctx, 1, "report", 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, files, 2)
	repo.AssertExpectations(t)
}
