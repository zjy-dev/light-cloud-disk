package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserRepo is a testify mock for UserRepo.
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

func (m *MockUserRepo) CheckPassword(ctx context.Context, user *User, rawPassword string) error {
	args := m.Called(ctx, user, rawPassword)
	return args.Error(0)
}

func newTestUserUsecase(repo *MockUserRepo) *UserUsecase {
	return NewUserUsecase(repo, log.DefaultLogger)
}

func TestRegister_Success(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByUsername", ctx, "alice").Return(nil, errors.New("not found"))
	repo.On("Create", ctx, mock.AnythingOfType("*biz.User")).Return(&User{
		ID:           1,
		Username:     "alice",
		Nickname:     "Alice",
		Email:        "alice@example.com",
		StorageLimit: 10 * 1024 * 1024 * 1024,
	}, nil)

	user, err := uc.Register(ctx, "alice", "password123", "Alice", "alice@example.com")

	assert.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "alice", user.Username)
	repo.AssertExpectations(t)
}

func TestRegister_AlreadyExists(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByUsername", ctx, "alice").Return(&User{ID: 1, Username: "alice"}, nil)

	user, err := uc.Register(ctx, "alice", "password123", "Alice", "alice@example.com")

	assert.Nil(t, user)
	assert.ErrorIs(t, err, ErrUserAlreadyExists)
	repo.AssertExpectations(t)
}

func TestLogin_Success(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	foundUser := &User{
		ID:       1,
		Username: "alice",
		Password: "$2a$10$hashedpassword",
	}
	repo.On("FindByUsername", ctx, "alice").Return(foundUser, nil)
	repo.On("CheckPassword", ctx, foundUser, "password123").Return(nil)

	user, err := uc.Login(ctx, "alice", "password123")

	assert.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	repo.AssertExpectations(t)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByUsername", ctx, "unknown").Return(nil, errors.New("not found"))

	user, err := uc.Login(ctx, "unknown", "password123")

	assert.Nil(t, user)
	assert.ErrorIs(t, err, ErrUserNotFound)
	repo.AssertExpectations(t)
}

func TestLogin_InvalidPassword(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	foundUser := &User{
		ID:       1,
		Username: "alice",
		Password: "$2a$10$hashedpassword",
	}
	repo.On("FindByUsername", ctx, "alice").Return(foundUser, nil)
	repo.On("CheckPassword", ctx, foundUser, "wrongpass").Return(errors.New("crypto/bcrypt: hashedPassword is not the hash of the given password"))

	user, err := uc.Login(ctx, "alice", "wrongpass")

	assert.Nil(t, user)
	assert.ErrorIs(t, err, ErrInvalidPassword)
	repo.AssertExpectations(t)
}

func TestGetUserInfo_Success(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	expected := &User{ID: 1, Username: "alice", Nickname: "Alice"}
	repo.On("FindByID", ctx, int64(1)).Return(expected, nil)

	user, err := uc.GetUserInfo(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expected, user)
	repo.AssertExpectations(t)
}

func TestGetUserInfo_NotFound(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByID", ctx, int64(999)).Return(nil, errors.New("not found"))

	user, err := uc.GetUserInfo(ctx, 999)

	assert.Nil(t, user)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestUpdateUserInfo_Success(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	existing := &User{
		ID:       1,
		Username: "alice",
		Nickname: "OldNick",
		Email:    "old@example.com",
		Avatar:   "old.png",
	}
	repo.On("FindByID", ctx, int64(1)).Return(existing, nil)
	repo.On("Update", ctx, mock.AnythingOfType("*biz.User")).Return(nil)

	err := uc.UpdateUserInfo(ctx, 1, "NewNick", "new@example.com", "new.png")

	assert.NoError(t, err)
	assert.Equal(t, "NewNick", existing.Nickname)
	assert.Equal(t, "new@example.com", existing.Email)
	assert.Equal(t, "new.png", existing.Avatar)
	repo.AssertExpectations(t)
}

func TestUpdateUserInfo_PartialUpdate(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	existing := &User{
		ID:       1,
		Username: "alice",
		Nickname: "OldNick",
		Email:    "old@example.com",
		Avatar:   "old.png",
	}
	repo.On("FindByID", ctx, int64(1)).Return(existing, nil)
	repo.On("Update", ctx, mock.AnythingOfType("*biz.User")).Return(nil)

	// Only update nickname, leave email and avatar empty
	err := uc.UpdateUserInfo(ctx, 1, "NewNick", "", "")

	assert.NoError(t, err)
	assert.Equal(t, "NewNick", existing.Nickname)
	assert.Equal(t, "old@example.com", existing.Email) // unchanged
	assert.Equal(t, "old.png", existing.Avatar)        // unchanged
	repo.AssertExpectations(t)
}

func TestUpdateUserInfo_NotFound(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByID", ctx, int64(999)).Return(nil, errors.New("not found"))

	err := uc.UpdateUserInfo(ctx, 999, "Nick", "", "")

	assert.ErrorIs(t, err, ErrUserNotFound)
	repo.AssertExpectations(t)
}

func TestUpdateStorageUsed_Success(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("UpdateStorageUsed", ctx, int64(1), int64(1024)).Return(nil)

	err := uc.UpdateStorageUsed(ctx, 1, 1024)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUpdateStorageUsed_Error(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("UpdateStorageUsed", ctx, int64(1), int64(-100)).Return(errors.New("db error"))

	err := uc.UpdateStorageUsed(ctx, 1, -100)

	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestRegister_DefaultStorageLimit(t *testing.T) {
	repo := new(MockUserRepo)
	uc := newTestUserUsecase(repo)
	ctx := context.Background()

	repo.On("FindByUsername", ctx, "bob").Return(nil, errors.New("not found"))
	repo.On("Create", ctx, mock.MatchedBy(func(u *User) bool {
		return u.StorageLimit == 10*1024*1024*1024
	})).Return(&User{
		ID:           2,
		Username:     "bob",
		StorageLimit: 10 * 1024 * 1024 * 1024,
	}, nil)

	user, err := uc.Register(ctx, "bob", "pass", "Bob", "bob@example.com")

	assert.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024*1024), user.StorageLimit)
	repo.AssertExpectations(t)
}

// Ensure User struct has expected zero values
func TestUserStruct_ZeroValue(t *testing.T) {
	u := &User{}
	assert.Zero(t, u.ID)
	assert.Empty(t, u.Username)
	assert.Zero(t, u.StorageUsed)
	assert.Zero(t, u.StorageLimit)
	assert.True(t, u.CreatedAt.IsZero())
	_ = time.Time{} // satisfy import
}
