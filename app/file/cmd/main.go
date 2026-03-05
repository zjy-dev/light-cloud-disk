package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"
	googlegrpc "google.golang.org/grpc"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

var (
	Name    = "file-service"
	Version = "v1.0.0"

	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../app/file/configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *kratosgrpc.Server, r *consul.Registry) *kratos.App {
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
	// Retry loop: wait for user-service to register with Consul.
	var conn *googlegrpc.ClientConn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = kratosgrpc.DialInsecure(
			context.Background(),
			kratosgrpc.WithEndpoint("discovery:///user-service"),
			kratosgrpc.WithDiscovery(r),
			kratosgrpc.WithTimeout(10*time.Second),
		)
		if err == nil {
			return userv1.NewUserServiceClient(conn)
		}
		time.Sleep(3 * time.Second)
	}
	panic(err)
}

func provideStorageConfig(c *conf.Storage) *biz.StorageConfig {
	cfg := &biz.StorageConfig{
		Mode:            biz.ModeLocal,
		PrimaryMaxBytes: 10 * 1024 * 1024 * 1024, // 10 GB default
		ThresholdPct:    80,
		EvictTargetPct:  90,
	}
	if c != nil {
		if c.Mode != "" {
			cfg.Mode = c.Mode
		}
		if c.PrimaryMaxBytes > 0 {
			cfg.PrimaryMaxBytes = c.PrimaryMaxBytes
		}
		if c.ThresholdPercent > 0 {
			cfg.ThresholdPct = c.ThresholdPercent
		}
		if c.EvictTargetPercent > 0 {
			cfg.EvictTargetPct = c.EvictTargetPercent
		}
	}
	// Env overrides (useful for container deployments)
	if m := os.Getenv("STORAGE_MODE"); m != "" {
		cfg.Mode = m
	}
	if v := os.Getenv("PRIMARY_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.PrimaryMaxBytes = n
		}
	}
	return cfg
}

// provideStoreDir resolves the local file store directory from env or config.
func provideStoreDir(u *conf.Upload) string {
	if dir := os.Getenv("FILE_STORE_DIR"); dir != "" {
		return dir
	}
	if u != nil && u.StoreDir != "" {
		return u.StoreDir
	}
	return "./store"
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

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Upload, bc.Storage, logger, r, userClient)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
