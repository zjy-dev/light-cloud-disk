package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

// kafkaProducer implements biz.MessageProducer using segmentio/kafka-go
type kafkaProducer struct {
	cloudMigrateWriter *kafka.Writer
	thumbnailWriter    *kafka.Writer
	log                *log.Helper
}

// NewMessageProducer creates the appropriate MessageProducer implementation.
// When Kafka brokers are configured (KAFKA_BROKERS env or config), it returns a
// Kafka-backed producer. Otherwise it returns an in-process goroutine MQ that
// handles messages using buffered channels, which is ideal for local mode without
// external MQ infrastructure.
func NewMessageProducer(
	c *conf.Data,
	storageCfg *conf.Storage,
	repo biz.FileRepo,
	objStore biz.ObjectStorage,
	cloudStore biz.CloudStorage,
	logger log.Logger,
) (biz.MessageProducer, func(), error) {
	helper := log.NewHelper(logger)
	brokers := resolveBrokers(c)

	if len(brokers) == 0 {
		// Determine store_dir for goroutine MQ (needed for local-mode migration)
		storeDir := os.Getenv("FILE_STORE_DIR")
		if storeDir == "" {
			storeDir = "./store"
		}
		mq, cleanup := newGoroutineMQ(repo, objStore, cloudStore, storeDir, logger)
		return mq, cleanup, nil
	}

	// Kafka mode
	cloudMigrateTopic := "cloud-migrate"
	thumbnailTopic := "file-thumbnail"
	if c.Kafka != nil {
		if c.Kafka.CloudMigrateTopic != "" {
			cloudMigrateTopic = c.Kafka.CloudMigrateTopic
		}
		if c.Kafka.ThumbnailTopic != "" {
			thumbnailTopic = c.Kafka.ThumbnailTopic
		}
	}

	helper.Infof("Connecting Kafka producer. brokers=%v, topics=[%s, %s]", brokers, cloudMigrateTopic, thumbnailTopic)

	newWriter := func(topic string) *kafka.Writer {
		return &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		}
	}

	p := &kafkaProducer{
		cloudMigrateWriter: newWriter(cloudMigrateTopic),
		thumbnailWriter:    newWriter(thumbnailTopic),
		log:                helper,
	}
	cleanup := func() {
		helper.Info("Closing Kafka producers.")
		p.Close()
	}
	return p, cleanup, nil
}

// resolveBrokers reads broker addresses from KAFKA_BROKERS env or config.
// If KAFKA_BROKERS env is explicitly set (even to empty), it takes precedence.
func resolveBrokers(c *conf.Data) []string {
	if env, ok := os.LookupEnv("KAFKA_BROKERS"); ok {
		if env == "" {
			return nil // Explicitly disabled
		}
		return strings.Split(env, ",")
	}
	if c.Kafka != nil {
		return c.Kafka.Brokers
	}
	return nil
}

func (p *kafkaProducer) SendCloudMigrateMessage(ctx context.Context, msg *biz.CloudMigrateMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = p.cloudMigrateWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.FileMD5),
		Value: data,
	})
	if err != nil {
		p.log.Errorf("Failed to publish cloud-migrate message: %v", err)
	}
	return err
}

func (p *kafkaProducer) SendThumbnailMessage(ctx context.Context, msg *biz.ThumbnailMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = p.thumbnailWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", msg.FileID)),
		Value: data,
	})
	if err != nil {
		p.log.Errorf("Failed to publish thumbnail message: %v", err)
	}
	return err
}

func (p *kafkaProducer) Close() error {
	var errs []error
	if err := p.cloudMigrateWriter.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := p.thumbnailWriter.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// noopProducer is used when Kafka is not configured
type noopProducer struct{}

func (p *noopProducer) SendCloudMigrateMessage(_ context.Context, _ *biz.CloudMigrateMessage) error {
	return nil
}

func (p *noopProducer) SendThumbnailMessage(_ context.Context, _ *biz.ThumbnailMessage) error {
	return nil
}

func (p *noopProducer) Close() error {
	return nil
}
