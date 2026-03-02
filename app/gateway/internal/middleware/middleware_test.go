package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestToken(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func TestJWTAuth_ValidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	token := createTestToken("test-secret", jwt.MapClaims{
		"user_id": float64(42),
		"exp":     float64(time.Now().Add(time.Hour).Unix()),
	})

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	var capturedUserID int64
	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		capturedUserID = c.GetInt64("user_id")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), capturedUserID)
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "InvalidToken")
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	token := createTestToken("test-secret", jwt.MapClaims{
		"user_id": float64(42),
		"exp":     float64(time.Now().Add(-time.Hour).Unix()),
	})

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "correct-secret")
	defer os.Unsetenv("JWT_SECRET")

	token := createTestToken("wrong-secret", jwt.MapClaims{
		"user_id": float64(42),
		"exp":     float64(time.Now().Add(time.Hour).Unix()),
	})

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_MissingUserID(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	token := createTestToken("test-secret", jwt.MapClaims{
		"exp": float64(time.Now().Add(time.Hour).Unix()),
		// no user_id
	})

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(JWTAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCORS_Headers(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(CORS())
	r.OPTIONS("/test", func(c *gin.Context) {})
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	c.Request, _ = http.NewRequest("OPTIONS", "/test", nil)
	c.Request.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
}
