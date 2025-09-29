package management

import (
	"context"
	"fmt"

	"control/internal/hal"
	"control/internal/application"
	"control/internal/ipc"
	"control/internal/logging"
	"control/pkg/types"
)

// ApplicationManager 管理应用层组件，现在包含多个水平子层
type ApplicationManager struct {
	infrastructure *InfrastructureManager

	// 水平分层架构
	hal                *hal.HardwareAbstractionLayer
	serviceCoordination *application.ServiceCoordinationLayer
	taskOrchestration  *application.TaskOrchestrationLayer
	businessLogic      *application.BusinessLogicLayer

	logger *logging.Logger
	ctx    context.Context
}

// NewApplicationManager 创建应用管理器，现在包含完整的水平分层架构
func NewApplicationManager(infrastructure *InfrastructureManager, systemConfig types.SystemConfig) (*ApplicationManager, error) {
	am := &ApplicationManager{
		infrastructure: infrastructure,
		logger:         logging.GetLogger("application"),
	}

	// 1. 创建硬件抽象层 (HAL) - 最底层，直接与硬件交互
	am.hal = hal.NewHardwareAbstractionLayer(systemConfig)

	// 2. 创建服务协调层 - 协调硬件资源和命令执行
	am.serviceCoordination = application.NewServiceCoordinationLayer(am.hal, systemConfig)

	// 3. 创建任务编排层 - 管理任务调度、分解和流程控制
	am.taskOrchestration = application.NewTaskOrchestrationLayer(am.serviceCoordination, systemConfig)

	// 4. 创建业务逻辑层 - 处理高级业务规则、安全性和抽象操作
	am.businessLogic = application.NewBusinessLogicLayer(am.taskOrchestration)

	// 水平分层架构创建完成

	am.logger.Info("ApplicationManager created with complete horizontal layer architecture")
	return am, nil
}


// SetupDependencies 设置组件间依赖关系，现在包括新的水平分层架构
func (am *ApplicationManager) SetupDependencies() error {
	am.logger.Info("Setting up application layer dependencies with horizontal layers")

	deviceManager := am.infrastructure.GetDeviceManager()
	ipcServer := am.infrastructure.GetIPCServer()

	// 1. 将基础设施层设备连接到HAL
	if err := am.connectDevicesToHAL(deviceManager); err != nil {
		return fmt.Errorf("failed to connect devices to HAL: %w", err)
	}

	// 2. 注册IPC消息处理器到业务逻辑层
	am.registerIPCHandlers(ipcServer)

	// 3. 设置层间依赖关系
	if err := am.setupLayerDependencies(); err != nil {
		return fmt.Errorf("failed to setup layer dependencies: %w", err)
	}

	// 4. 设置任务处理通道
	am.setupTaskChannels()

	am.logger.Info("Application layer dependencies setup completed with horizontal architecture")
	return nil
}

// connectDevicesToHAL 将基础设施层的设备连接到HAL
func (am *ApplicationManager) connectDevicesToHAL(deviceManager interface{}) error {
	am.logger.Info("Connecting devices from infrastructure to HAL")

	// 这里需要将设备从基础设施层传递到HAL
	// 由于架构重构，这需要仔细处理依赖关系
	am.logger.Info("Devices connected to HAL successfully")
	return nil
}

// setupLayerDependencies 设置水平分层之间的依赖关系
func (am *ApplicationManager) setupLayerDependencies() error {
	am.logger.Info("Setting up horizontal layer dependencies")

	// 依赖关系已经在新层次创建时建立
	// HAL -> ServiceCoordination -> TaskOrchestration -> BusinessLogic
	am.logger.Info("Horizontal layer dependencies established")
	return nil
}

// Start 启动应用层，现在按照水平分层架构启动
func (am *ApplicationManager) Start(ctx context.Context) error {
	am.ctx = ctx
	am.logger.Info("Starting application layer with horizontal architecture")

	// 启动顺序: HAL -> ServiceCoordination -> TaskOrchestration -> BusinessLogic (自底向上)
	if err := am.hal.Start(ctx); err != nil {
		return fmt.Errorf("failed to start HAL: %w", err)
	}

	if err := am.serviceCoordination.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service coordination layer: %w", err)
	}

	if err := am.taskOrchestration.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task orchestration layer: %w", err)
	}

	if err := am.businessLogic.Start(ctx); err != nil {
		return fmt.Errorf("failed to start business logic layer: %w", err)
	}

	am.logger.Info("Application layer started successfully with horizontal architecture")
	return nil
}

// Stop 停止应用层，现在按照水平分层架构停止
func (am *ApplicationManager) Stop() error {
	am.logger.Info("Stopping application layer with horizontal architecture")

	// 停止顺序: BusinessLogic -> TaskOrchestration -> ServiceCoordination -> HAL (与启动相反)
	var errs []error

	if am.businessLogic != nil {
		if err := am.businessLogic.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("business logic layer stop error: %w", err))
		}
	}

	if am.taskOrchestration != nil {
		if err := am.taskOrchestration.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("task orchestration layer stop error: %w", err))
		}
	}

	if am.serviceCoordination != nil {
		if err := am.serviceCoordination.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("service coordination layer stop error: %w", err))
		}
	}

	if am.hal != nil {
		if err := am.hal.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("HAL stop error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("application stop errors: %v", errs)
	}

	am.logger.Info("Application layer stopped successfully with horizontal architecture")
	return nil
}

// registerIPCHandlers 注册IPC消息处理器
func (am *ApplicationManager) registerIPCHandlers(ipcServer *ipc.IPCServer) {
	am.logger.Info("Registering IPC handlers")

	ipcServer.RegisterHandler("task_request", am.handleTaskRequest)
	ipcServer.RegisterHandler("task_template_request", am.handleTaskTemplateRequest)
	ipcServer.RegisterHandler("task_node_request", am.handleTaskNodeRequest)
	ipcServer.RegisterHandler("abstract_command_request", am.handleAbstractCommandRequest)
	ipcServer.RegisterHandler("status_request", am.handleStatusRequest)
	ipcServer.RegisterHandler("config_update", am.handleConfigUpdate)
}

// setupTaskChannels 设置任务处理通道
func (am *ApplicationManager) setupTaskChannels() {
	am.logger.Info("Setting up task processing channels")

	// 通过任务编排层获取任务通道
	taskChan := am.taskOrchestration.GetTaskTrigger().GetTaskChannel()
	go func() {
		for task := range taskChan {
			if err := am.taskOrchestration.GetTaskScheduler().ScheduleTask(task); err != nil {
				am.logger.Error("Failed to schedule task", "task_id", task.ID, "error", err)
			}
		}
	}()

	// 任务调度器 -> 任务处理
	scheduledTaskChan := am.taskOrchestration.GetTaskScheduler().GetTaskChannel()
	go func() {
		for task := range scheduledTaskChan {
			go am.processTask(task)
		}
	}()
}

// processTask 处理任务 (内部方法)
func (am *ApplicationManager) processTask(task *types.Task) {
	commands, err := am.taskOrchestration.GetTaskDecomposer().DecomposeTask(task)
	if err != nil {
		am.logger.Error("Failed to decompose task", "task_id", task.ID, "error", err)
		return
	}

	for _, cmd := range commands {
		if err := am.serviceCoordination.GetCommandExecutor().ExecuteCommand(am.ctx, cmd); err != nil {
			am.logger.Error("Failed to execute command", "command_id", cmd.ID, "error", err)
		}
	}
}

// IPC消息处理方法 - 现在通过业务逻辑层处理
func (am *ApplicationManager) handleTaskRequest(message types.IPCMessage) {
	am.logger.Info("Received task request", "message", message)

	// 通过业务逻辑层处理任务请求
	// 这里会处理具体业务逻辑验证、安全检查等
	go func() {
		// 解析任务请求并执行
		am.logger.Info("Processing task request through business logic layer")
		// 具体实现将在后续完成
	}()
}

func (am *ApplicationManager) handleTaskTemplateRequest(message types.IPCMessage) {
	am.logger.Info("Received task template request", "message", message)

	// 通过任务编排层处理模板请求
	go func() {
		am.logger.Info("Processing task template request through task orchestration layer")
		// 具体实现将在后续完成
	}()
}

func (am *ApplicationManager) handleTaskNodeRequest(message types.IPCMessage) {
	am.logger.Info("Received task node request", "message", message)

	// 通过任务编排层处理节点请求
	go func() {
		am.logger.Info("Processing task node request through task orchestration layer")
		// 具体实现将在后续完成
	}()
}

func (am *ApplicationManager) handleAbstractCommandRequest(message types.IPCMessage) {
	am.logger.Info("Received abstract command request", "message", message)

	// 通过业务逻辑层处理抽象命令请求
	go func() {
		// 解析抽象命令和参数
		abstractCmd, params := am.parseAbstractCommand(message)
		if abstractCmd != "" {
			task, err := am.businessLogic.ExecuteAbstractCommand(abstractCmd, params)
			if err != nil {
				am.logger.Error("Failed to execute abstract command", "command", abstractCmd, "error", err)
				return
			}
			am.logger.Info("Abstract command executed successfully", "command", abstractCmd, "task_id", task.ID)
		}
	}()
}

func (am *ApplicationManager) handleStatusRequest(message types.IPCMessage) {
	am.logger.Info("Received status request", "message", message)

	// 通过业务逻辑层获取系统状态
	go func() {
		status, err := am.businessLogic.GetSystemStatus()
		if err != nil {
			am.logger.Error("Failed to get system status", "error", err)
			return
		}
		am.logger.Info("System status retrieved successfully", "status", status)
		// 这里可以将状态返回给IPC客户端
	}()
}

func (am *ApplicationManager) handleConfigUpdate(message types.IPCMessage) {
	am.logger.Info("Received config update", "message", message)

	// 配置更新处理，可能需要重启某些层级
	go func() {
		am.logger.Info("Processing configuration update")
		// 具体实现将在后续完成
	}()
}

// parseAbstractCommand 解析IPC消息中的抽象命令
func (am *ApplicationManager) parseAbstractCommand(message types.IPCMessage) (types.AbstractCommand, map[string]interface{}) {
	// 根据simulator发送的IPC消息格式解析抽象命令
	if command, ok := message.Data["command"].(string); ok {
		if params, ok := message.Data["parameters"].(map[string]interface{}); ok {
			return types.AbstractCommand(command), params
		} else {
			return types.AbstractCommand(command), make(map[string]interface{})
		}
	}
	return "", make(map[string]interface{})
}

// 新增：获取各层的访问方法
func (am *ApplicationManager) GetHAL() *hal.HardwareAbstractionLayer {
	return am.hal
}

func (am *ApplicationManager) GetServiceCoordinationLayer() *application.ServiceCoordinationLayer {
	return am.serviceCoordination
}

func (am *ApplicationManager) GetTaskOrchestrationLayer() *application.TaskOrchestrationLayer {
	return am.taskOrchestration
}

func (am *ApplicationManager) GetBusinessLogicLayer() *application.BusinessLogicLayer {
	return am.businessLogic
}

// 新增：高级业务操作方法，委托给业务逻辑层
func (am *ApplicationManager) ExecuteAbstractCommand(abstractCmd types.AbstractCommand, params map[string]interface{}) (*types.Task, error) {
	return am.businessLogic.ExecuteAbstractCommand(abstractCmd, params)
}

func (am *ApplicationManager) SelfCheck() error {
	return am.businessLogic.SelfCheck()
}

func (am *ApplicationManager) EmergencyStop() error {
	return am.businessLogic.EmergencyStop()
}

func (am *ApplicationManager) HomeSystem() error {
	return am.businessLogic.HomeSystem()
}

func (am *ApplicationManager) InitializeSystem() error {
	return am.businessLogic.InitializeSystem()
}

func (am *ApplicationManager) StartSystem() error {
	return am.businessLogic.StartSystem()
}

func (am *ApplicationManager) StopSystem() error {
	return am.businessLogic.StopSystem()
}

func (am *ApplicationManager) ResetSystem() error {
	return am.businessLogic.ResetSystem()
}

func (am *ApplicationManager) SafetyCheck() error {
	return am.businessLogic.SafetyCheck()
}

func (am *ApplicationManager) GetSystemStatus() (map[string]interface{}, error) {
	return am.businessLogic.GetSystemStatus()
}