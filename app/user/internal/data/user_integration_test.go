//go:build integration

package data

import (
	"context"
	"os"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/conf"
)

// These integration tests require a running MySQL instance
// Run with: go test -tags=integration -v ./app/user/internal/data/
//
// Required env vars: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
// Defaults: localhost:3306, root, root123, cloud_disk_test

func setupTestData(t *testing.T) (*Data, func()) {
	t.Helper()

	// Set test defaults if env vars are missing
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

	logger := log.DefaultLogger
	d, cleanup, err := NewData(&conf.Data{}, logger)
	require.NoError(t, err)

	// Auto-migrate schema for tests
	err = d.db.AutoMigrate(&UserPO{})
	require.NoError(t, err)

	// Clean users table before each test
	d.db.Exec("TRUNCATE TABLE users")

	return d, cleanup
}

func TestIntegration_UserRepo_Create(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewUserRepo(d, log.DefaultLogger)
	ctx := context.Background()

	user, err := repo.Create(ctx, &biz.User{
		Username:     "testuser",
		Password:     "password123",
		Nickname:     "Test User",
		Email:        "test@example.com",
		StorageLimit: 10 * 1024 * 1024 * 1024,
	})

	assert.NoError(t, err)
	assert.Greater(t, user.ID, int64(0))
	assert.Equal(t, "testuser", user.Username)
	// Password should be hashed
	assert.NotEqual(t, "password123", user.Password)
}

func TestIntegration_UserRepo_FindByUsername(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewUserRepo(d, log.DefaultLogger)
	ctx := context.Background()

	_, err := repo.Create(ctx, &biz.User{
		Username: "findme",
		Password: "pass",
		Nickname: "Find Me",
	})
	require.NoError(t, err)

	found, err := repo.FindByUsername(ctx, "findme")
	assert.NoError(t, err)
	assert.Equal(t, "findme", found.Username)

	_, err = repo.FindByUsername(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestIntegration_UserRepo_FindByID(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewUserRepo(d, log.DefaultLogger)
	ctx := context.Background()

	created, err := repo.Create(ctx, &biz.User{
		Username: "byid",
		Password: "pass",
	})
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "byid", found.Username)
}

func TestIntegration_UserRepo_Update(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewUserRepo(d, log.DefaultLogger)
	ctx := context.Background()

	created, err := repo.Create(ctx, &biz.User{
		Username: "updateme",
		Password: "pass",
		Nickname: "Old",
	})
	require.NoError(t, err)

	created.Nickname = "New"
	created.Email = "new@example.com"
	err = repo.Update(ctx, created)
	assert.NoError(t, err)

	found, _ := repo.FindByID(ctx, created.ID)
	assert.Equal(t, "New", found.Nickname)
	assert.Equal(t, "new@example.com", found.Email)
}

func TestIntegration_UserRepo_UpdateStorageUsed(t *testing.T) {
	d, cleanup := setupTestData(t)
	defer cleanup()

	repo := NewUserRepo(d, log.DefaultLogger)
	ctx := context.Background()

	created, err := repo.Create(ctx, &biz.User{
		Username:    "storage",
		Password:    "pass",
		StorageUsed: 0,
	})
	require.NoError(t, err)

	err = repo.UpdateStorageUsed(ctx, created.ID, 1024)
	assert.NoError(t, err)

	found, _ := repo.FindByID(ctx, created.ID)
	assert.Equal(t, int64(1024), found.StorageUsed)

	err = repo.UpdateStorageUsed(ctx, created.ID, 512)
	assert.NoError(t, err)

	found, _ = repo.FindByID(ctx, created.ID)
	assert.Equal(t, int64(1536), found.StorageUsed)
}
