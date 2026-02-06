package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) Create(ctx context.Context, user *User) (*User, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepo) FindByUsername(ctx context.Context, username string) (*User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepo) FindByID(ctx context.Context, id int64) (*User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepo) Update(ctx context.Context, user *User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepo) UpdateStorageUsed(ctx context.Context, userID int64, delta int64) error {
	args := m.Called(ctx, userID, delta)
	return args.Error(0)
}

func TestUserUsecase_Register(t *testing.T) {
	mockRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewUserUsecase(mockRepo, logger)

	ctx := context.Background()

	t.Run("成功注册新用户", func(t *testing.T) {
		mockRepo.On("FindByUsername", ctx, "newuser").Return(nil, ErrUserNotFound).Once()
		mockRepo.On("Create", ctx, mock.AnythingOfType("*biz.User")).Return(&User{
			ID:       1,
			Username: "newuser",
			Nickname: "New User",
		}, nil).Once()

		user, err := uc.Register(ctx, "newuser", "password123", "New User", "new@example.com")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "newuser", user.Username)
	})

	t.Run("用户名已存在", func(t *testing.T) {
		existingUser := &User{ID: 1, Username: "existinguser"}
		mockRepo.On("FindByUsername", ctx, "existinguser").Return(existingUser, nil).Once()

		user, err := uc.Register(ctx, "existinguser", "password123", "Existing", "exist@example.com")

		assert.Error(t, err)
		assert.Equal(t, ErrUserAlreadyExists, err)
		assert.Nil(t, user)
	})
}

func TestUserUsecase_Login(t *testing.T) {
	mockRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewUserUsecase(mockRepo, logger)

	ctx := context.Background()

	t.Run("登录成功", func(t *testing.T) {
		existingUser := &User{
			ID:       1,
			Username: "testuser",
			Password: "correctpassword",
		}
		mockRepo.On("FindByUsername", ctx, "testuser").Return(existingUser, nil).Once()

		user, err := uc.Login(ctx, "testuser", "correctpassword")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "testuser", user.Username)
	})

	t.Run("用户不存在", func(t *testing.T) {
		mockRepo.On("FindByUsername", ctx, "nonexistent").Return(nil, ErrUserNotFound).Once()

		user, err := uc.Login(ctx, "nonexistent", "anypassword")

		assert.Error(t, err)
		assert.Equal(t, ErrUserNotFound, err)
		assert.Nil(t, user)
	})

	t.Run("密码错误", func(t *testing.T) {
		existingUser := &User{
			ID:       1,
			Username: "testuser",
			Password: "correctpassword",
		}
		mockRepo.On("FindByUsername", ctx, "testuser").Return(existingUser, nil).Once()

		user, err := uc.Login(ctx, "testuser", "wrongpassword")

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidUserPassword, err)
		assert.Nil(t, user)
	})
}

func TestUserUsecase_GetUserInfo(t *testing.T) {
	mockRepo := new(MockUserRepo)
	logger := log.DefaultLogger
	uc := NewUserUsecase(mockRepo, logger)

	ctx := context.Background()

	t.Run("获取用户信息成功", func(t *testing.T) {
		expectedUser := &User{
			ID:           1,
			Username:     "testuser",
			Nickname:     "Test User",
			Email:        "test@example.com",
			StorageUsed:  1024,
			StorageLimit: 10737418240,
		}
		mockRepo.On("FindByID", ctx, int64(1)).Return(expectedUser, nil).Once()

		user, err := uc.GetUserInfo(ctx, 1)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, int64(10737418240), user.StorageLimit)
	})

	t.Run("用户不存在", func(t *testing.T) {
		mockRepo.On("FindByID", ctx, int64(999)).Return(nil, ErrUserNotFound).Once()

		user, err := uc.GetUserInfo(ctx, 999)

		assert.Error(t, err)
		assert.Nil(t, user)
	})
}
