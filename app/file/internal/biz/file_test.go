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
func (m *MockFileRepo) CreateChunkRecord(ctx context.Context, rec *ChunkRecord) error {
	return m.Called(ctx, rec).Error(0)
}
func (m *MockFileRepo) FindChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) ([]*ChunkRecord, error) {
	args := m.Called(ctx, fileMD5, fileSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ChunkRecord), args.Error(1)
}
func (m *MockFileRepo) FindChunkRecordByIndex(ctx context.Context, fileMD5 string, fileSize int64, chunkIndex int32) (*ChunkRecord, error) {
	args := m.Called(ctx, fileMD5, fileSize, chunkIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ChunkRecord), args.Error(1)
}
func (m *MockFileRepo) CountChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) (int32, error) {
	args := m.Called(ctx, fileMD5, fileSize)
	return args.Get(0).(int32), args.Error(1)
}
func (m *MockFileRepo) DeleteChunkRecords(ctx context.Context, fileMD5 string, fileSize int64) error {
	return m.Called(ctx, fileMD5, fileSize).Error(0)
}
func (m *MockFileRepo) UpdateChunkStorageLocation(ctx context.Context, id int64, storageType, storagePath string) error {
	return m.Called(ctx, id, storageType, storagePath).Error(0)
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
// CompleteUpload tests
// ---------------------------------------------------------------------------

func TestCompleteUpload_ExistingStore(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 2, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), file.ID)
	repo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestCompleteUpload_UpdateStorageFailsGracefully(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 3, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(errors.New("user-service unavailable"))

	file, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.NotNil(t, file)
	repo.AssertExpectations(t)
}

func TestCompleteUpload_LockFailed(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(false, nil)

	_, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

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

func TestCompleteUpload_DedupAfterLock(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 5, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(5), file.ID)
	repo.AssertExpectations(t)
}

func TestCompleteUpload_UniqueConstraintRace(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	repo.On("CountChunkRecords", ctx, "abc123", int64(2048)).Return(int32(2), nil)
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered, UploadStatus: "completed", RefCount: 1,
	}, nil)
	repo.On("IncrStoreRefCount", ctx, "abc123").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 6, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)

	file, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(6), file.ID)
	repo.AssertExpectations(t)
}

func TestCompleteUpload_FullFlow(t *testing.T) {
	repo := new(MockFileRepo)
	userClient := new(MockUserClient)
	uc := newTestFileUsecase(repo, userClient)
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	repo.On("CountChunkRecords", ctx, "abc123", int64(2048)).Return(int32(2), nil)
	repo.On("CreateStoreWithStatus", ctx, mock.AnythingOfType("*biz.FileStore")).Return(&FileStore{
		ID: 42, FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered, UploadStatus: "uploading", RefCount: 1, TotalChunks: 2,
	}, nil)
	repo.On("UpdateStoreStatus", ctx, int64(42), "completed").Return(nil)
	repo.On("Create", ctx, mock.AnythingOfType("*biz.File")).Return(&File{
		ID: 10, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	repo.On("ClearChunkInfo", ctx, "abc123").Return(nil)
	userClient.On("UpdateStorageUsed", ctx, int64(100), int64(2048)).Return(nil)
	repo.On("GetDiskUsage", ctx, "local").Return(int64(0), nil)

	file, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 2048, 2)

	assert.NoError(t, err)
	assert.Equal(t, int64(10), file.ID)
	assert.Equal(t, "file.zip", file.Name)
	repo.AssertCalled(t, "UpdateStoreStatus", ctx, int64(42), "completed")
	repo.AssertExpectations(t)
}

func TestCompleteUpload_IncompleteChunks(t *testing.T) {
	repo := new(MockFileRepo)
	uc := newTestFileUsecase(repo, new(MockUserClient))
	ctx := context.Background()

	repo.On("FindStoreByMD5", ctx, "abc123").Return(nil, errors.New("not found"))
	repo.On("AcquireMergeLock", ctx, "abc123", mock.Anything).Return(true, nil)
	repo.On("ReleaseMergeLock", ctx, "abc123").Return(nil)
	repo.On("FindStoreByMD5AndStatus", ctx, "abc123", "completed").Return(nil, errors.New("not found"))
	// Only 3 of 5 chunk records
	repo.On("CountChunkRecords", ctx, "abc123", int64(5120)).Return(int32(3), nil)

	_, err := uc.CompleteUpload(ctx, 100, 0, "file.zip", "abc123", 5120, 5)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete chunks")
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

func TestGetDownloadPlan_MixedChunkStorage(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(7)).Return(&File{
		ID: 7, UserID: 100, Name: "file.zip", FileMD5: "abc123", Size: 2048,
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "abc123").Return(&FileStore{
		FileMD5: "abc123", StorePath: "abc123.zip", StorageType: StorageScattered,
	}, nil)
	d.repo.On("FindChunkRecords", ctx, "abc123", int64(2048)).Return([]*ChunkRecord{
		{
			FileMD5: "abc123", ChunkIndex: 0, ChunkSize: 1024,
			InstanceID: "10.0.0.1:9003", StorageType: StorageLocal, Checksum: "sum-0",
		},
		{
			FileMD5: "abc123", ChunkIndex: 1, ChunkSize: 1024,
			StorageType: StorageOSS, StorePath: "chunks/abc123/000001.part", Checksum: "sum-1",
		},
	}, nil)
	d.cloudStore.On("PresignGetURL", ctx, "chunks/abc123/000001.part", time.Hour).
		Return("http://oss/chunks/abc123/000001.part", nil).Once()

	plan, err := d.uc.GetDownloadPlan(ctx, 100, 7)

	assert.NoError(t, err)
	assert.Equal(t, int32(2), plan.TotalChunks)
	assert.Len(t, plan.Chunks, 2)
	assert.Equal(t, "http://10.0.0.1:9003/api/v1/chunks/abc123/0", plan.Chunks[0].DownloadURL)
	assert.Equal(t, "http://oss/chunks/abc123/000001.part", plan.Chunks[1].DownloadURL)
	d.repo.AssertExpectations(t)
	d.cloudStore.AssertExpectations(t)
}

func TestGetDownloadPlan_SingleOSSObjectWithoutChunkRecords(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindByID", ctx, int64(8)).Return(&File{
		ID: 8, UserID: 100, Name: "archive.zip", FileMD5: "def456", Size: 4096,
	}, nil)
	d.repo.On("FindStoreByMD5", ctx, "def456").Return(&FileStore{
		FileMD5: "def456", StorePath: "def456.zip", StorageType: StorageOSS,
	}, nil)
	d.repo.On("FindChunkRecords", ctx, "def456", int64(4096)).Return([]*ChunkRecord{}, nil)
	d.cloudStore.On("PresignGetURL", ctx, "def456.zip", time.Hour).
		Return("http://oss/def456.zip", nil).Once()

	plan, err := d.uc.GetDownloadPlan(ctx, 100, 8)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), plan.TotalChunks)
	assert.Len(t, plan.Chunks, 1)
	assert.Equal(t, "http://oss/def456.zip", plan.Chunks[0].DownloadURL)
	d.repo.AssertExpectations(t)
	d.cloudStore.AssertExpectations(t)
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
	d.repo.On("FindLRUStores", ctx, StorageScattered, 100).Return([]*FileStore{}, nil)
	d.mq.On("SendCloudMigrateMessage", ctx, mock.AnythingOfType("*biz.CloudMigrateMessage")).Return(nil)

	d.uc.maybeEvictToCloud(ctx, 9*1024*1024*1024) // 9 GB > 8 GB threshold

	d.mq.AssertCalled(t, "SendCloudMigrateMessage", ctx, mock.Anything)
}

func TestMaybeEvictToCloud_IncludesScatteredCandidates(t *testing.T) {
	d := newTestDeps()
	ctx := context.Background()

	d.repo.On("FindLRUStores", ctx, StorageLocal, 100).Return([]*FileStore{}, nil)
	d.repo.On("FindLRUStores", ctx, StorageScattered, 100).Return([]*FileStore{
		{FileMD5: "scatter-1", StorePath: "scatter-1.bin", Size: 800 * 1024 * 1024},
	}, nil)
	d.mq.On("SendCloudMigrateMessage", ctx, mock.MatchedBy(func(msg *CloudMigrateMessage) bool {
		return msg.FileMD5 == "scatter-1"
	})).Return(nil).Once()

	d.uc.maybeEvictToCloud(ctx, 9*1024*1024*1024)

	d.mq.AssertExpectations(t)
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
