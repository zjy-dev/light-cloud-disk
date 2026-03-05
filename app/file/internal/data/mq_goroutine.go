package data

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

const (
	mqBufferSize      = 256
	mqShutdownTimeout = 10 * time.Second
)

// goroutineMQ implements biz.MessageProducer using in-process buffered channels.
// Used in place of Kafka when KAFKA_BROKERS is not configured,
// so that the single-server local mode works without external MQ infra.
type goroutineMQ struct {
	cloudMigrateCh chan *biz.CloudMigrateMessage
	thumbnailCh    chan *biz.ThumbnailMessage

	repo       biz.FileRepo
	objStore   biz.ObjectStorage
	cloudStore biz.CloudStorage
	storeDir   string

	log  *log.Helper
	wg   sync.WaitGroup
	done chan struct{}
}

// newGoroutineMQ creates a goroutine-backed message queue that processes
// cloud-migrate and thumbnail messages in-process.
func newGoroutineMQ(
	repo biz.FileRepo,
	objStore biz.ObjectStorage,
	cloudStore biz.CloudStorage,
	storeDir string,
	logger log.Logger,
) (biz.MessageProducer, func()) {
	helper := log.NewHelper(logger)

	mq := &goroutineMQ{
		cloudMigrateCh: make(chan *biz.CloudMigrateMessage, mqBufferSize),
		thumbnailCh:    make(chan *biz.ThumbnailMessage, mqBufferSize),
		repo:           repo,
		objStore:       objStore,
		cloudStore:     cloudStore,
		storeDir:       storeDir,
		log:            helper,
		done:           make(chan struct{}),
	}

	mq.wg.Add(2)
	go mq.consumeCloudMigrate()
	go mq.consumeThumbnails()

	helper.Info("Goroutine MQ started (in-process message processing, no Kafka).")

	cleanup := func() {
		close(mq.done)
		mq.wg.Wait()
		helper.Info("Goroutine MQ stopped.")
	}
	return mq, cleanup
}

func (mq *goroutineMQ) SendCloudMigrateMessage(ctx context.Context, msg *biz.CloudMigrateMessage) error {
	select {
	case mq.cloudMigrateCh <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-mq.done:
		return nil
	}
}

func (mq *goroutineMQ) SendThumbnailMessage(ctx context.Context, msg *biz.ThumbnailMessage) error {
	select {
	case mq.thumbnailCh <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-mq.done:
		return nil
	}
}

func (mq *goroutineMQ) Close() error {
	return nil
}

// consumeCloudMigrate processes cloud migrate messages: primary storage -> OSS.
func (mq *goroutineMQ) consumeCloudMigrate() {
	defer mq.wg.Done()
	for {
		select {
		case msg := <-mq.cloudMigrateCh:
			if err := mq.handleCloudMigrate(msg); err != nil {
				mq.log.Errorf("goroutine-mq: cloud-migrate failed for %s: %v", msg.FileMD5, err)
			}
		case <-mq.done:
			// Drain remaining messages
			for {
				select {
				case msg := <-mq.cloudMigrateCh:
					if err := mq.handleCloudMigrate(msg); err != nil {
						mq.log.Errorf("goroutine-mq: cloud-migrate drain failed for %s: %v", msg.FileMD5, err)
					}
				default:
					return
				}
			}
		}
	}
}

func (mq *goroutineMQ) consumeThumbnails() {
	defer mq.wg.Done()
	for {
		select {
		case msg := <-mq.thumbnailCh:
			mq.log.Infof("goroutine-mq: thumbnail stub for file_id=%d path=%s", msg.FileID, msg.FilePath)
		case <-mq.done:
			return
		}
	}
}

// handleCloudMigrate downloads from primary storage and uploads to OSS,
// then updates DB and disk usage counters.
func (mq *goroutineMQ) handleCloudMigrate(msg *biz.CloudMigrateMessage) error {
	ctx := context.Background()

	mq.log.Infof("goroutine-mq: cloud-migrate md5=%s key=%s size=%d", msg.FileMD5, msg.SourceKey, msg.FileSize)

	// Determine current storage type from DB
	store, err := mq.repo.FindStoreByMD5(ctx, msg.FileMD5)
	if err != nil {
		return err
	}

	// 1) Read from primary storage
	var reader io.ReadCloser
	switch store.StorageType {
	case biz.StorageLocal:
		localPath := filepath.Join(mq.storeDir, msg.SourceKey)
		f, err := os.Open(localPath)
		if err != nil {
			return err
		}
		reader = f
	case biz.StorageSeaweedFS:
		r, err := mq.objStore.Get(ctx, msg.SourceKey)
		if err != nil {
			return err
		}
		reader = r
	default:
		mq.log.Warnf("goroutine-mq: skipping cloud-migrate for %s (already on %s)", msg.FileMD5, store.StorageType)
		return nil
	}
	defer reader.Close()

	// 2) Upload to OSS
	if err := mq.cloudStore.Put(ctx, msg.SourceKey, reader, msg.FileSize); err != nil {
		return err
	}

	// 3) Update DB: storage_type -> oss
	if err := mq.repo.UpdateStorageLocation(ctx, msg.FileMD5, biz.StorageOSS, msg.SourceKey); err != nil {
		return err
	}

	// 4) Delete from primary storage
	switch store.StorageType {
	case biz.StorageLocal:
		localPath := filepath.Join(mq.storeDir, msg.SourceKey)
		_ = os.Remove(localPath)
	case biz.StorageSeaweedFS:
		_ = mq.objStore.Delete(ctx, msg.SourceKey)
	}

	// 5) Decrease primary disk usage counter
	_ = mq.repo.IncrDiskUsage(ctx, store.StorageType, -msg.FileSize)

	mq.log.Infof("goroutine-mq: cloud-migrate done for md5=%s, moved %d bytes to OSS", msg.FileMD5, msg.FileSize)
	return nil
}
