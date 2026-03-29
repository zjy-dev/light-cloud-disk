package data

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

const (
	kafkaWriteTimeout = 10 * time.Second
	kafkaReadTimeout  = 10 * time.Second
	kafkaGroupID      = "file-service"
)

// kafkaProducer implements biz.MessageProducer using Kafka.
type kafkaProducer struct {
	cloudMigrateWriter *kafka.Writer
	thumbnailWriter    *kafka.Writer
	log                *log.Helper
}

func newKafkaProducer(brokers []string, cloudMigrateTopic, thumbnailTopic string, logger log.Logger) biz.MessageProducer {
	helper := log.NewHelper(logger)
	p := &kafkaProducer{
		cloudMigrateWriter: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        cloudMigrateTopic,
			Balancer:     &kafka.LeastBytes{},
			WriteTimeout: kafkaWriteTimeout,
			RequiredAcks: kafka.RequireAll,
		},
		thumbnailWriter: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        thumbnailTopic,
			Balancer:     &kafka.LeastBytes{},
			WriteTimeout: kafkaWriteTimeout,
			RequiredAcks: kafka.RequireAll,
		},
		log: helper,
	}
	helper.Infof("Kafka producer created: brokers=%v cloud_migrate_topic=%s thumbnail_topic=%s",
		brokers, cloudMigrateTopic, thumbnailTopic)
	return p
}

func (p *kafkaProducer) SendCloudMigrateMessage(ctx context.Context, msg *biz.CloudMigrateMessage) error {
	val, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.cloudMigrateWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.FileMD5),
		Value: val,
	})
}

func (p *kafkaProducer) SendThumbnailMessage(ctx context.Context, msg *biz.ThumbnailMessage) error {
	val, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.thumbnailWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.FilePath),
		Value: val,
	})
}

func (p *kafkaProducer) Close() error {
	e1 := p.cloudMigrateWriter.Close()
	e2 := p.thumbnailWriter.Close()
	if e1 != nil {
		return e1
	}
	return e2
}

// KafkaConsumer reads from Kafka topics and processes messages.
type KafkaConsumer struct {
	cloudMigrateReader *kafka.Reader
	thumbnailReader    *kafka.Reader

	repo       biz.FileRepo
	cloudStore biz.CloudStorage
	storeDir   string

	log  *log.Helper
	wg   sync.WaitGroup
	done chan struct{}
}

// NewKafkaConsumer creates a consumer that reads from cloud-migrate and thumbnail topics.
func NewKafkaConsumer(
	brokers []string,
	cloudMigrateTopic, thumbnailTopic string,
	repo biz.FileRepo,
	cloudStore biz.CloudStorage,
	storeDir string,
	logger log.Logger,
) *KafkaConsumer {
	helper := log.NewHelper(logger)

	c := &KafkaConsumer{
		cloudMigrateReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       cloudMigrateTopic,
			GroupID:     kafkaGroupID,
			StartOffset: kafka.LastOffset,
			MaxWait:     kafkaReadTimeout,
		}),
		thumbnailReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       thumbnailTopic,
			GroupID:     kafkaGroupID,
			StartOffset: kafka.LastOffset,
			MaxWait:     kafkaReadTimeout,
		}),
		repo:       repo,
		cloudStore: cloudStore,
		storeDir:   storeDir,
		log:        helper,
		done:       make(chan struct{}),
	}

	helper.Infof("Kafka consumer created: brokers=%v topics=[%s, %s]",
		brokers, cloudMigrateTopic, thumbnailTopic)
	return c
}

// Start begins consuming messages in background goroutines.
func (c *KafkaConsumer) Start(ctx context.Context) error {
	c.wg.Add(2)
	go c.consumeCloudMigrate(ctx)
	go c.consumeThumbnails(ctx)
	c.log.Info("Kafka consumer started.")
	return nil
}

// Stop gracefully shuts down the consumer.
func (c *KafkaConsumer) Stop(ctx context.Context) error {
	close(c.done)
	c.wg.Wait()
	e1 := c.cloudMigrateReader.Close()
	e2 := c.thumbnailReader.Close()
	c.log.Info("Kafka consumer stopped.")
	if e1 != nil {
		return e1
	}
	return e2
}

func (c *KafkaConsumer) consumeCloudMigrate(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-c.done:
			return
		default:
		}

		msg, err := c.cloudMigrateReader.FetchMessage(ctx)
		if err != nil {
			select {
			case <-c.done:
				return
			default:
				c.log.Errorf("kafka: cloud-migrate fetch error: %v", err)
				time.Sleep(time.Second)
				continue
			}
		}

		var cm biz.CloudMigrateMessage
		if err := json.Unmarshal(msg.Value, &cm); err != nil {
			c.log.Errorf("kafka: cloud-migrate unmarshal error: %v", err)
			_ = c.cloudMigrateReader.CommitMessages(ctx, msg)
			continue
		}

		if err := c.handleCloudMigrate(&cm); err != nil {
			c.log.Errorf("kafka: cloud-migrate handler error for %s: %v", cm.FileMD5, err)
		}
		_ = c.cloudMigrateReader.CommitMessages(ctx, msg)
	}
}

func (c *KafkaConsumer) consumeThumbnails(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-c.done:
			return
		default:
		}

		msg, err := c.thumbnailReader.FetchMessage(ctx)
		if err != nil {
			select {
			case <-c.done:
				return
			default:
				c.log.Errorf("kafka: thumbnail fetch error: %v", err)
				time.Sleep(time.Second)
				continue
			}
		}

		var tm biz.ThumbnailMessage
		if err := json.Unmarshal(msg.Value, &tm); err != nil {
			c.log.Errorf("kafka: thumbnail unmarshal error: %v", err)
			_ = c.thumbnailReader.CommitMessages(ctx, msg)
			continue
		}

		c.log.Infof("kafka: thumbnail stub for file_id=%d path=%s", tm.FileID, tm.FilePath)
		_ = c.thumbnailReader.CommitMessages(ctx, msg)
	}
}

// handleCloudMigrate reads from local storage and uploads to OSS.
func (c *KafkaConsumer) handleCloudMigrate(msg *biz.CloudMigrateMessage) error {
	ctx := context.Background()

	c.log.Infof("kafka: cloud-migrate md5=%s key=%s size=%d", msg.FileMD5, msg.SourceKey, msg.FileSize)
	if err := migrateFileStoreToCloud(ctx, c.repo, c.cloudStore, c.storeDir, msg, c.log); err != nil {
		return err
	}
	c.log.Infof("kafka: cloud-migrate done for md5=%s", msg.FileMD5)
	return nil
}
