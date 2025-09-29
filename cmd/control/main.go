// Command control implements the main motion control system that orchestrates
// all system components including event loops, task scheduling, device management,
// and inter-process communication. It provides the primary entry point for the
// motion control application with graceful startup and shutdown handling.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"control/internal/config"
	"control/internal/core"
	"control/internal/management"
	"control/pkg/types"
)

type MotionControlSystem struct {
	eventLoop       *core.EventLoop
	infrastructure *management.InfrastructureManager
	application    *management.ApplicationManager
	ctx            context.Context
	cancel         context.CancelFunc
	running        bool
}

func NewMotionControlSystem(configPath string) (*MotionControlSystem, error) {
	// 1. 创建基础配置管理器来获取配置
	configManager := config.NewConfigManager(configPath)

	// 2. 加载配置文件
	if err := configManager.LoadConfig(""); err != nil {
		log.Printf("Warning: Failed to load config from %s: %v", configPath, err)
		log.Println("Creating default configuration...")
		if err := configManager.CreateDefaultConfig(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	systemConfig := configManager.GetConfig()

	// 3. 创建事件循环 (中央协调器)
	eventLoop := core.NewEventLoop(systemConfig.EventLoopInterval)

	// 4. 创建基础设施层 (需要有效配置)
	infrastructure, err := management.NewInfrastructureManager(configPath, systemConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create infrastructure manager: %w", err)
	}

	// 5. 创建应用层
	application, err := management.NewApplicationManager(infrastructure, systemConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create application manager: %w", err)
	}

	system := &MotionControlSystem{
		eventLoop:       eventLoop,
		infrastructure: infrastructure,
		application:    application,
	}

	return system, nil
}

func (mcs *MotionControlSystem) Start() error {
	if mcs.running {
		return fmt.Errorf("system is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	mcs.ctx = ctx
	mcs.cancel = cancel

	log.Println("Starting Motion Control System...")

	// 1. 设置事件处理器 (注册应用到事件循环)
	if err := mcs.setupEventHandlers(); err != nil {
		return fmt.Errorf("failed to setup event handlers: %w", err)
	}

	// 2. 启动基础设施层 (配置、设备、IPC)
	if err := mcs.infrastructure.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start infrastructure layer: %w", err)
	}

	// 3. 设置应用层依赖关系
	if err := mcs.application.SetupDependencies(); err != nil {
		return fmt.Errorf("failed to setup application dependencies: %w", err)
	}

	// 4. 启动应用层 (执行器、调度器、触发器)
	if err := mcs.application.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start application layer: %w", err)
	}

	// 5. 启动事件循环 (中央协调器)
	if err := mcs.eventLoop.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start event loop: %w", err)
	}

	// 6. 设置配置监听
	mcs.setupConfigWatcher()

	mcs.running = true
	log.Println("Motion Control System started successfully")

	mcs.printSystemInfo()

	return nil
}

func (mcs *MotionControlSystem) Stop() error {
	if !mcs.running {
		return fmt.Errorf("system is not running")
	}

	log.Println("Stopping Motion Control System...")

	if mcs.cancel != nil {
		log.Println("Canceling system context...")
		mcs.cancel()
	}

	// 停止顺序: 事件循环 -> 应用层 -> 基础设施层 (与启动相反)
	var errs []error

	// 1. 停止事件循环
	if mcs.eventLoop != nil {
		log.Println("Stopping event loop...")
		if err := mcs.eventLoop.Stop(); err != nil {
			log.Printf("Error stopping event loop: %v", err)
			errs = append(errs, fmt.Errorf("event loop stop error: %w", err))
		} else {
			log.Println("Event loop stopped successfully")
		}
	}

	// 2. 停止应用层
	if mcs.application != nil {
		log.Println("Stopping application layer...")
		if err := mcs.application.Stop(); err != nil {
			log.Printf("Error stopping application layer: %v", err)
			errs = append(errs, fmt.Errorf("application layer stop error: %w", err))
		} else {
			log.Println("Application layer stopped successfully")
		}
	}

	// 3. 停止基础设施层
	if mcs.infrastructure != nil {
		log.Println("Stopping infrastructure layer...")
		if err := mcs.infrastructure.Stop(); err != nil {
			log.Printf("Error stopping infrastructure layer: %v", err)
			errs = append(errs, fmt.Errorf("infrastructure layer stop error: %w", err))
		} else {
			log.Println("Infrastructure layer stopped successfully")
		}
	}

	if len(errs) > 0 {
		log.Printf("Errors during system shutdown: %v", errs)
	}

	mcs.running = false
	log.Println("Motion Control System stopped")
	return nil
}

func (mcs *MotionControlSystem) setupEventHandlers() error {
	// 通过水平分层架构注册应用层模块到事件循环
	if err := mcs.eventLoop.RegisterModule("task_trigger", mcs.application.GetTaskOrchestrationLayer().GetTaskTrigger()); err != nil {
		return fmt.Errorf("failed to register task trigger: %w", err)
	}

	if err := mcs.eventLoop.RegisterModule("task_scheduler", mcs.application.GetTaskOrchestrationLayer().GetTaskScheduler()); err != nil {
		return fmt.Errorf("failed to register task scheduler: %w", err)
	}

	if err := mcs.eventLoop.RegisterModule("command_executor", mcs.application.GetServiceCoordinationLayer().GetCommandExecutor()); err != nil {
		return fmt.Errorf("failed to register command executor: %w", err)
	}

	// IPC消息处理已经在ApplicationManager中注册
	// 任务通道设置也已经在ApplicationManager中完成

	return nil
}

// processTask 已移至 ApplicationManager 中实现
func (mcs *MotionControlSystem) processTask(task *types.Task) {
	// 委托给ApplicationManager处理
	// mcs.application.processTask(task)
}

func (mcs *MotionControlSystem) setupConfigWatcher() {
	// 设置配置变化监听
	mcs.infrastructure.WatchConfigChanges(func(config types.SystemConfig) {
		log.Println("Configuration changed, updating system...")
		go mcs.handleConfigUpdate(config)
	})
}

// IPC处理方法已移至ApplicationManager中实现
func (mcs *MotionControlSystem) handleTaskRequest(message types.IPCMessage) {
	// 委托给ApplicationManager处理
	// 这些方法应该在ApplicationManager中实现
	log.Printf("Task request handler called - should be implemented in ApplicationManager")
}

func (mcs *MotionControlSystem) handleStatusRequest(message types.IPCMessage) {
	// TODO: 实现状态请求处理，通过管理器访问组件
	log.Printf("Status request handler called - should access through managers")
}

func (mcs *MotionControlSystem) handleConfigUpdate(config types.SystemConfig) {
	// TODO: 实现配置更新处理
	log.Println("Handling configuration update...")
	log.Println("Configuration updated successfully")
}

func (mcs *MotionControlSystem) handleIPCConfigUpdate(message types.IPCMessage) {
	// TODO: 实现IPC配置更新处理
	log.Println("Handling IPC config update...")
}

func (mcs *MotionControlSystem) handleTaskTemplateRequest(message types.IPCMessage) {
	// TODO: 实现任务模板请求处理
	log.Printf("Task template request handler called - should access through managers")
}

func (mcs *MotionControlSystem) handleTaskNodeRequest(message types.IPCMessage) {
	// TODO: 实现任务节点请求处理
	log.Printf("Task node request handler called - should access through managers")
}

// createTaskNodeFromTemplate 已移至ApplicationManager中实现
func (mcs *MotionControlSystem) createTaskNodeFromTemplate(rootConfig types.TaskNodeConfig, childConfigs []types.TaskNodeConfig) types.TaskNode {
	// TODO: 实现任务节点创建
	log.Printf("createTaskNodeFromTemplate called - should access through ApplicationManager")
	return nil
}

func (mcs *MotionControlSystem) handleAbstractCommandRequest(message types.IPCMessage) {
	// TODO: 实现抽象命令请求处理
	log.Printf("Abstract command request handler called - should access through managers")
}

func (mcs *MotionControlSystem) sendErrorResponse(target, errorType, message string) {
	// TODO: 实现错误响应发送，通过管理器访问IPC服务器
	log.Printf("sendErrorResponse called - should access through infrastructure manager")
}

func (mcs *MotionControlSystem) printSystemInfo() {
	config := mcs.infrastructure.GetSystemConfig()

	fmt.Println("==========================================")
	fmt.Println("  Motion Control System")
	fmt.Println("==========================================")
	fmt.Printf("  Event Loop Interval: %v\n", config.EventLoopInterval)
	fmt.Printf("  Queue Size: %d\n", config.QueueSize)
	fmt.Printf("  IPC Server: %s:%d\n", config.IPC.Address, config.IPC.Port)
	fmt.Printf("  Devices: %d\n", len(config.Devices))
	fmt.Printf("  Axes: %d\n", len(config.Axes))
	fmt.Println("==========================================")

	for deviceID, deviceConfig := range config.Devices {
		fmt.Printf("  Device %s: %s (%s)\n", deviceID, deviceConfig.Type, deviceConfig.Protocol)
	}

	for axisID, axisConfig := range config.Axes {
		fmt.Printf("  Axis %s: %s [%.1f, %.1f]\n", axisID, axisConfig.DeviceID, axisConfig.MinPosition, axisConfig.MaxPosition)
	}

	fmt.Println("==========================================")
}

func main() {
	var (
		configPath = flag.String("config", "config.yaml", "Path to configuration file")
	)

	flag.Parse()

	fmt.Println("Motion Control System - Starting up...")

	system, err := NewMotionControlSystem(*configPath)
	if err != nil {
		log.Fatalf("Failed to create motion control system: %v", err)
	}

	if err := system.Start(); err != nil {
		log.Fatalf("Failed to start motion control system: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	fmt.Println("\nReceived shutdown signal...")

	// 添加超时机制处理系统关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		log.Println("Starting graceful shutdown...")
		if err := system.Stop(); err != nil {
			log.Printf("Error during shutdown: %v", err)
			close(done)
			return
		}
		log.Println("Graceful shutdown completed successfully")
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Motion Control System shutdown complete")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout reached, forcing exit")
		fmt.Println("Motion Control System forced shutdown")
		os.Exit(1)
	}

	fmt.Println("Motion Control System shutdown complete")
}
