package application

import (
	"context"
	"fmt"

	"control/pkg/types"
	"control/internal/logging"
)

// BusinessLogicLayer handles high-level business rules, safety, and abstract operations
type BusinessLogicLayer struct {
	taskOrchestration *TaskOrchestrationLayer
	safetyManager     *SafetyManager
	validationManager *ValidationManager
	businessRules     *BusinessRules
	logger            *logging.Logger
	ctx               context.Context
}

// SafetyManager handles safety-related operations
type SafetyManager struct {
	logger *logging.Logger
}

// ValidationManager handles input validation and business rule validation
type ValidationManager struct {
	logger *logging.Logger
}

// BusinessRules contains business logic rules and constraints
type BusinessRules struct {
	logger *logging.Logger
}

// NewBusinessLogicLayer creates a new business logic layer
func NewBusinessLogicLayer(taskOrchestration *TaskOrchestrationLayer) *BusinessLogicLayer {
	return &BusinessLogicLayer{
		taskOrchestration: taskOrchestration,
		safetyManager:     &SafetyManager{logger: logging.GetLogger("safety_manager")},
		validationManager: &ValidationManager{logger: logging.GetLogger("validation_manager")},
		businessRules:     &BusinessRules{logger: logging.GetLogger("business_rules")},
		logger:            logging.GetLogger("business_logic"),
	}
}

// Start initializes the business logic layer
func (bll *BusinessLogicLayer) Start(ctx context.Context) error {
	bll.ctx = ctx
	bll.logger.Info("Starting Business Logic Layer")

	// Initialize safety checks
	if err := bll.initializeSafety(); err != nil {
		return fmt.Errorf("failed to initialize safety: %w", err)
	}

	// Initialize business rules
	if err := bll.initializeBusinessRules(); err != nil {
		return fmt.Errorf("failed to initialize business rules: %w", err)
	}

	bll.logger.Info("Business Logic Layer started successfully")
	return nil
}

// Stop gracefully shuts down the business logic layer
func (bll *BusinessLogicLayer) Stop() error {
	bll.logger.Info("Stopping Business Logic Layer")

	// Perform safety cleanup
	bll.cleanupSafety()

	bll.logger.Info("Business Logic Layer stopped successfully")
	return nil
}

// initializeSafety initializes safety systems
func (bll *BusinessLogicLayer) initializeSafety() error {
	bll.logger.Info("Initializing safety systems")
	// Safety system initialization would go here
	return nil
}

// initializeBusinessRules initializes business rules
func (bll *BusinessLogicLayer) initializeBusinessRules() error {
	bll.logger.Info("Initializing business rules")
	// Business rules initialization would go here
	return nil
}

// cleanupSafety performs safety cleanup
func (bll *BusinessLogicLayer) cleanupSafety() {
	bll.logger.Info("Performing safety cleanup")
}

// ExecuteAbstractCommand executes an abstract command with business logic validation
func (bll *BusinessLogicLayer) ExecuteAbstractCommand(abstractCmd types.AbstractCommand, params map[string]interface{}) (*types.Task, error) {
	bll.logger.Info("Executing abstract command with business logic", "command", abstractCmd)

	// Validate input parameters
	if err := bll.validationManager.ValidateCommand(abstractCmd, params); err != nil {
		bll.logger.Error("Command validation failed", "command", abstractCmd, "error", err)
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Apply business rules
	if err := bll.businessRules.ApplyRules(abstractCmd, params); err != nil {
		bll.logger.Error("Business rules validation failed", "command", abstractCmd, "error", err)
		return nil, fmt.Errorf("business rules validation failed: %w", err)
	}

	// Perform safety checks
	if err := bll.safetyManager.CheckCommandSafety(abstractCmd, params); err != nil {
		bll.logger.Error("Safety check failed", "command", abstractCmd, "error", err)
		return nil, fmt.Errorf("safety check failed: %w", err)
	}

	// Execute through task orchestration
	task, err := bll.taskOrchestration.ExecuteAbstractCommand(abstractCmd, params)
	if err != nil {
		bll.logger.Error("Abstract command execution failed", "command", abstractCmd, "error", err)
		return nil, err
	}

	bll.logger.Info("Abstract command executed successfully with business logic", "command", abstractCmd, "task_id", task.ID)
	return task, nil
}

// SelfCheck performs system self-check
func (bll *BusinessLogicLayer) SelfCheck() error {
	bll.logger.Info("Performing system self-check")

	// Execute self-check abstract command
	_, err := bll.ExecuteAbstractCommand(types.AbstractSelfCheck, map[string]interface{}{
		"comprehensive": true,
		"timeout":      30,
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

	// Emergency stop has highest priority and bypasses normal validation
	task, err := bll.taskOrchestration.ExecuteAbstractCommand(types.AbstractEmergencyStop, map[string]interface{}{
		"immediate": true,
		"reason":    "emergency_stop_triggered",
	})

	if err != nil {
		bll.logger.Error("Emergency stop execution failed", "error", err)
		return fmt.Errorf("emergency stop failed: %w", err)
	}

	bll.logger.Info("Emergency stop executed successfully", "task_id", task.ID)
	return nil
}

// HomeSystem executes system homing sequence
func (bll *BusinessLogicLayer) HomeSystem() error {
	bll.logger.Info("Executing system homing sequence")

	task, err := bll.ExecuteAbstractCommand(types.AbstractHome, map[string]interface{}{
		"all_axes": true,
		"timeout":  300,
	})

	if err != nil {
		bll.logger.Error("System homing failed", "error", err)
		return fmt.Errorf("system homing failed: %w", err)
	}

	bll.logger.Info("System homing completed successfully", "task_id", task.ID)
	return nil
}

// InitializeSystem executes system initialization
func (bll *BusinessLogicLayer) InitializeSystem() error {
	bll.logger.Info("Executing system initialization")

	task, err := bll.ExecuteAbstractCommand(types.AbstractInitialize, map[string]interface{}{
		"include_self_check": true,
		"include_homing":     true,
		"timeout":           600,
	})

	if err != nil {
		bll.logger.Error("System initialization failed", "error", err)
		return fmt.Errorf("system initialization failed: %w", err)
	}

	bll.logger.Info("System initialization completed successfully", "task_id", task.ID)
	return nil
}

// StartSystem starts the system
func (bll *BusinessLogicLayer) StartSystem() error {
	bll.logger.Info("Starting system")

	task, err := bll.ExecuteAbstractCommand(types.AbstractStart, map[string]interface{}{
		"gradual_start": true,
		"safety_check":  true,
	})

	if err != nil {
		bll.logger.Error("System start failed", "error", err)
		return fmt.Errorf("system start failed: %w", err)
	}

	bll.logger.Info("System started successfully", "task_id", task.ID)
	return nil
}

// StopSystem stops the system
func (bll *BusinessLogicLayer) StopSystem() error {
	bll.logger.Info("Stopping system")

	task, err := bll.ExecuteAbstractCommand(types.AbstractStop, map[string]interface{}{
		"graceful": true,
		"timeout":  60,
	})

	if err != nil {
		bll.logger.Error("System stop failed", "error", err)
		return fmt.Errorf("system stop failed: %w", err)
	}

	bll.logger.Info("System stopped successfully", "task_id", task.ID)
	return nil
}

// ResetSystem resets the system
func (bll *BusinessLogicLayer) ResetSystem() error {
	bll.logger.Info("Resetting system")

	task, err := bll.ExecuteAbstractCommand(types.AbstractReset, map[string]interface{}{
		"full_reset": true,
		"clear_errors": true,
	})

	if err != nil {
		bll.logger.Error("System reset failed", "error", err)
		return fmt.Errorf("system reset failed: %w", err)
	}

	bll.logger.Info("System reset successfully", "task_id", task.ID)
	return nil
}

// SafetyCheck performs safety check
func (bll *BusinessLogicLayer) SafetyCheck() error {
	bll.logger.Info("Performing safety check")

	task, err := bll.ExecuteAbstractCommand(types.AbstractSafetyCheck, map[string]interface{}{
		"comprehensive": true,
		"timeout":      30,
	})

	if err != nil {
		bll.logger.Error("Safety check failed", "error", err)
		return fmt.Errorf("safety check failed: %w", err)
	}

	bll.logger.Info("Safety check completed successfully", "task_id", task.ID)
	return nil
}

// GetSystemStatus returns overall system status
func (bll *BusinessLogicLayer) GetSystemStatus() (map[string]interface{}, error) {
	bll.logger.Info("Getting system status")

	// Get device statuses from task orchestration
	deviceStatuses := bll.taskOrchestration.GetServiceCoordination().GetAllDeviceStatuses()

	// Get task status
	// This would be enhanced to get actual task statistics

	status := map[string]interface{}{
		"devices": deviceStatuses,
		"safety": map[string]interface{}{
			"status": "normal",
			"last_check": "recent",
		},
		"system": map[string]interface{}{
			"state": "operational",
			"mode": "automatic",
		},
	}

	bll.logger.Info("System status retrieved successfully")
	return status, nil
}

// ValidateCommand validates a command with business rules
func (bll *BusinessLogicLayer) ValidateCommand(abstractCmd types.AbstractCommand, params map[string]interface{}) error {
	return bll.validationManager.ValidateCommand(abstractCmd, params)
}

// CheckCommandSafety checks command safety
func (bll *BusinessLogicLayer) CheckCommandSafety(abstractCmd types.AbstractCommand, params map[string]interface{}) error {
	return bll.safetyManager.CheckCommandSafety(abstractCmd, params)
}

// SafetyManager methods
func (sm *SafetyManager) CheckCommandSafety(abstractCmd types.AbstractCommand, params map[string]interface{}) error {
	sm.logger.Info("Checking command safety", "command", abstractCmd)

	// Basic safety checks
	switch abstractCmd {
	case types.AbstractEmergencyStop:
		// Emergency stop is always allowed
		return nil
	case types.AbstractStart:
		// Check if system is safe to start
		return sm.checkSystemSafetyForStart()
	default:
		// Default safety check
		return sm.checkGeneralSafety()
	}
}

func (sm *SafetyManager) checkSystemSafetyForStart() error {
	// Implement safety checks for starting the system
	sm.logger.Info("Checking system safety for start")
	return nil
}

func (sm *SafetyManager) checkGeneralSafety() error {
	// Implement general safety checks
	sm.logger.Info("Checking general safety")
	return nil
}

// ValidationManager methods
func (vm *ValidationManager) ValidateCommand(abstractCmd types.AbstractCommand, params map[string]interface{}) error {
	vm.logger.Info("Validating command", "command", abstractCmd)

	// Basic parameter validation
	if params == nil {
		return fmt.Errorf("parameters cannot be nil")
	}

	// Command-specific validation
	switch abstractCmd {
	case types.AbstractEmergencyStop:
		return vm.validateEmergencyStopParams(params)
	case types.AbstractStart:
		return vm.validateStartParams(params)
	default:
		return vm.validateGeneralParams(params)
	}
}

func (vm *ValidationManager) validateEmergencyStopParams(params map[string]interface{}) error {
	// Emergency stop has minimal parameter requirements
	return nil
}

func (vm *ValidationManager) validateStartParams(params map[string]interface{}) error {
	// Validate start parameters
	return nil
}

func (vm *ValidationManager) validateGeneralParams(params map[string]interface{}) error {
	// General parameter validation
	return nil
}

// BusinessRules methods
func (br *BusinessRules) ApplyRules(abstractCmd types.AbstractCommand, params map[string]interface{}) error {
	br.logger.Info("Applying business rules", "command", abstractCmd)

	// Apply business rules based on command type
	switch abstractCmd {
	case types.AbstractStart:
		return br.applyStartRules(params)
	case types.AbstractStop:
		return br.applyStopRules(params)
	default:
		return br.applyGeneralRules(params)
	}
}

func (br *BusinessRules) applyStartRules(params map[string]interface{}) error {
	br.logger.Info("Applying start rules")
	// Implement start-specific business rules
	return nil
}

func (br *BusinessRules) applyStopRules(params map[string]interface{}) error {
	br.logger.Info("Applying stop rules")
	// Implement stop-specific business rules
	return nil
}

func (br *BusinessRules) applyGeneralRules(params map[string]interface{}) error {
	br.logger.Info("Applying general rules")
	// Implement general business rules
	return nil
}