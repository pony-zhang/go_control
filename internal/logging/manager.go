package logging

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	// 全局日志管理器实例
	defaultManager *Manager
	once           sync.Once
)

// Manager 日志管理器，负责管理多个日志实例
type Manager struct {
	mu       sync.RWMutex
	loggers  map[string]*Logger
	config   *Config
	shutdown chan struct{}
}

// NewManager 创建新的日志管理器
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		loggers:  make(map[string]*Logger),
		config:   config,
		shutdown: make(chan struct{}),
	}

	// 创建默认日志器
	defaultLogger, err := NewLogger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create default logger: %w", err)
	}
	m.loggers["default"] = defaultLogger

	return m, nil
}

// GetManager 获取全局日志管理器实例
func GetManager() *Manager {
	once.Do(func() {
		defaultManager, _ = NewManager(DefaultConfig())
	})
	return defaultManager
}

// GetLogger 获取指定名称的日志器
func (m *Manager) GetLogger(name string) (*Logger, error) {
	m.mu.RLock()
	logger, exists := m.loggers[name]
	m.mu.RUnlock()

	if exists {
		return logger, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 再次检查，防止并发创建
	if logger, exists := m.loggers[name]; exists {
		return logger, nil
	}

	// 创建新的日志器实例，可以基于不同的配置
	logger, err := NewLogger(m.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger %s: %w", name, err)
	}

	// 为不同的模块添加前缀
	if name != "default" {
		logger = logger.With("module", name)
	}

	m.loggers[name] = logger
	return logger, nil
}

// UpdateConfig 更新所有日志器的配置
func (m *Manager) UpdateConfig(config *Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	// 更新所有日志器
	for name, logger := range m.loggers {
		logger.UpdateLevel(config.Level)
		logger.Info("Logger configuration updated", "logger", name)
	}

	return nil
}

// GetLoggerNames 获取所有日志器名称
func (m *Manager) GetLoggerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.loggers))
	for name := range m.loggers {
		names = append(names, name)
	}
	return names
}

// RemoveLogger 移除指定的日志器
func (m *Manager) RemoveLogger(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "default" {
		return // 不允许移除默认日志器
	}

	delete(m.loggers, name)
}

// Close 关闭日志管理器
func (m *Manager) Close() error {
	close(m.shutdown)

	// 如果有文件输出，需要关闭文件
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, logger := range m.loggers {
		if logger.config.Output == "file" {
			logger.Info("Closing file logger", "logger", name)
		}
	}

	return nil
}

// 便捷函数：使用默认日志管理器获取日志器
func GetLogger(name string) *Logger {
	m := GetManager()
	logger, err := m.GetLogger(name)
	if err != nil {
		// 如果获取失败，返回默认日志器
		logger, _ = m.GetLogger("default")
		logger.Error("Failed to get logger", "requested_name", name, "error", err)
	}
	return logger
}

// 便捷函数：获取默认日志器
func Default() *Logger {
	return GetLogger("default")
}

// 便捷函数：使用上下文记录日志
func WithContext(ctx context.Context) *Logger {
	return Default().WithContext(ctx)
}

// 便捷函数：使用全局默认日志器记录各级别日志
func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}