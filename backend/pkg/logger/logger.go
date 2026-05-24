package logger

import (
	"os"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建 zap logger，支持日志滚动存储
func New(level string, env string) (*zap.Logger, error) {
	var cfg zap.Config
	if env == "prod" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	if lvl, err := zapcore.ParseLevel(level); err == nil {
		cfg.Level = zap.NewAtomicLevelAt(lvl)
	}

	// 生产环境启用日志滚动存储
	if env == "prod" {
		logDir := getLogDir()
		// 确保日志目录存在
		_ = os.MkdirAll(logDir, 0o755)

		// 主日志文件 - 每个文件最大 100MB，保留 30 天，最多保留 10 个文件
		mainLog := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "app.log"),
			MaxSize:    100, // MB
			MaxBackups: 10,
			MaxAge:     30, // days
			Compress:   true,
		}

		// 错误日志单独一份
		errorLog := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "error.log"),
			MaxSize:    50, // MB
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}

		// 使用 tee core 同时写入文件和 stdout
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg.EncoderConfig),
			zapcore.AddSync(mainLog),
			cfg.Level,
		)

		errorCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg.EncoderConfig),
			zapcore.AddSync(errorLog),
			zap.NewAtomicLevelAt(zapcore.ErrorLevel),
		)

		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(cfg.EncoderConfig),
			zapcore.AddSync(os.Stdout),
			cfg.Level,
		)

		core := zapcore.NewTee(fileCore, errorCore, consoleCore)
		return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0)), nil
	}

	return cfg.Build(zap.AddCallerSkip(0))
}

// getLogDir 返回日志目录，优先使用 LOG_DIR 环境变量
func getLogDir() string {
	if dir := os.Getenv("LOG_DIR"); dir != "" {
		return dir
	}
	return "/var/log/ai-script"
}
