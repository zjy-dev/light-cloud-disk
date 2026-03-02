package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/client"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/handler"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/middleware"
)

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", ":8080", "HTTP listen address")
}

func main() {
	flag.Parse()
	godotenv.Load()

	if envAddr := os.Getenv("GATEWAY_ADDR"); envAddr != "" {
		addr = envAddr
	}

	// Create gRPC clients via Consul discovery
	clients := client.NewServiceClients()

	// Create handlers
	userHandler := handler.NewUserHandler(clients)
	fileHandler := handler.NewFileHandler(clients)

	// Setup Gin
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public routes (no auth)
	api := r.Group("/api/v1")
	{
		api.POST("/user/register", userHandler.Register)
		api.POST("/user/login", userHandler.Login)
		api.GET("/share/:share_id", fileHandler.GetShare)
	}

	// Protected routes (JWT auth required)
	auth := api.Group("")
	auth.Use(middleware.JWTAuth())
	{
		// User
		auth.GET("/user/info", userHandler.GetUserInfo)
		auth.PUT("/user/info", userHandler.UpdateUserInfo)

		// File
		auth.POST("/file/check-upload", fileHandler.CheckUpload)
		auth.POST("/file/upload-chunk", fileHandler.UploadChunk)
		auth.POST("/file/merge-chunks", fileHandler.MergeChunks)
		auth.GET("/files", fileHandler.ListFiles)
		auth.POST("/file/folder", fileHandler.CreateFolder)
		auth.PUT("/file/rename", fileHandler.RenameFile)
		auth.DELETE("/files", fileHandler.DeleteFile)
		auth.PUT("/file/move", fileHandler.MoveFile)
		auth.GET("/file/download/:file_id", fileHandler.GetDownloadURL)
		auth.GET("/files/search", fileHandler.SearchFiles)

		// Trash
		auth.GET("/trash", fileHandler.ListTrash)
		auth.POST("/trash/restore", fileHandler.RestoreFile)
		auth.DELETE("/trash", fileHandler.PermanentDelete)

		// Share
		auth.POST("/share", fileHandler.CreateShare)
	}

	if err := r.Run(addr); err != nil {
		panic(err)
	}
}
