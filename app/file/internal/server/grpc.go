package server

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/service"
)

func NewGRPCServer(c *conf.Server, fileSvc *service.FileService, logger log.Logger) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	}

	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}

	srv := grpc.NewServer(opts...)
	filev1.RegisterFileServiceServer(srv, fileSvc)

	return srv
}
