package data

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	ossCredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

// ossClient implements biz.CloudStorage using Alibaba Cloud OSS v2 SDK.
type ossClient struct {
	client *oss.Client
	bucket string
	log    *log.Helper
}

// NewOSSClient creates an Alibaba Cloud OSS client.
func NewOSSClient(c *conf.Storage, logger log.Logger) (biz.CloudStorage, error) {
	helper := log.NewHelper(logger)

	if c == nil || c.Oss == nil || c.Oss.Endpoint == "" {
		helper.Warn("alicloud OSS not configured, using noop cloud storage")
		return &noopCloudStorage{}, nil
	}

	accessKeyID := os.Getenv("OSS_ACCESS_KEY_ID")
	if accessKeyID == "" {
		accessKeyID = c.Oss.AccessKeyId
	}
	accessKeySecret := os.Getenv("OSS_ACCESS_KEY_SECRET")
	if accessKeySecret == "" {
		accessKeySecret = c.Oss.AccessKeySecret
	}

	region := c.Oss.Region
	if region == "" {
		region = "cn-hangzhou"
	}

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(ossCredentials.NewStaticCredentialsProvider(
			accessKeyID, accessKeySecret,
		)).
		WithRegion(region).
		WithEndpoint(c.Oss.Endpoint)

	client := oss.NewClient(cfg)

	bucket := c.Oss.Bucket
	if bucket == "" {
		bucket = "light-cloud-disk"
	}

	helper.Infof("alicloud OSS client connected to %s, bucket=%s", c.Oss.Endpoint, bucket)

	return &ossClient{
		client: client,
		bucket: bucket,
		log:    helper,
	}, nil
}

func (o *ossClient) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	_, err := o.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
		Body:   reader,
	})
	return err
}

func (o *ossClient) Delete(ctx context.Context, key string) error {
	_, err := o.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	})
	return err
}

func (o *ossClient) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := o.client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (o *ossClient) PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	result, err := o.client.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	}, oss.PresignExpires(expires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// noopCloudStorage is used when cloud storage is not configured.
type noopCloudStorage struct{}

func (n *noopCloudStorage) Put(_ context.Context, _ string, _ io.Reader, _ int64) error {
	return nil
}
func (n *noopCloudStorage) Delete(_ context.Context, _ string) error { return nil }
func (n *noopCloudStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}
func (n *noopCloudStorage) PresignGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}
