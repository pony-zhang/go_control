// Package core implements the central event loop that coordinates all motion control operations.
// The event loop provides a true event-driven system that handles various event types
// including system events, task events, device events, network events, and timer events.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/internal/logging"
)

type EventLoop struct {
	modules       map[string]Module
	modulesLock   sync.RWMutex
	handlers      map[EventType][]EventHandler
	handlersLock  sync.RWMutex
	eventQueue    chan Event
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	logger        *logging.Logger
	bufferSize    int
	timer         *time.Ticker
	timerInterval time.Duration
}

func NewEventLoop(timerInterval time.Duration) *EventLoop {
	return &EventLoop{
		modules:       make(map[string]Module),
		handlers:      make(map[EventType][]EventHandler),
		eventQueue:    make(chan Event, 1000),
		logger:        logging.GetLogger("event_loop"),
		bufferSize:    1000,
		timerInterval: timerInterval,
	}
}

func (el *EventLoop) Start(ctx context.Context) error {
	if el.running {
		return fmt.Errorf("event loop is already running")
	}

	el.ctx, el.cancel = context.WithCancel(ctx)
	el.running = true

	// 启动定时器
	if el.timerInterval > 0 {
		el.timer = time.NewTicker(el.timerInterval)
	}

	// 启动事件循环
	el.wg.Add(1)
	go el.run()

	// 发送系统启动事件
	el.EmitEvent(NewSystemEvent(EventTypeSystemStart, "event_loop", nil))

	el.logger.Info("Event loop started", "timer_interval", el.timerInterval)
	return nil
}

func (el *EventLoop) Stop() error {
	if !el.running {
		return fmt.Errorf("event loop is not running")
	}

	el.logger.Info("Starting event loop shutdown...")

	// 发送系统停止事件
	el.EmitEvent(NewSystemEvent(EventTypeSystemStop, "event_loop", nil))

	// 等待一小段时间让系统停止事件被处理
	time.Sleep(10 * time.Millisecond)

	// 取消事件循环的 context
	el.cancel()
	el.logger.Info("Event loop context canceled")

	// 停止定时器
	if el.timer != nil {
		el.timer.Stop()
	}

	// 停止所有注册的模块
	el.modulesLock.Lock()
	moduleCount := len(el.modules)
	el.logger.Info("Stopping modules", "count", moduleCount)

	for name, module := range el.modules {
		el.logger.Info("Stopping module", "module_name", name)
		if err := module.Stop(); err != nil {
			el.logger.Error("Error stopping module", "module_name", name, "error", err)
		} else {
			el.logger.Info("Module stopped successfully", "module_name", name)
		}
	}
	el.modulesLock.Unlock()

	// 等待所有 goroutine 完成
	el.logger.Info("Waiting for event loop goroutines to complete...")
	el.wg.Wait()
	el.logger.Info("All event loop goroutines completed")

	el.running = false
	el.logger.Info("Event loop stopped successfully")
	return nil
}

func (el *EventLoop) RegisterModule(name string, module Module) error {
	el.modulesLock.Lock()
	defer el.modulesLock.Unlock()

	if _, exists := el.modules[name]; exists {
		return fmt.Errorf("module %s already registered", name)
	}

	el.modules[name] = module

	// 自动注册模块的事件处理器
	subscribedEvents := module.GetSubscribedEvents()
	for _, eventType := range subscribedEvents {
		if err := el.RegisterHandler(eventType, module); err != nil {
			el.logger.Error("Error registering event handler", "module_name", name, "event_type", eventType, "error", err)
			// 继续执行，不要因为一个事件类型注册失败而影响整个模块注册
		}
	}

	el.logger.Info("Module registered", "module_name", name, "subscribed_events", subscribedEvents)

	if el.running {
		if err := module.Start(el.ctx); err != nil {
			el.logger.Error("Error starting module", "module_name", name, "error", err)
			return err
		}
	}

	return nil
}

func (el *EventLoop) UnregisterModule(name string) error {
	el.modulesLock.Lock()
	defer el.modulesLock.Unlock()

	module, exists := el.modules[name]
	if !exists {
		return fmt.Errorf("module %s not found", name)
	}

	if el.running {
		if err := module.Stop(); err != nil {
			el.logger.Error("Error stopping module", "module_name", name, "error", err)
		}
	}

	delete(el.modules, name)
	el.logger.Info("Module unregistered", "module_name", name)
	return nil
}

func (el *EventLoop) run() {
	defer el.wg.Done()
	defer el.logger.Info("Event loop run() completed")

	el.logger.Info("Event loop run() started")

	for {
		select {
		case <-el.ctx.Done():
			el.logger.Info("Event loop received context cancellation, exiting...")
			return

		// 统一事件队列
		case event := <-el.eventQueue:
			el.handleEvent(event)

		// 定时器事件
		case <-el.timer.C:
			event := NewTimerEvent("event_loop", el.timerInterval, 0, nil)
			el.handleEvent(event)
		}
	}
}

func (el *EventLoop) handleEvent(event Event) {
	el.logger.Debug("Handling event", "event_type", event.Type(), "source", event.Source(), "timestamp", event.Timestamp())

	el.handlersLock.RLock()
	handlers, exists := el.handlers[event.Type()]
	el.handlersLock.RUnlock()

	if !exists {
		el.logger.Debug("No handlers found for event type", "event_type", event.Type())
		return
	}

	// 异步处理事件，避免阻塞事件循环
	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h.HandleEvent(event); err != nil {
				el.logger.Error("Error handling event", "handler_name", h.Name(), "event_type", event.Type(), "error", err)
			}
		}(handler)
	}
}

// RegisterHandler 注册事件处理器
func (el *EventLoop) RegisterHandler(eventType EventType, handler EventHandler) error {
	el.handlersLock.Lock()
	defer el.handlersLock.Unlock()

	if el.handlers == nil {
		el.handlers = make(map[EventType][]EventHandler)
	}

	el.handlers[eventType] = append(el.handlers[eventType], handler)
	el.logger.Debug("Event handler registered", "event_type", eventType, "handler_name", handler.Name())
	return nil
}

// UnregisterHandler 注销事件处理器
func (el *EventLoop) UnregisterHandler(eventType EventType, handler EventHandler) error {
	el.handlersLock.Lock()
	defer el.handlersLock.Unlock()

	handlers, exists := el.handlers[eventType]
	if !exists {
		return fmt.Errorf("no handlers registered for event type %s", eventType)
	}

	for i, h := range handlers {
		if h.Name() == handler.Name() {
			el.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			el.logger.Debug("Event handler unregistered", "event_type", eventType, "handler_name", handler.Name())
			return nil
		}
	}

	return fmt.Errorf("handler %s not found for event type %s", handler.Name(), eventType)
}

// EmitEvent 发送事件到事件循环
func (el *EventLoop) EmitEvent(event Event) error {
	if !el.running {
		return fmt.Errorf("event loop is not running")
	}

	select {
	case el.eventQueue <- event:
		el.logger.Debug("Event emitted", "event_type", event.Type(), "source", event.Source())
		return nil
	default:
		el.logger.Warn("Event queue is full, dropping event", "event_type", event.Type(), "source", event.Source())
		return fmt.Errorf("event queue is full")
	}
}

// GetModuleStatus 获取所有模块的状态
func (el *EventLoop) GetModuleStatus() map[string]interface{} {
	el.modulesLock.RLock()
	defer el.modulesLock.RUnlock()

	status := make(map[string]interface{})
	for name, module := range el.modules {
		status[name] = module.Status()
	}
	return status
}

// GetHandlers 获取事件处理器统计信息
func (el *EventLoop) GetHandlers() map[string]int {
	el.handlersLock.RLock()
	defer el.handlersLock.RUnlock()

	stats := make(map[string]int)
	for eventType, handlers := range el.handlers {
		stats[string(eventType)] = len(handlers)
	}
	return stats
}