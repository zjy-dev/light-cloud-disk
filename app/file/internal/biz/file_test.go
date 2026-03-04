package biz

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Mock FileRepo
// ---------------------------------------------------------------------------

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
	return m.Called(ctx, file).Error(0)
}
func (m *MockFileRepo) SoftDelete(ctx context.Context, userID int64, ids []int64) error {
	return m.Called(ctx, userID, ids).Error(0)
}
func (m *MockFileRepo) Restore(ctx context.Context, userID int64, ids []int64) error {
	return m.Called(ctx, userID, ids).Error(0)
}
func (m *MockFileRepo) PermanentDelete(ctx context.Context, userID int64, ids []int64) error {
	return m.Called(ctx, userID, ids).Error(0)
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
	return m.Called(ctx, store).Error(0)
}
func (m *MockFileRepo) IncrStoreRefCount(ctx context.Context, md5 string) error {
	return m.Called(ctx, md5).Error(0)
}
func (m *MockFileRepo) DecrStoreRefCount(ctx context.Context, md5 string) error {
	return m.Called(ctx, md5).Error(0)
}
func (m *MockFileRepo) UpdateStorageLocation(ctx context.Context, fileMD5 string, storageType string, newPath string) error {
	return m.Called(ctx, fileMD5, storageType, newPath).Error(0)
}
func (m *MockFileRepo) UpdateLastAccessed(ctx context.Context, fileMD5 string) error {
	return m.Called(ctx, fileMD5).Error(0)
}
func (m *MockFileRepo) FindLRUStores(ctx context.Context, storageType string, limit int) ([]*FileStore, error) {
	args := m.Called(ctx, storageType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FileStore), args.Error(1)
}
func (m *MockFileRepo) SumSizeByStorageType(ctx context.Context, storageType string) (int64, error) {
	args := m.Called(ctx, storageType)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockFileRepo) CreateShare(ctx context.Context, share *Share) error {
	return m.Called(ctx, share).Error(0)
}
func (m *MockFileRepo) FindShareByID(ctx context.Context, id string) (*Share, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Share), args.Error(1)
}
func (m *MockFileRepo) DeleteExpiredShares(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockFileRepo) GetUploadedChunks(ctx context.Context, fileMD5 string) ([]int32, error) {
	args := m.Called(ctx, fileMD5)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]int32), args.Error(1)
}
func (m *MockFileRepo) SaveChunkData(ctx context.Context, fileMD5 string, chunkIndex int32, data []byte) error {
	return m.Called(ctx, fileMD5, chunkIndex, data).Error(0)
}
func (m *MockFileRepo) MergeChunkData(ctx context.Context, fileMD5, fileName string, totalChunks int32) (string, error) {
	args := m.Called(ctx, fileMD5, fileName, totalChunks)
	return args.String(0), args.Error(1)
}
func (m *MockFileRepo) SaveChunkInfo(ctx context.Context, chunk *ChunkInfo) error {
	return m.Called(ctx, chunk).Error(0)
}
func (m *MockFileRepo) ClearChunkInfo(ctx context.Context, fileMD5 string) error {
	return m.Called(ctx, fileMD5).Error(0)
}
func (m *MockFileRepo) GetDiskUsage(ctx context.Context, diskType string) (int64, error) {
	args := m.Called(ctx, diskType)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockFileRepo) IncrDiskUsage(ctx context.Context, diskType string, delta int64) error {
	return m.Called(ctx, diskType, delta).Error(0)
}

// ---------------------------------------------------------------------------
// Mock UserClient
// ---------------------------------------------------------------------------

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error {
	return m.Called(ctx, userID, delta).Error(0)
}

// ---------------------------------------------------------------------------
// Mock MessageProducer
// ---------------------------------------------------------------------------

type MockMessageProducer struct {
	mock.Mock
}

func (m *MockMessageProducer) SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *MockMessageProducer) SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *MockMessageProducer) Close() error {
	return m.Called().Error(0)
}

// ---------------------------------------------------------------------------
// Mock ObjectStorage (SeaweedFS)
// ---------------------------------------------------------------------------

type MockObjectStorage struct {
	mock.Mock
}

func (m *MockObjectStorage) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	return m.Called(ctx, key, reader, size).Error(0)
}
func (m *MockObjectStorage) Delete(ctx context.Context, key string) error {
	return m.Called(ctx, key).Error(0)
}
func (m *MockObjectStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockObjectStorage) PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	args := m.Called(ctx, key, expires)
	return args.String(0), args.Error(1)
}

// ---------------------------------------------------------------------------
// Mock CloudStorage (OSS)
// ---------------------------------------------------------------------------

type MockCloudStorage struct {
	mock.Mock
}

func (m *MockCloudStorage) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	return m.Called(ctx, key, reader, size).Error(0)
}
func (m *MockCloudStorage) Delete(ctx context.Context, key string) error {
	return m.Called(ctx, key).Error(0)
}
func (m *MockCloudStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockCloudStorage) PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	args := m.Called(ctx, key, expires)
	return args.String(0), args.Error(1)
}

// ---------------------------------------------------------------------------
// Test helper constructors
// ---------------------------------------------------------------------------

var defaultStorageCfg = &StorageConfig{
	LocalMaxBytes:         10 * 1024 * 1024 * 1024, // 10GB
	SeaweedFSMaxBytes:     50 * 1024 * 1024 * 1024, // 50GB
	SeaweedFSThresholdPct: 80,
}

func newTestFileUsecase(repo *MockFileRepo, userClient *MockUserClient) *FileUsecase {
	mq := new(MockMessageProducer)
	mq.On("SendCloudMigrateMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("SendThumbnailMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("Close").Return(nil).Maybe()

	objStore := new(MockObjectStorage)
	objStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	objStore.On("PresignGetURL", mock.Anything, mock.Anything, mock.Anything).Return("http://swf/presigned", nil).Maybe()

	cloudStore := new(MockCloudStorage)
	cloudStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	cloudStore.On("PresignGetURL", mock.Anything, mock.Anything, mock.Anything).Return("http://oss/presigned", nil).Maybe()

	return NewFileUsecase(repo, userClient, mq, objStore, cloudStore, defaultStorageCfg, log.DefaultLogger)
}

type testDeps struct {
	repo       *MockFileRepo
	userClient *MockUserClient
	mq         *MockMessageProducer
	objStore   *MockObjectStorage
	cloudStore *MockCloudStorage
	uc         *FileUsecase
}

func newTestDeps() *testDeps {
	d := &testDeps{
		repo:       new(MockFileRepo),
		userClient: new(MockUserClient),
		mq:         new(MockMessageProducer),
		objStore:   new(MockObjectStorage),
		cloudStore: new(MockCloudStorage),
	}
	d.uc = NewFileUsecase(d.repo, d.userClient, d.mq, d.objStore, d.cloudStore, defaultStorageCfg, log.DefaultLogger)
	return d
}

// ---------------------------------------------------------------------------
// CheckUpload tests
// ---------------------------------------------------------------------------

func TestCheckUpload_FastUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", Size: 1024, StorePath: "abc123.bin", StorageType: StorageSeaweedFS,
	}, nil)

	canFast, chunks, diskFull, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.True(t, canFast)
	assert.Nil(t, chunks)
	assert.False(t, diskFull)
	repo.AssertExpectations(t)
}

func TestCheckUpload_ResumeUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32{0, 1, 3}, nil)

	canFast, chunks, diskFull, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Equal(t, []int32{0, 1, 3}, chunks)
	assert.False(t, diskFull)
	repo.AssertExpectations(t)
}

func TestCheckUpload_NewUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32(nil), nil)

	canFast, chunks, diskFull, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Nil(t, chunks)
	assert.False(t, diskFull)
	repo.AssertExpectations(t)
}

func TestCheckUpload_DiskFull(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	// Mock usage close to limit
	repo.On("GetDiskUsage", ctx, "local").Return(defaultStorageCfg.LocalMaxBytes-100, nil)

	canFast, chunks, diskFull, err := uc.CheckUpload(ctx, "abc123", 1024, 5) // 1024 > 100 remaining

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Nil(t, chunks)
	assert.True(t, diskFull)
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// SaveChunk tests
// ---------------------------------------------------------------------------

func TestSaveChunk_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	data := []byte("chunk-data")
	repo.On("SaveChunkData", ctx, "abc123", int32(2), data).Return(nil)
	repo.On("IncrDiskUsage", ctx, "local", int64(len(data))).Return(nil)
	repo.On("SaveChunkInfo", ctx, mock.MatchedBy(func(c *ChunkInfo) bool {
		return c.FileMD5 == "abc123" && c.ChunkIndex == 2 && c.ChunkSize == int64(len(data)) && c.Uploaded
	})).Return(nil)

	err := uc.SaveChunk(ctx, "abc123", 2, int64(len(data)), data)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSaveChunk_SizeMismatch(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	err := uc.SaveChunk(ctx, "abc123", 0, 999, []byte("short"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chunk size mismatch")
}

// ---------------------------------------------------------------------------
// MergeChunks tests: dedup hit with existing store
// ---------------------------------------------------------------------------

func TestMergeChunks_ExistingStore(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageSeaweedFS, RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 2, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
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

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageSeaweedFS, RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 3, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(errors.New("user-service unavailable"))

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.NotNil(t, file)
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListFiles tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// CreateFolder tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// RenameFile tests
// ---------------------------------------------------------------------------

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

	err := uc.RenameFile(ctx, 999, 1, "new.txt")

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

// ---------------------------------------------------------------------------
// DeleteFiles tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// MoveFiles tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Trash tests
// ---------------------------------------------------------------------------

func TestListTrash_Success(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	deletedAt := time.Now()
	trashFiles := []*File{{ID: 1, Name: "deleted.txt", DeletedAt: &deletedAt}}
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

// ---------------------------------------------------------------------------
// GetDownloadURL tests: presigned URL from storage
// ---------------------------------------------------------------------------

func TestGetDownloadURL_SeaweedFS(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(1)).Return(&File{
		ID: 1, UserID: 100, Name: "file.zip", FileMD5: "abc123",
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageSeaweedFS,
	}, nil)
	d.repo.On("UpdateLastAccessed", ctx, "abc123").Return(nil)
	d.objStore.On("PresignGetURL", ctx, "abc123.zip", time.Hour).Return("http://swf/presigned/abc123.zip", nil)

	url, name, err := d.uc.GetDownloadURL(ctx, 100, 1)

	assert.NoError(t, err)
	assert.Equal(t, "http://swf/presigned/abc123.zip", url)
	assert.Equal(t, "file.zip", name)
}

func TestGetDownloadURL_OSS(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(1)).Return(&File{
		ID: 1, UserID: 100, Name: "file.zip", FileMD5: "abc123",
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageOSS,
	}, nil)
	d.repo.On("UpdateLastAccessed", ctx, "abc123").Return(nil)
	d.cloudStore.On("PresignGetURL", ctx, "abc123.zip", time.Hour).Return("http://oss/presigned/abc123.zip", nil)

	url, name, err := d.uc.GetDownloadURL(ctx, 100, 1)

	assert.NoError(t, err)
	assert.Equal(t, "http://oss/presigned/abc123.zip", url)
	assert.Equal(t, "file.zip", name)
}

func TestGetDownloadURL_NotFound(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindByID", ctx, int64(999)).Return(nil, errors.New("not found"))

	_, _, err := uc.GetDownloadURL(ctx, 100, 999)

	assert.ErrorIs(t, err, ErrFileNotFound)
}

// ---------------------------------------------------------------------------
// GetDiskUsage tests
// ---------------------------------------------------------------------------

func TestGetDiskUsage(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("GetDiskUsage", ctx, "local").Return(int64(1024), nil)
	repo.On("GetDiskUsage", ctx, "seaweedfs").Return(int64(2048), nil)

	localUsed, localMax, swfUsed, swfMax, err := uc.GetDiskUsage(ctx)

	assert.NoError(t, err)
	assert.Equal(t, int64(1024), localUsed)
	assert.Equal(t, defaultStorageCfg.LocalMaxBytes, localMax)
	assert.Equal(t, int64(2048), swfUsed)
	assert.Equal(t, defaultStorageCfg.SeaweedFSMaxBytes, swfMax)
}

// ---------------------------------------------------------------------------
// Share tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SearchFiles tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// LRU eviction tests
// ---------------------------------------------------------------------------

func TestMaybeEvictToCloud_TriggersWhenAboveThreshold(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	// 50GB max, 80% threshold = 40GB. Pass 42GB to trigger eviction
	d.repo.On("FindLRUStores", ctx, StorageSeaweedFS, 100).Return([]*FileStore{
		{FileMD5: "file1", StorePath: "file1.bin", Size: 1 * 1024 * 1024 * 1024}, // 1 GB
		{FileMD5: "file2", StorePath: "file2.bin", Size: 1 * 1024 * 1024 * 1024}, // 1 GB
	}, nil)
	d.mq.On("SendCloudMigrateMessage", ctx, mock.AnythingOfType("*biz.CloudMigrateMessage")).Return(nil)

	d.uc.maybeEvictToCloud(ctx, 42*1024*1024*1024) // 42 GB > 40 GB threshold

	d.mq.AssertCalled(t, "SendCloudMigrateMessage", ctx, mock.Anything)
}

func TestMaybeEvictToCloud_NoOp_BelowThreshold(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.uc.maybeEvictToCloud(ctx, 20*1024*1024*1024) // 20 GB < 40 GB threshold

	d.mq.AssertNotCalled(t, "SendCloudMigrateMessage", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestIsMediaFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"jpg image", "photo.jpg", true},
		{"JPEG upper", "PHOTO.JPEG", true},
		{"png image", "screenshot.png", true},
		{"mp4 video", "video.mp4", true},
		{"zip file", "archive.zip", false},
		{"pdf file", "doc.pdf", false},
		{"no ext", "readme", false},
		{"webp image", "img.webp", true},
		{"mkv video", "movie.mkv", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isMediaFile(tt.fileName))
		})
	}
}

func TestDetectMIME(t *testing.T) {
	assert.Equal(t, "image/jpeg", detectMIME("photo.jpg"))
	assert.Equal(t, "image/png", detectMIME("img.png"))
	assert.Equal(t, "application/octet-stream", detectMIME("unknown"))
}
