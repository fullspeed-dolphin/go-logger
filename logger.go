// Package logger 提供基于 zap 的结构化日志封装。
// 设计为零业务耦合，可跨 Go 服务复用。
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Config 日志配置
type Config struct {
	Level      string `yaml:"level"`       // debug | info | warn | error
	OutputPath string `yaml:"output_path"` // 文件路径，空则仅 stdout
	AppName    string `yaml:"app_name"`    // 应用名，作为固定字段 app 输出
}

// Init 初始化全局 Logger（JSON 格式，适配 ELK/Loki）。
// 幂等：重复调用仅首次生效。
func Init(cfg *Config) {
	once.Do(func() {
		level := parseLevel(cfg.Level)

		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		writers := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}
		if cfg.OutputPath != "" {
			if f, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				writers = append(writers, zapcore.AddSync(f))
			}
		}

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(writers...),
			level,
		)

		l := zap.New(core,
			zap.AddCaller(),
			zap.AddStacktrace(zapcore.ErrorLevel),
		)
		if cfg.AppName != "" {
			l = l.With(zap.String("app", cfg.AppName))
		}
		globalLogger = l
	})
}

// L 返回全局 Logger。未初始化时使用默认配置 fallback。
func L() *zap.Logger {
	if globalLogger == nil {
		Init(&Config{Level: "info", AppName: "default"})
	}
	return globalLogger
}

// Sync 刷新缓冲。建议在 main 退出前 defer 调用。
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
