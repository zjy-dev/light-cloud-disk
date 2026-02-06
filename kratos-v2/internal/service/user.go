package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/internal/biz"
)

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

	// TODO: Generate JWT token
	token := "jwt_token_placeholder"

	return &pb.LoginReply{
		Token:    token,
		ExpireAt: 0,
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
