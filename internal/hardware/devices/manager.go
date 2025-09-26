package devices

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"control/internal/hardware/comm"
	"control/internal/hardware/protocols/custom"
	"control/internal/hardware/protocols/modbus"
	"control/internal/hardware/protocols/serial"
	"control/pkg/types"
)

// DeviceType 设备类型
type DeviceType int

const (
	DeviceTypeUnknown DeviceType = iota
	DeviceTypeModbusTCP
	DeviceTypeModbusRTU
	DeviceTypeModbusASCII
	DeviceTypeSerial
	DeviceTypeCustom
)

// DeviceConfig 设备配置
type DeviceConfig struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Type        DeviceType             `yaml:"type"`
	Protocol    string                 `yaml:"protocol"`
	Enabled     bool                   `yaml:"enabled"`
	Config      interface{}             `yaml:"config"`
	Properties  map[string]interface{} `yaml:"properties"`
	Parameters  map[string]interface{} `yaml:"parameters"`
	Timeout     time.Duration          `yaml:"timeout"`
	RetryCount  int                    `yaml:"retry_count"`
	AutoConnect bool                   `yaml:"auto_connect"`
}

// Device 设备接口
type Device interface {
	// 基础信息
	GetID() string
	GetName() string
	GetType() DeviceType
	GetStatus() comm.ConnectionStatus
	GetConfig() DeviceConfig

	// 连接管理
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Reconnect(ctx context.Context) error
	IsConnected() bool

	// 数据操作
	Read(ctx context.Context, address string, length int) ([]byte, error)
	Write(ctx context.Context, address string, data []byte) error
	BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error)
	BulkWrite(ctx context.Context, data map[string][]byte) error

	// 事件处理
	AddEventHandler(handler comm.EventHandler)
	RemoveEventHandler(handler comm.EventHandler)

	// 获取通信接口
	GetCommunication() comm.CommunicationInterface
}

// BaseDevice 基础设备实现
type BaseDevice struct {
	config       DeviceConfig
	comm         comm.CommunicationInterface
	discovery    comm.DiscoveryInterface
	status       comm.ConnectionStatus
	lastError    error
	eventHandler []comm.EventHandler
	mu           sync.RWMutex
}

// NewBaseDevice 创建基础设备
func NewBaseDevice(config DeviceConfig) *BaseDevice {
	return &BaseDevice{
		config:       config,
		status:       comm.StatusDisconnected,
		eventHandler: make([]comm.EventHandler, 0),
	}
}

// GetID 获取设备ID
func (bd *BaseDevice) GetID() string {
	return bd.config.ID
}

// GetName 获取设备名称
func (bd *BaseDevice) GetName() string {
	return bd.config.Name
}

// GetType 获取设备类型
func (bd *BaseDevice) GetType() DeviceType {
	return bd.config.Type
}

// GetStatus 获取设备状态
func (bd *BaseDevice) GetStatus() comm.ConnectionStatus {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	if bd.comm != nil {
		return bd.comm.GetStatus()
	}
	return bd.status
}

// GetConfig 获取设备配置
func (bd *BaseDevice) GetConfig() DeviceConfig {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.config
}

// Connect 连接设备
func (bd *BaseDevice) Connect(ctx context.Context) error {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	if bd.comm == nil {
		return fmt.Errorf("communication interface not initialized")
	}

	bd.status = comm.StatusConnecting
	err := bd.comm.Connect(ctx)
	if err != nil {
		bd.status = comm.StatusError
		bd.lastError = err
		return err
	}

	bd.status = comm.StatusConnected
	bd.lastError = nil
	return nil
}

// Disconnect 断开设备连接
func (bd *BaseDevice) Disconnect(ctx context.Context) error {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	if bd.comm == nil {
		return nil
	}

	err := bd.comm.Disconnect(ctx)
	bd.status = comm.StatusDisconnected
	bd.lastError = err
	return err
}

// Reconnect 重连设备
func (bd *BaseDevice) Reconnect(ctx context.Context) error {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	if bd.comm == nil {
		return fmt.Errorf("communication interface not initialized")
	}

	err := bd.comm.Reconnect(ctx)
	if err != nil {
		bd.status = comm.StatusError
		bd.lastError = err
		return err
	}

	bd.status = comm.StatusConnected
	bd.lastError = nil
	return nil
}

// IsConnected 检查设备是否连接
func (bd *BaseDevice) IsConnected() bool {
	return bd.GetStatus() == comm.StatusConnected
}

// Read 读取设备数据
func (bd *BaseDevice) Read(ctx context.Context, address string, length int) ([]byte, error) {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if bd.comm == nil {
		return nil, fmt.Errorf("communication interface not initialized")
	}

	return bd.comm.Read(ctx, address, length)
}

// Write 写入设备数据
func (bd *BaseDevice) Write(ctx context.Context, address string, data []byte) error {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if bd.comm == nil {
		return fmt.Errorf("communication interface not initialized")
	}

	return bd.comm.Write(ctx, address, data)
}

// BulkRead 批量读取
func (bd *BaseDevice) BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error) {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if bd.comm == nil {
		return nil, fmt.Errorf("communication interface not initialized")
	}

	return bd.comm.BulkRead(ctx, addresses)
}

// BulkWrite 批量写入
func (bd *BaseDevice) BulkWrite(ctx context.Context, data map[string][]byte) error {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if bd.comm == nil {
		return fmt.Errorf("communication interface not initialized")
	}

	return bd.comm.BulkWrite(ctx, data)
}

// AddEventHandler 添加事件处理器
func (bd *BaseDevice) AddEventHandler(handler comm.EventHandler) {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	bd.eventHandler = append(bd.eventHandler, handler)
	if bd.comm != nil {
		bd.comm.AddEventHandler(handler)
	}
}

// RemoveEventHandler 移除事件处理器
func (bd *BaseDevice) RemoveEventHandler(handler comm.EventHandler) {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	for i, h := range bd.eventHandler {
		if h == handler {
			bd.eventHandler = append(bd.eventHandler[:i], bd.eventHandler[i+1:]...)
			break
		}
	}
	if bd.comm != nil {
		bd.comm.RemoveEventHandler(handler)
	}
}

// GetCommunication 获取通信接口
func (bd *BaseDevice) GetCommunication() comm.CommunicationInterface {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.comm
}

// setCommunication 设置通信接口
func (bd *BaseDevice) setCommunication(comm comm.CommunicationInterface) {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	// 移除旧接口的事件处理器
	if bd.comm != nil {
		for _, handler := range bd.eventHandler {
			bd.comm.RemoveEventHandler(handler)
		}
	}

	bd.comm = comm

	// 添加新接口的事件处理器
	if bd.comm != nil {
		for _, handler := range bd.eventHandler {
			bd.comm.AddEventHandler(handler)
		}
	}
}

// ModbusDevice Modbus设备
type ModbusDevice struct {
	*BaseDevice
	client *modbus.ModbusClient
}

// NewModbusDevice 创建Modbus设备
func NewModbusDevice(config DeviceConfig) (*ModbusDevice, error) {
	modbusConfig, ok := config.Config.(modbus.ModbusConfig)
	if !ok {
		return nil, fmt.Errorf("invalid Modbus config")
	}

	client := modbus.NewModbusClient(modbusConfig)
	device := &ModbusDevice{
		BaseDevice: NewBaseDevice(config),
		client:     client,
	}

	device.setCommunication(client)
	return device, nil
}

// SerialDevice 串口设备
type SerialDevice struct {
	*BaseDevice
	client *serial.SerialClient
}

// NewSerialDevice 创建串口设备
func NewSerialDevice(config DeviceConfig) (*SerialDevice, error) {
	serialConfig, ok := config.Config.(serial.SerialConfig)
	if !ok {
		return nil, fmt.Errorf("invalid serial config")
	}

	client := serial.NewSerialClient(serialConfig)
	device := &SerialDevice{
		BaseDevice: NewBaseDevice(config),
		client:     client,
	}

	device.setCommunication(client)
	return device, nil
}

// CustomDevice 自定义协议设备
type CustomDevice struct {
	*BaseDevice
	client *custom.CustomProtocolClient
}

// NewCustomDevice 创建自定义协议设备
func NewCustomDevice(config DeviceConfig) (*CustomDevice, error) {
	customConfig, ok := config.Config.(custom.CustomProtocolConfig)
	if !ok {
		return nil, fmt.Errorf("invalid custom protocol config")
	}

	client := custom.NewCustomProtocolClient(customConfig)
	device := &CustomDevice{
		BaseDevice: NewBaseDevice(config),
		client:     client,
	}

	// 注册命令和响应定义
	if commands, ok := config.Properties["commands"].([]custom.CommandDefinition); ok {
		for _, cmd := range commands {
			client.RegisterCommand(cmd)
		}
	}

	if responses, ok := config.Properties["responses"].([]custom.ResponseDefinition); ok {
		for _, resp := range responses {
			client.RegisterResponse(resp)
		}
	}

	device.setCommunication(client)
	return device, nil
}

// DeviceManager 设备管理器
type DeviceManager struct {
	devices    map[string]Device
	configs    map[string]DeviceConfig
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	eventChan  chan DeviceEvent
	autoScan   bool
	scanTicker *time.Ticker
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	Type      string      `json:"type"`
	DeviceID  string      `json:"device_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager() *DeviceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeviceManager{
		devices:   make(map[string]Device),
		configs:   make(map[string]DeviceConfig),
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan DeviceEvent, 100),
	}
}

// AddDevice 添加设备
func (dm *DeviceManager) AddDevice(config DeviceConfig) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查设备是否已存在
	if _, exists := dm.devices[config.ID]; exists {
		return fmt.Errorf("device %s already exists", config.ID)
	}

	// 保存配置
	dm.configs[config.ID] = config

	// 创建设备实例
	device, err := dm.createDevice(config)
	if err != nil {
		return fmt.Errorf("failed to create device %s: %w", config.ID, err)
	}

	// 添加事件处理器
	device.AddEventHandler(dm)

	dm.devices[config.ID] = device

	// 如果启用了自动连接，立即连接
	if config.AutoConnect {
		go func() {
			ctx, cancel := context.WithTimeout(dm.ctx, config.Timeout)
			defer cancel()
			if err := device.Connect(ctx); err != nil {
				log.Printf("Failed to auto-connect device %s: %v", config.ID, err)
			}
		}()
	}

	// 发送设备添加事件
	dm.emitEvent(DeviceEvent{
		Type:      "device_added",
		DeviceID:  config.ID,
		Timestamp: time.Now(),
		Data:      config,
	})

	log.Printf("Device %s (%s) added successfully", config.ID, config.Name)
	return nil
}

// RemoveDevice 移除设备
func (dm *DeviceManager) RemoveDevice(deviceID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	// 断开连接
	if device.IsConnected() {
		if err := device.Disconnect(dm.ctx); err != nil {
			log.Printf("Failed to disconnect device %s: %v", deviceID, err)
		}
	}

	// 移除设备
	delete(dm.devices, deviceID)
	delete(dm.configs, deviceID)

	// 发送设备移除事件
	dm.emitEvent(DeviceEvent{
		Type:      "device_removed",
		DeviceID:  deviceID,
		Timestamp: time.Now(),
	})

	log.Printf("Device %s removed successfully", deviceID)
	return nil
}

// GetDevice 获取设备
func (dm *DeviceManager) GetDevice(deviceID string) (Device, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

// ListDevices 列出所有设备
func (dm *DeviceManager) ListDevices() []Device {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	devices := make([]Device, 0, len(dm.devices))
	for _, device := range dm.devices {
		devices = append(devices, device)
	}

	return devices
}

// ListDeviceConfigs 列出所有设备配置
func (dm *DeviceManager) ListDeviceConfigs() []DeviceConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	configs := make([]DeviceConfig, 0, len(dm.configs))
	for _, config := range dm.configs {
		configs = append(configs, config)
	}

	return configs
}

// ConnectDevice 连接设备
func (dm *DeviceManager) ConnectDevice(deviceID string) error {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return err
	}

	config := device.GetConfig()
	ctx, cancel := context.WithTimeout(dm.ctx, config.Timeout)
	defer cancel()

	return device.Connect(ctx)
}

// DisconnectDevice 断开设备
func (dm *DeviceManager) DisconnectDevice(deviceID string) error {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return err
	}

	return device.Disconnect(dm.ctx)
}

// ReconnectDevice 重连设备
func (dm *DeviceManager) ReconnectDevice(deviceID string) error {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return err
	}

	config := device.GetConfig()
	ctx, cancel := context.WithTimeout(dm.ctx, config.Timeout)
	defer cancel()

	return device.Reconnect(ctx)
}

// ReadDevice 读取设备数据
func (dm *DeviceManager) ReadDevice(deviceID, address string, length int) ([]byte, error) {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return nil, err
	}

	config := device.GetConfig()
	ctx, cancel := context.WithTimeout(dm.ctx, config.Timeout)
	defer cancel()

	return device.Read(ctx, address, length)
}

// WriteDevice 写入设备数据
func (dm *DeviceManager) WriteDevice(deviceID, address string, data []byte) error {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return err
	}

	config := device.GetConfig()
	ctx, cancel := context.WithTimeout(dm.ctx, config.Timeout)
	defer cancel()

	return device.Write(ctx, address, data)
}

// StartAutoScan 开始自动扫描
func (dm *DeviceManager) StartAutoScan(interval time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.autoScan {
		return
	}

	dm.autoScan = true
	dm.scanTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-dm.scanTicker.C:
				dm.scanDevices()
			case <-dm.ctx.Done():
				return
			}
		}
	}()

	log.Printf("Auto scan started with interval %v", interval)
}

// StopAutoScan 停止自动扫描
func (dm *DeviceManager) StopAutoScan() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if !dm.autoScan {
		return
	}

	dm.autoScan = false
	if dm.scanTicker != nil {
		dm.scanTicker.Stop()
		dm.scanTicker = nil
	}

	log.Printf("Auto scan stopped")
}

// scanDevices 扫描设备
func (dm *DeviceManager) scanDevices() {
	dm.mu.RLock()
	configs := make([]DeviceConfig, 0, len(dm.configs))
	for _, config := range dm.configs {
		configs = append(configs, config)
	}
	dm.mu.RUnlock()

	for _, config := range configs {
		device, err := dm.GetDevice(config.ID)
		if err != nil {
			continue
		}

		if !device.IsConnected() {
			// 尝试重连断开的设备
			go func(deviceID string) {
				if err := dm.ReconnectDevice(deviceID); err != nil {
					log.Printf("Failed to reconnect device %s: %v", deviceID, err)
				}
			}(config.ID)
		}
	}
}

// Events 获取事件通道
func (dm *DeviceManager) Events() <-chan DeviceEvent {
	return dm.eventChan
}

// emitEvent 发送事件
func (dm *DeviceManager) emitEvent(event DeviceEvent) {
	select {
	case dm.eventChan <- event:
	default:
		log.Printf("Event channel full, dropping event: %s", event.Type)
	}
}

// createDevice 创建设备实例
func (dm *DeviceManager) createDevice(config DeviceConfig) (Device, error) {
	switch config.Type {
	case DeviceTypeModbusTCP, DeviceTypeModbusRTU, DeviceTypeModbusASCII:
		return NewModbusDevice(config)
	case DeviceTypeSerial:
		return NewSerialDevice(config)
	case DeviceTypeCustom:
		return NewCustomDevice(config)
	default:
		return nil, fmt.Errorf("unsupported device type: %v", config.Type)
	}
}

// Stop 停止设备管理器
func (dm *DeviceManager) Stop() error {
	dm.cancel()

	// 断开所有设备连接
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, device := range dm.devices {
		if device.IsConnected() {
			if err := device.Disconnect(dm.ctx); err != nil {
				log.Printf("Failed to disconnect device %s: %v", device.GetID(), err)
			}
		}
	}

	// 停止自动扫描
	if dm.autoScan {
		dm.StopAutoScan()
	}

	close(dm.eventChan)
	return nil
}

// EventHandler 实现comm.EventHandler接口
func (dm *DeviceManager) OnConnected() {
	// 设备连接事件在具体设备处理
}

func (dm *DeviceManager) OnDisconnected() {
	// 设备断开事件在具体设备处理
}

func (dm *DeviceManager) OnError(err error) {
	// 设备错误事件在具体设备处理
}

func (dm *DeviceManager) OnDataReceived(address string, data []byte) {
	// 数据接收事件在具体设备处理
}

func (dm *DeviceManager) OnDataWritten(address string, data []byte) {
	// 数据发送事件在具体设备处理
}

// GetDeviceStatus 获取设备状态（兼容旧的DeviceStatus）
func (dm *DeviceManager) GetDeviceStatus(deviceID string) types.DeviceStatus {
	device, err := dm.GetDevice(deviceID)
	if err != nil {
		return types.DeviceStatus{
			ID:        types.DeviceID(deviceID),
			Connected: false,
			Error:     err,
			LastSeen:  time.Now(),
		}
	}

	// 转换状态
	connected := device.IsConnected()
	var deviceErr error
	if !connected && device.GetStatus() == comm.StatusError {
		deviceErr = device.GetCommunication().GetLastError()
	}

	return types.DeviceStatus{
		ID:        types.DeviceID(deviceID),
		Connected: connected,
		Error:     deviceErr,
		LastSeen:  time.Now(),
	}
}