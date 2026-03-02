package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
		Formatter: func(param gin.LogFormatterParams) string {
			return param.TimeStamp.Format(time.RFC3339) +
				" | " + param.Method +
				" | " + param.Path +
				" | " + param.ClientIP +
				" | " + param.Latency.String() +
				" | " + param.StatusCodeColor() + " " + param.ResetColor() +
				"\n"
		},
	})
}
