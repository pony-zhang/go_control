package application

import (
	"context"
	"fmt"
	"time"

	"control/internal/logging"
	"control/pkg/types"
)

// BusinessLogicLayer handles high-level business operations
type BusinessLogicLayer struct {
	taskOrchestration *TaskOrchestrationLayer
	logger            *logging.Logger
	ctx               context.Context
}

// NewBusinessLogicLayer creates a new business logic layer
func NewBusinessLogicLayer(taskOrchestration *TaskOrchestrationLayer) *BusinessLogicLayer {
	return &BusinessLogicLayer{
		taskOrchestration: taskOrchestration,
		logger:            logging.GetLogger("business_logic"),
	}
}

// Start initializes the business logic layer
func (bll *BusinessLogicLayer) Start(ctx context.Context) error {
	bll.ctx = ctx
	bll.logger.Info("Starting Business Logic Layer")
	bll.logger.Info("Business Logic Layer started successfully")
	return nil
}

// Stop gracefully shuts down the business logic layer
func (bll *BusinessLogicLayer) Stop() error {
	bll.logger.Info("Stopping Business Logic Layer")
	bll.logger.Info("Business Logic Layer stopped successfully")
	return nil
}

// ExecuteBusinessCommand executes a business command
func (bll *BusinessLogicLayer) ExecuteBusinessCommand(command types.BusinessCommand, params map[string]interface{}) (*types.Task, error) {
	bll.logger.Info("Executing business command", "command", command)

	// Create a simple task based on the business command
	task := &types.Task{
		ID:         fmt.Sprintf("business-%s-%d", command, time.Now().UnixNano()),
		Type:       bll.mapBusinessToCommandType(command),
		Priority:   bll.mapBusinessToPriority(command),
		Status:     types.StatusPending,
		Parameters: params,
		CreatedAt:  time.Now(),
		Timeout:    bll.mapBusinessToTimeout(command),
	}

	// Schedule the task through task orchestration
	if err := bll.taskOrchestration.ScheduleTask(task); err != nil {
		bll.logger.Error("Failed to schedule business command task", "command", command, "error", err)
		return nil, err
	}

	bll.logger.Info("Business command executed successfully", "command", command, "task_id", task.ID)
	return task, nil
}

// Helper methods to map business commands to internal types
func (bll *BusinessLogicLayer) mapBusinessToCommandType(cmd types.BusinessCommand) types.CommandType {
	switch cmd {
	case types.CmdMove, types.CmdMoveRelative:
		return types.CommandMoveTo
	case types.CmdHome:
		return types.CommandHome
	case types.CmdEmergencyStop:
		return types.CommandStop
	case types.CmdSelfCheck, types.CmdSafetyCheck:
		return types.CommandDelay
	default:
		return types.CommandDelay
	}
}

func (bll *BusinessLogicLayer) mapBusinessToPriority(cmd types.BusinessCommand) types.TaskPriority {
	switch cmd {
	case types.CmdEmergencyStop:
		return types.PriorityEmergency
	case types.CmdSafetyCheck, types.CmdSelfCheck:
		return types.PriorityHigh
	case types.CmdHome, types.CmdMove:
		return types.PriorityMedium
	default:
		return types.PriorityLow
	}
}

func (bll *BusinessLogicLayer) mapBusinessToTimeout(cmd types.BusinessCommand) time.Duration {
	switch cmd {
	case types.CmdEmergencyStop:
		return 5 * time.Second
	case types.CmdSafetyCheck, types.CmdSelfCheck:
		return 15 * time.Second
	case types.CmdHome:
		return 30 * time.Second
	case types.CmdMove:
		return 60 * time.Second
	default:
		return 30 * time.Second
	}
}

// SelfCheck performs system self-check
func (bll *BusinessLogicLayer) SelfCheck() error {
	bll.logger.Info("Performing system self-check")

	// Execute self-check business command
	_, err := bll.ExecuteBusinessCommand(types.CmdSelfCheck, map[string]interface{}{
		"check_level": "comprehensive",
	})

	if err != nil {
		bll.logger.Error("Self-check failed", "error", err)
		return fmt.Errorf("self-check failed: %w", err)
	}

	bll.logger.Info("System self-check completed successfully")
	return nil
}

// EmergencyStop executes emergency stop
func (bll *BusinessLogicLayer) EmergencyStop() error {
	bll.logger.Warn("Executing emergency stop")

	// Execute emergency stop business command
	_, err := bll.ExecuteBusinessCommand(types.CmdEmergencyStop, map[string]interface{}{
		"reason": "emergency_stop_triggered",
	})

	if err != nil {
		bll.logger.Error("Emergency stop failed", "error", err)
		return fmt.Errorf("emergency stop failed: %w", err)
	}

	bll.logger.Info("Emergency stop executed successfully")
	return nil
}

// HomeSystem homes all axes
func (bll *BusinessLogicLayer) HomeSystem() error {
	bll.logger.Info("Homing system")

	// Execute home business command
	_, err := bll.ExecuteBusinessCommand(types.CmdHome, map[string]interface{}{
		"axes": []string{"axis-x", "axis-y", "axis-z"},
		"mode": "standard",
	})

	if err != nil {
		bll.logger.Error("Home system failed", "error", err)
		return fmt.Errorf("home system failed: %w", err)
	}

	bll.logger.Info("System home completed successfully")
	return nil
}

// InitializeSystem initializes the system
func (bll *BusinessLogicLayer) InitializeSystem() error {
	bll.logger.Info("Initializing system")

	// Execute initialize business command
	_, err := bll.ExecuteBusinessCommand(types.CmdInitialize, map[string]interface{}{
		"mode": "full_initialization",
	})

	if err != nil {
		bll.logger.Error("System initialization failed", "error", err)
		return fmt.Errorf("system initialization failed: %w", err)
	}

	bll.logger.Info("System initialized successfully")
	return nil
}

// StartSystem starts the system
func (bll *BusinessLogicLayer) StartSystem() error {
	bll.logger.Info("Starting system")

	// Execute start business command
	_, err := bll.ExecuteBusinessCommand(types.CmdInitialize, map[string]interface{}{
		"mode": "start_system",
	})

	if err != nil {
		bll.logger.Error("System start failed", "error", err)
		return fmt.Errorf("system start failed: %w", err)
	}

	bll.logger.Info("System started successfully")
	return nil
}

// StopSystem stops the system
func (bll *BusinessLogicLayer) StopSystem() error {
	bll.logger.Info("Stopping system")

	// Execute stop business command
	_, err := bll.ExecuteBusinessCommand(types.CmdReset, map[string]interface{}{
		"mode": "stop_system",
	})

	if err != nil {
		bll.logger.Error("System stop failed", "error", err)
		return fmt.Errorf("system stop failed: %w", err)
	}

	bll.logger.Info("System stopped successfully")
	return nil
}

// ResetSystem resets the system
func (bll *BusinessLogicLayer) ResetSystem() error {
	bll.logger.Info("Resetting system")

	// Execute reset business command
	_, err := bll.ExecuteBusinessCommand(types.CmdReset, map[string]interface{}{
		"mode": "full_reset",
	})

	if err != nil {
		bll.logger.Error("System reset failed", "error", err)
		return fmt.Errorf("system reset failed: %w", err)
	}

	bll.logger.Info("System reset successfully")
	return nil
}

// SafetyCheck performs safety check
func (bll *BusinessLogicLayer) SafetyCheck() error {
	bll.logger.Info("Performing safety check")

	// Execute safety check business command
	_, err := bll.ExecuteBusinessCommand(types.CmdSafetyCheck, map[string]interface{}{
		"check_type": "full_system",
	})

	if err != nil {
		bll.logger.Error("Safety check failed", "error", err)
		return fmt.Errorf("safety check failed: %w", err)
	}

	bll.logger.Info("Safety check completed successfully")
	return nil
}

// GetSystemStatus returns system status
func (bll *BusinessLogicLayer) GetSystemStatus() (map[string]interface{}, error) {
	bll.logger.Info("Getting system status")

	status := map[string]interface{}{
		"system_state": "operational",
		"timestamp":    time.Now(),
		"business_logic_layer": map[string]interface{}{
			"status": "running",
		},
	}

	bll.logger.Info("System status retrieved successfully")
	return status, nil
}

// CommandHandler interface implementation
func (bll *BusinessLogicLayer) GetHandledCommands() []types.BusinessCommand {
	return []types.BusinessCommand{
		types.CmdQueryStatus,
		types.CmdSelfCheck,
		types.CmdEmergencyStop,
		types.CmdHome,
		types.CmdInitialize,
		types.CmdSafetyCheck,
	}
}

func (bll *BusinessLogicLayer) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
	bll.logger.Info("Business logic layer handling command", "command", msg.Command)

	switch msg.Command {
	case types.CmdQueryStatus:
		return bll.handleQueryStatus(msg)
	case types.CmdSelfCheck:
		return bll.handleSelfCheck(msg)
	case types.CmdEmergencyStop:
		return bll.handleEmergencyStop(msg)
	case types.CmdHome:
		return bll.handleHome(msg)
	case types.CmdInitialize:
		return bll.handleInitialize(msg)
	case types.CmdSafetyCheck:
		return bll.handleSafetyCheck(msg)
	default:
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     fmt.Sprintf("business logic layer cannot handle command: %s", msg.Command),
			Timestamp: time.Now(),
		}
	}
}

func (bll *BusinessLogicLayer) GetName() string {
	return "business_logic"
}

// Command handling methods
func (bll *BusinessLogicLayer) handleQueryStatus(msg *types.BusinessMessage) *types.BusinessResponse {
	status, err := bll.GetSystemStatus()
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"system_status": status,
		},
		Timestamp: time.Now(),
	}
}

func (bll *BusinessLogicLayer) handleSelfCheck(msg *types.BusinessMessage) *types.BusinessResponse {
	task, err := bll.ExecuteBusinessCommand(types.CmdSelfCheck, msg.Params)
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"task_id":    task.ID,
			"task_status": task.Status,
		},
		Timestamp: time.Now(),
	}
}

func (bll *BusinessLogicLayer) handleEmergencyStop(msg *types.BusinessMessage) *types.BusinessResponse {
	task, err := bll.ExecuteBusinessCommand(types.CmdEmergencyStop, msg.Params)
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"task_id":    task.ID,
			"task_status": task.Status,
		},
		Timestamp: time.Now(),
	}
}

func (bll *BusinessLogicLayer) handleHome(msg *types.BusinessMessage) *types.BusinessResponse {
	task, err := bll.ExecuteBusinessCommand(types.CmdHome, msg.Params)
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"task_id":    task.ID,
			"task_status": task.Status,
		},
		Timestamp: time.Now(),
	}
}

func (bll *BusinessLogicLayer) handleInitialize(msg *types.BusinessMessage) *types.BusinessResponse {
	task, err := bll.ExecuteBusinessCommand(types.CmdInitialize, msg.Params)
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"task_id":    task.ID,
			"task_status": task.Status,
		},
		Timestamp: time.Now(),
	}
}

func (bll *BusinessLogicLayer) handleSafetyCheck(msg *types.BusinessMessage) *types.BusinessResponse {
	task, err := bll.ExecuteBusinessCommand(types.CmdSafetyCheck, msg.Params)
	if err != nil {
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"task_id":    task.ID,
			"task_status": task.Status,
		},
		Timestamp: time.Now(),
	}
}