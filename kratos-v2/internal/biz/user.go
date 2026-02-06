package biz

import (
	"context"
	"errors"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidUserPassword = errors.New("invalid password")
)

type User struct {
	ID           int64
	Username     string
	Password     string
	Nickname     string
	Email        string
	Avatar       string
	StorageUsed  int64
	StorageLimit int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepo interface {
	Create(ctx context.Context, user *User) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByID(ctx context.Context, id int64) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error
}

type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

func (uc *UserUsecase) Register(ctx context.Context, username, password, nickname, email string) (*User, error) {
	existing, _ := uc.repo.FindByUsername(ctx, username)
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	user := &User{
		Username:     username,
		Password:     password,
		Nickname:     nickname,
		Email:        email,
		StorageLimit: 10 * 1024 * 1024 * 1024, // 默认10GB配额
	}

	return uc.repo.Create(ctx, user)
}

func (uc *UserUsecase) Login(ctx context.Context, username, password string) (*User, error) {
	user, err := uc.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user.Password != password {
		return nil, ErrInvalidUserPassword
	}

	return user, nil
}

func (uc *UserUsecase) GetUserInfo(ctx context.Context, userID int64) (*User, error) {
	return uc.repo.FindByID(ctx, userID)
}

func (uc *UserUsecase) UpdateUserInfo(ctx context.Context, userID int64, nickname, email, avatar string) error {
	user, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if nickname != "" {
		user.Nickname = nickname
	}
	if email != "" {
		user.Email = email
	}
	if avatar != "" {
		user.Avatar = avatar
	}

	return uc.repo.Update(ctx, user)
}
