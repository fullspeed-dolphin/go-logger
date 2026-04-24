// Package httpmw 提供基于 net/http 的结构化日志中间件。
// 与 logger 分开，避免只想用 logger 的项目被动依赖 net/http。
package httpmw

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fullspeed-dolphin/go-logger"

	"go.uber.org/zap"
)

// OpenIDExtractor 从请求 context 中提取登录用户 openid。
// 上游 auth 中间件把 openid 注入 context 后，日志中间件通过它读取。
// 返回空字符串表示未登录，日志将不附加 open_id 字段。
type OpenIDExtractor func(ctx context.Context) string

// Config 日志中间件配置
type Config struct {
	// OpenIDExtractor 可选。留空则不记录 open_id。
	OpenIDExtractor OpenIDExtractor
	// MaxBodyBytes body 日志最大字节数，默认 1024；超长截断并追加 "...(truncated)"。
	MaxBodyBytes int
	// SensitiveHeaders 需要脱敏的 header 名（大小写不敏感），默认 ["Authorization"]。
	SensitiveHeaders []string
}

// Middleware 返回结构化日志中间件。零值 Config 即可使用。
func Middleware(cfg Config) func(http.HandlerFunc) http.HandlerFunc {
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

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 1. trace_id：优先复用请求头 X-Trace-ID
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = generateTraceID()
			}
			ctx := logger.ContextWithTraceID(r.Context(), traceID)

			// 2. open_id（可选）
			var openID string
			if cfg.OpenIDExtractor != nil {
				openID = cfg.OpenIDExtractor(ctx)
				if openID != "" {
					ctx = logger.ContextWithUserOpenID(ctx, openID)
				}
			}
			r = r.WithContext(ctx)

			// 3. 响应头回传 trace_id
			w.Header().Set("X-Trace-ID", traceID)

			// 4. 读取 body（不破坏下游）
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// 5. headers（脱敏）
			headers := make(map[string]string, len(r.Header))
			for k, v := range r.Header {
				if len(v) == 0 {
					continue
				}
				if _, hit := sensitive[strings.ToLower(k)]; hit {
					headers[k] = maskToken(v[0])
				} else {
					headers[k] = v[0]
				}
			}

			// 6. request_start
			reqFields := []zap.Field{
				zap.String("trace_id", traceID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Any("headers", headers),
			}
			if openID != "" {
				reqFields = append(reqFields, zap.String("open_id", openID))
			}
			if len(bodyBytes) > 0 && hasBody(r.Method) {
				body := string(bodyBytes)
				if len(body) > maxBody {
					body = body[:maxBody] + "...(truncated)"
				}
				reqFields = append(reqFields, zap.String("body", body))
			}
			logger.L().Info("request_start", reqFields...)

			// 7. 包装 writer 捕获状态码
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next(wrapped, r)

			// 8. request_end
			respFields := []zap.Field{
				zap.String("trace_id", traceID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Duration("duration", time.Since(start)),
			}
			if openID != "" {
				respFields = append(respFields, zap.String("open_id", openID))
			}
			switch {
			case wrapped.statusCode >= 500:
				logger.L().Error("request_end", respFields...)
			case wrapped.statusCode >= 400:
				logger.L().Warn("request_end", respFields...)
			default:
				logger.L().Info("request_end", respFields...)
			}
		}
	}
}

// responseWriter 捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
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
