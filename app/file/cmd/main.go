package main

import (
	"context"
	"flag"
	"os"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"

	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

var (
	Name    = "file-service"
	Version = "v1.0.0"

	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../app/file/configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, r *consul.Registry) *kratos.App {
	return kratos.New(
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(gs),
		kratos.Registrar(r),
	)
}

func newRegistry() *consul.Registry {
	addr := os.Getenv("CONSUL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8500"
	}

	consulCli, err := consulapi.NewClient(&consulapi.Config{
		Address: addr,
	})
	if err != nil {
		panic(err)
	}
	return consul.New(consulCli)
}

func newUserServiceClient(r *consul.Registry) userv1.UserServiceClient {
	conn, err := kratosgrpc.DialInsecure(
		context.Background(),
		kratosgrpc.WithEndpoint("discovery:///user-service"),
		kratosgrpc.WithDiscovery(r),
	)
	if err != nil {
		panic(err)
	}
	return userv1.NewUserServiceClient(conn)
}

func provideStorageConfig(c *conf.Storage) *biz.StorageConfig {
	cfg := &biz.StorageConfig{
		LocalMaxBytes:         10 * 1024 * 1024 * 1024, // 10 GB default
		SeaweedFSMaxBytes:     50 * 1024 * 1024 * 1024, // 50 GB default
		SeaweedFSThresholdPct: 80,
	}
	if c != nil {
		if c.LocalMaxBytes > 0 {
			cfg.LocalMaxBytes = c.LocalMaxBytes
		}
		if c.Seaweedfs != nil {
			if c.Seaweedfs.MaxBytes > 0 {
				cfg.SeaweedFSMaxBytes = c.Seaweedfs.MaxBytes
			}
			if c.Seaweedfs.ThresholdPercent > 0 {
				cfg.SeaweedFSThresholdPct = c.Seaweedfs.ThresholdPercent
			}
		}
	}
	return cfg
}

func main() {
	flag.Parse()

	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", Name,
		"service.version", Version,
	)

	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	r := newRegistry()
	userClient := newUserServiceClient(r)

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Storage, logger, r, userClient)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
