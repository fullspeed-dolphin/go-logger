# go-logger

基于 [zap](https://github.com/uber-go/zap) 的结构化日志封装，面向 speexpay.com 系列 Go 服务复用。

- JSON 格式输出，适配 ELK / Loki。
- 内置 `trace_id` / `open_id` 的 context 注入与提取。
- 附带 `net/http` 结构化日志中间件（`httpmw`），可选装。
- 零业务耦合：`logger` 子包不依赖 `net/http`。

## 安装

```bash
go get github.com/speexpay/go-logger
```

> 若仓库尚未推到远端，可在各服务 `go.mod` 里加 `replace`：
> ```
> replace github.com/speexpay/go-logger => ../go-logger
> ```

## 快速开始

### 1. 初始化（main.go）

```go
import (
    "github.com/speexpay/go-logger"
)

func main() {
    logger.Init(&logger.Config{
        Level:   "info",      // debug | info | warn | error
        AppName: "sport-api", // 固定字段 app 会出现在每条日志里
        // OutputPath: "/var/log/sport-api.log", // 可选；空则仅 stdout
    })
    defer logger.Sync()

    logger.L().Info("server started")
}
```

### 2. 业务代码里带 context 打日志

```go
// 自动携带 trace_id 和 open_id（若 context 中存在）
logger.WithContext(ctx).Info("processing activity",
    zap.String("activity_id", id),
)

// 只带 trace_id
logger.WithTraceID(ctx).Warn("slow query", zap.Duration("elapsed", d))

// 全局 logger（不带 context 字段）
logger.L().Error("db connect failed", zap.Error(err))
```

### 3. HTTP 中间件

```go
import (
    "net/http"

    "github.com/speexpay/go-logger"
    "github.com/speexpay/go-logger/httpmw"
)

// 上游 auth 中间件把 openid 放进 context，日志中间件通过 extractor 读取
openIDExtractor := func(ctx context.Context) string {
    v, _ := ctx.Value(myauth.OpenIDKey).(string)
    return v
}

logMW := httpmw.Middleware(httpmw.Config{
    OpenIDExtractor:  openIDExtractor,
    MaxBodyBytes:     1024,
    SensitiveHeaders: []string{"Authorization", "Cookie"},
})

http.HandleFunc("/ping", logMW(pingHandler))
```

**请求开始日志示例：**
```json
{
  "timestamp": "2026-04-19T10:30:00.123+0800",
  "level": "info",
  "msg": "request_start",
  "app": "sport-api",
  "trace_id": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
  "method": "POST",
  "path": "/api/activity/details",
  "query": "page=1",
  "remote_addr": "192.168.1.100:54321",
  "open_id": "oXyz123456",
  "headers": {"Content-Type": "application/json", "Authorization": "Bearer e****f8g2"},
  "body": "{\"start_date\":\"2026-04-01\"}"
}
```

## Context key

| Key                    | 说明                                    |
| ---------------------- | --------------------------------------- |
| `logger.TraceIDKey`    | trace_id（16 字节十六进制）             |
| `logger.UserOpenIDKey` | 登录用户 openid                         |

对应工具方法：`ContextWithTraceID / TraceIDFromContext / ContextWithUserOpenID / OpenIDFromContext`。

## 目录结构

```
go-logger/
├── go.mod
├── logger.go        # Init / L / Sync / Config
├── context.go       # trace_id / openid 的 context 工具 + WithContext
└── httpmw/
    └── middleware.go # net/http 结构化日志中间件
```

## 版本

`v0.1.0` — 从 `sport-api/pkg/logger` 抽取独立。

