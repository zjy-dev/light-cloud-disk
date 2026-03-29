package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

func migrateFileStoreToCloud(
	ctx context.Context,
	repo biz.FileRepo,
	cloudStore biz.CloudStorage,
	storeDir string,
	msg *biz.CloudMigrateMessage,
	helper *log.Helper,
) error {
	store, err := repo.FindStoreByMD5(ctx, msg.FileMD5)
	if err != nil {
		return err
	}

	records, err := repo.FindChunkRecords(ctx, msg.FileMD5, store.Size)
	if err != nil {
		return err
	}
	if len(records) > 0 {
		return migrateScatteredStoreToCloud(ctx, repo, cloudStore, msg.FileMD5, store, records)
	}

	if store.StorageType != biz.StorageLocal {
		helper.Warnf("cloud-migrate: skipping %s (already on %s)", msg.FileMD5, store.StorageType)
		return nil
	}

	return migrateLocalStoreToCloud(ctx, repo, cloudStore, storeDir, msg)
}

func migrateLocalStoreToCloud(
	ctx context.Context,
	repo biz.FileRepo,
	cloudStore biz.CloudStorage,
	storeDir string,
	msg *biz.CloudMigrateMessage,
) error {
	localPath := filepath.Join(storeDir, msg.SourceKey)
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := cloudStore.Put(ctx, msg.SourceKey, f, msg.FileSize); err != nil {
		return err
	}
	if err := repo.UpdateStorageLocation(ctx, msg.FileMD5, biz.StorageOSS, msg.SourceKey); err != nil {
		return err
	}
	_ = os.Remove(localPath)
	_ = repo.IncrDiskUsage(ctx, "local", -msg.FileSize)
	return nil
}

func migrateScatteredStoreToCloud(
	ctx context.Context,
	repo biz.FileRepo,
	cloudStore biz.CloudStorage,
	fileMD5 string,
	store *biz.FileStore,
	records []*biz.ChunkRecord,
) error {
	var migratedBytes int64
	for _, record := range records {
		if record.StorageType == biz.StorageOSS {
			continue
		}
		if record.StorageType != "" && record.StorageType != biz.StorageLocal {
			continue
		}

		f, err := os.Open(record.StorePath)
		if err != nil {
			return err
		}

		objectKey := scatteredChunkObjectKey(fileMD5, record.ChunkIndex)
		putErr := cloudStore.Put(ctx, objectKey, f, record.ChunkSize)
		_ = f.Close()
		if putErr != nil {
			return putErr
		}

		if err := repo.UpdateChunkStorageLocation(ctx, record.ID, biz.StorageOSS, objectKey); err != nil {
			return err
		}
		_ = os.Remove(record.StorePath)
		migratedBytes += record.ChunkSize
	}

	if migratedBytes == 0 {
		return nil
	}
	if err := repo.UpdateStorageLocation(ctx, fileMD5, biz.StorageOSS, store.StorePath); err != nil {
		return err
	}
	_ = repo.IncrDiskUsage(ctx, "local", -migratedBytes)
	return nil
}

func scatteredChunkObjectKey(fileMD5 string, chunkIndex int32) string {
	return fmt.Sprintf("chunks/%s/%06d.part", fileMD5, chunkIndex)
}