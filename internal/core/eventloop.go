// Package core implements the central event loop that coordinates all motion control operations.
// The event loop provides a configurable interval-based processing system that drives
// task execution, device communication, and system state updates in a unified timing framework.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/internal/logging"
)

type EventLoop struct {
	interval    time.Duration
	modules     map[string]Module
	modulesLock sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	logger      *logging.Logger
}

func NewEventLoop(interval time.Duration) *EventLoop {
	return &EventLoop{
		interval: interval,
		modules:  make(map[string]Module),
		logger:   logging.GetLogger("event_loop"),
	}
}

func (el *EventLoop) Start(ctx context.Context) error {
	if el.running {
		return fmt.Errorf("event loop is already running")
	}

	el.ctx, el.cancel = context.WithCancel(ctx)
	el.running = true

	el.wg.Add(1)
	go el.run()

	el.logger.Info("Event loop started", "interval", el.interval)
	return nil
}

func (el *EventLoop) Stop() error {
	if !el.running {
		return fmt.Errorf("event loop is not running")
	}

	el.cancel()

	el.modulesLock.Lock()
	for name, module := range el.modules {
		el.logger.Info("Stopping module", "module_name", name)
		if err := module.Stop(); err != nil {
			el.logger.Error("Error stopping module", "module_name", name, "error", err)
		}
	}
	el.modulesLock.Unlock()

	el.wg.Wait()
	el.running = false

	el.logger.Info("Event loop stopped")
	return nil
}

func (el *EventLoop) RegisterModule(name string, module Module) error {
	el.modulesLock.Lock()
	defer el.modulesLock.Unlock()

	if _, exists := el.modules[name]; exists {
		return fmt.Errorf("module %s already registered", name)
	}

	el.modules[name] = module
	el.logger.Info("Module registered", "module_name", name)

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

	ticker := time.NewTicker(el.interval)
	defer ticker.Stop()

	for {
		select {
		case <-el.ctx.Done():
			return
		case <-ticker.C:
			el.processCycle()
		}
	}
}

func (el *EventLoop) processCycle() {
	el.modulesLock.RLock()
	defer el.modulesLock.RUnlock()

	for name, module := range el.modules {
		if err := module.Process(); err != nil {
			el.logger.Error("Error processing module", "module_name", name, "error", err)
		}
	}
}

func (el *EventLoop) GetModuleStatus() map[string]interface{} {
	el.modulesLock.RLock()
	defer el.modulesLock.RUnlock()

	status := make(map[string]interface{})
	for name, module := range el.modules {
		status[name] = module.Status()
	}
	return status
}