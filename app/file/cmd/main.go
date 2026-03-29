package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"
	googlegrpc "google.golang.org/grpc"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/data"
	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/server"
)

var (
	Name    = "file-service"
	Version = "v1.0.0"

	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../app/file/configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *kratosgrpc.Server, r *consul.Registry, httpPort string, extras ...transport.Server) *kratos.App {
	servers := []transport.Server{gs}
	servers = append(servers, extras...)
	meta := map[string]string{}
	if httpPort != "" {
		meta["http_port"] = httpPort
	}
	return kratos.New(
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(meta),
		kratos.Logger(logger),
		kratos.Server(servers...),
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
		PrimaryMaxBytes: 10 * 1024 * 1024 * 1024, // 10 GB default
		ThresholdPct:    80,
		EvictTargetPct:  90,
	}
	if c != nil {
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

func provideTempDir(u *conf.Upload) string {
	if dir := os.Getenv("FILE_TMP_DIR"); dir != "" {
		return dir
	}
	if u != nil && u.TempDir != "" {
		return u.TempDir
	}
	return "./tmp"
}

// provideErasureConfig reads erasure coding config from env vars.
func provideErasureConfig() *biz.ErasureConfig {
	cfg := &biz.ErasureConfig{DataShards: 4, ParityShards: 2, MinFileSize: 1 << 20}
	if v := os.Getenv("ERASURE_DATA_SHARDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DataShards = n
		}
	}
	if v := os.Getenv("ERASURE_PARITY_SHARDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ParityShards = n
		}
	}
	if v := os.Getenv("ERASURE_MIN_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MinFileSize = n
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

	grpcSrv, fileUsecase, cleanup, err := wireApp(bc.Server, bc.Data, bc.Upload, bc.Storage, logger, userClient)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// Build chunk HTTP server outside Wire (needs env-based config)
	httpAddr := os.Getenv("FILE_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":9003"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	tmpDir := provideTempDir(bc.Upload)
	if os.Getenv("FILE_TMP_DIR") == "" {
		_ = os.Setenv("FILE_TMP_DIR", tmpDir)
	}
	chunkHTTP := server.NewChunkHTTPServer(fileUsecase, httpAddr, jwtSecret, tmpDir, logger)

	// Extract HTTP port for Consul metadata so the gateway can build upload plans.
	httpPort := ""
	if httpAddr != "" {
		if _, p, ok := strings.Cut(httpAddr, ":"); ok && p != "" {
			httpPort = p
		}
	}

	// Optionally start Kafka consumer alongside the producer.
	extras := []transport.Server{chunkHTTP}
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		cloudMigrateTopic := "cloud-migrate"
		thumbnailTopic := "file-thumbnail"
		if bc.Kafka != nil {
			if bc.Kafka.CloudMigrateTopic != "" {
				cloudMigrateTopic = bc.Kafka.CloudMigrateTopic
			}
			if bc.Kafka.ThumbnailTopic != "" {
				thumbnailTopic = bc.Kafka.ThumbnailTopic
			}
		}
		consumer := data.NewKafkaConsumer(
			strings.Split(brokers, ","),
			cloudMigrateTopic, thumbnailTopic,
			fileUsecase.Repo(), fileUsecase.CloudStore(), tmpDir,
			logger,
		)
		extras = append(extras, consumer)
	}

	app := newApp(logger, grpcSrv, r, httpPort, extras...)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
