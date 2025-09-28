package hal

import (
	"context"
	"fmt"
	"sync"

	"control/pkg/types"
	"control/internal/logging"
)

// ProtocolManager manages communication protocols for different device types
type ProtocolManager struct {
	protocols  map[string]Protocol
	protocolsLock sync.RWMutex
	config     types.SystemConfig
	logger     *logging.Logger
	ctx        context.Context
}

// Protocol defines the interface for communication protocols
type Protocol interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Connect(deviceID types.DeviceID, config types.DeviceConfig) error
	Disconnect(deviceID types.DeviceID) error
	Write(deviceID types.DeviceID, cmd types.MotionCommand) error
	Read(deviceID types.DeviceID, register string) (interface{}, error)
	GetStatus(deviceID types.DeviceID) (types.DeviceStatus, error)
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager(config types.SystemConfig) *ProtocolManager {
	return &ProtocolManager{
		protocols: make(map[string]Protocol),
		config:    config,
		logger:    logging.GetLogger("hal_protocol_manager"),
	}
}

// Start initializes all protocol handlers
func (pm *ProtocolManager) Start(ctx context.Context) error {
	pm.ctx = ctx
	pm.logger.Info("Starting HAL Protocol Manager")

	// Initialize protocol handlers based on configuration
	// This would typically involve loading protocol-specific implementations

	// For example: initialize Modbus protocol
	if pm.hasDevicesWithProtocol("modbus") {
		if err := pm.initializeProtocol("modbus"); err != nil {
			return fmt.Errorf("failed to initialize modbus protocol: %w", err)
		}
	}

	// Initialize serial protocol
	if pm.hasDevicesWithProtocol("serial") {
		if err := pm.initializeProtocol("serial"); err != nil {
			return fmt.Errorf("failed to initialize serial protocol: %w", err)
		}
	}

	// Initialize custom protocol
	if pm.hasDevicesWithProtocol("custom") {
		if err := pm.initializeProtocol("custom"); err != nil {
			return fmt.Errorf("failed to initialize custom protocol: %w", err)
		}
	}

	pm.logger.Info("HAL Protocol Manager started successfully")
	return nil
}

// Stop gracefully shuts down all protocol handlers
func (pm *ProtocolManager) Stop() error {
	pm.logger.Info("Stopping HAL Protocol Manager")

	pm.protocolsLock.Lock()
	defer pm.protocolsLock.Unlock()

	var errs []error

	for protocolName, protocol := range pm.protocols {
		pm.logger.Info("Stopping protocol", "protocol", protocolName)
		if err := protocol.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("protocol %s stop error: %w", protocolName, err))
		}
	}

	pm.protocols = make(map[string]Protocol)

	if len(errs) > 0 {
		return fmt.Errorf("protocol manager stop errors: %v", errs)
	}

	pm.logger.Info("HAL Protocol Manager stopped successfully")
	return nil
}

// initializeProtocol initializes a specific protocol handler
func (pm *ProtocolManager) initializeProtocol(protocolName string) error {
	pm.logger.Info("Initializing protocol", "protocol", protocolName)

	// This would create protocol-specific instances
	// For now, we'll create a mock protocol handler
	protocol := &MockProtocol{name: protocolName}

	pm.protocolsLock.Lock()
	pm.protocols[protocolName] = protocol
	pm.protocolsLock.Unlock()

	return nil
}

// hasDevicesWithProtocol checks if any devices use the specified protocol
func (pm *ProtocolManager) hasDevicesWithProtocol(protocol string) bool {
	for _, deviceConfig := range pm.config.Devices {
		if deviceConfig.Protocol == protocol {
			return true
		}
	}
	return false
}

// GetProtocol returns a protocol handler by name
func (pm *ProtocolManager) GetProtocol(protocolName string) (Protocol, error) {
	pm.protocolsLock.RLock()
	defer pm.protocolsLock.RUnlock()

	protocol, exists := pm.protocols[protocolName]
	if !exists {
		return nil, fmt.Errorf("protocol %s not found", protocolName)
	}

	return protocol, nil
}

// ExecuteCommand executes a command through the appropriate protocol
func (pm *ProtocolManager) ExecuteCommand(deviceID types.DeviceID, cmd types.MotionCommand) error {
	deviceConfig, exists := pm.config.Devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s configuration not found", deviceID)
	}

	protocol, err := pm.GetProtocol(deviceConfig.Protocol)
	if err != nil {
		return fmt.Errorf("failed to get protocol %s for device %s: %w", deviceConfig.Protocol, deviceID, err)
	}

	return protocol.Write(deviceID, cmd)
}

// ReadDevice reads from a device through the appropriate protocol
func (pm *ProtocolManager) ReadDevice(deviceID types.DeviceID, register string) (interface{}, error) {
	deviceConfig, exists := pm.config.Devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s configuration not found", deviceID)
	}

	protocol, err := pm.GetProtocol(deviceConfig.Protocol)
	if err != nil {
		return nil, fmt.Errorf("failed to get protocol %s for device %s: %w", deviceConfig.Protocol, deviceID, err)
	}

	return protocol.Read(deviceID, register)
}

// GetDeviceStatus returns device status through the appropriate protocol
func (pm *ProtocolManager) GetDeviceStatus(deviceID types.DeviceID) (types.DeviceStatus, error) {
	deviceConfig, exists := pm.config.Devices[deviceID]
	if !exists {
		return types.DeviceStatus{}, fmt.Errorf("device %s configuration not found", deviceID)
	}

	protocol, err := pm.GetProtocol(deviceConfig.Protocol)
	if err != nil {
		return types.DeviceStatus{}, fmt.Errorf("failed to get protocol %s for device %s: %w", deviceConfig.Protocol, deviceID, err)
	}

	return protocol.GetStatus(deviceID)
}

// ConnectDevice connects a device through its protocol
func (pm *ProtocolManager) ConnectDevice(deviceID types.DeviceID) error {
	deviceConfig, exists := pm.config.Devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s configuration not found", deviceID)
	}

	protocol, err := pm.GetProtocol(deviceConfig.Protocol)
	if err != nil {
		return fmt.Errorf("failed to get protocol %s for device %s: %w", deviceConfig.Protocol, deviceID, err)
	}

	return protocol.Connect(deviceID, deviceConfig)
}

// DisconnectDevice disconnects a device through its protocol
func (pm *ProtocolManager) DisconnectDevice(deviceID types.DeviceID) error {
	deviceConfig, exists := pm.config.Devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s configuration not found", deviceID)
	}

	protocol, err := pm.GetProtocol(deviceConfig.Protocol)
	if err != nil {
		return fmt.Errorf("failed to get protocol %s for device %s: %w", deviceConfig.Protocol, deviceID, err)
	}

	return protocol.Disconnect(deviceID)
}

// GetAvailableProtocols returns list of available protocols
func (pm *ProtocolManager) GetAvailableProtocols() []string {
	pm.protocolsLock.RLock()
	defer pm.protocolsLock.RUnlock()

	protocols := make([]string, 0, len(pm.protocols))
	for name := range pm.protocols {
		protocols = append(protocols, name)
	}

	return protocols
}

// MockProtocol is a simple mock implementation for testing
type MockProtocol struct {
	name string
}

func (mp *MockProtocol) Name() string {
	return mp.name
}

func (mp *MockProtocol) Start(ctx context.Context) error {
	return nil
}

func (mp *MockProtocol) Stop() error {
	return nil
}

func (mp *MockProtocol) Connect(deviceID types.DeviceID, config types.DeviceConfig) error {
	return nil
}

func (mp *MockProtocol) Disconnect(deviceID types.DeviceID) error {
	return nil
}

func (mp *MockProtocol) Write(deviceID types.DeviceID, cmd types.MotionCommand) error {
	return nil
}

func (mp *MockProtocol) Read(deviceID types.DeviceID, register string) (interface{}, error) {
	return "mock_value", nil
}

func (mp *MockProtocol) GetStatus(deviceID types.DeviceID) (types.DeviceStatus, error) {
	return types.DeviceStatus{
		Connected: true,
		Position: types.Point{
			X: 0.0,
			Y: 0.0,
			Z: 0.0,
		},
		Velocity: types.Velocity{
			Linear:  0.0,
			Angular: 0.0,
		},
		Error: nil,
	}, nil
}