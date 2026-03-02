package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/biz"
)

type UserPO struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Password     string `gorm:"size:256;not null"`
	Nickname     string `gorm:"size:64"`
	Email        string `gorm:"size:128"`
	Avatar       string `gorm:"size:256"`
	StorageUsed  int64  `gorm:"default:0"`
	StorageLimit int64  `gorm:"default:10737418240"` // 10GB
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserPO) TableName() string {
	return "users"
}

type userRepo struct {
	data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *userRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	po := &UserPO{
		Username:     user.Username,
		Password:     string(hashedPassword),
		Nickname:     user.Nickname,
		Email:        user.Email,
		StorageLimit: user.StorageLimit,
	}

	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}

	user.ID = po.ID
	user.CreatedAt = po.CreatedAt
	return user, nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	var po UserPO
	if err := r.data.db.WithContext(ctx).Where("username = ?", username).First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, biz.ErrUserNotFound
		}
		return nil, err
	}
	return r.poToDomain(&po), nil
}

func (r *userRepo) FindByID(ctx context.Context, id int64) (*biz.User, error) {
	var po UserPO
	if err := r.data.db.WithContext(ctx).First(&po, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, biz.ErrUserNotFound
		}
		return nil, err
	}
	return r.poToDomain(&po), nil
}

func (r *userRepo) Update(ctx context.Context, user *biz.User) error {
	return r.data.db.WithContext(ctx).Model(&UserPO{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"nickname": user.Nickname,
		"email":    user.Email,
		"avatar":   user.Avatar,
	}).Error
}

func (r *userRepo) UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error {
	return r.data.db.WithContext(ctx).Model(&UserPO{}).Where("id = ?", userID).
		UpdateColumn("storage_used", gorm.Expr("storage_used + ?", delta)).Error
}

func (r *userRepo) poToDomain(po *UserPO) *biz.User {
	return &biz.User{
		ID:           po.ID,
		Username:     po.Username,
		Password:     po.Password,
		Nickname:     po.Nickname,
		Email:        po.Email,
		Avatar:       po.Avatar,
		StorageUsed:  po.StorageUsed,
		StorageLimit: po.StorageLimit,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}
