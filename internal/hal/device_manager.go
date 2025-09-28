package hal

import (
	"context"
	"fmt"
	"sync"

	"control/pkg/types"
	"control/internal/core"
	"control/internal/logging"
)

// DeviceManager manages all hardware devices through unified interfaces
type DeviceManager struct {
	devices     map[types.DeviceID]core.Device
	devicesLock sync.RWMutex
	config      types.SystemConfig
	logger      *logging.Logger
	ctx         context.Context
}

// NewDeviceManager creates a new device manager
func NewDeviceManager(config types.SystemConfig) *DeviceManager {
	return &DeviceManager{
		devices: make(map[types.DeviceID]core.Device),
		config:  config,
		logger:  logging.GetLogger("hal_device_manager"),
	}
}

// Start initializes all devices based on configuration
func (dm *DeviceManager) Start(ctx context.Context) error {
	dm.ctx = ctx
	dm.logger.Info("Starting HAL Device Manager")

	// Initialize devices from configuration
	for deviceID, deviceConfig := range dm.config.Devices {
		if err := dm.initializeDevice(deviceID, deviceConfig); err != nil {
			dm.logger.Error("Failed to initialize device", "device_id", deviceID, "error", err)
			return fmt.Errorf("failed to initialize device %s: %w", deviceID, err)
		}
	}

	dm.logger.Info("HAL Device Manager started successfully")
	return nil
}

// Stop gracefully shuts down all devices
func (dm *DeviceManager) Stop() error {
	dm.logger.Info("Stopping HAL Device Manager")

	dm.devicesLock.Lock()
	defer dm.devicesLock.Unlock()

	var errs []error

	for deviceID, device := range dm.devices {
		dm.logger.Info("Stopping device", "device_id", deviceID)
		device.Disconnect()
		// Note: Device-specific cleanup could be added here
	}

	dm.devices = make(map[types.DeviceID]core.Device)

	if len(errs) > 0 {
		return fmt.Errorf("device manager stop errors: %v", errs)
	}

	dm.logger.Info("HAL Device Manager stopped successfully")
	return nil
}

// initializeDevice creates and initializes a device based on configuration
func (dm *DeviceManager) initializeDevice(deviceID types.DeviceID, config types.DeviceConfig) error {
	// This would delegate to protocol-specific device factories
	// For now, we'll use the existing device creation logic

	// Note: In a full implementation, this would use the protocol manager
	// to create protocol-specific device instances
	dm.logger.Info("Initializing device", "device_id", deviceID, "type", config.Type, "protocol", config.Protocol)

	// The actual device creation would happen here through the infrastructure layer
	// For now, we'll log and continue
	return nil
}

// GetDevice returns a device by ID
func (dm *DeviceManager) GetDevice(deviceID types.DeviceID) (core.Device, error) {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

// AddDevice adds a device to the manager
func (dm *DeviceManager) AddDevice(device core.Device) error {
	dm.devicesLock.Lock()
	defer dm.devicesLock.Unlock()

	deviceID := device.ID()
	if _, exists := dm.devices[deviceID]; exists {
		return fmt.Errorf("device %s already exists", deviceID)
	}

	if err := device.Connect(); err != nil {
		return fmt.Errorf("failed to connect device %s: %w", deviceID, err)
	}

	dm.devices[deviceID] = device
	dm.logger.Info("Device added", "device_id", deviceID, "device_type", device.Type())

	return nil
}

// RemoveDevice removes a device from the manager
func (dm *DeviceManager) RemoveDevice(deviceID types.DeviceID) error {
	dm.devicesLock.Lock()
	defer dm.devicesLock.Unlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Disconnect()
	delete(dm.devices, deviceID)

	dm.logger.Info("Device removed", "device_id", deviceID)
	return nil
}

// GetAllDevices returns all devices
func (dm *DeviceManager) GetAllDevices() map[types.DeviceID]core.Device {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	devices := make(map[types.DeviceID]core.Device)
	for id, device := range dm.devices {
		devices[id] = device
	}

	return devices
}

// GetAllDeviceStatuses returns status of all devices
func (dm *DeviceManager) GetAllDeviceStatuses() map[types.DeviceID]types.DeviceStatus {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	statuses := make(map[types.DeviceID]types.DeviceStatus)
	for deviceID, device := range dm.devices {
		statuses[deviceID] = device.Status()
	}

	return statuses
}

// GetDeviceCount returns the number of managed devices
func (dm *DeviceManager) GetDeviceCount() int {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()
	return len(dm.devices)
}

// IsDeviceConnected checks if a device is connected
func (dm *DeviceManager) IsDeviceConnected(deviceID types.DeviceID) bool {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return false
	}

	return device.Status().Connected
}

// GetDevicesByType returns devices of a specific type
func (dm *DeviceManager) GetDevicesByType(deviceType string) []core.Device {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	var devices []core.Device
	for _, device := range dm.devices {
		if device.Type() == deviceType {
			devices = append(devices, device)
		}
	}

	return devices
}

// GetDevicesByProtocol returns devices using a specific protocol
func (dm *DeviceManager) GetDevicesByProtocol(protocol string) []core.Device {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	var devices []core.Device
	for deviceID, device := range dm.devices {
		// This would need to be enhanced to track protocol per device
		// For now, we'll filter based on configuration
		if config, exists := dm.config.Devices[deviceID]; exists && config.Protocol == protocol {
			devices = append(devices, device)
		}
	}

	return devices
}