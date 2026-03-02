//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/data"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/server"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/service"
)

func wireApp(*conf.Server, *conf.Data, log.Logger, *consul.Registry, userv1.UserServiceClient) (*kratos.App, func(), error) {
	panic(wire.Build(
		server.ProviderSet,
		data.ProviderSet,
		data.NewUserClient,
		biz.ProviderSet,
		service.ProviderSet,
		newApp,
	))
}
