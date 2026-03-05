package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/client"
)

type FileHandler struct {
	clients *client.ServiceClients
}

func NewFileHandler(clients *client.ServiceClients) *FileHandler {
	return &FileHandler{clients: clients}
}

func (h *FileHandler) CheckUpload(c *gin.Context) {
	var req filev1.CheckUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := h.clients.File.CheckUpload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if reply.DiskFull {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "disk full, please retry later", "disk_full": true})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) UploadChunk(c *gin.Context) {
	fileMD5 := c.PostForm("file_md5")
	if fileMD5 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_md5 is required"})
		return
	}

	chunkIndex, err := strconv.ParseInt(c.PostForm("chunk_index"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk_index"})
		return
	}

	fileHeader, err := c.FormFile("chunk_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk_file is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read chunk_file"})
		return
	}
	defer file.Close()

	chunkData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read chunk_file data"})
		return
	}

	var chunkSize int64
	if chunkSizeStr := c.PostForm("chunk_size"); chunkSizeStr != "" {
		chunkSize, err = strconv.ParseInt(chunkSizeStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk_size"})
			return
		}
	} else {
		chunkSize = int64(len(chunkData))
	}

	req := &filev1.UploadChunkRequest{
		FileMd5:    fileMD5,
		ChunkIndex: int32(chunkIndex),
		ChunkSize:  int32(chunkSize),
		ChunkData:  chunkData,
	}

	reply, err := h.clients.File.UploadChunk(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *FileHandler) MergeChunks(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req filev1.MergeChunksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.File.MergeChunks(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
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

	// For local-mode files, rewrite "local://..." to a gateway-served stream URL.
	// This keeps the response format consistent for the frontend.
	if strings.HasPrefix(reply.DownloadUrl, "local://") {
		reply.DownloadUrl = fmt.Sprintf("/api/v1/file/stream/%d", fileID)
	}

	c.JSON(http.StatusOK, reply)
}

// StreamFile streams a locally-stored file directly to the HTTP client.
// Registered as GET /api/v1/file/stream/:file_id (behind JWT).
func (h *FileHandler) StreamFile(c *gin.Context) {
	userID := c.GetInt64("user_id")
	fileID, _ := strconv.ParseInt(c.Param("file_id"), 10, 64)

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
				// RFC 5987 encoding for non-ASCII filenames
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
}

func (h *FileHandler) GetDiskUsage(c *gin.Context) {
	reply, err := h.clients.File.GetDiskUsage(c.Request.Context(), &filev1.GetDiskUsageRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}
