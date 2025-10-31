package management

import (
	"context"
	"fmt"

	"control/internal/config"
	"control/internal/hardware"
	"control/internal/ipc"
	"control/internal/logging"
	"control/pkg/types"
)

// InfrastructureManager 管理基础设施层组件
type InfrastructureManager struct {
	configManager   *config.ConfigManager
	hardwareFactory *hardware.HardwareFactory
	ipcServer       *ipc.IPCServer
	logger          *logging.Logger
	ctx             context.Context
}

// NewInfrastructureManager 创建基础设施管理器
func NewInfrastructureManager(configPath string, systemConfig types.SystemConfig) (*InfrastructureManager, error) {
	im := &InfrastructureManager{
		logger: logging.GetLogger("infrastructure"),
	}

	// 1. 创建配置管理器 (最底层，无依赖)
	im.configManager = config.NewConfigManager(configPath)
	if err := im.configManager.LoadConfig(""); err != nil {
		im.logger.Warn("Failed to load config", "error", err, "path", configPath)
		im.logger.Info("Creating default configuration...")
		if err := im.configManager.CreateDefaultConfig(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	// 2. 创建硬件工厂 (直接使用HAL)
	im.hardwareFactory = hardware.NewHardwareFactory()

	// 3. 创建IPC服务器 (依赖配置)
	im.ipcServer = ipc.NewIPCServer(systemConfig.IPC)

	return im, nil
}

// GetConfigManager 获取配置管理器
func (im *InfrastructureManager) GetConfigManager() *config.ConfigManager {
	return im.configManager
}

// GetHardwareFactory 获取硬件工厂
func (im *InfrastructureManager) GetHardwareFactory() *hardware.HardwareFactory {
	return im.hardwareFactory
}

// GetIPCServer 获取IPC服务器
func (im *InfrastructureManager) GetIPCServer() *ipc.IPCServer {
	return im.ipcServer
}

// GetSystemConfig 获取系统配置
func (im *InfrastructureManager) GetSystemConfig() types.SystemConfig {
	return im.configManager.GetConfig()
}

// Start 启动基础设施层
func (im *InfrastructureManager) Start(ctx context.Context) error {
	im.ctx = ctx
	im.logger.Info("Starting infrastructure layer")

	// 启动硬件抽象层
	if err := im.hardwareFactory.Start(); err != nil {
		im.logger.Warn("Failed to start hardware factory", "error", err)
	}

	// 启动IPC服务器
	if err := im.ipcServer.Start(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	// 启动配置监听
	if err := im.configManager.StartWatching(ctx); err != nil {
		im.logger.Warn("Failed to start config watcher", "error", err)
	}

	im.logger.Info("Infrastructure layer started successfully")
	return nil
}

// Stop 停止基础设施层
func (im *InfrastructureManager) Stop() error {
	im.logger.Info("Stopping infrastructure layer")

	// 停止顺序: IPC -> 设备 -> 配置 (与启动相反)
	var errs []error

	if im.ipcServer != nil {
		if err := im.ipcServer.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("IPC server stop error: %w", err))
		}
	}

	if im.hardwareFactory != nil {
		if err := im.hardwareFactory.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("hardware factory stop error: %w", err))
		}
	}

	if im.configManager != nil {
		if err := im.configManager.StopWatching(); err != nil {
			errs = append(errs, fmt.Errorf("config watcher stop error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("infrastructure stop errors: %v", errs)
	}

	im.logger.Info("Infrastructure layer stopped successfully")
	return nil
}

// WatchConfigChanges 监听配置变化
func (im *InfrastructureManager) WatchConfigChanges(callback func(types.SystemConfig)) {
	im.configManager.WatchChanges(func(config types.SystemConfig) {
		im.logger.Info("Configuration changed, updating infrastructure...")
		callback(config)
	})
}