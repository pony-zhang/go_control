// Package comm provides base communication interfaces and implementations
package comm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/internal/logging"
)

// BaseCommunication 基础通信实现
type BaseCommunication struct {
	config        ConnectionConfig
	status        ConnectionStatus
	lastError     error
	eventHandlers []EventHandler
	errorHandler  ErrorHandler
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	logger        *logging.Logger
}

// NewBaseCommunication 创建基础通信实例
func NewBaseCommunication(config ConnectionConfig) *BaseCommunication {
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseCommunication{
		config:       config,
		status:       StatusDisconnected,
		eventHandlers: make([]EventHandler, 0),
		ctx:          ctx,
		cancel:       cancel,
		logger:       logging.GetLogger("base_communication"),
	}
}

// GetStatus 获取连接状态
func (bc *BaseCommunication) GetStatus() ConnectionStatus {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.status
}

// SetLastError 设置最后错误
func (bc *BaseCommunication) SetLastError(err error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.lastError = err
}

// GetLastError 获取最后错误
func (bc *BaseCommunication) GetLastError() error {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.lastError
}

// IsConnected 检查是否连接
func (bc *BaseCommunication) IsConnected() bool {
	return bc.GetStatus() == StatusConnected
}

// AddEventHandler 添加事件处理器
func (bc *BaseCommunication) AddEventHandler(handler EventHandler) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.eventHandlers = append(bc.eventHandlers, handler)
}

// RemoveEventHandler 移除事件处理器
func (bc *BaseCommunication) RemoveEventHandler(handler EventHandler) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	for i, h := range bc.eventHandlers {
		if h == handler {
			bc.eventHandlers = append(bc.eventHandlers[:i], bc.eventHandlers[i+1:]...)
			break
		}
	}
}

// SetErrorHandler 设置错误处理器
func (bc *BaseCommunication) SetErrorHandler(handler ErrorHandler) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.errorHandler = handler
}

// emitEvent 触发事件
func (bc *BaseCommunication) emitEvent(callback func(EventHandler)) {
	bc.mutex.RLock()
	handlers := make([]EventHandler, len(bc.eventHandlers))
	copy(handlers, bc.eventHandlers)
	bc.mutex.RUnlock()

	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					bc.logger.Error("Event handler panic", "panic", r)
				}
			}()
			callback(handler)
		}()
	}
}

// EmitConnected 触发连接事件
func (bc *BaseCommunication) EmitConnected() {
	bc.emitEvent(func(h EventHandler) { h.OnConnected() })
}

// EmitDisconnected 触发断开事件
func (bc *BaseCommunication) EmitDisconnected() {
	bc.emitEvent(func(h EventHandler) { h.OnDisconnected() })
}

// EmitError 触发错误事件
func (bc *BaseCommunication) EmitError(err error) {
	bc.emitEvent(func(h EventHandler) { h.OnError(err) })
}

// EmitDataReceived 触发数据接收事件
func (bc *BaseCommunication) EmitDataReceived(address string, data []byte) {
	bc.emitEvent(func(h EventHandler) { h.OnDataReceived(address, data) })
}

// EmitDataWritten 触发数据发送事件
func (bc *BaseCommunication) EmitDataWritten(address string, data []byte) {
	bc.emitEvent(func(h EventHandler) { h.OnDataWritten(address, data) })
}

// SetStatus 设置状态
func (bc *BaseCommunication) SetStatus(status ConnectionStatus) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.status = status
}

// HandleWithError 错误处理
func (bc *BaseCommunication) HandleWithError(err error) error {
	bc.SetLastError(err)

	if bc.errorHandler != nil {
		err = bc.errorHandler.HandleError(err)
	}

	bc.EmitError(err)
	return err
}

// RetryWithTimeout 带超时的重试机制
func (bc *BaseCommunication) RetryWithTimeout(ctx context.Context, operation func() error) error {
	var lastErr error

	for i := 0; i <= bc.config.RetryCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否应该重试
		if bc.errorHandler != nil && !bc.errorHandler.ShouldRetry(err) {
			return err
		}

		// 如果是最后一次尝试，直接返回错误
		if i == bc.config.RetryCount {
			break
		}

		// 计算重试延迟
		delay := bc.config.RetryInterval
		if bc.errorHandler != nil {
			if customDelay := bc.errorHandler.GetRetryDelay(err); customDelay > 0 {
				delay = customDelay
			}
		}

		// 等待重试
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		bc.logger.Warn("Retry after error", "attempt", i+1, "max_attempts", bc.config.RetryCount, "error", err)
	}

	return fmt.Errorf("operation failed after %d retries, last error: %w", bc.config.RetryCount, lastErr)
}

// Context 获取上下文
func (bc *BaseCommunication) Context() context.Context {
	return bc.ctx
}

// Cancel 取消操作
func (bc *BaseCommunication) Cancel() {
	bc.cancel()
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct{}

func (de *DefaultErrorHandler) HandleError(err error) error {
	return err
}

func (de *DefaultErrorHandler) ShouldRetry(err error) bool {
	// 默认重试网络错误和超时错误
	return isNetworkError(err) || isTimeoutError(err)
}

func (de *DefaultErrorHandler) GetRetryDelay(err error) time.Duration {
	// 默认退避策略：1s, 2s, 4s...
	return 0 // 使用默认的重试间隔
}

// isNetworkError 检查是否是网络错误
func isNetworkError(err error) bool {
	// 这里可以添加具体的网络错误判断逻辑
	return false
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	// 这里可以添加具体的超时错误判断逻辑
	return false
}