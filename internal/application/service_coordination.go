package application

import (
	"context"
	"fmt"

	"control/pkg/types"
	"control/internal/hal"
	"control/internal/core"
	"control/internal/logging"
)

// ServiceCoordinationLayer coordinates between HAL and upper layers, managing device operations
type ServiceCoordinationLayer struct {
	hal              *hal.HardwareAbstractionLayer
	commandExecutor  *core.CommandExecutor
	executionQueue   *core.ExecutionQueue
	logger           *logging.Logger
	ctx              context.Context
}

// NewServiceCoordinationLayer creates a new service coordination layer
func NewServiceCoordinationLayer(hal *hal.HardwareAbstractionLayer, config types.SystemConfig) *ServiceCoordinationLayer {
	return &ServiceCoordinationLayer{
		hal:             hal,
		commandExecutor: core.NewCommandExecutor(config),
		executionQueue:  core.NewExecutionQueue(config.QueueSize),
		logger:          logging.GetLogger("service_coordination"),
	}
}

// Start initializes the service coordination layer
func (scl *ServiceCoordinationLayer) Start(ctx context.Context) error {
	scl.ctx = ctx
	scl.logger.Info("Starting Service Coordination Layer")

	// Start execution queue
	if err := scl.executionQueue.Start(ctx); err != nil {
		return fmt.Errorf("failed to start execution queue: %w", err)
	}

	// Start command executor
	if err := scl.commandExecutor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start command executor: %w", err)
	}

	// Connect devices from HAL to command executor
	if err := scl.connectHALDevices(); err != nil {
		return fmt.Errorf("failed to connect HAL devices: %w", err)
	}

	scl.logger.Info("Service Coordination Layer started successfully")
	return nil
}

// Stop gracefully shuts down the service coordination layer
func (scl *ServiceCoordinationLayer) Stop() error {
	scl.logger.Info("Stopping Service Coordination Layer")

	var errs []error

	// Stop command executor
	if scl.commandExecutor != nil {
		if err := scl.commandExecutor.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("command executor stop error: %w", err))
		}
	}

	// Stop execution queue
	if scl.executionQueue != nil {
		if err := scl.executionQueue.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("execution queue stop error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("service coordination stop errors: %v", errs)
	}

	scl.logger.Info("Service Coordination Layer stopped successfully")
	return nil
}

// connectHALDevices connects devices from HAL to the command executor
func (scl *ServiceCoordinationLayer) connectHALDevices() error {
	// This would typically iterate through HAL devices and connect them
	// to the command executor. For now, we'll log this activity.
	scl.logger.Info("Connecting HAL devices to command executor")
	return nil
}

// ExecuteCommand executes a command through the coordination layer
func (scl *ServiceCoordinationLayer) ExecuteCommand(ctx context.Context, cmd types.MotionCommand) error {
	scl.logger.Info("Executing command through service coordination",
		"command_id", cmd.ID, "device_id", cmd.DeviceID, "command_type", cmd.CommandType)

	// Execute command through HAL
	if err := scl.hal.ExecuteCommand(ctx, cmd); err != nil {
		scl.logger.Error("Command execution failed through HAL", "command_id", cmd.ID, "error", err)
		return err
	}

	// Also execute through command executor for queuing and tracking
	if err := scl.commandExecutor.ExecuteCommand(ctx, cmd); err != nil {
		scl.logger.Error("Command execution failed through executor", "command_id", cmd.ID, "error", err)
		return err
	}

	scl.logger.Info("Command executed successfully", "command_id", cmd.ID)
	return nil
}

// ReadDevice reads from a device through the coordination layer
func (scl *ServiceCoordinationLayer) ReadDevice(deviceID types.DeviceID, register string) (interface{}, error) {
	scl.logger.Info("Reading from device through service coordination", "device_id", deviceID, "register", register)

	return scl.hal.ReadDevice(deviceID, register)
}

// GetDeviceStatus returns device status through the coordination layer
func (scl *ServiceCoordinationLayer) GetDeviceStatus(deviceID types.DeviceID) (types.DeviceStatus, error) {
	return scl.hal.GetDeviceStatus(deviceID)
}

// GetAllDeviceStatuses returns statuses of all devices
func (scl *ServiceCoordinationLayer) GetAllDeviceStatuses() map[types.DeviceID]types.DeviceStatus {
	return scl.hal.GetAllDeviceStatuses()
}

// AllocateDeviceResource allocates a device resource through HAL
func (scl *ServiceCoordinationLayer) AllocateDeviceResource(deviceID types.DeviceID) error {
	return scl.hal.AllocateResource("device", string(deviceID))
}

// ReleaseDeviceResource releases a device resource through HAL
func (scl *ServiceCoordinationLayer) ReleaseDeviceResource(deviceID types.DeviceID) error {
	return scl.hal.ReleaseResource("device", string(deviceID))
}

// AllocateAxisResource allocates an axis resource through HAL
func (scl *ServiceCoordinationLayer) AllocateAxisResource(axisID string) error {
	return scl.hal.AllocateResource("axis", axisID)
}

// ReleaseAxisResource releases an axis resource through HAL
func (scl *ServiceCoordinationLayer) ReleaseAxisResource(axisID string) error {
	return scl.hal.ReleaseResource("axis", axisID)
}

// GetCommandExecutor returns the command executor for upper layers
func (scl *ServiceCoordinationLayer) GetCommandExecutor() *core.CommandExecutor {
	return scl.commandExecutor
}

// GetExecutionQueue returns the execution queue for upper layers
func (scl *ServiceCoordinationLayer) GetExecutionQueue() *core.ExecutionQueue {
	return scl.executionQueue
}

// GetHAL returns the hardware abstraction layer
func (scl *ServiceCoordinationLayer) GetHAL() *hal.HardwareAbstractionLayer {
	return scl.hal
}