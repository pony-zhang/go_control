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
	"control/internal/device"
	"control/internal/ipc"
	"control/pkg/types"
)

type MotionControlSystem struct {
	configManager      *config.ConfigManager
	eventLoop          *core.EventLoop
	taskTrigger        *core.TaskTrigger
	taskScheduler      *core.TaskScheduler
	taskDecomposer     *core.TaskNodeDecomposer
	commandExecutor    *core.CommandExecutor
	executionQueue     *core.ExecutionQueue
	deviceManager      *device.DeviceManager
	commandMappingMgr  *core.CommandMappingManager
	ipcServer          *ipc.IPCServer
	ctx                context.Context
	cancel             context.CancelFunc
	running            bool
}

func NewMotionControlSystem(configPath string) (*MotionControlSystem, error) {
	configManager := config.NewConfigManager(configPath)

	if err := configManager.LoadConfig(""); err != nil {
		log.Printf("Warning: Failed to load config from %s: %v", configPath, err)
		log.Println("Creating default configuration...")
		if err := configManager.CreateDefaultConfig(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	systemConfig := configManager.GetConfig()

	eventLoop := core.NewEventLoop(systemConfig.EventLoopInterval)
	taskTrigger := core.NewTaskTrigger(systemConfig)
	taskScheduler := core.NewTaskScheduler(systemConfig)
	taskDecomposer := core.NewTaskNodeDecomposer(systemConfig)
	commandExecutor := core.NewCommandExecutor(systemConfig)
	deviceManager := device.NewDeviceManager(systemConfig)
	commandMappingMgr := core.NewCommandMappingManager(systemConfig, taskDecomposer)
	ipcServer := ipc.NewIPCServer(systemConfig.IPC)

	system := &MotionControlSystem{
		configManager:     configManager,
		eventLoop:         eventLoop,
		taskTrigger:       taskTrigger,
		taskScheduler:     taskScheduler,
		taskDecomposer:    taskDecomposer,
		commandExecutor:   commandExecutor,
		deviceManager:     deviceManager,
		commandMappingMgr: commandMappingMgr,
		ipcServer:         ipcServer,
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

	if err := mcs.setupEventHandlers(); err != nil {
		return fmt.Errorf("failed to setup event handlers: %w", err)
	}

	if err := mcs.deviceManager.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start device manager: %w", err)
	}

	for deviceID, device := range mcs.deviceManager.GetAllDevices() {
		if err := mcs.commandExecutor.AddDevice(device); err != nil {
			log.Printf("Failed to add device %s to executor: %v", deviceID, err)
		}
		mcs.taskDecomposer.SetDevice(device)
	}

	if err := mcs.ipcServer.Start(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	if err := mcs.commandExecutor.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start command executor: %w", err)
	}

	if err := mcs.taskScheduler.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start task scheduler: %w", err)
	}

	if err := mcs.taskTrigger.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start task trigger: %w", err)
	}

	if err := mcs.eventLoop.Start(mcs.ctx); err != nil {
		return fmt.Errorf("failed to start event loop: %w", err)
	}

	if err := mcs.configManager.StartWatching(mcs.ctx); err != nil {
		log.Printf("Warning: Failed to start config watcher: %v", err)
	}

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
		mcs.cancel()
	}

	if mcs.configManager != nil {
		if err := mcs.configManager.StopWatching(); err != nil {
			log.Printf("Error stopping config watcher: %v", err)
		}
	}

	if mcs.eventLoop != nil {
		if err := mcs.eventLoop.Stop(); err != nil {
			log.Printf("Error stopping event loop: %v", err)
		}
	}

	if mcs.taskTrigger != nil {
		if err := mcs.taskTrigger.Stop(); err != nil {
			log.Printf("Error stopping task trigger: %v", err)
		}
	}

	if mcs.taskScheduler != nil {
		if err := mcs.taskScheduler.Stop(); err != nil {
			log.Printf("Error stopping task scheduler: %v", err)
		}
	}

	if mcs.commandExecutor != nil {
		if err := mcs.commandExecutor.Stop(); err != nil {
			log.Printf("Error stopping command executor: %v", err)
		}
	}

	if mcs.ipcServer != nil {
		if err := mcs.ipcServer.Stop(); err != nil {
			log.Printf("Error stopping IPC server: %v", err)
		}
	}

	if mcs.deviceManager != nil {
		if err := mcs.deviceManager.Stop(); err != nil {
			log.Printf("Error stopping device manager: %v", err)
		}
	}

	mcs.running = false
	log.Println("Motion Control System stopped")
	return nil
}

func (mcs *MotionControlSystem) setupEventHandlers() error {
	if err := mcs.eventLoop.RegisterModule("task_trigger", mcs.taskTrigger); err != nil {
		return fmt.Errorf("failed to register task trigger: %w", err)
	}

	if err := mcs.eventLoop.RegisterModule("task_scheduler", mcs.taskScheduler); err != nil {
		return fmt.Errorf("failed to register task scheduler: %w", err)
	}

	if err := mcs.eventLoop.RegisterModule("command_executor", mcs.commandExecutor); err != nil {
		return fmt.Errorf("failed to register command executor: %w", err)
	}

	taskChan := mcs.taskTrigger.GetTaskChannel()
	go func() {
		for task := range taskChan {
			if err := mcs.taskScheduler.ScheduleTask(task); err != nil {
				log.Printf("Failed to schedule task %s: %v", task.ID, err)
			}
		}
	}()

	scheduledTaskChan := mcs.taskScheduler.GetTaskChannel()
	go func() {
		for task := range scheduledTaskChan {
			go mcs.processTask(task)
		}
	}()

	mcs.ipcServer.RegisterHandler("task_request", mcs.handleTaskRequest)
	mcs.ipcServer.RegisterHandler("task_template_request", mcs.handleTaskTemplateRequest)
	mcs.ipcServer.RegisterHandler("task_node_request", mcs.handleTaskNodeRequest)
	mcs.ipcServer.RegisterHandler("abstract_command_request", mcs.handleAbstractCommandRequest)
	mcs.ipcServer.RegisterHandler("status_request", mcs.handleStatusRequest)
	mcs.ipcServer.RegisterHandler("config_update", mcs.handleIPCConfigUpdate)

	mcs.configManager.WatchChanges(func(config types.SystemConfig) {
		log.Println("Configuration changed, updating system...")
		go mcs.handleConfigUpdate(config)
	})

	return nil
}

func (mcs *MotionControlSystem) processTask(task *types.Task) {
	commands, err := mcs.taskDecomposer.DecomposeTask(task)
	if err != nil {
		log.Printf("Failed to decompose task %s: %v", task.ID, err)
		return
	}

	for _, cmd := range commands {
		if err := mcs.commandExecutor.ExecuteCommand(mcs.ctx, cmd); err != nil {
			log.Printf("Failed to execute command %s: %v", cmd.ID, err)
		}
	}
}

func (mcs *MotionControlSystem) handleTaskRequest(message types.IPCMessage) {
	log.Printf("Received task request: %v", message)

	if taskData, ok := message.Data["task"]; ok {
		task := &types.Task{
			ID:         fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Type:       types.CommandType(taskData.(map[string]interface{})["type"].(string)),
			Priority:   types.PriorityMedium,
			Status:     types.StatusPending,
			CreatedAt:  time.Now(),
			Timeout:    30 * time.Second,
		}

		if err := mcs.taskScheduler.ScheduleTask(task); err != nil {
			log.Printf("Failed to schedule task from IPC: %v", err)
			return
		}

		response := types.IPCMessage{
			Type:      "task_response",
			Source:    "control_system",
			Target:    message.Source,
			Data:      map[string]interface{}{"task_id": task.ID, "status": "scheduled"},
			Timestamp: time.Now(),
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		}

		if err := mcs.ipcServer.SendToClient(message.Source, response); err != nil {
			log.Printf("Failed to send task response: %v", err)
		}
	}
}

func (mcs *MotionControlSystem) handleStatusRequest(message types.IPCMessage) {
	status := map[string]interface{}{
		"system": map[string]interface{}{
			"running":    mcs.running,
			"uptime":     time.Since(time.Now()).String(),
			"config":     mcs.configManager.GetConfigPath(),
		},
		"devices": mcs.deviceManager.GetDeviceStatuses(),
		"scheduler": mcs.taskScheduler.Status(),
		"executor": mcs.commandExecutor.Status(),
		"queue": mcs.commandExecutor.GetExecutionQueue().GetStatus(),
		"event_loop": mcs.eventLoop.GetModuleStatus(),
	}

	response := types.IPCMessage{
		Type:      "status_response",
		Source:    "control_system",
		Target:    message.Source,
		Data:      status,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := mcs.ipcServer.SendToClient(message.Source, response); err != nil {
		log.Printf("Failed to send status response: %v", err)
	}
}

func (mcs *MotionControlSystem) handleConfigUpdate(config types.SystemConfig) {
	log.Println("Handling configuration update...")

	mcs.taskTrigger = core.NewTaskTrigger(config)
	mcs.taskScheduler = core.NewTaskScheduler(config)
	mcs.taskDecomposer = core.NewTaskNodeDecomposer(config)

	log.Println("Configuration updated successfully")
}

func (mcs *MotionControlSystem) handleIPCConfigUpdate(message types.IPCMessage) {
	log.Println("Handling IPC config update...")

	if configData, ok := message.Data["config"]; ok {
		log.Printf("Config data received: %v", configData)
	}
}

func (mcs *MotionControlSystem) handleTaskTemplateRequest(message types.IPCMessage) {
	log.Printf("Received task template request: %v", message)

	action, ok := message.Data["action"].(string)
	if !ok {
		mcs.sendErrorResponse(message.Source, "missing_action", "Action parameter is required")
		return
	}

	switch action {
	case "list":
		templates := mcs.configManager.ListTaskTemplates()
		response := types.IPCMessage{
			Type:      "task_template_response",
			Source:    "control_system",
			Target:    message.Source,
			Data:      map[string]interface{}{"templates": templates},
			Timestamp: time.Now(),
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		}
		mcs.ipcServer.SendToClient(message.Source, response)

	case "get":
		templateName, ok := message.Data["template_name"].(string)
		if !ok {
			mcs.sendErrorResponse(message.Source, "missing_template_name", "Template name is required")
			return
		}

		template, err := mcs.configManager.GetTaskTemplate(templateName)
		if err != nil {
			mcs.sendErrorResponse(message.Source, "template_not_found", err.Error())
			return
		}

		response := types.IPCMessage{
			Type:      "task_template_response",
			Source:    "control_system",
			Target:    message.Source,
			Data:      map[string]interface{}{"template": template},
			Timestamp: time.Now(),
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		}
		mcs.ipcServer.SendToClient(message.Source, response)

	case "execute":
		templateName, ok := message.Data["template_name"].(string)
		if !ok {
			mcs.sendErrorResponse(message.Source, "missing_template_name", "Template name is required")
			return
		}

		template, err := mcs.configManager.GetTaskTemplate(templateName)
		if err != nil {
			mcs.sendErrorResponse(message.Source, "template_not_found", err.Error())
			return
		}

		// Create task from template
		task := &types.Task{
			ID:         fmt.Sprintf("template-%s-%d", templateName, time.Now().UnixNano()),
			Type:       template.Type,
			Priority:   template.Priority,
			Status:     types.StatusPending,
			Parameters: template.Parameters,
			CreatedAt:  time.Now(),
			Timeout:    template.Timeout,
		}

		// Create task node from template
		if len(template.Nodes) > 0 {
			task.RootNode = mcs.createTaskNodeFromTemplate(template.Nodes[0], template.Nodes[1:])
		}

		if err := mcs.taskScheduler.ScheduleTask(task); err != nil {
			mcs.sendErrorResponse(message.Source, "schedule_failed", err.Error())
			return
		}

		response := types.IPCMessage{
			Type:      "task_template_response",
			Source:    "control_system",
			Target:    message.Source,
			Data:      map[string]interface{}{"task_id": task.ID, "status": "scheduled"},
			Timestamp: time.Now(),
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		}
		mcs.ipcServer.SendToClient(message.Source, response)

	default:
		mcs.sendErrorResponse(message.Source, "invalid_action", fmt.Sprintf("Unknown action: %s", action))
	}
}

func (mcs *MotionControlSystem) handleTaskNodeRequest(message types.IPCMessage) {
	log.Printf("Received task node request: %v", message)

	nodeData, ok := message.Data["node"].(map[string]interface{})
	if !ok {
		mcs.sendErrorResponse(message.Source, "invalid_node_data", "Node data is required")
		return
	}

	nodeType, ok := nodeData["type"].(string)
	if !ok {
		mcs.sendErrorResponse(message.Source, "missing_node_type", "Node type is required")
		return
	}

	node, err := mcs.taskDecomposer.CreateTaskNodeFromConfig(nodeType, nodeData)
	if err != nil {
		mcs.sendErrorResponse(message.Source, "node_creation_failed", err.Error())
		return
	}

	// Create task from node
	task := &types.Task{
		ID:        fmt.Sprintf("node-%d", time.Now().UnixNano()),
		Type:      types.CommandType(nodeType),
		Priority:  types.PriorityMedium,
		Status:    types.StatusPending,
		RootNode:  node,
		CreatedAt: time.Now(),
		Timeout:   30 * time.Second,
	}

	if err := mcs.taskScheduler.ScheduleTask(task); err != nil {
		mcs.sendErrorResponse(message.Source, "schedule_failed", err.Error())
		return
	}

	response := types.IPCMessage{
		Type:      "task_node_response",
		Source:    "control_system",
		Target:    message.Source,
		Data:      map[string]interface{}{"task_id": task.ID, "status": "scheduled"},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}
	mcs.ipcServer.SendToClient(message.Source, response)
}

func (mcs *MotionControlSystem) createTaskNodeFromTemplate(rootConfig types.TaskNodeConfig, childConfigs []types.TaskNodeConfig) types.TaskNode {
	rootNode, err := mcs.taskDecomposer.CreateTaskNodeFromConfig(string(rootConfig.Type), rootConfig.Parameters)
	if err != nil {
		log.Printf("Failed to create root node: %v", err)
		return nil
	}

	for _, childConfig := range childConfigs {
		childNode, err := mcs.taskDecomposer.CreateTaskNodeFromConfig(string(childConfig.Type), childConfig.Parameters)
		if err != nil {
			log.Printf("Failed to create child node: %v", err)
			continue
		}
		rootNode.AddChild(childNode)
	}

	return rootNode
}

func (mcs *MotionControlSystem) handleAbstractCommandRequest(message types.IPCMessage) {
	log.Printf("Received abstract command request: %v", message)

	commandStr, ok := message.Data["command"].(string)
	if !ok {
		mcs.sendErrorResponse(message.Source, "missing_command", "Command parameter is required")
		return
	}

	// Handle special list command
	if commandStr == "list" {
		commands := mcs.commandMappingMgr.GetAvailableCommands()
		commandDescriptions := make(map[string]string)
		for _, cmd := range commands {
			if description, err := mcs.commandMappingMgr.GetCommandDescription(cmd); err == nil {
				commandDescriptions[string(cmd)] = description
			}
		}

		response := types.IPCMessage{
			Type:      "abstract_command_response",
			Source:    "control_system",
			Target:    message.Source,
			Data: map[string]interface{}{
				"command": "list",
				"commands": commandDescriptions,
			},
			Timestamp: time.Now(),
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		}

		if err := mcs.ipcServer.SendToClient(message.Source, response); err != nil {
			log.Printf("Failed to send command list response: %v", err)
		}
		return
	}

	abstractCmd := types.AbstractCommand(commandStr)

	// Get optional parameters
	params, _ := message.Data["parameters"].(map[string]interface{})

	// Execute abstract command through mapping manager
	task, err := mcs.commandMappingMgr.ExecuteAbstractCommand(abstractCmd, params)
	if err != nil {
		mcs.sendErrorResponse(message.Source, "command_execution_failed", err.Error())
		return
	}

	// Schedule the task
	if err := mcs.taskScheduler.ScheduleTask(task); err != nil {
		mcs.sendErrorResponse(message.Source, "schedule_failed", err.Error())
		return
	}

	response := types.IPCMessage{
		Type:      "abstract_command_response",
		Source:    "control_system",
		Target:    message.Source,
		Data: map[string]interface{}{
			"task_id": task.ID,
			"command": string(abstractCmd),
			"status":  "scheduled",
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := mcs.ipcServer.SendToClient(message.Source, response); err != nil {
		log.Printf("Failed to send abstract command response: %v", err)
	}
}

func (mcs *MotionControlSystem) sendErrorResponse(target, errorType, message string) {
	response := types.IPCMessage{
		Type:      "error_response",
		Source:    "control_system",
		Target:    target,
		Data:      map[string]interface{}{"error_type": errorType, "message": message},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}
	if err := mcs.ipcServer.SendToClient(target, response); err != nil {
		log.Printf("Failed to send error response: %v", err)
	}
}

func (mcs *MotionControlSystem) printSystemInfo() {
	config := mcs.configManager.GetConfig()

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
	if err := system.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	fmt.Println("Motion Control System shutdown complete")
}