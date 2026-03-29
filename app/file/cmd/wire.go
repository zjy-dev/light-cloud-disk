//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/data"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/server"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/service"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

func wireApp(*conf.Server, *conf.Data, *conf.Upload, *conf.Storage, log.Logger, userv1.UserServiceClient) (*kratosgrpc.Server, *biz.FileUsecase, func(), error) {
	panic(wire.Build(
		server.ProviderSet,
		data.ProviderSet,
		data.NewUserClient,
		biz.ProviderSet,
		service.ProviderSet,
		provideStorageConfig,
		provideStoreDir,
	))
}
