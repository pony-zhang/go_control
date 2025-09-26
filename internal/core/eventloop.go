package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type EventLoop struct {
	interval    time.Duration
	modules     map[string]Module
	modulesLock sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
}

func NewEventLoop(interval time.Duration) *EventLoop {
	return &EventLoop{
		interval: interval,
		modules:  make(map[string]Module),
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

	log.Printf("Event loop started with interval %v", el.interval)
	return nil
}

func (el *EventLoop) Stop() error {
	if !el.running {
		return fmt.Errorf("event loop is not running")
	}

	el.cancel()

	el.modulesLock.Lock()
	for name, module := range el.modules {
		log.Printf("Stopping module: %s", name)
		if err := module.Stop(); err != nil {
			log.Printf("Error stopping module %s: %v", name, err)
		}
	}
	el.modulesLock.Unlock()

	el.wg.Wait()
	el.running = false

	log.Println("Event loop stopped")
	return nil
}

func (el *EventLoop) RegisterModule(name string, module Module) error {
	el.modulesLock.Lock()
	defer el.modulesLock.Unlock()

	if _, exists := el.modules[name]; exists {
		return fmt.Errorf("module %s already registered", name)
	}

	el.modules[name] = module
	log.Printf("Module registered: %s", name)

	if el.running {
		if err := module.Start(el.ctx); err != nil {
			log.Printf("Error starting module %s: %v", name, err)
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
			log.Printf("Error stopping module %s: %v", name, err)
		}
	}

	delete(el.modules, name)
	log.Printf("Module unregistered: %s", name)
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
			log.Printf("Error processing module %s: %v", name, err)
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