// Package device provides comprehensive device management and communication protocol support.
// It handles device discovery, connection management, and protocol abstraction for various
// hardware devices including motor controllers, sensors, and I/O modules used in motion control systems.
package device

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/pkg/types"
	"control/internal/core"
	"control/internal/hardware"
	"control/internal/logging"
)

type BaseDevice struct {
	id         types.DeviceID
	deviceType string
	status     types.DeviceStatus
	config     types.DeviceConfig
	lock       sync.RWMutex
	connected  bool
}

func NewBaseDevice(id types.DeviceID, deviceType string, config types.DeviceConfig) *BaseDevice {
	return &BaseDevice{
		id:         id,
		deviceType: deviceType,
		config:     config,
		status: types.DeviceStatus{
			ID:        id,
			Connected: false,
			Position:  types.Point{X: 0, Y: 0, Z: 0},
			Velocity:  types.Velocity{Linear: 0, Angular: 0},
		},
	}
}

func (bd *BaseDevice) ID() types.DeviceID {
	return bd.id
}

func (bd *BaseDevice) Type() string {
	return bd.deviceType
}

func (bd *BaseDevice) Status() types.DeviceStatus {
	bd.lock.RLock()
	defer bd.lock.RUnlock()
	return bd.status
}

func (bd *BaseDevice) SetStatus(status types.DeviceStatus) {
	bd.lock.Lock()
	defer bd.lock.Unlock()
	bd.status = status
}

func (bd *BaseDevice) IsConnected() bool {
	bd.lock.RLock()
	defer bd.lock.RUnlock()
	return bd.connected
}

func (bd *BaseDevice) SetConnected(connected bool) {
	bd.lock.Lock()
	defer bd.lock.Unlock()
	bd.connected = connected
	bd.status.Connected = connected
	bd.status.LastSeen = time.Now()
}

type MockDevice struct {
	*BaseDevice
	position types.Point
	velocity types.Velocity
}

func NewMockDevice(id types.DeviceID, config types.DeviceConfig) *MockDevice {
	base := NewBaseDevice(id, "mock", config)
	return &MockDevice{
		BaseDevice: base,
		position:   types.Point{X: 0, Y: 0, Z: 0},
		velocity:   types.Velocity{Linear: 0, Angular: 0},
	}
}

func (md *MockDevice) Connect() error {
	md.SetConnected(true)
	logging.GetLogger("mock_device").Info("Mock device connected", "device_id", md.id)
	return nil
}

func (md *MockDevice) Disconnect() {
	md.SetConnected(false)
	logging.GetLogger("mock_device").Info("Mock device disconnected", "device_id", md.id)
}

func (md *MockDevice) Write(cmd types.MotionCommand) error {
	if !md.IsConnected() {
		return fmt.Errorf("device %s is not connected", md.id)
	}

	md.position = cmd.Position
	md.velocity = cmd.Velocity

	md.SetStatus(types.DeviceStatus{
		ID:        md.id,
		Connected: true,
		Position:  md.position,
		Velocity:  md.velocity,
		LastSeen:  time.Now(),
	})

	logging.GetLogger("mock_device").Info("Mock device executed command",
		"device_id", md.id, "command_type", cmd.CommandType, "position", md.position, "velocity", md.velocity)

	return nil
}

func (md *MockDevice) Read(reg string) (interface{}, error) {
	if !md.IsConnected() {
		return nil, fmt.Errorf("device %s is not connected", md.id)
	}

	switch reg {
	case "position":
		return md.position, nil
	case "velocity":
		return md.velocity, nil
	case "connected":
		return md.IsConnected(), nil
	default:
		return nil, fmt.Errorf("unknown register: %s", reg)
	}
}

type ModbusDevice struct {
	*BaseDevice
	client   *ModbusClient
	endpoint string
}

func NewModbusDevice(id types.DeviceID, config types.DeviceConfig) *ModbusDevice {
	base := NewBaseDevice(id, "modbus", config)
	return &ModbusDevice{
		BaseDevice: base,
		endpoint:   config.Endpoint,
	}
}

func (md *ModbusDevice) Connect() error {
	client, err := NewModbusClient(md.endpoint, md.config.Timeout)
	if err != nil {
		return fmt.Errorf("failed to create modbus client: %w", err)
	}

	md.client = client
	md.SetConnected(true)
	logging.GetLogger("modbus_device").Info("Modbus device connected", "device_id", md.id, "endpoint", md.endpoint)
	return nil
}

func (md *ModbusDevice) Disconnect() {
	if md.client != nil {
		md.client.Close()
		md.client = nil
	}
	md.SetConnected(false)
	logging.GetLogger("modbus_device").Info("Modbus device disconnected", "device_id", md.id)
}

func (md *ModbusDevice) Write(cmd types.MotionCommand) error {
	if !md.IsConnected() || md.client == nil {
		return fmt.Errorf("device %s is not connected", md.id)
	}

	switch cmd.CommandType {
	case types.CommandMoveTo:
		if err := md.writePositionRegisters(cmd.Position); err != nil {
			return err
		}
	case types.CommandStop:
		if err := md.writeStopCommand(); err != nil {
			return err
		}
	case types.CommandHome:
		if err := md.writeHomeCommand(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported command type: %s", cmd.CommandType)
	}

	md.updateStatus(cmd)
	return nil
}

func (md *ModbusDevice) Read(reg string) (interface{}, error) {
	if !md.IsConnected() || md.client == nil {
		return nil, fmt.Errorf("device %s is not connected", md.id)
	}

	switch reg {
	case "position":
		return md.readPositionRegisters()
	case "velocity":
		return md.readVelocityRegisters()
	case "status":
		return md.readStatusRegister()
	default:
		return nil, fmt.Errorf("unknown register: %s", reg)
	}
}

func (md *ModbusDevice) writePositionRegisters(pos types.Point) error {
	if md.client == nil {
		return fmt.Errorf("modbus client not initialized")
	}

	values := []uint16{
		uint16(pos.X * 1000),
		uint16(pos.Y * 1000),
		uint16(pos.Z * 1000),
	}

	return md.client.WriteHoldingRegisters(0, values)
}

func (md *ModbusDevice) writeStopCommand() error {
	if md.client == nil {
		return fmt.Errorf("modbus client not initialized")
	}
	return md.client.WriteSingleRegister(100, 0)
}

func (md *ModbusDevice) writeHomeCommand() error {
	if md.client == nil {
		return fmt.Errorf("modbus client not initialized")
	}
	return md.client.WriteSingleRegister(101, 1)
}

func (md *ModbusDevice) readPositionRegisters() (types.Point, error) {
	if md.client == nil {
		return types.Point{}, fmt.Errorf("modbus client not initialized")
	}

	values, err := md.client.ReadHoldingRegisters(0, 3)
	if err != nil {
		return types.Point{}, err
	}

	return types.Point{
		X: float64(values[0]) / 1000,
		Y: float64(values[1]) / 1000,
		Z: float64(values[2]) / 1000,
	}, nil
}

func (md *ModbusDevice) readVelocityRegisters() (types.Velocity, error) {
	if md.client == nil {
		return types.Velocity{}, fmt.Errorf("modbus client not initialized")
	}

	values, err := md.client.ReadHoldingRegisters(10, 2)
	if err != nil {
		return types.Velocity{}, err
	}

	return types.Velocity{
		Linear:  float64(values[0]) / 1000,
		Angular: float64(values[1]) / 1000,
	}, nil
}

func (md *ModbusDevice) readStatusRegister() (uint16, error) {
	if md.client == nil {
		return 0, fmt.Errorf("modbus client not initialized")
	}
	values, err := md.client.ReadHoldingRegisters(20, 1)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("no status register value")
	}
	return values[0], nil
}

func (md *ModbusDevice) updateStatus(cmd types.MotionCommand) {
	status := md.Status()
	status.Position = cmd.Position
	status.Velocity = cmd.Velocity
	status.LastSeen = time.Now()
	md.SetStatus(status)
}

type ModbusClient struct {
	endpoint string
	timeout  time.Duration
}

func NewModbusClient(endpoint string, timeout time.Duration) (*ModbusClient, error) {
	return &ModbusClient{
		endpoint: endpoint,
		timeout:  timeout,
	}, nil
}

func (mc *ModbusClient) ReadHoldingRegisters(address, quantity uint16) ([]uint16, error) {
	logging.GetLogger("modbus_client").Debug("Modbus read holding registers", "address", address, "quantity", quantity)

	values := make([]uint16, quantity)
	for i := range values {
		values[i] = uint16(i * 100)
	}

	return values, nil
}

func (mc *ModbusClient) WriteHoldingRegisters(address uint16, values []uint16) error {
	logging.GetLogger("modbus_client").Debug("Modbus write holding registers", "address", address, "values", values)
	return nil
}

func (mc *ModbusClient) WriteSingleRegister(address, value uint16) error {
	logging.GetLogger("modbus_client").Debug("Modbus write single register", "address", address, "value", value)
	return nil
}

func (mc *ModbusClient) Close() {
	logging.GetLogger("modbus_client").Info("Modbus client closed")
}

// HardwareDevice wraps the new hardware abstraction layer for compatibility
type HardwareDevice struct {
	*BaseDevice
	config       types.DeviceConfig
	hardwareMgr *hardware.HardwareFactory
	hwDeviceID  string
}

func NewHardwareDevice(id types.DeviceID, config types.DeviceConfig, hardwareMgr *hardware.HardwareFactory) *HardwareDevice {
	base := NewBaseDevice(id, "hardware", config)
	return &HardwareDevice{
		BaseDevice:  base,
		config:      config,
		hardwareMgr: hardwareMgr,
		hwDeviceID:  string(id),
	}
}

func (hd *HardwareDevice) Connect() error {
	// The hardware manager already handles connection, just update status
	hd.SetConnected(true)
	logging.GetLogger("hardware_device").Info("Hardware device connected", "device_id", hd.id)
	return nil
}

func (hd *HardwareDevice) Disconnect() {
	// The hardware manager handles disconnection
	hd.SetConnected(false)
	logging.GetLogger("hardware_device").Info("Hardware device disconnected", "device_id", hd.id)
}

func (hd *HardwareDevice) Write(cmd types.MotionCommand) error {
	if !hd.IsConnected() {
		return fmt.Errorf("device %s is not connected", hd.id)
	}

	// Convert command to hardware operations
	switch cmd.CommandType {
	case types.CommandMoveTo:
		return hd.writePositionToHardware(cmd.Position, cmd.Velocity)
	case types.CommandStop:
		return hd.writeStopToHardware()
	case types.CommandHome:
		return hd.writeHomeToHardware()
	default:
		return fmt.Errorf("unsupported command type: %s", cmd.CommandType)
	}
}

func (hd *HardwareDevice) Read(reg string) (interface{}, error) {
	if !hd.IsConnected() {
		return nil, fmt.Errorf("device %s is not connected", hd.id)
	}

	switch reg {
	case "position":
		return hd.readPositionFromHardware()
	case "velocity":
		return hd.readVelocityFromHardware()
	case "status":
		return hd.readStatusFromHardware()
	default:
		return nil, fmt.Errorf("unknown register: %s", reg)
	}
}

func (hd *HardwareDevice) writePositionToHardware(pos types.Point, vel types.Velocity) error {
	// Convert position data to bytes and write to hardware device
	positionData := []byte{
		byte(pos.X * 1000),
		byte(pos.Y * 1000),
		byte(pos.Z * 1000),
	}

	velocityData := []byte{
		byte(vel.Linear * 1000),
		byte(vel.Angular * 1000),
	}

	// Write position
	if err := hd.hardwareMgr.WriteDevice(hd.hwDeviceID, "position", positionData); err != nil {
		return fmt.Errorf("failed to write position: %w", err)
	}

	// Write velocity
	if err := hd.hardwareMgr.WriteDevice(hd.hwDeviceID, "velocity", velocityData); err != nil {
		return fmt.Errorf("failed to write velocity: %w", err)
	}

	hd.updateStatus(types.MotionCommand{
		Position: pos,
		Velocity: vel,
	})

	return nil
}

func (hd *HardwareDevice) writeStopToHardware() error {
	stopCmd := []byte{0x00}
	if err := hd.hardwareMgr.WriteDevice(hd.hwDeviceID, "stop", stopCmd); err != nil {
		return fmt.Errorf("failed to write stop command: %w", err)
	}
	return nil
}

func (hd *HardwareDevice) writeHomeToHardware() error {
	homeCmd := []byte{0x01}
	if err := hd.hardwareMgr.WriteDevice(hd.hwDeviceID, "home", homeCmd); err != nil {
		return fmt.Errorf("failed to write home command: %w", err)
	}
	return nil
}

func (hd *HardwareDevice) readPositionFromHardware() (types.Point, error) {
	data, err := hd.hardwareMgr.ReadDevice(hd.hwDeviceID, "position", 3)
	if err != nil {
		return types.Point{}, fmt.Errorf("failed to read position: %w", err)
	}

	if len(data) < 3 {
		return types.Point{}, fmt.Errorf("insufficient position data")
	}

	return types.Point{
		X: float64(data[0]) / 1000,
		Y: float64(data[1]) / 1000,
		Z: float64(data[2]) / 1000,
	}, nil
}

func (hd *HardwareDevice) readVelocityFromHardware() (types.Velocity, error) {
	data, err := hd.hardwareMgr.ReadDevice(hd.hwDeviceID, "velocity", 2)
	if err != nil {
		return types.Velocity{}, fmt.Errorf("failed to read velocity: %w", err)
	}

	if len(data) < 2 {
		return types.Velocity{}, fmt.Errorf("insufficient velocity data")
	}

	return types.Velocity{
		Linear:  float64(data[0]) / 1000,
		Angular: float64(data[1]) / 1000,
	}, nil
}

func (hd *HardwareDevice) readStatusFromHardware() (uint16, error) {
	data, err := hd.hardwareMgr.ReadDevice(hd.hwDeviceID, "status", 1)
	if err != nil {
		return 0, fmt.Errorf("failed to read status: %w", err)
	}

	if len(data) < 1 {
		return 0, fmt.Errorf("insufficient status data")
	}

	return uint16(data[0]), nil
}

func (hd *HardwareDevice) updateStatus(cmd types.MotionCommand) {
	status := hd.Status()
	status.Position = cmd.Position
	status.Velocity = cmd.Velocity
	status.LastSeen = time.Now()
	hd.SetStatus(status)
}

type DeviceManager struct {
	devices       map[types.DeviceID]core.Device
	devicesLock   sync.RWMutex
	config        types.SystemConfig
	hardwareMgr   *hardware.HardwareFactory
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        *logging.Logger
}

func NewDeviceManager(config types.SystemConfig) *DeviceManager {
	return &DeviceManager{
		devices:     make(map[types.DeviceID]core.Device),
		config:      config,
		hardwareMgr: hardware.NewHardwareFactory(),
		logger:      logging.GetLogger("device_manager"),
	}
}

func (dm *DeviceManager) Start(ctx context.Context) error {
	dm.ctx, dm.cancel = context.WithCancel(ctx)

	// Start hardware abstraction layer
	if err := dm.hardwareMgr.Start(); err != nil {
		dm.logger.Warn("Failed to start hardware manager", "error", err)
	}

	// Convert and add devices from configuration
	for deviceID, deviceConfig := range dm.config.Devices {
		// Add device to hardware manager
		if err := dm.hardwareMgr.AddDeviceFromConfig(string(deviceID), deviceConfig); err != nil {
			dm.logger.Error("Failed to add hardware device", "device_id", deviceID, "error", err)
			continue
		}

		// Create wrapper device for compatibility with existing system
		device, err := dm.createDevice(deviceID, deviceConfig)
		if err != nil {
			dm.logger.Error("Failed to create device wrapper", "device_id", deviceID, "error", err)
			continue
		}

		dm.devicesLock.Lock()
		dm.devices[deviceID] = device
		dm.devicesLock.Unlock()

		// Connect to hardware device
		if err := dm.hardwareMgr.ConnectDevice(string(deviceID)); err != nil {
			dm.logger.Error("Failed to connect hardware device", "device_id", deviceID, "error", err)
		} else if err := device.Connect(); err != nil {
			dm.logger.Error("Failed to connect device wrapper", "device_id", deviceID, "error", err)
		}
	}

	dm.logger.Info("Device manager started", "device_count", len(dm.devices))
	return nil
}

func (dm *DeviceManager) Stop() error {
	if dm.cancel != nil {
		dm.cancel()
	}

	// Stop hardware abstraction layer
	if dm.hardwareMgr != nil {
		if err := dm.hardwareMgr.Stop(); err != nil {
			dm.logger.Error("Error stopping hardware manager", "error", err)
		}
	}

	dm.devicesLock.Lock()
	for _, device := range dm.devices {
		device.Disconnect()
	}
	dm.devicesLock.Unlock()

	dm.wg.Wait()

	dm.logger.Info("Device manager stopped")
	return nil
}

func (dm *DeviceManager) createDevice(deviceID types.DeviceID, config types.DeviceConfig) (core.Device, error) {
	// Try to create a device using the new hardware abstraction layer
	if _, err := dm.hardwareMgr.CreateDeviceConfig(string(deviceID), config); err == nil {
		// Hardware layer supports this protocol, create a compatible wrapper
		return NewHardwareDevice(deviceID, config, dm.hardwareMgr), nil
	}

	// Fall back to legacy implementations
	switch config.Protocol {
	case "mock":
		return NewMockDevice(deviceID, config), nil
	case "modbus":
		return NewModbusDevice(deviceID, config), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}

func (dm *DeviceManager) GetDevice(deviceID types.DeviceID) (core.Device, error) {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

func (dm *DeviceManager) GetAllDevices() map[types.DeviceID]core.Device {
	dm.devicesLock.RLock()
	defer dm.devicesLock.RUnlock()

	devices := make(map[types.DeviceID]core.Device)
	for id, device := range dm.devices {
		devices[id] = device
	}

	return devices
}

func (dm *DeviceManager) GetDeviceStatuses() map[string]interface{} {
	statuses := make(map[string]interface{})

	// Get legacy device statuses
	for deviceID, device := range dm.GetAllDevices() {
		statuses[string(deviceID)] = device.Status()
	}

	// Get hardware device statuses if available
	if dm.hardwareMgr != nil {
		hwDevices := dm.hardwareMgr.ListDevices()
		for _, hwDeviceStatus := range hwDevices {
			deviceID := string(hwDeviceStatus.ID)
			// Don't override if already present (legacy device takes precedence)
			if _, exists := statuses[deviceID]; !exists {
				statuses[deviceID] = hwDeviceStatus
			}
		}
	}

	return statuses
}

// GetHardwareDevices returns the raw hardware device list for advanced operations
func (dm *DeviceManager) GetHardwareDevices() []types.DeviceStatus {
	if dm.hardwareMgr == nil {
		return []types.DeviceStatus{}
	}
	return dm.hardwareMgr.ListDevices()
}