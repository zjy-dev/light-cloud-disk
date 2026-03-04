package data

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

// seaweedFSClient implements biz.ObjectStorage using S3-compatible API
type seaweedFSClient struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	endpoint  string
	log       *log.Helper
}

// NewSeaweedFSClient creates a SeaweedFS S3-compatible client
func NewSeaweedFSClient(c *conf.Storage, logger log.Logger) (biz.ObjectStorage, error) {
	helper := log.NewHelper(logger)

	if c == nil || c.Seaweedfs == nil || c.Seaweedfs.Endpoint == "" {
		helper.Warn("SeaweedFS is not configured. Falling back to no-op object storage.")
		return &noopObjectStorage{}, nil
	}

	endpoint := c.Seaweedfs.Endpoint
	region := c.Seaweedfs.Region
	if region == "" {
		region = "us-east-1"
	}
	bucket := c.Seaweedfs.Bucket
	if bucket == "" {
		bucket = "light-cloud-disk"
	}

	accessKey := os.Getenv("SEAWEEDFS_ACCESS_KEY")
	if accessKey == "" {
		accessKey = c.Seaweedfs.AccessKey
	}
	secretKey := os.Getenv("SEAWEEDFS_SECRET_KEY")
	if secretKey == "" {
		secretKey = c.Seaweedfs.SecretKey
	}

	cfg := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	// Ensure bucket exists
	_, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Ignore errors when bucket already exists
		helper.Infof("CreateBucket returned for %s: %v (it may already exist)", bucket, err)
	}

	helper.Infof("SeaweedFS client is ready. endpoint=%s bucket=%s", endpoint, bucket)

	return &seaweedFSClient{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
		endpoint:  endpoint,
		log:       helper,
	}, nil
}

func (s *seaweedFSClient) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
	})
	return err
}

func (s *seaweedFSClient) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *seaweedFSClient) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *seaweedFSClient) PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// noopObjectStorage is used when SeaweedFS is not configured
type noopObjectStorage struct{}

func (n *noopObjectStorage) Put(_ context.Context, _ string, _ io.Reader, _ int64) error {
	return nil
}
func (n *noopObjectStorage) Delete(_ context.Context, _ string) error { return nil }
func (n *noopObjectStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}
func (n *noopObjectStorage) PresignGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}
