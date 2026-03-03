package service

import (
	"context"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/go-kratos/kratos/v2/log"

	pb "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/biz"
)

func generateJWT(userID int64) (string, int64, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-jwt-secret"
	}
	expireAt := time.Now().Add(24 * time.Hour).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     expireAt,
	})
	signed, err := token.SignedString([]byte(secret))
	return signed, expireAt, err
}

type UserService struct {
	pb.UnimplementedUserServiceServer

	uc  *biz.UserUsecase
	log *log.Helper
}

func NewUserService(uc *biz.UserUsecase, logger log.Logger) *UserService {
	return &UserService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterReply, error) {
	user, err := s.uc.Register(ctx, req.Username, req.Password, req.Nickname, req.Email)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterReply{
		UserId:   user.ID,
		Username: user.Username,
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	user, err := s.uc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	token, expireAt, err := generateJWT(user.ID)
	if err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		Token:    token,
		ExpireAt: expireAt,
		User: &pb.UserInfo{
			Id:           user.ID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			Email:        user.Email,
			StorageUsed:  user.StorageUsed,
			StorageLimit: user.StorageLimit,
			CreatedAt:    user.CreatedAt.Unix(),
		},
	}, nil
}

func (s *UserService) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoReply, error) {
	user, err := s.uc.GetUserInfo(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.GetUserInfoReply{
		User: &pb.UserInfo{
			Id:           user.ID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			Email:        user.Email,
			Avatar:       user.Avatar,
			StorageUsed:  user.StorageUsed,
			StorageLimit: user.StorageLimit,
			CreatedAt:    user.CreatedAt.Unix(),
		},
	}, nil
}

func (s *UserService) UpdateUserInfo(ctx context.Context, req *pb.UpdateUserInfoRequest) (*pb.UpdateUserInfoReply, error) {
	err := s.uc.UpdateUserInfo(ctx, req.UserId, req.Nickname, req.Email, req.Avatar)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateUserInfoReply{Success: true}, nil
}

func (s *UserService) UpdateStorageUsed(ctx context.Context, req *pb.UpdateStorageUsedRequest) (*pb.UpdateStorageUsedReply, error) {
	err := s.uc.UpdateStorageUsed(ctx, req.UserId, req.Delta)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateStorageUsedReply{Success: true}, nil
}
