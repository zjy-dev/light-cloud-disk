package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/client"
)

type FileHandler struct {
	clients *client.ServiceClients
}

type downloadPlanChunkResponse struct {
	ChunkIndex  int32    `json:"chunkIndex"`
	ChunkSize   int64    `json:"chunkSize"`
	DownloadURL string   `json:"downloadUrl"`
	Checksum    string   `json:"checksum"`
	BackupURLs  []string `json:"backupUrls,omitempty"`
}

type downloadPlanResponse struct {
	FileName          string                      `json:"fileName"`
	FileMD5           string                      `json:"fileMd5"`
	FileSize          int64                       `json:"fileSize"`
	TotalChunks       int32                       `json:"totalChunks"`
	Chunks            []downloadPlanChunkResponse `json:"chunks"`
	DirectDownloadURL string                      `json:"directDownloadUrl,omitempty"`
}

var recoveryProxyHTTPClient = &http.Client{Timeout: 30 * time.Second}

func NewFileHandler(clients *client.ServiceClients) *FileHandler {
	return &FileHandler{clients: clients}
}

func (h *FileHandler) CheckUpload(c *gin.Context) {
	var req filev1.CheckUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Route by file MD5 so all operations for the same file hit the same instance.
	reply, err := h.clients.FileClientByKey(req.FileMd5).CheckUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// For direct upload mode, build an upload plan with per-chunk instance assignments.
	if reply.UploadMode == "direct" && !reply.CanFastUpload {
		addrs := h.clients.PickNFileHTTPAddrs(req.FileMd5, int(req.TotalChunks))
		if len(addrs) > 0 {
			assignments := make([]*filev1.ChunkAssignment, req.TotalChunks)
			for i := int32(0); i < req.TotalChunks; i++ {
				addr := addrs[int(i)%len(addrs)]
				assignments[i] = &filev1.ChunkAssignment{
					ChunkIndex: i,
					TargetAddr: addr,
					UploadUrl:  fmt.Sprintf("http://%s/api/v1/chunks/%s/%d", addr, req.FileMd5, i),
				}
			}
			chunkSize := req.FileSize / int64(req.TotalChunks)
			reply.UploadPlan = &filev1.UploadPlan{
				TotalChunks: req.TotalChunks,
				ChunkSize:   chunkSize,
				Assignments: assignments,
			}
		}
	}

	c.JSON(http.StatusOK, reply)
}

// ---- Presigned multipart upload ----

func (h *FileHandler) InitPresignedUpload(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.InitPresignedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.FileClientByKey(req.FileMd5).InitPresignedUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) ReportUploadedPart(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req filev1.ReportUploadedPartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.ReportUploadedPart(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) CompletePresignedUpload(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req filev1.CompletePresignedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.CompletePresignedUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) AbortPresignedUpload(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req filev1.AbortPresignedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.AbortPresignedUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) CompleteUpload(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	// Any file-service instance can complete — they share DB/Redis state.
	reply, err := h.clients.File.CompleteUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) GetDownloadPlan(c *gin.Context) {
	userID := c.GetInt64("user_id")
	fileID, err := strconv.ParseInt(c.Param("file_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
		return
	}

	reply, err := h.clients.File.GetDownloadPlan(c.Request.Context(), &filev1.GetDownloadPlanRequest{
		UserId: userID,
		FileId: fileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	chunks := make([]downloadPlanChunkResponse, len(reply.Chunks))
	for i, chunk := range reply.Chunks {
		backupURLs := []string(nil)
		if strings.Contains(chunk.DownloadUrl, "/api/v1/chunks/") {
			backupURLs = []string{fmt.Sprintf("/api/v1/file/chunks/%s/%d/recovery?file_size=%d", reply.FileMd5, chunk.ChunkIndex, reply.FileSize)}
		}
		chunks[i] = downloadPlanChunkResponse{
			ChunkIndex:  chunk.ChunkIndex,
			ChunkSize:   chunk.ChunkSize,
			DownloadURL: chunk.DownloadUrl,
			Checksum:    chunk.Checksum,
			BackupURLs:  backupURLs,
		}
	}

	c.JSON(http.StatusOK, downloadPlanResponse{
		FileName:          reply.FileName,
		FileMD5:           reply.FileMd5,
		FileSize:          reply.FileSize,
		TotalChunks:       reply.TotalChunks,
		Chunks:            chunks,
		DirectDownloadURL: reply.DirectDownloadUrl,
	})
}

func (h *FileHandler) RecoverChunk(c *gin.Context) {
	fileMD5 := c.Param("md5")
	chunkIndex, err := strconv.ParseInt(c.Param("index"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
		return
	}

	fileSize, err := strconv.ParseInt(c.Query("file_size"), 10, 64)
	if err != nil || fileSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_size"})
		return
	}

	addrs := h.clients.PickNFileHTTPAddrs(fmt.Sprintf("%s:%d:recovery", fileMD5, chunkIndex), 3)
	if len(addrs) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no healthy file-service instances available for recovery"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	lastErr := "chunk recovery failed"
	for _, addr := range addrs {
		target := fmt.Sprintf("http://%s/api/v1/chunks/recover/%s/%d?file_size=%d", addr, fileMD5, chunkIndex, fileSize)
		req, reqErr := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
		if reqErr != nil {
			lastErr = reqErr.Error()
			continue
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, doErr := recoveryProxyHTTPClient.Do(req)
		if doErr != nil {
			h.clients.MarkFileInstanceUnhealthy(addr)
			lastErr = doErr.Error()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode >= http.StatusInternalServerError {
				h.clients.MarkFileInstanceUnhealthy(addr)
			}
			if msg := strings.TrimSpace(string(body)); msg != "" {
				lastErr = msg
			}
			continue
		}

		defer resp.Body.Close()
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, resp.Body, nil)
		return
	}

	c.JSON(http.StatusBadGateway, gin.H{"error": lastErr})
}

func (h *FileHandler) ListFiles(c *gin.Context) {
	userID := c.GetInt64("user_id")
	parentID, _ := strconv.ParseInt(c.DefaultQuery("parent_id", "0"), 10, 64)
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)

	reply, err := h.clients.File.ListFiles(c.Request.Context(), &filev1.ListFilesRequest{
		UserId:   userID,
		ParentId: parentID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) CreateFolder(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.CreateFolder(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) RenameFile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.RenameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.RenameFile(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.DeleteFile(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) MoveFile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.MoveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.MoveFile(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) ListTrash(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)

	reply, err := h.clients.File.ListTrash(c.Request.Context(), &filev1.ListTrashRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) RestoreFile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.RestoreFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.RestoreFile(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) PermanentDelete(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.PermanentDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.PermanentDelete(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) CreateShare(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.CreateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.CreateShare(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) GetShare(c *gin.Context) {
	shareID := c.Param("share_id")

	var req filev1.GetShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow GET without body and read password from query params
		req.Password = c.Query("password")
	}
	req.ShareId = shareID

	reply, err := h.clients.File.GetShare(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) SearchFiles(c *gin.Context) {
	userID := c.GetInt64("user_id")
	keyword := c.Query("keyword")
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)

	reply, err := h.clients.File.SearchFiles(c.Request.Context(), &filev1.SearchFilesRequest{
		UserId:   userID,
		Keyword:  keyword,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) GetDownloadURL(c *gin.Context) {
	userID := c.GetInt64("user_id")
	fileID, _ := strconv.ParseInt(c.Param("file_id"), 10, 64)

	reply, err := h.clients.File.GetDownloadURL(c.Request.Context(), &filev1.GetDownloadURLRequest{
		UserId: userID,
		FileId: fileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	reply.DownloadUrl = fmt.Sprintf("/api/v1/file/stream/%d", fileID)

	c.JSON(http.StatusOK, reply)
}

// StreamFile streams a locally-stored file directly to the HTTP client.
// Registered as GET /api/v1/file/stream/:file_id (behind JWT).
func (h *FileHandler) StreamFile(c *gin.Context) {
	userID := c.GetInt64("user_id")
	fileID, _ := strconv.ParseInt(c.Param("file_id"), 10, 64)
	downloadReply, err := h.clients.File.GetDownloadURL(c.Request.Context(), &filev1.GetDownloadURLRequest{
		UserId: userID,
		FileId: fileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if strings.HasPrefix(downloadReply.DownloadUrl, "local://") {
		stream, err := h.clients.File.StreamFileContent(c.Request.Context(), &filev1.StreamFileContentRequest{
			UserId: userID,
			FileId: fileID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		headerSent := false
		for {
			msg, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				if !headerSent {
					c.JSON(http.StatusInternalServerError, gin.H{"error": recvErr.Error()})
				}
				return
			}

			if !headerSent {
				contentType := msg.ContentType
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				c.Header("Content-Type", contentType)
				if msg.FileSize > 0 {
					c.Header("Content-Length", strconv.FormatInt(msg.FileSize, 10))
				}
				if msg.FileName != "" {
					asciiName := strings.Map(func(r rune) rune {
						if r > 127 {
							return '_'
						}
						return r
					}, msg.FileName)
					c.Header("Content-Disposition", fmt.Sprintf(
						"attachment; filename=\"%s\"; filename*=UTF-8''%s",
						asciiName,
						url.PathEscape(msg.FileName),
					))
				}
				c.Status(http.StatusOK)
				headerSent = true
			}

			if len(msg.Chunk) > 0 {
				if _, writeErr := c.Writer.Write(msg.Chunk); writeErr != nil {
					return
				}
				c.Writer.Flush()
			}
		}
		return
	}

	proxyReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, downloadReply.DownloadUrl, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid download url"})
		return
	}
	proxyResp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "proxy download failed"})
		return
	}
	defer proxyResp.Body.Close()

	if proxyResp.StatusCode >= http.StatusBadRequest {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream storage returned error"})
		return
	}

	contentType := proxyResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	if downloadReply.FileName != "" {
		asciiName := strings.Map(func(r rune) rune {
			if r > 127 {
				return '_'
			}
			return r
		}, downloadReply.FileName)
		c.Header("Content-Disposition", fmt.Sprintf(
			"attachment; filename=\"%s\"; filename*=UTF-8''%s",
			asciiName,
			url.PathEscape(downloadReply.FileName),
		))
	}
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, proxyResp.Body); err != nil {
		return
	}
}

func (h *FileHandler) GetDiskUsage(c *gin.Context) {
	reply, err := h.clients.File.GetDiskUsage(c.Request.Context(), &filev1.GetDiskUsageRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}
