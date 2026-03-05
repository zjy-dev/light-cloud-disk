package client

import (
	"context"
	"os"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"
	googlegrpc "google.golang.org/grpc"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
)

type ServiceClients struct {
	User userv1.UserServiceClient
	File filev1.FileServiceClient
}

func NewServiceClients() *ServiceClients {
	r := newRegistry()

	userConn, err := dialWithRetry(r, "discovery:///user-service")
	if err != nil {
		panic(err)
	}

	fileConn, err := dialWithRetry(r, "discovery:///file-service")
	if err != nil {
		panic(err)
	}

	return &ServiceClients{
		User: userv1.NewUserServiceClient(userConn),
		File: filev1.NewFileServiceClient(fileConn),
	}
}

// dialWithRetry dials a gRPC service via Consul discovery, retrying up to 10 times
// with a 10-second discovery timeout per attempt.
func dialWithRetry(r *consul.Registry, endpoint string) (*googlegrpc.ClientConn, error) {
	var conn *googlegrpc.ClientConn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = grpc.DialInsecure(
			context.Background(),
			grpc.WithEndpoint(endpoint),
			grpc.WithDiscovery(r),
			grpc.WithTimeout(10*time.Second),
		)
		if err == nil {
			return conn, nil
		}
		time.Sleep(3 * time.Second)
	}
	return nil, err
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
