//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"github.com/J-Y-Zhang/light-cloud-disk/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/data"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/server"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/service"
)

func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		server.ProviderSet,
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		newApp,
	))
}
