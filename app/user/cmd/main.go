package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"

	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/conf"
)

var (
	Name    = "user-service"
	Version = "v1.0.0"

	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../app/user/configs", "config path, eg: -conf config.yaml")
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

	app, cleanup, err := wireApp(bc.Server, bc.Data, logger, r)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
