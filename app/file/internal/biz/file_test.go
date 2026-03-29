package biz

import (
	"context"
	"errors"
	"fmt"
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
func (m *MockFileRepo) CreateUploadSession(ctx context.Context, session *UploadSession) error {
	return m.Called(ctx, session).Error(0)
}
func (m *MockFileRepo) FindUploadSession(ctx context.Context, userID int64, fileMD5 string) (*UploadSession, error) {
	args := m.Called(ctx, userID, fileMD5)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UploadSession), args.Error(1)
}
func (m *MockFileRepo) FindUploadSessionByID(ctx context.Context, sessionID string) (*UploadSession, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UploadSession), args.Error(1)
}
func (m *MockFileRepo) UpdateUploadSessionStatus(ctx context.Context, sessionID string, status string) error {
	return m.Called(ctx, sessionID, status).Error(0)
}
func (m *MockFileRepo) SaveUploadPart(ctx context.Context, sessionID string, partNumber int32, etag string, size int64) error {
	return m.Called(ctx, sessionID, partNumber, etag, size).Error(0)
}
func (m *MockFileRepo) FindUploadedParts(ctx context.Context, sessionID string) ([]UploadedPart, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]UploadedPart), args.Error(1)
}
func (m *MockFileRepo) DeleteUploadSession(ctx context.Context, sessionID string) error {
	return m.Called(ctx, sessionID).Error(0)
}

// New distributed lock + SET-based chunk methods

func (m *MockFileRepo) AcquireChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, fileMD5, chunkIndex, ttl)
	return args.Bool(0), args.Error(1)
}
func (m *MockFileRepo) ReleaseChunkLock(ctx context.Context, fileMD5 string, chunkIndex int32) error {
	return m.Called(ctx, fileMD5, chunkIndex).Error(0)
}
func (m *MockFileRepo) AcquireMergeLock(ctx context.Context, fileMD5 string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, fileMD5, ttl)
	return args.Bool(0), args.Error(1)
}
func (m *MockFileRepo) ReleaseMergeLock(ctx context.Context, fileMD5 string) error {
	return m.Called(ctx, fileMD5).Error(0)
}
func (m *MockFileRepo) AddUploadedChunk(ctx context.Context, fileMD5 string, chunkIndex int32) error {
	return m.Called(ctx, fileMD5, chunkIndex).Error(0)
}
func (m *MockFileRepo) IsChunkUploaded(ctx context.Context, fileMD5 string, chunkIndex int32) (bool, error) {
	args := m.Called(ctx, fileMD5, chunkIndex)
	return args.Bool(0), args.Error(1)
}
func (m *MockFileRepo) CountUploadedChunks(ctx context.Context, fileMD5 string) (int32, error) {
	args := m.Called(ctx, fileMD5)
	return args.Get(0).(int32), args.Error(1)
}
func (m *MockFileRepo) CreateStoreWithStatus(ctx context.Context, store *FileStore) (*FileStore, error) {
	args := m.Called(ctx, store)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FileStore), args.Error(1)
}
func (m *MockFileRepo) UpdateStoreStatus(ctx context.Context, id int64, status string) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *MockFileRepo) FindStoreByMD5AndStatus(ctx context.Context, md5, status string) (*FileStore, error) {
	args := m.Called(ctx, md5, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FileStore), args.Error(1)
}
func (m *MockFileRepo) CreateErasureShard(ctx context.Context, shard *ErasureShard) error {
	return m.Called(ctx, shard).Error(0)
}
func (m *MockFileRepo) FindErasureShards(ctx context.Context, fileStoreID int64) ([]*ErasureShard, error) {
	args := m.Called(ctx, fileStoreID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ErasureShard), args.Error(1)
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
func (m *MockCloudStorage) InitMultipartUpload(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}
func (m *MockCloudStorage) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (string, error) {
	args := m.Called(ctx, key, uploadID, partNumber, expires)
	return args.String(0), args.Error(1)
}
func (m *MockCloudStorage) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error {
	return m.Called(ctx, key, uploadID, parts).Error(0)
}
func (m *MockCloudStorage) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	return m.Called(ctx, key, uploadID).Error(0)
}

// ---------------------------------------------------------------------------
// Test helper constructors
// ---------------------------------------------------------------------------

var defaultStorageCfg = &StorageConfig{
	PrimaryMaxBytes: 10 * 1024 * 1024 * 1024, // 10 GB
	ThresholdPct:    80,
	EvictTargetPct:  90,
}

func newTestFileUsecase(repo *MockFileRepo, userClient *MockUserClient) *FileUsecase {
	return newTestFileUsecaseWithCfg(repo, userClient, defaultStorageCfg)
}

func newTestFileUsecaseWithCfg(repo *MockFileRepo, userClient *MockUserClient, cfg *StorageConfig) *FileUsecase {
	mq := new(MockMessageProducer)
	mq.On("SendCloudMigrateMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("SendThumbnailMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("Close").Return(nil).Maybe()

	cloudStore := new(MockCloudStorage)
	cloudStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	cloudStore.On("PresignGetURL", mock.Anything, mock.Anything, mock.Anything).Return("http://oss/presigned", nil).Maybe()

	return NewFileUsecase(repo, userClient, mq, cloudStore, cfg, nil, nil, "/tmp/test-store", log.DefaultLogger)
}

type testDeps struct {
	repo       *MockFileRepo
	userClient *MockUserClient
	mq         *MockMessageProducer
	cloudStore *MockCloudStorage
	uc         *FileUsecase
}

func newTestDeps() *testDeps {
	d := &testDeps{
		repo:       new(MockFileRepo),
		userClient: new(MockUserClient),
		mq:         new(MockMessageProducer),
		cloudStore: new(MockCloudStorage),
	}
	d.uc = NewFileUsecase(d.repo, d.userClient, d.mq, d.cloudStore, defaultStorageCfg, nil, nil, "/tmp/test-store", log.DefaultLogger)
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
		FileMD5: "abc123", Size: 1024, StorePath: "abc123.bin", StorageType: StorageLocal, UploadStatus: "completed",
	}, nil)

	canFast, chunks, diskFull, uploadMode, uploadStatus, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.True(t, canFast)
	assert.Nil(t, chunks)
	assert.False(t, diskFull)
	assert.Equal(t, "direct", uploadMode)
	assert.Equal(t, "completed", uploadStatus)
	repo.AssertExpectations(t)
}

func TestCheckUpload_UploadingStore(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "uploading").Return(&FileStore{
		FileMD5: "abc123", UploadStatus: "uploading",
	}, nil)
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32{0, 1}, nil)

	canFast, chunks, diskFull, uploadMode, uploadStatus, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Equal(t, []int32{0, 1}, chunks)
	assert.False(t, diskFull)
	assert.Equal(t, "direct", uploadMode)
	assert.Equal(t, "uploading", uploadStatus)
	repo.AssertExpectations(t)
}

func TestCheckUpload_ResumeUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "uploading").Return(nil, errors.New("not found"))
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32{0, 1, 3}, nil)

	canFast, chunks, diskFull, uploadMode, uploadStatus, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Equal(t, []int32{0, 1, 3}, chunks)
	assert.False(t, diskFull)
	assert.Equal(t, "direct", uploadMode)
	assert.Empty(t, uploadStatus)
	repo.AssertExpectations(t)
}

func TestCheckUpload_NewUpload(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "uploading").Return(nil, errors.New("not found"))
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("GetUploadedChunks", ctx, "abc123").Return([]int32(nil), nil)

	canFast, chunks, diskFull, uploadMode, _, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Nil(t, chunks)
	assert.False(t, diskFull)
	assert.Equal(t, "direct", uploadMode)
	repo.AssertExpectations(t)
}

func TestCheckUpload_DiskFull(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "uploading").Return(nil, errors.New("not found"))
	repo.On("GetDiskUsage", ctx, "local").Return(defaultStorageCfg.PrimaryMaxBytes-100, nil)

	canFast, chunks, diskFull, uploadMode, _, err := uc.CheckUpload(ctx, "abc123", 1024, 5)

	assert.NoError(t, err)
	assert.False(t, canFast)
	assert.Nil(t, chunks)
	assert.True(t, diskFull)
	assert.Equal(t, "presigned", uploadMode)
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
	repo.On("AcquireChunkLock", ctx, "abc123", int32(2), mock.Anything).Return(true, nil)
	repo.On("IsChunkUploaded", ctx, "abc123", int32(2)).Return(false, nil)
	repo.On("SaveChunkData", ctx, "abc123", int32(2), data).Return(nil)
	repo.On("IncrDiskUsage", ctx, "local", int64(len(data))).Return(nil)
	repo.On("AddUploadedChunk", ctx, "abc123", int32(2)).Return(nil)
	repo.On("ReleaseChunkLock", ctx, "abc123", int32(2)).Return(nil)

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

func TestSaveChunk_AlreadyUploaded(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	data := []byte("chunk-data")
	repo.On("AcquireChunkLock", ctx, "abc123", int32(0), mock.Anything).Return(true, nil)
	repo.On("IsChunkUploaded", ctx, "abc123", int32(0)).Return(true, nil)
	repo.On("ReleaseChunkLock", ctx, "abc123", int32(0)).Return(nil)

	err := uc.SaveChunk(ctx, "abc123", 0, int64(len(data)), data)

	assert.NoError(t, err)
	repo.AssertNotCalled(t, "SaveChunkData")
}

func TestSaveChunk_LockFailed(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	data := []byte("chunk-data")
	repo.On("AcquireChunkLock", ctx, "abc123", int32(0), mock.Anything).Return(false, nil)

	err := uc.SaveChunk(ctx, "abc123", 0, int64(len(data)), data)

	assert.ErrorIs(t, err, ErrChunkLocked)
}

// ---------------------------------------------------------------------------
// MergeChunks tests
// ---------------------------------------------------------------------------

func TestMergeChunks_ExistingStore(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "completed", RefCount: 1,
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
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "completed", RefCount: 1,
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

func TestMergeChunks_LockFailed(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	// No existing completed store → proceeds to lock acquisition
	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(false, nil)

	_, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.ErrorIs(t, err, ErrMergeLocked)
}

// T040: SaveChunk with lock — verify lock is released even on write failure
func TestSaveChunk_WriteErrorReleasesLock(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	data := []byte("chunk-data")
	repo.On("AcquireChunkLock", ctx, "abc123", int32(0), mock.Anything).Return(true, nil)
	repo.On("IsChunkUploaded", ctx, "abc123", int32(0)).Return(false, nil)
	repo.On("SaveChunkData", ctx, "abc123", int32(0), data).Return(errors.New("disk I/O error"))
	repo.On("ReleaseChunkLock", ctx, "abc123", int32(0)).Return(nil)

	err := uc.SaveChunk(ctx, "abc123", 0, int64(len(data)), data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk I/O error")
	repo.AssertCalled(t, "ReleaseChunkLock", ctx, "abc123", int32(0))
}

// T041: MergeChunks dedup after acquiring lock (step 3)
func TestMergeChunks_DedupAfterLock(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	// Step 1: no completed store initially
	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	// Step 2: lock acquired
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	// Step 3: another instance completed the merge while we waited for the lock
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 5, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(5), file.ID)
	// Verify merge was not performed (no MergeChunkData call)
	repo.AssertNotCalled(t, "MergeChunkData")
	repo.AssertExpectations(t)
}

// T044: MergeChunks UNIQUE constraint race — CreateStoreWithStatus returns already-completed store
func TestMergeChunks_UniqueConstraintRace(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	// Step 1: no completed store initially
	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	// Step 2: lock acquired
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	// Step 3: no completed store after lock either
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	// Step 4: UNIQUE constraint hit — another instance already created and completed the store
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 6, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(6), file.ID)
	// Verify no actual merge was performed
	repo.AssertNotCalled(t, "MergeChunkData")
	repo.AssertExpectations(t)
}

// T044: Full MergeChunks flow (steps 4→11) — new file merged, stored locally, status transitions
func TestMergeChunks_FullFlow(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	mq := new(MockMessageProducer)
	mq.On("SendCloudMigrateMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("SendThumbnailMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("Close").Return(nil).Maybe()
	cloudStore := new(MockCloudStorage)
	cloudStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewFileUsecase(repo, userClient, mq, cloudStore, defaultStorageCfg, nil, nil, "/tmp/test-store", log.DefaultLogger)
	ctx := context.Background()

	// Step 1: no existing store
	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	// Step 2: lock acquired
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	// Step 3: no completed store after lock
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	// Step 4: create store with uploading status → returns new store
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		ID: 42, FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "uploading", RefCount: 1,
	}, nil)
	// Step 5: verify all chunks present
	repo.On("CountUploadedChunks", ctx, "abc123").Return(int32(2), nil)
	// Step 6: merge chunks
	repo.On("MergeChunkData", ctx, "abc123", "file.zip", int32(2)).Return("/tmp/test-store/abc123.zip", nil)
	// Step 7: check disk usage for storage decision
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("IncrDiskUsage", ctx, "local", int64(2048)).Return(nil)
	// Step 9: update storage location and status → completed
	repo.On("UpdateStorageLocation", ctx, "abc123", StorageLocal, "abc123.zip").Return(nil)
	repo.On("UpdateStoreStatus", ctx, int64(42), "completed").Return(nil)
	// Step 10: create file record
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 10, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	// Step 11: cleanup
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(10), file.ID)
	assert.Equal(t, "file.zip", file.Name)
	repo.AssertCalled(t, "MergeChunkData", ctx, "abc123", "file.zip", int32(2))
	repo.AssertCalled(t, "UpdateStoreStatus", ctx, int64(42), "completed")
	repo.AssertExpectations(t)
}

// T057: MergeChunks fails when not all chunks are present (cross-instance failover)
func TestMergeChunks_IncompleteChunks(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		ID: 99, FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal, UploadStatus: "uploading", RefCount: 1,
	}, nil)
	// Only 3 of 5 chunks present
	repo.On("CountUploadedChunks", ctx, "abc123").Return(int32(3), nil)

	_, err := uc.MergeChunks(ctx, 100, 0, "file.zip", "abc123", 5120, 5)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete chunks")
	repo.AssertNotCalled(t, "MergeChunkData")
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
// GetDownloadURL tests
// ---------------------------------------------------------------------------

func TestGetDownloadURL_LocalStorage(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(1)).Return(&File{
		ID: 1, UserID: 100, Name: "file.zip", FileMD5: "abc123",
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocal,
	}, nil)
	d.repo.On("UpdateLastAccessed", ctx, "abc123").Return(nil)

	url, name, err := d.uc.GetDownloadURL(ctx, 100, 1)

	assert.NoError(t, err)
	assert.Equal(t, "local://abc123.zip", url)
	assert.Equal(t, "file.zip", name)
}

func TestGetDownloadURL_LocalEC(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(1)).Return(&File{
		ID: 1, UserID: 100, Name: "file.zip", FileMD5: "abc123",
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageLocalEC,
	}, nil)
	d.repo.On("UpdateLastAccessed", ctx, "abc123").Return(nil)

	url, name, err := d.uc.GetDownloadURL(ctx, 100, 1)

	assert.NoError(t, err)
	assert.Equal(t, "local://abc123.zip", url)
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

	repo.On("GetDiskUsage", ctx, "local").Return(int64(2048), nil)

	primaryUsed, primaryMax, primaryType, err := uc.GetDiskUsage(ctx)

	assert.NoError(t, err)
	assert.Equal(t, int64(2048), primaryUsed)
	assert.Equal(t, defaultStorageCfg.PrimaryMaxBytes, primaryMax)
	assert.Equal(t, "local", primaryType)
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

	d.repo.On("FindLRUStores", ctx, StorageLocal, 100).Return([]*FileStore{
		{FileMD5: "file1", StorePath: "file1.bin", Size: 500 * 1024 * 1024},
		{FileMD5: "file2", StorePath: "file2.bin", Size: 500 * 1024 * 1024},
	}, nil)
	d.mq.On("SendCloudMigrateMessage", ctx, mock.AnythingOfType("*biz.CloudMigrateMessage")).Return(nil)

	d.uc.maybeEvictToCloud(ctx, 9*1024*1024*1024) // 9 GB > 8 GB threshold

	d.mq.AssertCalled(t, "SendCloudMigrateMessage", ctx, mock.Anything)
}

func TestMaybeEvictToCloud_NoOp_BelowThreshold(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.uc.maybeEvictToCloud(ctx, 5*1024*1024*1024) // 5 GB < 8 GB threshold

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

// ---------------------------------------------------------------------------
// Presigned Upload tests
// ---------------------------------------------------------------------------

func TestInitPresignedUpload_FastUpload(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", Size: 1024, StorePath: "abc123.bin", StorageType: StorageLocal, UploadStatus: "completed",
	}, nil)
	d.repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	d.repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{ID: 42, Name: "test.txt"}, nil)
	d.userClient.On("UpdateStorageUsed", ctx, int64(1), int64(1024)).Return(nil)

	result, err := d.uc.InitPresignedUpload(ctx, 1, 0, "test.txt", "abc123", 1024, 3)

	assert.NoError(t, err)
	assert.True(t, result.CanFastUpload)
	assert.NotNil(t, result.File)
	d.repo.AssertExpectations(t)
}

func TestInitPresignedUpload_NewSession(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	fileSize := int64(15 * 1024 * 1024)

	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	d.repo.On("FindUploadSession", ctx, int64(1), "abc123").Return(nil, ErrSessionNotFound)
	d.cloudStore.On("InitMultipartUpload", mock.Anything, mock.MatchedBy(func(key string) bool { return true })).Return("upload-id-1", nil)
	d.cloudStore.On("PresignUploadPart", mock.Anything, mock.Anything, "upload-id-1", mock.Anything, mock.Anything).Return("http://oss/part?presigned", nil)
	d.repo.On("CreateUploadSession", ctx, mock.AnythingOfType("*biz.UploadSession")).Return(nil)

	result, err := d.uc.InitPresignedUpload(ctx, 1, 0, "test.txt", "abc123", fileSize, 3)

	assert.NoError(t, err)
	assert.False(t, result.CanFastUpload)
	assert.NotEmpty(t, result.SessionID)
	assert.Equal(t, 3, len(result.PendingParts))
	assert.Equal(t, "oss", result.StorageTarget)
	d.repo.AssertExpectations(t)
}

func TestInitPresignedUpload_ResumeSession(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	existingSession := &UploadSession{
		ID:            "existing-session",
		UserID:        1,
		FileMD5:       "abc123",
		TotalParts:    3,
		PartSize:      defaultPartSize,
		StorageTarget: "oss",
		ObjectKey:     "abc123/test.txt",
		S3UploadID:    "upload-id-old",
		Status:        "uploading",
		ExpiresAt:     time.Now().Add(1 * time.Hour),
	}

	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	d.repo.On("FindUploadSession", ctx, int64(1), "abc123").Return(existingSession, nil)
	d.repo.On("FindUploadedParts", ctx, "existing-session").Return([]UploadedPart{
		{PartNumber: 1, ETag: "etag1"},
	}, nil)
	d.cloudStore.On("PresignUploadPart", mock.Anything, "abc123/test.txt", "upload-id-old", mock.Anything, mock.Anything).Return("http://oss/part?presigned", nil)

	result, err := d.uc.InitPresignedUpload(ctx, 1, 0, "test.txt", "abc123", 1024, 3)

	assert.NoError(t, err)
	assert.Equal(t, "existing-session", result.SessionID)
	assert.Equal(t, 1, len(result.CompletedParts))
	assert.Equal(t, 2, len(result.PendingParts))
	d.repo.AssertExpectations(t)
}

func TestReportUploadedPart_Success(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(&UploadSession{
		ID: "session-1", UserID: 1, Status: "uploading",
	}, nil)
	d.repo.On("SaveUploadPart", ctx, "session-1", int32(1), "etag-abc", int64(5*1024*1024)).Return(nil)

	err := d.uc.ReportUploadedPart(ctx, int64(1), "session-1", 1, "etag-abc", 5*1024*1024)

	assert.NoError(t, err)
	d.repo.AssertExpectations(t)
}

func TestReportUploadedPart_Unauthorized(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(&UploadSession{
		ID: "session-1", UserID: 1, Status: "uploading",
	}, nil)

	err := d.uc.ReportUploadedPart(ctx, int64(999), "session-1", 1, "etag", 1024)

	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestReportUploadedPart_SessionNotFound(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "bad-session").Return(nil, ErrSessionNotFound)

	err := d.uc.ReportUploadedPart(ctx, int64(1), "bad-session", 1, "etag", 1024)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestReportUploadedPart_SessionCompleted(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-done").Return(&UploadSession{
		ID: "session-done", UserID: 1, Status: "completed",
	}, nil)

	err := d.uc.ReportUploadedPart(ctx, int64(1), "session-done", 1, "etag", 1024)

	assert.ErrorIs(t, err, ErrSessionCompleted)
}

func TestCompletePresignedUpload_Success(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	session := &UploadSession{
		ID:            "session-1",
		UserID:        1,
		FileName:      "test.txt",
		FileMD5:       "abc123",
		FileSize:      15 * 1024 * 1024,
		TotalParts:    3,
		ParentID:      0,
		StorageTarget: "oss",
		ObjectKey:     "abc123/test.txt",
		S3UploadID:    "upload-id-1",
		Status:        "uploading",
	}
	uploadedParts := []UploadedPart{
		{PartNumber: 1, ETag: "etag1"},
		{PartNumber: 2, ETag: "etag2"},
		{PartNumber: 3, ETag: "etag3"},
	}

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(session, nil)
	d.repo.On("FindUploadedParts", ctx, "session-1").Return(uploadedParts, nil)
	d.cloudStore.On("CompleteMultipartUpload", ctx, "abc123/test.txt", "upload-id-1", mock.AnythingOfType("[]biz.CompletedPart")).Return(nil)
	d.repo.On("CreateStore", ctx, mock.AnythingOfType("*biz.FileStore")).Return(nil)
	d.repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{ID: 42, Name: "test.txt"}, nil)
	d.userClient.On("UpdateStorageUsed", ctx, int64(1), int64(15*1024*1024)).Return(nil)
	d.repo.On("UpdateUploadSessionStatus", ctx, "session-1", "completed").Return(nil)

	file, err := d.uc.CompletePresignedUpload(ctx, int64(1), "session-1")

	assert.NoError(t, err)
	assert.NotNil(t, file)
	d.repo.AssertExpectations(t)
}

func TestCompletePresignedUpload_IncompleteParts(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	session := &UploadSession{
		ID:         "session-1",
		UserID:     1,
		TotalParts: 3,
		Status:     "uploading",
	}

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(session, nil)
	d.repo.On("FindUploadedParts", ctx, "session-1").Return([]UploadedPart{
		{PartNumber: 1, ETag: "etag1"},
	}, nil)

	file, err := d.uc.CompletePresignedUpload(ctx, int64(1), "session-1")

	assert.ErrorIs(t, err, ErrIncompleteUpload)
	assert.Nil(t, file)
}

func TestCompletePresignedUpload_Unauthorized(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(&UploadSession{
		ID: "session-1", UserID: 1, Status: "uploading",
	}, nil)

	file, err := d.uc.CompletePresignedUpload(ctx, int64(999), "session-1")

	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.Nil(t, file)
}

func TestAbortPresignedUpload_Success(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	session := &UploadSession{
		ID:            "session-1",
		UserID:        1,
		StorageTarget: "oss",
		ObjectKey:     "abc123/test.txt",
		S3UploadID:    "upload-id-1",
		Status:        "uploading",
	}

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(session, nil)
	d.cloudStore.On("AbortMultipartUpload", ctx, "abc123/test.txt", "upload-id-1").Return(nil)
	d.repo.On("UpdateUploadSessionStatus", ctx, "session-1", "aborted").Return(nil)

	err := d.uc.AbortPresignedUpload(ctx, int64(1), "session-1")

	assert.NoError(t, err)
	d.repo.AssertExpectations(t)
}

func TestAbortPresignedUpload_Unauthorized(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-1").Return(&UploadSession{
		ID: "session-1", UserID: 1, Status: "uploading",
	}, nil)

	err := d.uc.AbortPresignedUpload(ctx, int64(999), "session-1")

	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestAbortPresignedUpload_AlreadyCompleted(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindUploadSessionByID", ctx, "session-done").Return(&UploadSession{
		ID: "session-done", UserID: 1, Status: "completed",
	}, nil)

	err := d.uc.AbortPresignedUpload(ctx, int64(1), "session-done")

	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// MockErasureEncoder
// ---------------------------------------------------------------------------

type MockErasureEncoder struct{ mock.Mock }

func (m *MockErasureEncoder) Encode(filePath, storeDir, fileMD5 string, dataShards, parityShards int) ([]string, error) {
	args := m.Called(filePath, storeDir, fileMD5, dataShards, parityShards)
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockErasureEncoder) Reconstruct(shardPaths []string, dataShards, parityShards int) ([]byte, error) {
	args := m.Called(shardPaths, dataShards, parityShards)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockErasureEncoder) ShardChecksum(path string) (string, error) {
	args := m.Called(path)
	return args.String(0), args.Error(1)
}

// ---------------------------------------------------------------------------
// Erasure coding tests
// ---------------------------------------------------------------------------

func TestMergeChunks_WithErasureCoding(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	mq := new(MockMessageProducer)
	mq.On("SendCloudMigrateMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("SendThumbnailMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("Close").Return(nil).Maybe()
	cloudStore := new(MockCloudStorage)
	cloudStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	ecEnc := new(MockErasureEncoder)
	ecCfg := &ErasureConfig{DataShards: 4, ParityShards: 2, MinFileSize: 100}

	uc := NewFileUsecase(repo, userClient, mq, cloudStore, defaultStorageCfg, ecCfg, ecEnc, "/tmp/test-store", log.DefaultLogger)
	ctx := context.Background()

	// Setup standard merge flow
	repo.On("FindStoreByMD5", ctx, "ecmd5").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "ecmd5", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "ecmd5").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "ecmd5", "completed").Return(nil, errors.New("not found"))
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		ID: 100, FileMD5: "ecmd5", StorePath: "ecmd5.bin", StorageType: StorageLocal, UploadStatus: "uploading", RefCount: 1,
	}, nil)
	repo.On("CountUploadedChunks", ctx, "ecmd5").Return(int32(2), nil)
	repo.On("MergeChunkData", ctx, "ecmd5", "big.bin", int32(2)).Return("/tmp/test-store/ecmd5.bin", nil)
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("IncrDiskUsage", ctx, "local", int64(2048)).Return(nil)

	// Erasure coding: Encode is called, returns 6 shards
	shardPaths := []string{"/tmp/test-store/ecmd5.shard.0", "/tmp/test-store/ecmd5.shard.1", "/tmp/test-store/ecmd5.shard.2", "/tmp/test-store/ecmd5.shard.3", "/tmp/test-store/ecmd5.shard.4", "/tmp/test-store/ecmd5.shard.5"}
	ecEnc.On("Encode", "/tmp/test-store/ecmd5.bin", "/tmp/test-store", "ecmd5", 4, 2).Return(shardPaths, nil)
	ecEnc.On("ShardChecksum", mock.Anything).Return("shardmd5", nil)
	repo.On("CreateErasureShard", ctx, mock.AnythingOfType("*biz.ErasureShard")).Return(nil)

	// Storage type should become local_ec
	repo.On("UpdateStorageLocation", ctx, "ecmd5", StorageLocalEC, "ecmd5.bin").Return(nil)
	repo.On("UpdateStoreStatus", ctx, int64(100), "completed").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{ID: 200}, nil)
	repo.On("ClearChunkInfo", ctx, "ecmd5").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(1), int64(2048)).Return(nil)

	file, err := uc.MergeChunks(ctx, 1, 0, "big.bin", "ecmd5", 2048, 2)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	// Verify CreateErasureShard was called 6 times
	repo.AssertNumberOfCalls(t, "CreateErasureShard", 6)
	ecEnc.AssertCalled(t, "Encode", "/tmp/test-store/ecmd5.bin", "/tmp/test-store", "ecmd5", 4, 2)
}

func TestMergeChunks_ErasureSkippedBelowMinSize(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	mq := new(MockMessageProducer)
	mq.On("SendCloudMigrateMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("SendThumbnailMessage", mock.Anything, mock.Anything).Return(nil).Maybe()
	mq.On("Close").Return(nil).Maybe()
	cloudStore := new(MockCloudStorage)
	cloudStore.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	ecEnc := new(MockErasureEncoder)
	ecCfg := &ErasureConfig{DataShards: 4, ParityShards: 2, MinFileSize: 10000} // file too small

	uc := NewFileUsecase(repo, userClient, mq, cloudStore, defaultStorageCfg, ecCfg, ecEnc, "/tmp/test-store", log.DefaultLogger)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "small").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "small", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "small").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "small", "completed").Return(nil, errors.New("not found"))
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		ID: 50, FileMD5: "small", StorePath: "small.txt", StorageType: StorageLocal, UploadStatus: "uploading", RefCount: 1,
	}, nil)
	repo.On("CountUploadedChunks", ctx, "small").Return(int32(1), nil)
	repo.On("MergeChunkData", ctx, "small", "tiny.txt", int32(1)).Return("/tmp/test-store/small.txt", nil)
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)
	repo.On("IncrDiskUsage", ctx, "local", int64(50)).Return(nil)

	// No erasure coding, storage stays "local"
	repo.On("UpdateStorageLocation", ctx, "small", StorageLocal, "small.txt").Return(nil)
	repo.On("UpdateStoreStatus", ctx, int64(50), "completed").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{ID: 300}, nil)
	repo.On("ClearChunkInfo", ctx, "small").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(1), int64(50)).Return(nil)

	file, err := uc.MergeChunks(ctx, 1, 0, "tiny.txt", "small", 50, 1)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	// Encode should NOT be called
	ecEnc.AssertNotCalled(t, "Encode")
	// Storage location should be "local", not "local_ec"
	repo.AssertCalled(t, "UpdateStorageLocation", ctx, "small", StorageLocal, "small.txt")
}

func TestOpenLocalFile_EC_Reconstruct(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	ecEnc := new(MockErasureEncoder)
	ecCfg := &ErasureConfig{DataShards: 4, ParityShards: 2, MinFileSize: 100}
	d.uc.erasureCfg = ecCfg
	d.uc.erasureEnc = ecEnc

	d.repo.On("FindByID", ctx, int64(10)).Return(&File{
		ID: 10, UserID: 1, Name: "data.zip", FileMD5: "ecfile", Size: 2048, IsFolder: false,
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "ecfile").Return(&FileStore{
		ID: 42, FileMD5: "ecfile", StorageType: StorageLocalEC, StorePath: "ecfile.zip",
	}, nil)
	d.repo.On("UpdateLastAccessed", ctx, "ecfile").Return(nil)

	d.repo.On("FindErasureShards", ctx, int64(42)).Return([]*ErasureShard{
		{ShardIndex: 0, ShardPath: "ecfile.shard.0"},
		{ShardIndex: 1, ShardPath: "ecfile.shard.1"},
		{ShardIndex: 2, ShardPath: "ecfile.shard.2"},
		{ShardIndex: 3, ShardPath: "ecfile.shard.3"},
		{ShardIndex: 4, ShardPath: "ecfile.shard.4"},
		{ShardIndex: 5, ShardPath: "ecfile.shard.5"},
	}, nil)

	reconstructedData := []byte("reconstructed file content here")
	expectedPaths := make([]string, 6)
	for i := 0; i < 6; i++ {
		expectedPaths[i] = fmt.Sprintf("/tmp/test-store/ecfile.shard.%d", i)
	}
	ecEnc.On("Reconstruct", expectedPaths, 4, 2).Return(reconstructedData, nil)

	rc, name, size, mime, err := d.uc.OpenLocalFile(ctx, 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, "data.zip", name)
	assert.Equal(t, int64(len(reconstructedData)), size)
	assert.Contains(t, mime, "zip")
	data, _ := io.ReadAll(rc)
	rc.Close()
	assert.Equal(t, reconstructedData, data)
}

func TestEncodeWithErasure_EncoderNil(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	// uc with no erasure encoder
	uc := newTestFileUsecase(repo, userClient)

	result := uc.encodeWithErasure(context.Background(), "/some/path", "md5", 2048, 1)
	assert.False(t, result) // skipped because encoder is nil
}
