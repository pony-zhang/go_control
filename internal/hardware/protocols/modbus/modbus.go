// Package modbus implements Modbus protocol support for industrial automation
package modbus

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"control/internal/hardware/comm"

	"github.com/goburrow/modbus"
)

// modbusBytesToUint16 converts []byte to []uint16 for Modbus operations
func modbusBytesToUint16(data []byte) []uint16 {
	if len(data)%2 != 0 {
		// Pad with zero if odd length
		data = append(data, 0)
	}

	result := make([]uint16, len(data)/2)
	for i := 0; i < len(result); i++ {
		result[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
	}
	return result
}

// ModbusConfig Modbus配置
type ModbusConfig struct {
	comm.ConnectionConfig `yaml:",inline"`

	Type        string `yaml:"type"`        // "tcp", "rtu", "ascii"
	Address     string `yaml:"address"`     // TCP地址或串口路径
	Port        int    `yaml:"port"`        // TCP端口
	BaudRate    int    `yaml:"baud_rate"`   // 串口波特率
	DataBits    int    `yaml:"data_bits"`   // 数据位
	StopBits    int    `yaml:"stop_bits"`   // 停止位
	Parity      string `yaml:"parity"`      // 校验位: "N", "E", "O"
	SlaveID     byte   `yaml:"slave_id"`    // 从站ID
	Timeout     string `yaml:"timeout"`     // 超时时间字符串
}

// ModbusClient Modbus客户端实现
type ModbusClient struct {
	*comm.BaseCommunication
	config    ModbusConfig
	client    modbus.Client
	handler   modbus.ClientHandler
}

// NewModbusClient 创建Modbus客户端
func NewModbusClient(config ModbusConfig) *ModbusClient {
	base := comm.NewBaseCommunication(config.ConnectionConfig)
	return &ModbusClient{
		BaseCommunication: base,
		config:            config,
	}
}

// Connect 连接到Modbus设备
func (mc *ModbusClient) Connect(ctx context.Context) error {
	mc.SetStatus(comm.StatusConnecting)

	var err error

	switch mc.config.Type {
	case "tcp":
		err = mc.connectTCP(ctx)
	case "rtu":
		err = mc.connectRTU(ctx)
	case "ascii":
		err = mc.connectASCII(ctx)
	default:
		return fmt.Errorf("unsupported Modbus type: %s", mc.config.Type)
	}

	if err != nil {
		mc.SetStatus(comm.StatusError)
		return mc.HandleWithError(err)
	}

	mc.SetStatus(comm.StatusConnected)
	mc.EmitConnected()
	return nil
}

// connectTCP 连接TCP Modbus
func (mc *ModbusClient) connectTCP(ctx context.Context) error {
	address := fmt.Sprintf("%s:%d", mc.config.Address, mc.config.Port)
	handler := modbus.NewTCPClientHandler(address)

	// 配置超时
	timeout, err := time.ParseDuration(mc.config.Timeout)
	if err != nil {
		timeout = 10 * time.Second
	}
	handler.Timeout = timeout
	// handler.Logger = modbus.StdLogger // No logger field in newer versions

	if err := handler.Connect(); err != nil {
		return fmt.Errorf("failed to connect TCP Modbus: %w", err)
	}

	mc.handler = handler
	mc.client = modbus.NewClient(handler)
	return nil
}

// connectRTU 连接RTU Modbus
func (mc *ModbusClient) connectRTU(ctx context.Context) error {
	handler := modbus.NewRTUClientHandler(mc.config.Address)
	handler.BaudRate = mc.config.BaudRate
	handler.DataBits = mc.config.DataBits
	handler.StopBits = mc.config.StopBits
	handler.Parity = mc.config.Parity
	handler.SlaveId = mc.config.SlaveID
	handler.Timeout = 5 * time.Second
	// handler.Logger = modbus.StdLogger // No logger field in newer versions

	if err := handler.Connect(); err != nil {
		return fmt.Errorf("failed to connect RTU Modbus: %w", err)
	}

	mc.handler = handler
	mc.client = modbus.NewClient(handler)
	return nil
}

// connectASCII 连接ASCII Modbus
func (mc *ModbusClient) connectASCII(ctx context.Context) error {
	handler := modbus.NewASCIIClientHandler(mc.config.Address)
	handler.BaudRate = mc.config.BaudRate
	handler.DataBits = mc.config.DataBits
	handler.StopBits = mc.config.StopBits
	handler.Parity = mc.config.Parity
	handler.SlaveId = mc.config.SlaveID
	handler.Timeout = 5 * time.Second
	// handler.Logger = modbus.StdLogger // No logger field in newer versions

	if err := handler.Connect(); err != nil {
		return fmt.Errorf("failed to connect ASCII Modbus: %w", err)
	}

	mc.handler = handler
	mc.client = modbus.NewClient(handler)
	return nil
}

// Disconnect 断开连接
func (mc *ModbusClient) Disconnect(ctx context.Context) error {
	if mc.handler != nil {
		// mc.handler.Close() // ClientHandler interface doesn't have Close method
		mc.handler = nil
		mc.client = nil
	}

	mc.SetStatus(comm.StatusDisconnected)
	mc.EmitDisconnected()
	return nil
}

// Reconnect 重连
func (mc *ModbusClient) Reconnect(ctx context.Context) error {
	if err := mc.Disconnect(ctx); err != nil {
		return err
	}
	return mc.Connect(ctx)
}

// Read 读取数据
func (mc *ModbusClient) Read(ctx context.Context, address string, length int) ([]byte, error) {
	if !mc.IsConnected() {
		return nil, fmt.Errorf("Modbus client not connected")
	}

	// 解析地址
	addr, quantity, err := mc.parseAddress(address, length)
	if err != nil {
		return nil, err
	}

	// 执行读取操作
	var results []byte
	err = mc.RetryWithTimeout(ctx, func() error {
		var err error
		results, err = mc.client.ReadHoldingRegisters(addr, quantity)
		return err
	})

	if err != nil {
		return nil, mc.HandleWithError(err)
	}

	mc.EmitDataReceived(address, results)
	return results, nil
}

// Write 写入数据
func (mc *ModbusClient) Write(ctx context.Context, address string, data []byte) error {
	if !mc.IsConnected() {
		return fmt.Errorf("Modbus client not connected")
	}

	// 解析地址
	addr, _, err := mc.parseAddress(address, len(data)/2)
	if err != nil {
		return err
	}

	// 将[]byte转换为[]uint16
	values := make([]uint16, len(data)/2)
	for i := 0; i < len(values); i++ {
		values[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
	}

	// 执行写入操作 - convert uint16 values to bytes for Modbus
	byteValues := make([]byte, len(values)*2)
	for i, v := range values {
		byteValues[i*2] = byte(v >> 8)
		byteValues[i*2+1] = byte(v & 0xFF)
	}

	err = mc.RetryWithTimeout(ctx, func() error {
		_, err := mc.client.WriteMultipleRegisters(addr, uint16(len(values)), byteValues)
		return err
	})

	if err != nil {
		return mc.HandleWithError(err)
	}

	mc.EmitDataWritten(address, data)
	return nil
}

// BulkRead 批量读取
func (mc *ModbusClient) BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error) {
	results := make(map[string][]byte)

	for _, addr := range addresses {
		data, err := mc.Read(ctx, addr, 1) // 默认读取1个寄存器
		if err != nil {
			return nil, err
		}
		results[addr] = data
	}

	return results, nil
}

// BulkWrite 批量写入
func (mc *ModbusClient) BulkWrite(ctx context.Context, data map[string][]byte) error {
	for address, value := range data {
		if err := mc.Write(ctx, address, value); err != nil {
			return err
		}
	}
	return nil
}

// parseAddress 解析Modbus地址
func (mc *ModbusClient) parseAddress(address string, length int) (addr, quantity uint16, err error) {
	// 支持多种地址格式：
	// "40001" - 保持寄存器，地址1
	// " Holding:1" - 保持寄存器，地址1
	// "Input:100" - 输入寄存器，地址100

	// 简单实现，假设都是保持寄存器
	addrInt, err := strconv.Atoi(address)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Modbus address: %s", address)
	}

	// Modbus地址从0开始，但用户通常从1开始
	if addrInt > 0 {
		addrInt--
	}

	return uint16(addrInt), uint16(length), nil
}

// GetConfig 获取配置
func (mc *ModbusClient) GetConfig() interface{} {
	return mc.config
}

// SetConfig 设置配置
func (mc *ModbusClient) SetConfig(config interface{}) error {
	if cfg, ok := config.(ModbusConfig); ok {
		mc.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type for ModbusClient")
}

// ModbusDiscovery Modbus设备发现
type ModbusDiscovery struct {
	config ModbusConfig
}

// NewModbusDiscovery 创建Modbus设备发现器
func NewModbusDiscovery(config ModbusConfig) *ModbusDiscovery {
	return &ModbusDiscovery{
		config: config,
	}
}

// DiscoverDevices 发现Modbus设备
func (md *ModbusDiscovery) DiscoverDevices(ctx context.Context, timeout time.Duration) ([]comm.DeviceInfo, error) {
	var devices []comm.DeviceInfo

	// TCP设备发现
	if md.config.Type == "tcp" {
		devices = append(devices, md.discoverTCPDevices(ctx, timeout)...)
	}

	// 串口设备发现
	if md.config.Type == "rtu" || md.config.Type == "ascii" {
		devices = append(devices, md.discoverSerialDevices(ctx, timeout)...)
	}

	return devices, nil
}

// discoverTCPDevices 发现TCP Modbus设备
func (md *ModbusDiscovery) discoverTCPDevices(ctx context.Context, timeout time.Duration) []comm.DeviceInfo {
	// 这里可以实现TCP网络扫描
	// 简化实现，返回已知设备
	return []comm.DeviceInfo{
		{
			ID:      "modbus-tcp-001",
			Address: fmt.Sprintf("%s:%d", md.config.Address, md.config.Port),
			Type:    "modbus-tcp",
			Model:   "Generic Modbus TCP Device",
			Properties: map[string]interface{}{
				"type":     "tcp",
				"slave_id": md.config.SlaveID,
			},
		},
	}
}

// discoverSerialDevices 发现串口Modbus设备
func (md *ModbusDiscovery) discoverSerialDevices(ctx context.Context, timeout time.Duration) []comm.DeviceInfo {
	return []comm.DeviceInfo{
		{
			ID:      "modbus-serial-001",
			Address: md.config.Address,
			Type:    "modbus-serial",
			Model:   "Generic Modbus Serial Device",
			Properties: map[string]interface{}{
				"type":      md.config.Type,
				"baud_rate": md.config.BaudRate,
				"slave_id":  md.config.SlaveID,
			},
		},
	}
}

// PingDevice 测试设备连通性
func (md *ModbusDiscovery) PingDevice(ctx context.Context, address string) (bool, error) {
	client := NewModbusClient(md.config)

	err := client.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)

	// 尝试读取一个寄存器来测试连通性
	_, err = client.Read(ctx, "0", 1)
	return err == nil, nil
}