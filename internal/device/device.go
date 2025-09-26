package device

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"control/pkg/types"
	"control/internal/core"
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
	log.Printf("Mock device %s connected", md.id)
	return nil
}

func (md *MockDevice) Disconnect() {
	md.SetConnected(false)
	log.Printf("Mock device %s disconnected", md.id)
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

	log.Printf("Mock device %s executed command %s: pos=%v, vel=%v",
		md.id, cmd.CommandType, md.position, md.velocity)

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
	log.Printf("Modbus device %s connected to %s", md.id, md.endpoint)
	return nil
}

func (md *ModbusDevice) Disconnect() {
	if md.client != nil {
		md.client.Close()
		md.client = nil
	}
	md.SetConnected(false)
	log.Printf("Modbus device %s disconnected", md.id)
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
	log.Printf("Modbus read holding registers: address=%d, quantity=%d", address, quantity)

	values := make([]uint16, quantity)
	for i := range values {
		values[i] = uint16(i * 100)
	}

	return values, nil
}

func (mc *ModbusClient) WriteHoldingRegisters(address uint16, values []uint16) error {
	log.Printf("Modbus write holding registers: address=%d, values=%v", address, values)
	return nil
}

func (mc *ModbusClient) WriteSingleRegister(address, value uint16) error {
	log.Printf("Modbus write single register: address=%d, value=%d", address, value)
	return nil
}

func (mc *ModbusClient) Close() {
	log.Printf("Modbus client closed")
}

type DeviceManager struct {
	devices    map[types.DeviceID]core.Device
	devicesLock sync.RWMutex
	config     types.SystemConfig
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewDeviceManager(config types.SystemConfig) *DeviceManager {
	return &DeviceManager{
		devices: make(map[types.DeviceID]core.Device),
		config:  config,
	}
}

func (dm *DeviceManager) Start(ctx context.Context) error {
	dm.ctx, dm.cancel = context.WithCancel(ctx)

	for deviceID, deviceConfig := range dm.config.Devices {
		device, err := dm.createDevice(deviceID, deviceConfig)
		if err != nil {
			log.Printf("Failed to create device %s: %v", deviceID, err)
			continue
		}

		dm.devicesLock.Lock()
		dm.devices[deviceID] = device
		dm.devicesLock.Unlock()

		if err := device.Connect(); err != nil {
			log.Printf("Failed to connect device %s: %v", deviceID, err)
		}
	}

	log.Printf("Device manager started with %d devices", len(dm.devices))
	return nil
}

func (dm *DeviceManager) Stop() error {
	if dm.cancel != nil {
		dm.cancel()
	}

	dm.devicesLock.Lock()
	for _, device := range dm.devices {
		device.Disconnect()
	}
	dm.devicesLock.Unlock()

	dm.wg.Wait()

	log.Println("Device manager stopped")
	return nil
}

func (dm *DeviceManager) createDevice(deviceID types.DeviceID, config types.DeviceConfig) (core.Device, error) {
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

	for deviceID, device := range dm.GetAllDevices() {
		statuses[string(deviceID)] = device.Status()
	}

	return statuses
}