package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

// userClient implements biz.UserClient by calling user-service via gRPC
type userClient struct {
	client userv1.UserServiceClient
	log    *log.Helper
}

func NewUserClient(client userv1.UserServiceClient, logger log.Logger) biz.UserClient {
	return &userClient{
		client: client,
		log:    log.NewHelper(logger),
	}
}

func (c *userClient) UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error {
	_, err := c.client.UpdateStorageUsed(ctx, &userv1.UpdateStorageUsedRequest{
		UserId: userID,
		Delta:  delta,
	})
	return err
}
