// Package logger 提供基于 zap 的结构化日志封装。
// 设计为零业务耦合，可跨 Go 服务复用。
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Config 日志配置
type Config struct {
	Level      string `yaml:"level"`       // debug | info | warn | error
	OutputPath string `yaml:"output_path"` // 日志文件路径；空则仅 stdout，不落盘
	AppName    string `yaml:"app_name"`    // 应用名，作为固定字段 app 输出

	// ===== 仅在 OutputPath != "" 时生效的轮转参数（lumberjack） =====
	MaxSizeMB       int  `yaml:"max_size_mb"`      // 单文件最大 MB，默认 100
	MaxBackups      int  `yaml:"max_backups"`      // 保留的旧文件数，默认 7
	MaxAgeDays      int  `yaml:"max_age_days"`     // 旧文件最大保留天数，默认 30
	DisableCompress bool `yaml:"disable_compress"` // 默认压缩旧文件；置 true 关闭
	DisableConsole  bool `yaml:"disable_console"`  // 落盘时是否关闭 stdout；默认 false
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
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		writers := make([]zapcore.WriteSyncer, 0, 2)
		if !cfg.DisableConsole || cfg.OutputPath == "" {
			// 未显式禁用，或没配文件路径时，必须保留 stdout，否则一条日志都看不到
			writers = append(writers, zapcore.AddSync(os.Stdout))
		}
		if cfg.OutputPath != "" {
			writers = append(writers, zapcore.AddSync(newRotator(cfg)))
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

// newRotator 基于 Config 构造 lumberjack 轮转 writer。
// 零值字段回落到合理默认，避免使用者必须全填。
func newRotator(cfg *Config) *lumberjack.Logger {
	maxSize := cfg.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 7
	}
	maxAge := cfg.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 30
	}
	return &lumberjack.Logger{
		Filename:   cfg.OutputPath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   !cfg.DisableCompress,
		LocalTime:  true,
	}
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
