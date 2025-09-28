package hal

import (
	"context"
	"fmt"

	"control/pkg/types"
	"control/internal/logging"
)

// Hardware Abstraction Layer - provides unified interface for all hardware operations
type HardwareAbstractionLayer struct {
	deviceManager    *DeviceManager
	protocolManager  *ProtocolManager
	resourceManager  *ResourceManager
	logger          *logging.Logger
	ctx             context.Context
}

// NewHardwareAbstractionLayer creates a new HAL instance
func NewHardwareAbstractionLayer(config types.SystemConfig) *HardwareAbstractionLayer {
	return &HardwareAbstractionLayer{
		deviceManager:   NewDeviceManager(config),
		protocolManager: NewProtocolManager(config),
		resourceManager: NewResourceManager(config),
		logger:         logging.GetLogger("hal"),
	}
}

// Start initializes the HAL and all hardware resources
func (hal *HardwareAbstractionLayer) Start(ctx context.Context) error {
	hal.ctx = ctx
	hal.logger.Info("Starting Hardware Abstraction Layer")

	// Start protocol managers first
	if err := hal.protocolManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start protocol manager: %w", err)
	}

	// Start resource manager
	if err := hal.resourceManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start resource manager: %w", err)
	}

	// Start device manager
	if err := hal.deviceManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start device manager: %w", err)
	}

	hal.logger.Info("Hardware Abstraction Layer started successfully")
	return nil
}

// Stop gracefully shuts down the HAL
func (hal *HardwareAbstractionLayer) Stop() error {
	hal.logger.Info("Stopping Hardware Abstraction Layer")

	var errs []error

	// Stop in reverse order
	if hal.deviceManager != nil {
		if err := hal.deviceManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("device manager stop error: %w", err))
		}
	}

	if hal.resourceManager != nil {
		if err := hal.resourceManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("resource manager stop error: %w", err))
		}
	}

	if hal.protocolManager != nil {
		if err := hal.protocolManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("protocol manager stop error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("HAL stop errors: %v", errs)
	}

	hal.logger.Info("Hardware Abstraction Layer stopped successfully")
	return nil
}

// GetDeviceManager returns the device manager
func (hal *HardwareAbstractionLayer) GetDeviceManager() *DeviceManager {
	return hal.deviceManager
}

// GetProtocolManager returns the protocol manager
func (hal *HardwareAbstractionLayer) GetProtocolManager() *ProtocolManager {
	return hal.protocolManager
}

// GetResourceManager returns the resource manager
func (hal *HardwareAbstractionLayer) GetResourceManager() *ResourceManager {
	return hal.resourceManager
}

// ExecuteCommand executes a command through the HAL
func (hal *HardwareAbstractionLayer) ExecuteCommand(ctx context.Context, cmd types.MotionCommand) error {
	// Get device through HAL device manager
	device, err := hal.deviceManager.GetDevice(cmd.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", cmd.DeviceID, err)
	}

	// Execute command through device abstraction
	return device.Write(cmd)
}

// ReadDevice reads from a device through the HAL
func (hal *HardwareAbstractionLayer) ReadDevice(deviceID types.DeviceID, register string) (interface{}, error) {
	device, err := hal.deviceManager.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device %s: %w", deviceID, err)
	}

	return device.Read(register)
}

// GetDeviceStatus returns the status of a device
func (hal *HardwareAbstractionLayer) GetDeviceStatus(deviceID types.DeviceID) (types.DeviceStatus, error) {
	device, err := hal.deviceManager.GetDevice(deviceID)
	if err != nil {
		return types.DeviceStatus{}, fmt.Errorf("failed to get device %s: %w", deviceID, err)
	}

	return device.Status(), nil
}

// GetAllDeviceStatuses returns statuses of all devices
func (hal *HardwareAbstractionLayer) GetAllDeviceStatuses() map[types.DeviceID]types.DeviceStatus {
	return hal.deviceManager.GetAllDeviceStatuses()
}

// AllocateResource allocates a hardware resource
func (hal *HardwareAbstractionLayer) AllocateResource(resourceType string, resourceID string) error {
	return hal.resourceManager.Allocate(resourceType, resourceID)
}

// ReleaseResource releases a hardware resource
func (hal *HardwareAbstractionLayer) ReleaseResource(resourceType string, resourceID string) error {
	return hal.resourceManager.Release(resourceType, resourceID)
}