package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config 日志配置结构
type Config struct {
	Level      string `yaml:"level"`       // 日志级别: debug, info, warn, error
	Format     string `yaml:"format"`      // 输出格式: json, text
	Output     string `yaml:"output"`      // 输出目标: stdout, stderr, file
	OutputPath string `yaml:"output_path"` // 文件输出路径
	AddSource  bool   `yaml:"add_source"`  // 是否添加源码位置
	TimeFormat string `yaml:"time_format"`  // 时间格式
}

// Logger 封装的结构化日志器
type Logger struct {
	*slog.Logger
	config *Config
}

// NewLogger 创建新的日志器实例
func NewLogger(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 解析日志级别
	level := parseLevel(config.Level)

	// 创建日志处理器
	handler, err := createHandler(config, level)
	if err != nil {
		return nil, err
	}

	logger := slog.New(handler)
	return &Logger{
		Logger: logger,
		config: config,
	}, nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "text",
		Output:     "stdout",
		AddSource:  false,
		TimeFormat: time.RFC3339,
	}
}

// parseLevel 解析日志级别
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// createHandler 创建日志处理器
func createHandler(config *Config, level slog.Level) (slog.Handler, error) {
	var writer *os.File
	var err error

	// 确定输出目标
	switch strings.ToLower(config.Output) {
	case "stderr":
		writer = os.Stderr
	case "file":
		if config.OutputPath == "" {
			config.OutputPath = "logs/app.log"
		}
		// 确保日志目录存在
		if err := os.MkdirAll("logs", 0755); err != nil {
			return nil, err
		}
		writer, err = os.OpenFile(config.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
	default:
		writer = os.Stdout
	}

	// 创建基础选项
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// 根据格式创建处理器
	var handler slog.Handler
	if strings.ToLower(config.Format) == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	return handler, nil
}

// WithContext 返回带有上下文的日志器
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: slog.New(l.Logger.Handler()),
		config: l.config,
	}
}

// With 返回带有额外字段的日志器
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
		config: l.config,
	}
}

// WithGroup 返回带有分组的日志器
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: l.Logger.WithGroup(name),
		config: l.config,
	}
}

// UpdateLevel 动态更新日志级别
func (l *Logger) UpdateLevel(level string) {
	l.config.Level = level
	newLevel := parseLevel(level)

	// 重新创建处理器
	handler, err := createHandler(l.config, newLevel)
	if err != nil {
		l.Error("Failed to update log level", "error", err)
		return
	}

	l.Logger = slog.New(handler)
}

// GetConfig 获取当前配置
func (l *Logger) GetConfig() *Config {
	return l.config
}