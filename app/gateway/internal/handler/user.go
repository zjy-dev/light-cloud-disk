package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
	"github.com/J-Y-Zhang/light-cloud-disk/app/gateway/internal/client"
)

type UserHandler struct {
	clients *client.ServiceClients
}

func NewUserHandler(clients *client.ServiceClients) *UserHandler {
	return &UserHandler{clients: clients}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req userv1.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := h.clients.User.Register(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req userv1.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := h.clients.User.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetInt64("user_id")

	reply, err := h.clients.User.GetUserInfo(c.Request.Context(), &userv1.GetUserInfoRequest{
		UserId: userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

func (h *UserHandler) UpdateUserInfo(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req userv1.UpdateUserInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserId = userID

	reply, err := h.clients.User.UpdateUserInfo(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}
