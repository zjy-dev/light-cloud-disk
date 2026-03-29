package server

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v5"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
)

// ChunkHTTPServer exposes HTTP endpoints for direct chunk upload / download,
// bypassing the API gateway to eliminate the upload bandwidth bottleneck.
type ChunkHTTPServer struct {
	engine *gin.Engine
	addr   string
	uc     *biz.FileUsecase
	tmpDir string
	log    *log.Helper
}

const fileSizeHeader = "X-File-Size"

// NewChunkHTTPServer creates a Gin server for chunk upload / download.
// addr and jwtSecret are provided from env vars by the main package.
func NewChunkHTTPServer(uc *biz.FileUsecase, addr, jwtSecret, tmpDir string, logger log.Logger) *ChunkHTTPServer {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	s := &ChunkHTTPServer{
		engine: r,
		addr:   addr,
		uc:     uc,
		tmpDir: tmpDir,
		log:    log.NewHelper(logger),
	}

	// Chunk API group — all routes require JWT
	api := r.Group("/api/v1")
	api.Use(jwtAuthMiddleware(jwtSecret))
	{
		api.PUT("/chunks/:md5/:index", s.UploadChunk)
		api.GET("/chunks/:md5/:index", s.DownloadChunk)
		api.GET("/chunks/recover/:md5/:index", s.RecoverChunk)
	}

	return s
}

// Start begins listening. Called by kratos.App lifecycle.
func (s *ChunkHTTPServer) Start(_ context.Context) error {
	s.log.Infof("chunk HTTP server listening on %s", s.addr)
	return s.engine.Run(s.addr)
}

// Stop gracefully shuts down the HTTP server (no-op for gin.Engine.Run).
func (s *ChunkHTTPServer) Stop(_ context.Context) error {
	return nil
}

// UploadChunk handles PUT /api/v1/chunks/:md5/:index
// The request body is the raw chunk bytes.
func (s *ChunkHTTPServer) UploadChunk(c *gin.Context) {
	fileMD5 := c.Param("md5")
	chunkIndex, err := strconv.ParseInt(c.Param("index"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
		return
	}

	fileSize, err := parseFileSize(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64<<20)) // max 64 MB chunk
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	chunkSize := int64(len(body))
	if err := s.uc.SaveChunk(c.Request.Context(), fileMD5, int32(chunkIndex), chunkSize, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	chunkPath := chunkFilePath(s.tmpDir, fileMD5, int32(chunkIndex))
	if err := s.uc.PrepareChunkRecovery(c.Request.Context(), fileMD5, int32(chunkIndex), chunkPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare chunk recovery: " + err.Error()})
		return
	}

	// Write a chunk record for the scattered storage registry.
	instanceID := c.Request.Host
	if instanceID == "" {
		instanceID = s.addr
	}
	checksum := md5Bytes(body)
	rec := &biz.ChunkRecord{
		FileMD5:    fileMD5,
		FileSize:   fileSize,
		ChunkIndex: int32(chunkIndex),
		ChunkSize:  chunkSize,
		InstanceID: instanceID,
		StorePath:  chunkPath,
		Checksum:   checksum,
		StorageType: biz.StorageLocal,
	}
	if err := s.uc.CreateChunkRecord(c.Request.Context(), rec); err != nil {
		s.log.Warnf("failed to create chunk record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create chunk record: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"chunk_index": chunkIndex,
		"checksum":    checksum,
	})
}

// DownloadChunk handles GET /api/v1/chunks/:md5/:index
// Streams the raw chunk bytes from disk.
func (s *ChunkHTTPServer) DownloadChunk(c *gin.Context) {
	fileMD5 := c.Param("md5")
	chunkIndex, err := strconv.ParseInt(c.Param("index"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
		return
	}

	chunkPath := chunkFilePath(s.tmpDir, fileMD5, int32(chunkIndex))

	f, err := os.Open(chunkPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chunk not found"})
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	c.DataFromReader(http.StatusOK, stat.Size(), "application/octet-stream", f, nil)
}

// RecoverChunk handles GET /api/v1/chunks/recover/:md5/:index and rebuilds a
// chunk from its recovery shards in OSS.
func (s *ChunkHTTPServer) RecoverChunk(c *gin.Context) {
	fileMD5 := c.Param("md5")
	chunkIndex, err := strconv.ParseInt(c.Param("index"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
		return
	}

	fileSize, err := parseFileSize(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, chunkSize, err := s.uc.RecoverChunk(c.Request.Context(), fileMD5, fileSize, int32(chunkIndex))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.DataFromReader(http.StatusOK, chunkSize, "application/octet-stream", bytes.NewReader(data), nil)
}

// jwtAuthMiddleware validates bearer tokens using the same secret as the gateway.
func jwtAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		if uid, ok := claims["user_id"].(float64); ok {
			c.Set("user_id", int64(uid))
		}
		c.Next()
	}
}

func md5Bytes(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}

func parseFileSize(c *gin.Context) (int64, error) {
	rawSize := strings.TrimSpace(c.GetHeader(fileSizeHeader))
	if rawSize == "" {
		rawSize = strings.TrimSpace(c.Query("file_size"))
	}
	if rawSize == "" {
		return 0, fmt.Errorf("missing %s header", fileSizeHeader)
	}

	fileSize, err := strconv.ParseInt(rawSize, 10, 64)
	if err != nil || fileSize <= 0 {
		return 0, fmt.Errorf("invalid file size")
	}
	return fileSize, nil
}

func chunkFilePath(rootDir, fileMD5 string, chunkIndex int32) string {
	return filepath.Join(rootDir, fileMD5, fmt.Sprintf("%06d.part", chunkIndex))
}
