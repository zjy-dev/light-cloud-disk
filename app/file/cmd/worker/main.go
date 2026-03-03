package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
)

// TransferMessage mirrors biz.TransferMessage for deserialization.
type TransferMessage struct {
	FileMD5      string `json:"file_md5"`
	CurLocation  string `json:"cur_location"`
	DestLocation string `json:"dest_location"`
}

// ThumbnailMessage mirrors biz.ThumbnailMessage for deserialization.
type ThumbnailMessage struct {
	FileID   int64  `json:"file_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
}

func main() {
	godotenv.Load()

	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "file-worker",
	)
	helper := log.NewHelper(logger)

	brokers := resolveBrokers()
	if len(brokers) == 0 {
		helper.Fatal("KAFKA_BROKERS not configured")
	}

	transferTopic := envOrDefault("KAFKA_TRANSFER_TOPIC", "file-transfer")
	thumbnailTopic := envOrDefault("KAFKA_THUMBNAIL_TOPIC", "file-thumbnail")
	groupID := envOrDefault("KAFKA_GROUP_ID", "file-worker-group")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 2)

	// Transfer worker
	go func() {
		errCh <- runConsumer(ctx, helper, brokers, transferTopic, groupID, handleTransferMessage)
	}()

	// Thumbnail worker
	go func() {
		errCh <- runConsumer(ctx, helper, brokers, thumbnailTopic, groupID, handleThumbnailMessage)
	}()

	helper.Infof("file-worker started, brokers=%v, topics=[%s, %s], group=%s",
		brokers, transferTopic, thumbnailTopic, groupID)

	select {
	case sig := <-sigCh:
		helper.Infof("received signal %v, shutting down", sig)
		cancel()
	case err := <-errCh:
		if err != nil {
			helper.Errorf("consumer exited with error: %v", err)
			cancel()
		}
	}
}

// runConsumer starts a Kafka consumer loop with manual commit.
func runConsumer(ctx context.Context, helper *log.Helper, brokers []string, topic, groupID string, handler func(*log.Helper, []byte) error) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
	})
	defer r.Close()

	helper.Infof("consumer started for topic %s", topic)

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful shutdown
			}
			return fmt.Errorf("fetch from %s failed: %w", topic, err)
		}

		helper.Infof("[%s] received message key=%s offset=%d", topic, string(msg.Key), msg.Offset)

		if err := handler(helper, msg.Value); err != nil {
			helper.Errorf("[%s] handler error for key=%s: %v (message will be retried)", topic, string(msg.Key), err)
			// TODO: after N retries, send to dead-letter topic
			continue
		}

		// Commit offset only after successful processing
		if err := r.CommitMessages(ctx, msg); err != nil {
			helper.Errorf("[%s] commit failed for offset %d: %v", topic, msg.Offset, err)
		}
	}
}

func handleTransferMessage(helper *log.Helper, data []byte) error {
	var msg TransferMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("unmarshal transfer message: %w", err)
	}

	helper.Infof("transfer: md5=%s from=%s to=%s", msg.FileMD5, msg.CurLocation, msg.DestLocation)

	// TODO: implement actual OSS upload
	// 1. Read file from msg.CurLocation
	// 2. Upload to OSS at msg.DestLocation
	// 3. Update file_store table's store_path to OSS URL
	// 4. Optionally delete local file

	helper.Infof("transfer: completed for md5=%s (stub)", msg.FileMD5)
	return nil
}

func handleThumbnailMessage(helper *log.Helper, data []byte) error {
	var msg ThumbnailMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("unmarshal thumbnail message: %w", err)
	}

	helper.Infof("thumbnail: file_id=%d path=%s type=%s", msg.FileID, msg.FilePath, msg.FileType)

	// TODO: implement actual thumbnail generation
	// 1. Read media file from msg.FilePath
	// 2. Generate thumbnail (image resize / video frame extraction)
	// 3. Save thumbnail to store
	// 4. Update database with thumbnail path

	helper.Infof("thumbnail: completed for file_id=%d (stub)", msg.FileID)
	return nil
}

func resolveBrokers() []string {
	env := os.Getenv("KAFKA_BROKERS")
	if env == "" {
		return nil
	}
	return strings.Split(env, ",")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
