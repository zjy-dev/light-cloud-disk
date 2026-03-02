package client

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
)

type ServiceClients struct {
	User userv1.UserServiceClient
	File filev1.FileServiceClient
}

func NewServiceClients() *ServiceClients {
	r := newRegistry()

	userConn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///user-service"),
		grpc.WithDiscovery(r),
	)
	if err != nil {
		panic(err)
	}

	fileConn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///file-service"),
		grpc.WithDiscovery(r),
	)
	if err != nil {
		panic(err)
	}

	return &ServiceClients{
		User: userv1.NewUserServiceClient(userConn),
		File: filev1.NewFileServiceClient(fileConn),
	}
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
