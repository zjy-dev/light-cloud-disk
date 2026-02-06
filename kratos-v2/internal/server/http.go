package server

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/conf"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/service"
)

func NewHTTPServer(c *conf.Server, userSvc *service.UserService, fileSvc *service.FileService, logger log.Logger) *http.Server {
	opts := []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			validate.Validator(),
		),
	}

	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}

	srv := http.NewServer(opts...)

	userv1.RegisterUserServiceHTTPServer(srv, userSvc)
	filev1.RegisterFileServiceHTTPServer(srv, fileSvc)

	return srv
}
