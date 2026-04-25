// Package ginmw 提供基于 Gin 的结构化日志中间件。
// 与 httpmw 平行，适用于使用 gin-gonic/gin 的服务。
package ginmw

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fullspeed-dolphin/go-logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OpenIDExtractor 从 Gin context 中提取登录用户 openid。
// 典型实现：从 c.Get("openid") 读取 auth 中间件写入的值。
// 返回空字符串表示未登录，日志将不附加 open_id 字段。
type OpenIDExtractor func(c *gin.Context) string

// Config 日志中间件配置
type Config struct {
	// OpenIDExtractor 可选。留空则不记录 open_id。
	// 在 c.Next() 之后调用，确保 auth 中间件已执行。
	OpenIDExtractor OpenIDExtractor
	// MaxBodyBytes body 日志最大字节数，默认 1024；超长截断并追加 "...(truncated)"。
	MaxBodyBytes int
	// SensitiveHeaders 需要脱敏的 header 名（大小写不敏感），默认 ["Authorization"]。
	SensitiveHeaders []string
}

// Logging 返回 Gin 原生结构化日志中间件。零值 Config 即可使用。
func Logging(cfg Config) gin.HandlerFunc {
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 1024
	}
	sensitive := map[string]struct{}{}
	if len(cfg.SensitiveHeaders) == 0 {
		sensitive["authorization"] = struct{}{}
	} else {
		for _, h := range cfg.SensitiveHeaders {
			sensitive[strings.ToLower(h)] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		start := time.Now()

		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = generateTraceID()
		}
		ctx := logger.ContextWithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Trace-ID", traceID)

		var bodySnippet string
		if c.Request.Body != nil && hasBody(c.Request.Method) {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			bodySnippet = string(bodyBytes)
			if len(bodySnippet) > maxBody {
				bodySnippet = bodySnippet[:maxBody] + "...(truncated)"
			}
		}

		headers := make(map[string]string, len(c.Request.Header))
		for k, v := range c.Request.Header {
			if len(v) == 0 {
				continue
			}
			if _, hit := sensitive[strings.ToLower(k)]; hit {
				headers[k] = maskToken(v[0])
			} else {
				headers[k] = v[0]
			}
		}

		reqFields := []zap.Field{
			zap.String("trace_id", traceID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("remote_addr", c.ClientIP()),
			zap.Any("headers", headers),
		}
		if bodySnippet != "" {
			reqFields = append(reqFields, zap.String("body", bodySnippet))
		}
		logger.L().Info("request_start", reqFields...)

		c.Next()

		// auth 中间件已在 c.Next() 链中执行，此时可提取 openID
		var openID string
		if cfg.OpenIDExtractor != nil {
			openID = cfg.OpenIDExtractor(c)
			if openID != "" {
				rctx := logger.ContextWithUserOpenID(c.Request.Context(), openID)
				c.Request = c.Request.WithContext(rctx)
			}
		}

		status := c.Writer.Status()
		respFields := []zap.Field{
			zap.String("trace_id", traceID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("duration", time.Since(start)),
		}
		if openID != "" {
			respFields = append(respFields, zap.String("open_id", openID))
		}

		switch {
		case status >= 500:
			logger.L().Error("request_end", respFields...)
		case status >= 400:
			logger.L().Warn("request_end", respFields...)
		default:
			logger.L().Info("request_end", respFields...)
		}
	}
}

func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func hasBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}

func maskToken(token string) string {
	if token == "" {
		return "<none>"
	}
	if len(token) <= 12 {
		return "****"
	}
	return token[:8] + "****" + token[len(token)-4:]
}
