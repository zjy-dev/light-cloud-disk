//go:build integration

package data

import (
	"context"
	"os"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

// These tests require a running MySQL and Redis instance.
// Run with: go test -tags=integration -v ./app/file/internal/data/
//
// Required env vars: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, REDIS_ADDR
// Or defaults: localhost:3306, root, root123, cloud_disk_test, localhost:6379

func setupTestData(t *testing.T) (*Data, func()) {
	t.Helper()

	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost")
	}
	if os.Getenv("DB_PORT") == "" {
		os.Setenv("DB_PORT", "3306")
	}
	if os.Getenv("DB_USER") == "" {
		os.Setenv("DB_USER", "root")
	}
	if os.Getenv("DB_PASSWORD") == "" {
		os.Setenv("DB_PASSWORD", "root123")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "cloud_disk_test")
	}
	if os.Getenv("REDIS_ADDR") == "" {
		os.Setenv("REDIS_ADDR", "localhost:6379")
	}

	logger := log.DefaultLogger
	d, cleanup, err := NewData(&conf.Data{}, logger)
	require.NoError(t, err)

	// Auto-migrate for test
	err = d.db.AutoMigrate(&FilePO{}, &FileStorePO{}, &SharePO{})
	require.NoError(t, err)

	// Clean tables before each test
	d.db.Exec("TRUNCATE TABLE files")
	d.db.Exec("TRUNCATE TABLE file_stores")
	d.db.Exec("TRUNCATE TABLE shares")

	// Flush Redis test data
	d.redis.FlushDB(context.Background())

	return d, cleanup
}

func TestIntegration_FileRepo_CreateAndFind(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	created, err := repo.Create(ctx, &biz.File{
		UserID:   1,
		ParentID: 0,
		Name:     "test.txt",
		FileMD5:  "abc123",
		Size:     1024,
		IsFolder: false,
	})
	assert.NoError(t, err)
	assert.Greater(t, created.ID, int64(0))

	found, err := repo.FindByID(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", found.Name)
	assert.Equal(t, int64(1024), found.Size)
}

func TestIntegration_FileRepo_FindByUserAndParent(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "file1.txt"})
	repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "file2.txt"})
	repo.Create(ctx, &biz.File{UserID: 2, ParentID: 0, Name: "other.txt"}) // different user

	files, total, err := repo.FindByUserAndParent(ctx, 1, 0, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, files, 2)
}

func TestIntegration_FileRepo_SoftDeleteAndRestore(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	f, _ := repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "delete-me.txt"})

	err := repo.SoftDelete(ctx, []int64{f.ID})
	assert.NoError(t, err)

	// Should appear in trash
	trash, total, err := repo.FindTrash(ctx, 1, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, trash, 1)

	// Should not appear in normal listing
	files, total, err := repo.FindByUserAndParent(ctx, 1, 0, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, files)

	// Restore
	err = repo.Restore(ctx, []int64{f.ID})
	assert.NoError(t, err)

	files, total, _ = repo.FindByUserAndParent(ctx, 1, 0, 1, 20)
	assert.Equal(t, int64(1), total)
}

func TestIntegration_FileRepo_ChunkInfo(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	// Save chunks
	repo.SaveChunkInfo(ctx, &biz.ChunkInfo{FileMD5: "md5test", ChunkIndex: 0, ChunkSize: 512, Uploaded: true})
	repo.SaveChunkInfo(ctx, &biz.ChunkInfo{FileMD5: "md5test", ChunkIndex: 1, ChunkSize: 512, Uploaded: true})
	repo.SaveChunkInfo(ctx, &biz.ChunkInfo{FileMD5: "md5test", ChunkIndex: 3, ChunkSize: 512, Uploaded: true})

	chunks, err := repo.GetUploadedChunks(ctx, "md5test")
	assert.NoError(t, err)
	assert.Len(t, chunks, 3)

	// Clear
	err = repo.ClearChunkInfo(ctx, "md5test")
	assert.NoError(t, err)

	chunks, _ = repo.GetUploadedChunks(ctx, "md5test")
	assert.Empty(t, chunks)
}

func TestIntegration_FileRepo_Share(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	err := repo.CreateShare(ctx, &biz.Share{
		ID:       "share-abc",
		UserID:   1,
		FileID:   100,
		Password: "secret",
	})
	assert.NoError(t, err)

	share, err := repo.FindShareByID(ctx, "share-abc")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), share.UserID)
	assert.Equal(t, "secret", share.Password)

	_, err = repo.FindShareByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestIntegration_FileRepo_Search(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "report-2024.pdf"})
	repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "report-2025.pdf"})
	repo.Create(ctx, &biz.File{UserID: 1, ParentID: 0, Name: "photo.jpg"})

	files, total, err := repo.Search(ctx, 1, "report", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, files, 2)
}

func TestIntegration_FileRepo_FileStore(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewFileRepo(d, log.DefaultLogger)
	ctx := context.Background()

	err := repo.CreateStore(ctx, &biz.FileStore{
		FileMD5:   "store-md5",
		Size:      2048,
		StorePath: "/store/store-md5/file.zip",
		RefCount:  1,
	})
	assert.NoError(t, err)

	store, err := repo.FindStoreByMD5(ctx, "store-md5")
	assert.NoError(t, err)
	assert.Equal(t, int64(2048), store.Size)
	assert.Equal(t, int32(1), store.RefCount)

	err = repo.IncrStoreRefCount(ctx, "store-md5")
	assert.NoError(t, err)

	store, _ = repo.FindStoreByMD5(ctx, "store-md5")
	assert.Equal(t, int32(2), store.RefCount)

	err = repo.DecrStoreRefCount(ctx, "store-md5")
	assert.NoError(t, err)

	store, _ = repo.FindStoreByMD5(ctx, "store-md5")
	assert.Equal(t, int32(1), store.RefCount)
}
