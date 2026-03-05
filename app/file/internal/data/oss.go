package data

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	ossCredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/go-kratos/kratos/v2/log"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

// ossClient implements biz.CloudStorage using Alibaba Cloud OSS v2 SDK
type ossClient struct {
	client *oss.Client
	bucket string
	log    *log.Helper
}

// NewOSSClient creates an Alibaba Cloud OSS client
func NewOSSClient(c *conf.Storage, logger log.Logger) (biz.CloudStorage, error) {
	helper := log.NewHelper(logger)

	endpoint := os.Getenv("OSS_ENDPOINT")
	if endpoint == "" && c != nil && c.Oss != nil {
		endpoint = sanitizeOSSConfValue(c.Oss.Endpoint)
	}

	if endpoint == "" {
		helper.Warn("Alibaba Cloud OSS is not configured. Falling back to no-op cloud storage.")
		return &noopCloudStorage{}, nil
	}

	accessKeyID := os.Getenv("OSS_ACCESS_KEY_ID")
	if accessKeyID == "" {
		accessKeyID = sanitizeOSSConfValue(c.Oss.AccessKeyId)
	}
	accessKeySecret := os.Getenv("OSS_ACCESS_KEY_SECRET")
	if accessKeySecret == "" {
		accessKeySecret = sanitizeOSSConfValue(c.Oss.AccessKeySecret)
	}

	region := os.Getenv("OSS_REGION")
	if region == "" && c != nil && c.Oss != nil {
		region = sanitizeOSSConfValue(c.Oss.Region)
	}
	if region == "" {
		region = "cn-hangzhou"
	}

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(ossCredentials.NewStaticCredentialsProvider(
			accessKeyID, accessKeySecret,
		)).
		WithRegion(region).
		WithEndpoint(endpoint)

	client := oss.NewClient(cfg)

	bucket := os.Getenv("OSS_BUCKET")
	if bucket == "" && c != nil && c.Oss != nil {
		bucket = sanitizeOSSConfValue(c.Oss.Bucket)
	}
	if bucket == "" {
		bucket = "light-cloud-disk"
	}

	helper.Infof("Alibaba Cloud OSS client is ready. endpoint=%s bucket=%s", endpoint, bucket)

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

func (o *ossClient) InitMultipartUpload(ctx context.Context, key string) (string, error) {
	out, err := o.client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		return "", err
	}
	if out.UploadId == nil {
		return "", nil
	}
	return *out.UploadId, nil
}

func (o *ossClient) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (string, error) {
	result, err := o.client.Presign(ctx, &oss.UploadPartRequest{
		Bucket:     oss.Ptr(o.bucket),
		Key:        oss.Ptr(key),
		UploadId:   oss.Ptr(uploadID),
		PartNumber: int32(partNumber),
	}, oss.PresignExpires(expires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (o *ossClient) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []biz.CompletedPart) error {
	ossParts := make([]oss.UploadPart, len(parts))
	for i, p := range parts {
		ossParts[i] = oss.UploadPart{
			PartNumber: int32(p.PartNumber),
			ETag:       oss.Ptr(p.ETag),
		}
	}
	_, err := o.client.CompleteMultipartUpload(ctx, &oss.CompleteMultipartUploadRequest{
		Bucket:   oss.Ptr(o.bucket),
		Key:      oss.Ptr(key),
		UploadId: oss.Ptr(uploadID),
		CompleteMultipartUpload: &oss.CompleteMultipartUpload{
			Parts: ossParts,
		},
	})
	return err
}

func (o *ossClient) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := o.client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{
		Bucket:   oss.Ptr(o.bucket),
		Key:      oss.Ptr(key),
		UploadId: oss.Ptr(uploadID),
	})
	return err
}

// noopCloudStorage is used when cloud storage is not configured
type noopCloudStorage struct{}

func (n *noopCloudStorage) Put(_ context.Context, _ string, _ io.Reader, _ int64) error {
	return fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) Delete(_ context.Context, _ string) error {
	return fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) PresignGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	_ = key
	return "", fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) InitMultipartUpload(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) PresignUploadPart(_ context.Context, _, _ string, _ int32, _ time.Duration) (string, error) {
	return "", fmt.Errorf("noop cloud storage: not configured")
}
func (n *noopCloudStorage) CompleteMultipartUpload(_ context.Context, _, _ string, _ []biz.CompletedPart) error {
	return fmt.Errorf("noop cloud storage: not configured")
}

func sanitizeOSSConfValue(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return ""
	}
	return v
}
func (n *noopCloudStorage) AbortMultipartUpload(_ context.Context, _, _ string) error {
	return fmt.Errorf("noop cloud storage: not configured")
}
