package httpapi

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const headerRequestID = "X-Request-ID"

// RequestID 为每个请求分配/透传请求 ID，写入响应头与 gin 上下文。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(headerRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Header(headerRequestID, rid)
		c.Next()
	}
}

// LogWithZap 结构化记录每个请求的方法、路径、状态码与耗时。
func LogWithZap(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http_request",
			zap.String("request_id", c.GetString("request_id")),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}

// RecoverWithZap 捕获 panic，记录后返回统一 INTERNAL 错误体。
func RecoverWithZap(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic_recovered",
					zap.String("request_id", c.GetString("request_id")),
					zap.Any("error", err),
				)
				AbortWithError(c, ErrInternal("internal error"))
			}
		}()
		c.Next()
	}
}
