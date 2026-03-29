package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/client"
)

// --- Mock UserServiceClient ---

type mockUserClient struct {
	registerFn       func(ctx context.Context, in *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterReply, error)
	loginFn          func(ctx context.Context, in *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginReply, error)
	getUserInfoFn    func(ctx context.Context, in *userv1.GetUserInfoRequest, opts ...grpc.CallOption) (*userv1.GetUserInfoReply, error)
	updateUserInfoFn func(ctx context.Context, in *userv1.UpdateUserInfoRequest, opts ...grpc.CallOption) (*userv1.UpdateUserInfoReply, error)
	updateStorageFn  func(ctx context.Context, in *userv1.UpdateStorageUsedRequest, opts ...grpc.CallOption) (*userv1.UpdateStorageUsedReply, error)
}

func (m *mockUserClient) Register(ctx context.Context, in *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterReply, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserClient) Login(ctx context.Context, in *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginReply, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserClient) GetUserInfo(ctx context.Context, in *userv1.GetUserInfoRequest, opts ...grpc.CallOption) (*userv1.GetUserInfoReply, error) {
	if m.getUserInfoFn != nil {
		return m.getUserInfoFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserClient) UpdateUserInfo(ctx context.Context, in *userv1.UpdateUserInfoRequest, opts ...grpc.CallOption) (*userv1.UpdateUserInfoReply, error) {
	if m.updateUserInfoFn != nil {
		return m.updateUserInfoFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserClient) UpdateStorageUsed(ctx context.Context, in *userv1.UpdateStorageUsedRequest, opts ...grpc.CallOption) (*userv1.UpdateStorageUsedReply, error) {
	if m.updateStorageFn != nil {
		return m.updateStorageFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

// --- Mock FileServiceClient ---

type mockFileClient struct {
	checkUploadFn             func(ctx context.Context, in *filev1.CheckUploadRequest, opts ...grpc.CallOption) (*filev1.CheckUploadReply, error)
	uploadChunkFn             func(ctx context.Context, in *filev1.UploadChunkRequest, opts ...grpc.CallOption) (*filev1.UploadChunkReply, error)
	completeUploadFn          func(ctx context.Context, in *filev1.CompleteUploadRequest, opts ...grpc.CallOption) (*filev1.CompleteUploadReply, error)
	getDownloadPlanFn         func(ctx context.Context, in *filev1.GetDownloadPlanRequest, opts ...grpc.CallOption) (*filev1.GetDownloadPlanReply, error)
	listFilesFn               func(ctx context.Context, in *filev1.ListFilesRequest, opts ...grpc.CallOption) (*filev1.ListFilesReply, error)
	getDownloadURLFn          func(ctx context.Context, in *filev1.GetDownloadURLRequest, opts ...grpc.CallOption) (*filev1.GetDownloadURLReply, error)
	deleteFileFn              func(ctx context.Context, in *filev1.DeleteFileRequest, opts ...grpc.CallOption) (*filev1.DeleteFileReply, error)
	renameFileFn              func(ctx context.Context, in *filev1.RenameFileRequest, opts ...grpc.CallOption) (*filev1.RenameFileReply, error)
	createFolderFn            func(ctx context.Context, in *filev1.CreateFolderRequest, opts ...grpc.CallOption) (*filev1.CreateFolderReply, error)
	moveFileFn                func(ctx context.Context, in *filev1.MoveFileRequest, opts ...grpc.CallOption) (*filev1.MoveFileReply, error)
	listTrashFn               func(ctx context.Context, in *filev1.ListTrashRequest, opts ...grpc.CallOption) (*filev1.ListTrashReply, error)
	restoreFileFn             func(ctx context.Context, in *filev1.RestoreFileRequest, opts ...grpc.CallOption) (*filev1.RestoreFileReply, error)
	permanentDeleteFn         func(ctx context.Context, in *filev1.PermanentDeleteRequest, opts ...grpc.CallOption) (*filev1.PermanentDeleteReply, error)
	createShareFn             func(ctx context.Context, in *filev1.CreateShareRequest, opts ...grpc.CallOption) (*filev1.CreateShareReply, error)
	getShareFn                func(ctx context.Context, in *filev1.GetShareRequest, opts ...grpc.CallOption) (*filev1.GetShareReply, error)
	searchFilesFn             func(ctx context.Context, in *filev1.SearchFilesRequest, opts ...grpc.CallOption) (*filev1.SearchFilesReply, error)
	getDiskUsageFn            func(ctx context.Context, in *filev1.GetDiskUsageRequest, opts ...grpc.CallOption) (*filev1.GetDiskUsageReply, error)
	initPresignedUploadFn     func(ctx context.Context, in *filev1.InitPresignedUploadRequest, opts ...grpc.CallOption) (*filev1.InitPresignedUploadReply, error)
	reportUploadedPartFn      func(ctx context.Context, in *filev1.ReportUploadedPartRequest, opts ...grpc.CallOption) (*filev1.ReportUploadedPartReply, error)
	completePresignedUploadFn func(ctx context.Context, in *filev1.CompletePresignedUploadRequest, opts ...grpc.CallOption) (*filev1.CompletePresignedUploadReply, error)
	abortPresignedUploadFn    func(ctx context.Context, in *filev1.AbortPresignedUploadRequest, opts ...grpc.CallOption) (*filev1.AbortPresignedUploadReply, error)
}

func (m *mockFileClient) CheckUpload(ctx context.Context, in *filev1.CheckUploadRequest, opts ...grpc.CallOption) (*filev1.CheckUploadReply, error) {
	if m.checkUploadFn != nil {
		return m.checkUploadFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) UploadChunk(ctx context.Context, in *filev1.UploadChunkRequest, opts ...grpc.CallOption) (*filev1.UploadChunkReply, error) {
	if m.uploadChunkFn != nil {
		return m.uploadChunkFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) CompleteUpload(ctx context.Context, in *filev1.CompleteUploadRequest, opts ...grpc.CallOption) (*filev1.CompleteUploadReply, error) {
	if m.completeUploadFn != nil {
		return m.completeUploadFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) GetDownloadPlan(ctx context.Context, in *filev1.GetDownloadPlanRequest, opts ...grpc.CallOption) (*filev1.GetDownloadPlanReply, error) {
	if m.getDownloadPlanFn != nil {
		return m.getDownloadPlanFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) ListFiles(ctx context.Context, in *filev1.ListFilesRequest, opts ...grpc.CallOption) (*filev1.ListFilesReply, error) {
	if m.listFilesFn != nil {
		return m.listFilesFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) GetDownloadURL(ctx context.Context, in *filev1.GetDownloadURLRequest, opts ...grpc.CallOption) (*filev1.GetDownloadURLReply, error) {
	if m.getDownloadURLFn != nil {
		return m.getDownloadURLFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) DeleteFile(ctx context.Context, in *filev1.DeleteFileRequest, opts ...grpc.CallOption) (*filev1.DeleteFileReply, error) {
	if m.deleteFileFn != nil {
		return m.deleteFileFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) RenameFile(ctx context.Context, in *filev1.RenameFileRequest, opts ...grpc.CallOption) (*filev1.RenameFileReply, error) {
	if m.renameFileFn != nil {
		return m.renameFileFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) CreateFolder(ctx context.Context, in *filev1.CreateFolderRequest, opts ...grpc.CallOption) (*filev1.CreateFolderReply, error) {
	if m.createFolderFn != nil {
		return m.createFolderFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) MoveFile(ctx context.Context, in *filev1.MoveFileRequest, opts ...grpc.CallOption) (*filev1.MoveFileReply, error) {
	if m.moveFileFn != nil {
		return m.moveFileFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) ListTrash(ctx context.Context, in *filev1.ListTrashRequest, opts ...grpc.CallOption) (*filev1.ListTrashReply, error) {
	if m.listTrashFn != nil {
		return m.listTrashFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) RestoreFile(ctx context.Context, in *filev1.RestoreFileRequest, opts ...grpc.CallOption) (*filev1.RestoreFileReply, error) {
	if m.restoreFileFn != nil {
		return m.restoreFileFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) PermanentDelete(ctx context.Context, in *filev1.PermanentDeleteRequest, opts ...grpc.CallOption) (*filev1.PermanentDeleteReply, error) {
	if m.permanentDeleteFn != nil {
		return m.permanentDeleteFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) CreateShare(ctx context.Context, in *filev1.CreateShareRequest, opts ...grpc.CallOption) (*filev1.CreateShareReply, error) {
	if m.createShareFn != nil {
		return m.createShareFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) GetShare(ctx context.Context, in *filev1.GetShareRequest, opts ...grpc.CallOption) (*filev1.GetShareReply, error) {
	if m.getShareFn != nil {
		return m.getShareFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) SearchFiles(ctx context.Context, in *filev1.SearchFilesRequest, opts ...grpc.CallOption) (*filev1.SearchFilesReply, error) {
	if m.searchFilesFn != nil {
		return m.searchFilesFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) GetDiskUsage(ctx context.Context, in *filev1.GetDiskUsageRequest, opts ...grpc.CallOption) (*filev1.GetDiskUsageReply, error) {
	if m.getDiskUsageFn != nil {
		return m.getDiskUsageFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) InitPresignedUpload(ctx context.Context, in *filev1.InitPresignedUploadRequest, opts ...grpc.CallOption) (*filev1.InitPresignedUploadReply, error) {
	if m.initPresignedUploadFn != nil {
		return m.initPresignedUploadFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) ReportUploadedPart(ctx context.Context, in *filev1.ReportUploadedPartRequest, opts ...grpc.CallOption) (*filev1.ReportUploadedPartReply, error) {
	if m.reportUploadedPartFn != nil {
		return m.reportUploadedPartFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) CompletePresignedUpload(ctx context.Context, in *filev1.CompletePresignedUploadRequest, opts ...grpc.CallOption) (*filev1.CompletePresignedUploadReply, error) {
	if m.completePresignedUploadFn != nil {
		return m.completePresignedUploadFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) AbortPresignedUpload(ctx context.Context, in *filev1.AbortPresignedUploadRequest, opts ...grpc.CallOption) (*filev1.AbortPresignedUploadReply, error) {
	if m.abortPresignedUploadFn != nil {
		return m.abortPresignedUploadFn(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFileClient) StreamFileContent(_ context.Context, _ *filev1.StreamFileContentRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[filev1.StreamFileContentReply], error) {
	return nil, errors.New("not implemented")
}

// --- Test helpers ---

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestClients(userClient *mockUserClient, fileClient *mockFileClient) *client.ServiceClients {
	return &client.ServiceClients{
		User: userClient,
		File: fileClient,
	}
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	return result
}

// --- UserHandler tests ---

func TestUserHandler_Register_Success(t *testing.T) {
	userClient := &mockUserClient{
		registerFn: func(_ context.Context, in *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterReply, error) {
			return &userv1.RegisterReply{
				UserId:   1,
				Username: in.Username,
			}, nil
		},
	}

	h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/user/register", jsonBody(map[string]string{
		"username": "alice",
		"password": "pass123",
		"nickname": "Alice",
		"email":    "alice@example.com",
	}))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, float64(1), result["user_id"])
	assert.Equal(t, "alice", result["username"])
}

func TestUserHandler_Register_BadRequest(t *testing.T) {
	h := NewUserHandler(newTestClients(&mockUserClient{}, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/user/register", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_Register_GRPCError(t *testing.T) {
	userClient := &mockUserClient{
		registerFn: func(_ context.Context, _ *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterReply, error) {
			return nil, errors.New("user already exists")
		},
	}

	h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/user/register", jsonBody(map[string]string{
		"username": "alice",
		"password": "pass123",
	}))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	result := parseJSON(t, w)
	assert.Contains(t, result["error"], "already exists")
}

func TestUserHandler_Login_Success(t *testing.T) {
	userClient := &mockUserClient{
		loginFn: func(_ context.Context, in *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginReply, error) {
			return &userv1.LoginReply{
				Token:    "jwt-token-123",
				ExpireAt: 3600,
				User: &userv1.UserInfo{
					Id:       1,
					Username: in.Username,
				},
			}, nil
		},
	}

	h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/user/login", jsonBody(map[string]string{
		"username": "alice",
		"password": "pass123",
	}))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, "jwt-token-123", result["token"])
}

func TestUserHandler_GetUserInfo_Success(t *testing.T) {
	userClient := &mockUserClient{
		getUserInfoFn: func(_ context.Context, in *userv1.GetUserInfoRequest, _ ...grpc.CallOption) (*userv1.GetUserInfoReply, error) {
			return &userv1.GetUserInfoReply{
				User: &userv1.UserInfo{
					Id:       in.UserId,
					Username: "alice",
					Nickname: "Alice",
				},
			}, nil
		},
	}

	h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/user/info", nil)
	c.Set("user_id", int64(42))

	h.GetUserInfo(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_UpdateUserInfo_Success(t *testing.T) {
	userClient := &mockUserClient{
		updateUserInfoFn: func(_ context.Context, in *userv1.UpdateUserInfoRequest, _ ...grpc.CallOption) (*userv1.UpdateUserInfoReply, error) {
			assert.Equal(t, int64(42), in.UserId)
			return &userv1.UpdateUserInfoReply{Success: true}, nil
		},
	}

	h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/user/info", jsonBody(map[string]string{
		"nickname": "NewNick",
	}))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(42))

	h.UpdateUserInfo(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- FileHandler tests ---

func TestFileHandler_ListFiles_Success(t *testing.T) {
	fileClient := &mockFileClient{
		listFilesFn: func(_ context.Context, in *filev1.ListFilesRequest, _ ...grpc.CallOption) (*filev1.ListFilesReply, error) {
			return &filev1.ListFilesReply{
				Files: []*filev1.FileInfo{
					{Id: 1, Name: "doc.pdf", Size: 1024},
					{Id: 2, Name: "photos", IsFolder: true},
				},
				Total: 2,
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/files?parent_id=0&page=1&page_size=20", nil)
	c.Set("user_id", int64(1))

	h.ListFiles(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, float64(2), result["total"])
}

func TestFileHandler_CreateFolder_Success(t *testing.T) {
	fileClient := &mockFileClient{
		createFolderFn: func(_ context.Context, in *filev1.CreateFolderRequest, _ ...grpc.CallOption) (*filev1.CreateFolderReply, error) {
			assert.Equal(t, int64(42), in.UserId)
			return &filev1.CreateFolderReply{
				Success: true,
				Folder:  &filev1.FileInfo{Id: 10, Name: in.Name, IsFolder: true},
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/file/folder", jsonBody(map[string]any{
		"name":      "Documents",
		"parent_id": 0,
	}))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(42))

	h.CreateFolder(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileHandler_DeleteFile_Success(t *testing.T) {
	fileClient := &mockFileClient{
		deleteFileFn: func(_ context.Context, in *filev1.DeleteFileRequest, _ ...grpc.CallOption) (*filev1.DeleteFileReply, error) {
			assert.Equal(t, int64(42), in.UserId)
			return &filev1.DeleteFileReply{Success: true}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/files", jsonBody(map[string]any{
		"file_ids": []int64{1, 2},
	}))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(42))

	h.DeleteFile(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileHandler_SearchFiles_Success(t *testing.T) {
	fileClient := &mockFileClient{
		searchFilesFn: func(_ context.Context, in *filev1.SearchFilesRequest, _ ...grpc.CallOption) (*filev1.SearchFilesReply, error) {
			assert.Equal(t, "report", in.Keyword)
			return &filev1.SearchFilesReply{
				Files: []*filev1.FileInfo{{Id: 1, Name: "report.pdf"}},
				Total: 1,
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/files/search?keyword=report&page=1&page_size=20", nil)
	c.Set("user_id", int64(1))

	h.SearchFiles(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, float64(1), result["total"])
}

func TestFileHandler_GetShare_Success(t *testing.T) {
	fileClient := &mockFileClient{
		getShareFn: func(_ context.Context, in *filev1.GetShareRequest, _ ...grpc.CallOption) (*filev1.GetShareReply, error) {
			assert.Equal(t, "share123", in.ShareId)
			return &filev1.GetShareReply{
				File: &filev1.FileInfo{Id: 1, Name: "shared.pdf"},
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/share/share123?password=secret", nil)
	c.Params = gin.Params{{Key: "share_id", Value: "share123"}}

	h.GetShare(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileHandler_CheckUpload_Success(t *testing.T) {
	fileClient := &mockFileClient{
		checkUploadFn: func(_ context.Context, in *filev1.CheckUploadRequest, _ ...grpc.CallOption) (*filev1.CheckUploadReply, error) {
			return &filev1.CheckUploadReply{
				CanFastUpload:  true,
				UploadedChunks: nil,
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/file/check-upload", jsonBody(map[string]any{
		"file_md5":     "abc123",
		"file_size":    1024,
		"total_chunks": 5,
	}))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CheckUpload(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, true, result["can_fast_upload"])
}

func TestFileHandler_CheckUpload_DiskFull(t *testing.T) {
	fileClient := &mockFileClient{
		checkUploadFn: func(_ context.Context, _ *filev1.CheckUploadRequest, _ ...grpc.CallOption) (*filev1.CheckUploadReply, error) {
			return &filev1.CheckUploadReply{DiskFull: true, UploadMode: "presigned"}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/file/check-upload", jsonBody(map[string]any{
		"file_md5":     "abc123",
		"file_size":    1024,
		"total_chunks": 5,
	}))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CheckUpload(c)

	// Gateway now returns 200 with upload_mode="presigned" instead of 503
	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	assert.Equal(t, true, result["disk_full"])
	assert.Equal(t, "presigned", result["upload_mode"])
}

func TestFileHandler_GetDownloadPlan_AddsRecoveryBackupURLs(t *testing.T) {
	fileClient := &mockFileClient{
		getDownloadPlanFn: func(_ context.Context, _ *filev1.GetDownloadPlanRequest, _ ...grpc.CallOption) (*filev1.GetDownloadPlanReply, error) {
			return &filev1.GetDownloadPlanReply{
				FileName:    "file.zip",
				FileMd5:     "abc123",
				FileSize:    2048,
				TotalChunks: 2,
				Chunks: []*filev1.ChunkLocation{
					{ChunkIndex: 0, ChunkSize: 1024, DownloadUrl: "http://10.0.0.1:9003/api/v1/chunks/abc123/0", Checksum: "sum-0"},
					{ChunkIndex: 1, ChunkSize: 1024, DownloadUrl: "http://oss/chunks/abc123/000001.part", Checksum: "sum-1"},
				},
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/file/download-plan/7", nil)
	c.Params = gin.Params{{Key: "file_id", Value: "7"}}
	c.Set("user_id", int64(42))

	h.GetDownloadPlan(c)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseJSON(t, w)
	chunks := result["chunks"].([]any)
	firstChunk := chunks[0].(map[string]any)
	backupURLs := firstChunk["backupUrls"].([]any)
	assert.Equal(t, "/api/v1/file/chunks/abc123/0/recovery?file_size=2048", backupURLs[0])
	secondChunk := chunks[1].(map[string]any)
	_, hasBackup := secondChunk["backupUrls"]
	assert.False(t, hasBackup)
}

func TestFileHandler_GetDiskUsage_Success(t *testing.T) {
	fileClient := &mockFileClient{
		getDiskUsageFn: func(_ context.Context, _ *filev1.GetDiskUsageRequest, _ ...grpc.CallOption) (*filev1.GetDiskUsageReply, error) {
			return &filev1.GetDiskUsageReply{
				PrimaryUsedBytes: 2048,
				PrimaryMaxBytes:  53687091200,
				PrimaryType:      "local",
			}, nil
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/api/v1/disk-usage", nil)

	h.GetDiskUsage(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileHandler_RenameFile_GRPCError(t *testing.T) {
	fileClient := &mockFileClient{
		renameFileFn: func(_ context.Context, _ *filev1.RenameFileRequest, _ ...grpc.CallOption) (*filev1.RenameFileReply, error) {
			return nil, errors.New("file not found")
		},
	}

	h := NewFileHandler(newTestClients(&mockUserClient{}, fileClient))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/file/rename", jsonBody(map[string]any{
		"file_id":  999,
		"new_name": "renamed.txt",
	}))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(42))

	h.RenameFile(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	result := parseJSON(t, w)
	assert.Contains(t, result["error"], "file not found")
}
