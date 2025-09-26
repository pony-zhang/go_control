package serial

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"control/internal/hardware/comm"

	"github.com/jacobsa/go-serial/serial"
)

// SerialConfig 串口配置
type SerialConfig struct {
	comm.ConnectionConfig `yaml:",inline"`

	PortName     string `yaml:"port_name"`      // 串口名称，如 "/dev/ttyUSB0", "COM1"
	BaudRate     int    `yaml:"baud_rate"`      // 波特率
	DataBits     int    `yaml:"data_bits"`      // 数据位，通常为8
	StopBits     int    `yaml:"stop_bits"`      // 停止位，通常为1
	Parity       string `yaml:"parity"`         // 校验位: "N"无校验, "E"偶校验, "O"奇校验
	Timeout      string `yaml:"timeout"`        // 超时时间
	FlowControl  bool   `yaml:"flow_control"`   // 流控制
	RS485        bool   `yaml:"rs485"`          // RS485模式
}

// SerialClient 串口客户端
type SerialClient struct {
	*comm.BaseCommunication
	config   SerialConfig
	port     io.ReadWriteCloser
	mu       sync.Mutex
	stopChan chan struct{}
}

// NewSerialClient 创建串口客户端
func NewSerialClient(config SerialConfig) *SerialClient {
	base := comm.NewBaseCommunication(config.ConnectionConfig)
	return &SerialClient{
		BaseCommunication: base,
		config:            config,
		stopChan:          make(chan struct{}),
	}
}

// Connect 连接串口设备
func (sc *SerialClient) Connect(ctx context.Context) error {
	sc.SetStatus(comm.StatusConnecting)

	// 配置串口参数
	mode := &serial.OpenOptions{
		PortName:        sc.config.PortName,
		BaudRate:        uint(sc.config.BaudRate),
		DataBits:        uint(sc.config.DataBits),
		StopBits:        uint(sc.config.StopBits),
		MinimumReadSize: 1,
	}

	// 设置校验位
	switch sc.config.Parity {
	case "N", "n":
		mode.ParityMode = serial.PARITY_NONE
	case "E", "e":
		mode.ParityMode = serial.PARITY_EVEN
	case "O", "o":
		mode.ParityMode = serial.PARITY_ODD
	default:
		mode.ParityMode = serial.PARITY_NONE
	}

	// 设置流控制
	if sc.config.FlowControl {
		mode.RTSCTSFlowControl = true
	}

	// 打开串口
	port, err := serial.Open(*mode)
	if err != nil {
		sc.SetStatus(comm.StatusError)
		return fmt.Errorf("failed to open serial port %s: %w", sc.config.PortName, err)
	}

	sc.mu.Lock()
	sc.port = port
	sc.mu.Unlock()

	sc.SetStatus(comm.StatusConnected)
	sc.EmitConnected()

	// 启动数据监听
	go sc.listenForData()

	return nil
}

// Disconnect 断开连接
func (sc *SerialClient) Disconnect(ctx context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.port != nil {
		close(sc.stopChan)
		err := sc.port.Close()
		sc.port = nil
		sc.stopChan = make(chan struct{})

		if err != nil {
			return sc.HandleWithError(fmt.Errorf("failed to close serial port: %w", err))
		}
	}

	sc.SetStatus(comm.StatusDisconnected)
	sc.EmitDisconnected()
	return nil
}

// Reconnect 重连
func (sc *SerialClient) Reconnect(ctx context.Context) error {
	if err := sc.Disconnect(ctx); err != nil {
		return err
	}
	return sc.Connect(ctx)
}

// Read 读取数据
func (sc *SerialClient) Read(ctx context.Context, address string, length int) ([]byte, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("serial client not connected")
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.port == nil {
		return nil, fmt.Errorf("serial port not open")
	}

	// 解析地址（对于串口，地址可能是指令或寄存器地址）
	command := sc.buildReadCommand(address, length)
	if len(command) == 0 {
		return nil, fmt.Errorf("invalid read address: %s", address)
	}

	// 发送读取指令
	_, err := sc.port.Write(command)
	if err != nil {
		return nil, sc.HandleWithError(fmt.Errorf("failed to write read command: %w", err))
	}

	// 读取响应
	response := make([]byte, length+2) // +2 for possible checksum/response bytes
	n, err := sc.port.Read(response)
	if err != nil {
		return nil, sc.HandleWithError(fmt.Errorf("failed to read response: %w", err))
	}

	// 处理响应数据
	data := sc.processReadResponse(response[:n], address, length)
	sc.EmitDataReceived(address, data)
	return data, nil
}

// Write 写入数据
func (sc *SerialClient) Write(ctx context.Context, address string, data []byte) error {
	if !sc.IsConnected() {
		return fmt.Errorf("serial client not connected")
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.port == nil {
		return fmt.Errorf("serial port not open")
	}

	// 构建写入指令
	command := sc.buildWriteCommand(address, data)
	if len(command) == 0 {
		return fmt.Errorf("invalid write address: %s", address)
	}

	// 发送数据
	_, err := sc.port.Write(command)
	if err != nil {
		return sc.HandleWithError(fmt.Errorf("failed to write data: %w", err))
	}

	sc.EmitDataWritten(address, data)
	return nil
}

// BulkRead 批量读取
func (sc *SerialClient) BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error) {
	results := make(map[string][]byte)

	for _, addr := range addresses {
		data, err := sc.Read(ctx, addr, 1) // 默认读取1字节
		if err != nil {
			return nil, err
		}
		results[addr] = data
	}

	return results, nil
}

// BulkWrite 批量写入
func (sc *SerialClient) BulkWrite(ctx context.Context, data map[string][]byte) error {
	for address, value := range data {
		if err := sc.Write(ctx, address, value); err != nil {
			return err
		}
	}
	return nil
}

// listenForData 监听串口数据
func (sc *SerialClient) listenForData() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-sc.stopChan:
			return
		case <-sc.Context().Done():
			return
		default:
			sc.mu.Lock()
			if sc.port == nil {
				sc.mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// 设置读取超时
			n, err := sc.port.Read(buffer)
			sc.mu.Unlock()

			if err != nil {
				if err != io.EOF {
					sc.HandleWithError(fmt.Errorf("serial read error: %w", err))
				}
				continue
			}

			if n > 0 {
				// 处理接收到的数据
				data := make([]byte, n)
				copy(data, buffer[:n])
				sc.EmitDataReceived("unknown", data)
			}
		}
	}
}

// buildReadCommand 构建读取指令
func (sc *SerialClient) buildReadCommand(address string, length int) []byte {
	// 这里根据具体的串口协议实现
	// 例如：Modbus RTU协议、自定义协议等

	// 简化实现：构建Modbus RTU读取指令
	addr, err := strconv.Atoi(address)
	if err != nil {
		return nil
	}

	// Modbus RTU读取保持寄存器指令
	command := []byte{
		byte(1), // 从站地址 - TODO: Add SlaveID to SerialConfig
		0x03,                   // 功能码：读保持寄存器
		byte(addr >> 8),        // 起始地址高字节
		byte(addr & 0xFF),      // 起始地址低字节
		byte(length >> 8),      // 寄存器数量高字节
		byte(length & 0xFF),    // 寄存器数量低字节
	}

	// 添加CRC校验
	crc := sc.calculateCRC(command)
	command = append(command, byte(crc&0xFF), byte(crc>>8))

	return command
}

// buildWriteCommand 构建写入指令
func (sc *SerialClient) buildWriteCommand(address string, data []byte) []byte {
	// 简化实现：构建Modbus RTU写入指令
	addr, err := strconv.Atoi(address)
	if err != nil {
		return nil
	}

	// Modbus RTU写入多个寄存器指令
	command := []byte{
		byte(1), // 从站地址 - TODO: Add SlaveID to SerialConfig
		0x10,                   // 功能码：写多个寄存器
		byte(addr >> 8),        // 起始地址高字节
		byte(addr & 0xFF),      // 起始地址低字节
		byte(len(data) >> 1),   // 寄存器数量高字节
		byte(len(data) & 1),    // 寄存器数量低字节
		byte(len(data)),        // 字节数
	}

	// 添加数据
	command = append(command, data...)

	// 添加CRC校验
	crc := sc.calculateCRC(command)
	command = append(command, byte(crc&0xFF), byte(crc>>8))

	return command
}

// processReadResponse 处理读取响应
func (sc *SerialClient) processReadResponse(response []byte, address string, length int) []byte {
	// 这里根据具体的协议解析响应数据
	// 简化实现：假设响应格式为 [slave_id, function_code, data..., crc_low, crc_high]

	if len(response) < 5 {
		return nil
	}

	// 检查CRC
	crc := sc.calculateCRC(response[:len(response)-2])
	if byte(crc&0xFF) != response[len(response)-2] || byte(crc>>8) != response[len(response)-1] {
		return nil // CRC校验失败
	}

	// 返回数据部分（去掉从站地址、功能码和CRC）
	return response[2 : len(response)-2]
}

// calculateCRC 计算CRC16校验值
func (sc *SerialClient) calculateCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// GetConfig 获取配置
func (sc *SerialClient) GetConfig() interface{} {
	return sc.config
}

// SetConfig 设置配置
func (sc *SerialClient) SetConfig(config interface{}) error {
	if cfg, ok := config.(SerialConfig); ok {
		sc.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type for SerialClient")
}

// SerialDiscovery 串口设备发现
type SerialDiscovery struct {
	config SerialConfig
}

// NewSerialDiscovery 创建串口设备发现器
func NewSerialDiscovery(config SerialConfig) *SerialDiscovery {
	return &SerialDiscovery{
		config: config,
	}
}

// DiscoverDevices 发现串口设备
func (sd *SerialDiscovery) DiscoverDevices(ctx context.Context, timeout time.Duration) ([]comm.DeviceInfo, error) {
	// 这里可以实现串口扫描
	// 简化实现，返回已知设备
	return []comm.DeviceInfo{
		{
			ID:      "serial-device-001",
			Address: sd.config.PortName,
			Type:    "serial",
			Model:   "Generic Serial Device",
			Properties: map[string]interface{}{
				"baud_rate":    sd.config.BaudRate,
				"data_bits":    sd.config.DataBits,
				"stop_bits":    sd.config.StopBits,
				"parity":       sd.config.Parity,
				"flow_control": sd.config.FlowControl,
			},
		},
	}, nil
}

// PingDevice 测试设备连通性
func (sd *SerialDiscovery) PingDevice(ctx context.Context, address string) (bool, error) {
	client := NewSerialClient(sd.config)

	err := client.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)

	// 尝试读取一个字节来测试连通性
	_, err = client.Read(ctx, "0", 1)
	return err == nil, nil
}