package data

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

const (
	mqBufferSize      = 256
	mqShutdownTimeout = 10 * time.Second
)

// NewMessageProducer creates a Kafka-backed producer when KAFKA_BROKERS is set,
// otherwise falls back to the goroutine-backed in-process queue.
func NewMessageProducer(
	repo biz.FileRepo,
	cloudStore biz.CloudStorage,
	storeDir string,
	logger log.Logger,
) (biz.MessageProducer, func(), error) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers != "" {
		cloudMigrateTopic := "cloud-migrate"
		thumbnailTopic := "file-thumbnail"
		p := newKafkaProducer(strings.Split(brokers, ","), cloudMigrateTopic, thumbnailTopic, logger)
		cleanup := func() {
			_ = p.Close()
		}
		return p, cleanup, nil
	}
	mq, cleanup := newGoroutineMQ(repo, cloudStore, storeDir, logger)
	return mq, cleanup, nil
}

// goroutineMQ implements biz.MessageProducer using in-process buffered channels.
// Used as the default MQ when KAFKA_BROKERS is not configured,
// so that the single-server local mode works without external MQ infra.
type goroutineMQ struct {
	cloudMigrateCh chan *biz.CloudMigrateMessage
	thumbnailCh    chan *biz.ThumbnailMessage

	repo       biz.FileRepo
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
	cloudStore biz.CloudStorage,
	storeDir string,
	logger log.Logger,
) (biz.MessageProducer, func()) {
	helper := log.NewHelper(logger)

	mq := &goroutineMQ{
		cloudMigrateCh: make(chan *biz.CloudMigrateMessage, mqBufferSize),
		thumbnailCh:    make(chan *biz.ThumbnailMessage, mqBufferSize),
		repo:           repo,
		cloudStore:     cloudStore,
		storeDir:       storeDir,
		log:            helper,
		done:           make(chan struct{}),
	}

	mq.wg.Add(2)
	go mq.consumeCloudMigrate()
	go mq.consumeThumbnails()

	helper.Info("Goroutine MQ started (in-process message processing).")

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

// handleCloudMigrate reads from local storage and uploads to OSS,
// then updates DB and disk usage counters.
func (mq *goroutineMQ) handleCloudMigrate(msg *biz.CloudMigrateMessage) error {
	ctx := context.Background()

	mq.log.Infof("goroutine-mq: cloud-migrate md5=%s key=%s size=%d", msg.FileMD5, msg.SourceKey, msg.FileSize)
	if err := migrateFileStoreToCloud(ctx, mq.repo, mq.cloudStore, mq.storeDir, msg, mq.log); err != nil {
		return err
	}
	mq.log.Infof("goroutine-mq: cloud-migrate done for md5=%s", msg.FileMD5)
	return nil
}
