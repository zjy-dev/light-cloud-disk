package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	ossSDK "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	ossCredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// CloudMigrateMessage mirrors the biz-layer definition
type CloudMigrateMessage struct {
	FileMD5   string `json:"file_md5"`
	SourceKey string `json:"source_key"`
	FileSize  int64  `json:"file_size"`
}

// ThumbnailMessage mirrors the biz-layer definition
type ThumbnailMessage struct {
	FileID   int64  `json:"file_id"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"`
}

// workerDeps groups infrastructure dependencies used by the worker
type workerDeps struct {
	db        *gorm.DB
	rdb       *redis.Client
	s3Client  *s3.Client
	s3Bucket  string
	ossClient *ossSDK.Client
	ossBucket string
	log       *log.Helper
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
		helper.Fatal("Kafka brokers are missing. Please set KAFKA_BROKERS before starting file-worker.")
	}

	// Initialize dependencies
	deps := initDeps(helper)

	cloudMigrateTopic := envOrDefault("KAFKA_CLOUD_MIGRATE_TOPIC", "cloud-migrate")
	thumbnailTopic := envOrDefault("KAFKA_THUMBNAIL_TOPIC", "file-thumbnail")
	groupID := envOrDefault("KAFKA_GROUP_ID", "file-worker-group")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 2)

	// Cloud migration consumer
	go func() {
		errCh <- runConsumer(ctx, helper, brokers, cloudMigrateTopic, groupID, func(h *log.Helper, data []byte) error {
			return handleCloudMigrateMessage(h, data, deps)
		})
	}()

	// Thumbnail task consumer
	go func() {
		errCh <- runConsumer(ctx, helper, brokers, thumbnailTopic, groupID, handleThumbnailMessage)
	}()

	helper.Infof("File worker is up. brokers=%v, topics=[%s, %s], group=%s",
		brokers, cloudMigrateTopic, thumbnailTopic, groupID)

	select {
	case sig := <-sigCh:
		helper.Infof("Received signal %v. Shutting down file-worker gracefully.", sig)
		cancel()
	case err := <-errCh:
		if err != nil {
			helper.Errorf("A consumer stopped because of an error: %v", err)
			cancel()
		}
	}
}

func initDeps(helper *log.Helper) *workerDeps {
	// Initialize MySQL connection
	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") +
		"@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" +
		os.Getenv("DB_NAME") + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		helper.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// Initialize Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		helper.Fatalf("Failed to connect to Redis: %v", err)
	}

	deps := &workerDeps{db: db, rdb: rdb, log: helper}

	// Initialize SeaweedFS client (S3-compatible)
	if ep := os.Getenv("SEAWEEDFS_ENDPOINT"); ep != "" {
		region := envOrDefault("SEAWEEDFS_REGION", "us-east-1")
		cfg := aws.Config{
			Region: region,
			Credentials: credentials.NewStaticCredentialsProvider(
				os.Getenv("SEAWEEDFS_ACCESS_KEY"),
				os.Getenv("SEAWEEDFS_SECRET_KEY"),
				"",
			),
		}
		deps.s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = true
		})
		deps.s3Bucket = envOrDefault("SEAWEEDFS_BUCKET", "light-cloud-disk")
		helper.Infof("SeaweedFS connection established. endpoint=%s bucket=%s", ep, deps.s3Bucket)
	}

	// Initialize Alibaba Cloud OSS client
	if ep := os.Getenv("OSS_ENDPOINT"); ep != "" {
		ossCfg := ossSDK.LoadDefaultConfig().
			WithCredentialsProvider(ossCredentials.NewStaticCredentialsProvider(
				os.Getenv("OSS_ACCESS_KEY_ID"),
				os.Getenv("OSS_ACCESS_KEY_SECRET"),
			)).
			WithRegion(envOrDefault("OSS_REGION", "cn-hangzhou")).
			WithEndpoint(ep)

		deps.ossClient = ossSDK.NewClient(ossCfg)
		deps.ossBucket = envOrDefault("OSS_BUCKET", "light-cloud-disk")
		helper.Infof("Alibaba Cloud OSS connection established. endpoint=%s bucket=%s", ep, deps.ossBucket)
	}

	return deps
}

// runConsumer starts a Kafka consumer loop with manual offset commit
func runConsumer(ctx context.Context, helper *log.Helper, brokers []string, topic, groupID string, handler func(*log.Helper, []byte) error) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	helper.Infof("Consumer loop started for topic %s", topic)

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("failed to fetch message from topic %s: %w", topic, err)
		}

		helper.Infof("[%s] Message received. key=%s offset=%d", topic, string(msg.Key), msg.Offset)

		if err := handler(helper, msg.Value); err != nil {
			helper.Errorf("[%s] Handler failed for key=%s: %v (message will be retried)", topic, string(msg.Key), err)
			continue
		}

		if err := r.CommitMessages(ctx, msg); err != nil {
			helper.Errorf("[%s] Offset commit failed at %d: %v", topic, msg.Offset, err)
		}
	}
}

// handleCloudMigrateMessage downloads from SeaweedFS, uploads to OSS, and updates DB/Redis state
func handleCloudMigrateMessage(helper *log.Helper, data []byte, deps *workerDeps) error {
	var msg CloudMigrateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to decode cloud-migrate message: %w", err)
	}

	helper.Infof("cloud-migrate job received: md5=%s key=%s size=%d", msg.FileMD5, msg.SourceKey, msg.FileSize)

	if deps.s3Client == nil || deps.ossClient == nil {
		helper.Warn("Skipping cloud-migrate job because S3/OSS clients are not configured.")
		return nil
	}

	ctx := context.Background()

	// 1) Download object from SeaweedFS
	getOut, err := deps.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(deps.s3Bucket),
		Key:    aws.String(msg.SourceKey),
	})
	if err != nil {
		return fmt.Errorf("failed to download object from SeaweedFS: %w", err)
	}
	defer getOut.Body.Close()

	// 2) Upload object to OSS
	_, err = deps.ossClient.PutObject(ctx, &ossSDK.PutObjectRequest{
		Bucket: ossSDK.Ptr(deps.ossBucket),
		Key:    ossSDK.Ptr(msg.SourceKey),
		Body:   getOut.Body,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object to OSS: %w", err)
	}

	// 3) Update MySQL: file_stores.storage_type -> oss
	if err := deps.db.WithContext(ctx).Table("file_stores").
		Where("file_md5 = ?", msg.FileMD5).
		Updates(map[string]interface{}{
			"storage_type":     "oss",
			"last_accessed_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to update storage_type in MySQL: %w", err)
	}

	// 4) Delete old object from SeaweedFS
	_, err = deps.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(deps.s3Bucket),
		Key:    aws.String(msg.SourceKey),
	})
	if err != nil {
		helper.Warnf("Cloud-migrate completed, but cleanup failed for SeaweedFS key %s: %v", msg.SourceKey, err)
	}

	// 5) Update Redis usage counter: decrease seaweedfs by size (OSS is treated as unlimited)
	deps.rdb.IncrBy(ctx, "disk_usage:seaweedfs", -msg.FileSize)

	helper.Infof("Cloud-migrate finished: md5=%s, moved %d bytes to OSS", msg.FileMD5, msg.FileSize)
	return nil
}

func handleThumbnailMessage(helper *log.Helper, data []byte) error {
	var msg ThumbnailMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to decode thumbnail message: %w", err)
	}

	helper.Infof("thumbnail job received: file_id=%d path=%s type=%s", msg.FileID, msg.FilePath, msg.FileType)

	// TODO: implement real thumbnail generation flow
	// 1) Read media file from storage (SeaweedFS/OSS)
	// 2) Generate thumbnail (image resize or video frame extraction)
	// 3) Save thumbnail file
	// 4) Update thumbnail path in database

	helper.Infof("thumbnail job finished for file_id=%d (stub)", msg.FileID)
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
