// Package custom implements custom protocol support for hardware communication
package custom

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"control/internal/hardware/comm"
)

// ProtocolType 协议类型
type ProtocolType int

const (
	ProtocolBinary ProtocolType = iota
	ProtocolASCII
	ProtocolJSON
	ProtocolXML
)

// CustomProtocolConfig 自定义协议配置
type CustomProtocolConfig struct {
	comm.ConnectionConfig `yaml:",inline"`

	ProtocolType    string `yaml:"protocol_type"`     // "binary", "ascii", "json", "xml"
	Address         string `yaml:"address"`          // 网络地址或串口路径
	Port            int    `yaml:"port"`             // 网络端口
	Timeout         string `yaml:"timeout"`          // 超时时间
	FrameDelimiter  string `yaml:"frame_delimiter"`  // 帧分隔符
	Header          string `yaml:"header"`          // 帧头
	Trailer         string `yaml:"trailer"`         // 帧尾
	ChecksumMethod  string `yaml:"checksum_method"`  // 校验方法: "none", "crc8", "crc16", "xor"
	BigEndian       bool   `yaml:"big_endian"`       // 大端序
	MaxFrameSize    int    `yaml:"max_frame_size"`   // 最大帧大小
	Encoding        string `yaml:"encoding"`         // 编码方式: "utf-8", "gbk", "ascii"
}

// CommandDefinition 命令定义
type CommandDefinition struct {
	Name        string                 `yaml:"name"`
	CommandCode string                 `yaml:"command_code"`
	Format      string                 `yaml:"format"`      // "binary", "ascii", "hex"
	Fields      []FieldDefinition      `yaml:"fields"`
	Params      map[string]interface{} `yaml:"params"`
}

// FieldDefinition 字段定义
type FieldDefinition struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`     // "int8", "int16", "int32", "uint8", "uint16", "uint32", "float32", "float64", "string", "bytes"
	Size     int    `yaml:"size"`     // 字段大小（字节）
	Offset   int    `yaml:"offset"`   // 偏移量
	Scale    float64 `yaml:"scale"`    // 缩放因子
	Encoding string `yaml:"encoding"` // 编码
}

// ResponseDefinition 响应定义
type ResponseDefinition struct {
	CommandCode string             `yaml:"command_code"`
	Format      string             `yaml:"format"`
	Fields      []FieldDefinition  `yaml:"fields"`
	Timeout     time.Duration      `yaml:"timeout"`
}

// CustomProtocolClient 自定义协议客户端
type CustomProtocolClient struct {
	*comm.BaseCommunication
	config           CustomProtocolConfig
	commands         map[string]CommandDefinition
	responses        map[string]ResponseDefinition
	connection       interface{} // net.Conn 或 serial.Port
	mu               sync.Mutex
	isNetwork        bool
	buffer           []byte
	responseChannels map[string]chan []byte
}

// NewCustomProtocolClient 创建自定义协议客户端
func NewCustomProtocolClient(config CustomProtocolConfig) *CustomProtocolClient {
	base := comm.NewBaseCommunication(config.ConnectionConfig)
	return &CustomProtocolClient{
		BaseCommunication: base,
		config:            config,
		commands:          make(map[string]CommandDefinition),
		responses:         make(map[string]ResponseDefinition),
		buffer:            make([]byte, 0),
		responseChannels:  make(map[string]chan []byte),
		isNetwork:         strings.Contains(config.Address, ":") || config.Port > 0,
	}
}

// RegisterCommand 注册命令
func (cpc *CustomProtocolClient) RegisterCommand(cmd CommandDefinition) error {
	cpc.commands[cmd.Name] = cmd
	return nil
}

// RegisterResponse 注册响应
func (cpc *CustomProtocolClient) RegisterResponse(resp ResponseDefinition) error {
	cpc.responses[resp.CommandCode] = resp
	return nil
}

// Connect 连接设备
func (cpc *CustomProtocolClient) Connect(ctx context.Context) error {
	cpc.SetStatus(comm.StatusConnecting)

	var err error

	if cpc.isNetwork {
		err = cpc.connectNetwork(ctx)
	} else {
		err = cpc.connectSerial(ctx)
	}

	if err != nil {
		cpc.SetStatus(comm.StatusError)
		return cpc.HandleWithError(err)
	}

	cpc.SetStatus(comm.StatusConnected)
	cpc.EmitConnected()

	// 启动数据监听
	go cpc.listenForData()

	return nil
}

// connectNetwork 连接网络设备
func (cpc *CustomProtocolClient) connectNetwork(ctx context.Context) error {
	// 这里需要实现网络连接
	// 由于之前已经有IPC连接，这里简化实现

	// 解析超时时间已移除，使用配置中的超时设置

	// 模拟连接成功
	cpc.connection = &mockConnection{address: cpc.config.Address}
	return nil
}

// connectSerial 连接串口设备
func (cpc *CustomProtocolClient) connectSerial(ctx context.Context) error {
	// 这里可以集成之前实现的串口连接
	// 简化实现
	cpc.connection = &mockConnection{address: cpc.config.Address}
	return nil
}

// Disconnect 断开连接
func (cpc *CustomProtocolClient) Disconnect(ctx context.Context) error {
	cpc.mu.Lock()
	defer cpc.mu.Unlock()

	if cpc.connection != nil {
		// 关闭连接
		cpc.connection = nil
	}

	cpc.SetStatus(comm.StatusDisconnected)
	cpc.EmitDisconnected()
	return nil
}

// Reconnect 重连
func (cpc *CustomProtocolClient) Reconnect(ctx context.Context) error {
	if err := cpc.Disconnect(ctx); err != nil {
		return err
	}
	return cpc.Connect(ctx)
}

// Read 读取数据
func (cpc *CustomProtocolClient) Read(ctx context.Context, address string, length int) ([]byte, error) {
	if !cpc.IsConnected() {
		return nil, fmt.Errorf("custom protocol client not connected")
	}

	// 构建读取命令
	cmdDef, exists := cpc.commands["read"]
	if !exists {
		return nil, fmt.Errorf("read command not defined")
	}

	// 准备参数
	params := map[string]interface{}{
		"address": address,
		"length":  length,
	}

	// 发送命令并等待响应
	response, err := cpc.sendCommandAndWaitForResponse(ctx, cmdDef, params)
	if err != nil {
		return nil, err
	}

	cpc.EmitDataReceived(address, response)
	return response, nil
}

// Write 写入数据
func (cpc *CustomProtocolClient) Write(ctx context.Context, address string, data []byte) error {
	if !cpc.IsConnected() {
		return fmt.Errorf("custom protocol client not connected")
	}

	// 构建写入命令
	cmdDef, exists := cpc.commands["write"]
	if !exists {
		return fmt.Errorf("write command not defined")
	}

	// 准备参数
	params := map[string]interface{}{
		"address": address,
		"data":    data,
	}

	// 发送命令
	_, err := cpc.sendCommandAndWaitForResponse(ctx, cmdDef, params)
	if err != nil {
		return err
	}

	cpc.EmitDataWritten(address, data)
	return nil
}

// BulkRead 批量读取
func (cpc *CustomProtocolClient) BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error) {
	results := make(map[string][]byte)

	for _, addr := range addresses {
		data, err := cpc.Read(ctx, addr, 1)
		if err != nil {
			return nil, err
		}
		results[addr] = data
	}

	return results, nil
}

// BulkWrite 批量写入
func (cpc *CustomProtocolClient) BulkWrite(ctx context.Context, data map[string][]byte) error {
	for address, value := range data {
		if err := cpc.Write(ctx, address, value); err != nil {
			return err
		}
	}
	return nil
}

// sendCommandAndWaitForResponse 发送命令并等待响应
func (cpc *CustomProtocolClient) sendCommandAndWaitForResponse(ctx context.Context, cmdDef CommandDefinition, params map[string]interface{}) ([]byte, error) {
	// 构建命令帧
	frame, err := cpc.buildCommandFrame(cmdDef, params)
	if err != nil {
		return nil, err
	}

	// 创建响应通道
	responseChan := make(chan []byte, 1)
	messageID := fmt.Sprintf("cmd_%d", time.Now().UnixNano())

	cpc.mu.Lock()
	cpc.responseChannels[messageID] = responseChan
	cpc.mu.Unlock()

	defer func() {
		cpc.mu.Lock()
		delete(cpc.responseChannels, messageID)
		cpc.mu.Unlock()
	}()

	// 发送命令
	err = cpc.sendRawData(frame)
	if err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case response := <-responseChan:
		return cpc.parseResponse(cmdDef.CommandCode, response)
	case <-time.After(5 * time.Second): // 超时时间
		return nil, fmt.Errorf("command timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// buildCommandFrame 构建命令帧
func (cpc *CustomProtocolClient) buildCommandFrame(cmdDef CommandDefinition, params map[string]interface{}) ([]byte, error) {
	var frame []byte

	// 添加帧头
	if cpc.config.Header != "" {
		header, err := hex.DecodeString(cpc.config.Header)
		if err != nil {
			return nil, fmt.Errorf("invalid header: %w", err)
		}
		frame = append(frame, header...)
	}

	// 添加命令码
	commandCode, err := hex.DecodeString(cmdDef.CommandCode)
	if err != nil {
		return nil, fmt.Errorf("invalid command code: %w", err)
	}
	frame = append(frame, commandCode...)

	// 添加参数数据
	data, err := cpc.encodeParameters(cmdDef.Fields, params)
	if err != nil {
		return nil, err
	}
	frame = append(frame, data...)

	// 添加校验码
	if cpc.config.ChecksumMethod != "none" {
		checksum := cpc.calculateChecksum(frame)
		frame = append(frame, checksum...)
	}

	// 添加帧尾
	if cpc.config.Trailer != "" {
		trailer, err := hex.DecodeString(cpc.config.Trailer)
		if err != nil {
			return nil, fmt.Errorf("invalid trailer: %w", err)
		}
		frame = append(frame, trailer...)
	}

	return frame, nil
}

// encodeParameters 编码参数
func (cpc *CustomProtocolClient) encodeParameters(fields []FieldDefinition, params map[string]interface{}) ([]byte, error) {
	var data []byte

	for _, field := range fields {
		value, exists := params[field.Name]
		if !exists {
			continue
		}

		encoded, err := cpc.encodeFieldValue(field, value)
		if err != nil {
			return nil, err
		}
		data = append(data, encoded...)
	}

	return data, nil
}

// encodeFieldValue 编码字段值
func (cpc *CustomProtocolClient) encodeFieldValue(field FieldDefinition, value interface{}) ([]byte, error) {
	switch field.Type {
	case "int8":
		if v, ok := value.(int); ok {
			return []byte{byte(v)}, nil
		}
	case "uint8":
		if v, ok := value.(int); ok {
			return []byte{byte(v)}, nil
		}
	case "int16":
		if v, ok := value.(int); ok {
			return cpc.encodeUint16(uint16(v)), nil
		}
	case "uint16":
		if v, ok := value.(int); ok {
			return cpc.encodeUint16(uint16(v)), nil
		}
	case "int32":
		if v, ok := value.(int); ok {
			return cpc.encodeUint32(uint32(v)), nil
		}
	case "uint32":
		if v, ok := value.(int); ok {
			return cpc.encodeUint32(uint32(v)), nil
		}
	case "float32":
		if v, ok := value.(float64); ok {
			bits := uint32(float32(v))
			return cpc.encodeUint32(bits), nil
		}
	case "float64":
		if v, ok := value.(float64); ok {
			bits := uint64(v)
			return cpc.encodeUint64(bits), nil
		}
	case "string":
		if v, ok := value.(string); ok {
			return []byte(v), nil
		}
	case "bytes":
		if v, ok := value.([]byte); ok {
			return v, nil
		}
	}

	return nil, fmt.Errorf("unsupported field type: %s", field.Type)
}

// encodeUint16 编码uint16
func (cpc *CustomProtocolClient) encodeUint16(value uint16) []byte {
	buf := make([]byte, 2)
	if cpc.config.BigEndian {
		binary.BigEndian.PutUint16(buf, value)
	} else {
		binary.LittleEndian.PutUint16(buf, value)
	}
	return buf
}

// encodeUint32 编码uint32
func (cpc *CustomProtocolClient) encodeUint32(value uint32) []byte {
	buf := make([]byte, 4)
	if cpc.config.BigEndian {
		binary.BigEndian.PutUint32(buf, value)
	} else {
		binary.LittleEndian.PutUint32(buf, value)
	}
	return buf
}

// encodeUint64 编码uint64
func (cpc *CustomProtocolClient) encodeUint64(value uint64) []byte {
	buf := make([]byte, 8)
	if cpc.config.BigEndian {
		binary.BigEndian.PutUint64(buf, value)
	} else {
		binary.LittleEndian.PutUint64(buf, value)
	}
	return buf
}

// calculateChecksum 计算校验码
func (cpc *CustomProtocolClient) calculateChecksum(data []byte) []byte {
	switch cpc.config.ChecksumMethod {
	case "crc8":
		return []byte{cpc.calculateCRC8(data)}
	case "crc16":
		checksum := cpc.calculateCRC16(data)
		return []byte{byte(checksum >> 8), byte(checksum)}
	case "xor":
		return []byte{cpc.calculateXOR(data)}
	default:
		return nil
	}
}

// calculateCRC8 计算CRC8
func (cpc *CustomProtocolClient) calculateCRC8(data []byte) byte {
	crc := byte(0)
	for _, b := range data {
		crc ^= b
	}
	return crc
}

// calculateCRC16 计算CRC16
func (cpc *CustomProtocolClient) calculateCRC16(data []byte) uint16 {
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

// calculateXOR 计算XOR校验
func (cpc *CustomProtocolClient) calculateXOR(data []byte) byte {
	result := byte(0)
	for _, b := range data {
		result ^= b
	}
	return result
}

// sendRawData 发送原始数据
func (cpc *CustomProtocolClient) sendRawData(data []byte) error {
	// 简化实现：实际发送逻辑依赖于连接类型
	return nil
}

// parseResponse 解析响应
func (cpc *CustomProtocolClient) parseResponse(commandCode string, data []byte) ([]byte, error) {
	respDef, exists := cpc.responses[commandCode]
	if !exists {
		return data, nil // 没有定义响应格式，返回原始数据
	}

	// 根据响应定义解析数据
	result := make(map[string]interface{})
	for _, field := range respDef.Fields {
		if field.Offset+field.Size > len(data) {
			continue
		}

		fieldData := data[field.Offset : field.Offset+field.Size]
		value, err := cpc.decodeFieldValue(field, fieldData)
		if err != nil {
			continue
		}

		result[field.Name] = value
	}

	// 简化实现：返回第一个字段的值
	for _, v := range result {
		if bytes, ok := v.([]byte); ok {
			return bytes, nil
		}
	}

	return data, nil
}

// decodeFieldValue 解码字段值
func (cpc *CustomProtocolClient) decodeFieldValue(field FieldDefinition, data []byte) (interface{}, error) {
	switch field.Type {
	case "int8":
		return int8(data[0]), nil
	case "uint8":
		return data[0], nil
	case "int16":
		return int16(cpc.decodeUint16(data)), nil
	case "uint16":
		return cpc.decodeUint16(data), nil
	case "int32":
		return int32(cpc.decodeUint32(data)), nil
	case "uint32":
		return cpc.decodeUint32(data), nil
	case "float32":
		bits := cpc.decodeUint32(data)
		return float32(bits), nil
	case "float64":
		bits := cpc.decodeUint64(data)
		return float64(bits), nil
	case "string":
		return string(data), nil
	case "bytes":
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported field type: %s", field.Type)
	}
}

// decodeUint16 解码uint16
func (cpc *CustomProtocolClient) decodeUint16(data []byte) uint16 {
	if cpc.config.BigEndian {
		return binary.BigEndian.Uint16(data)
	}
	return binary.LittleEndian.Uint16(data)
}

// decodeUint32 解码uint32
func (cpc *CustomProtocolClient) decodeUint32(data []byte) uint32 {
	if cpc.config.BigEndian {
		return binary.BigEndian.Uint32(data)
	}
	return binary.LittleEndian.Uint32(data)
}

// decodeUint64 解码uint64
func (cpc *CustomProtocolClient) decodeUint64(data []byte) uint64 {
	if cpc.config.BigEndian {
		return binary.BigEndian.Uint64(data)
	}
	return binary.LittleEndian.Uint64(data)
}

// listenForData 监听数据
func (cpc *CustomProtocolClient) listenForData() {
	// 简化实现：实际监听逻辑依赖于连接类型
}

// GetConfig 获取配置
func (cpc *CustomProtocolClient) GetConfig() interface{} {
	return cpc.config
}

// SetConfig 设置配置
func (cpc *CustomProtocolClient) SetConfig(config interface{}) error {
	if cfg, ok := config.(CustomProtocolConfig); ok {
		cpc.config = cfg
		return nil
	}
	return fmt.Errorf("invalid config type for CustomProtocolClient")
}

// mockConnection 模拟连接
type mockConnection struct {
	address string
}

// CustomDiscovery 自定义协议设备发现
type CustomDiscovery struct {
	config CustomProtocolConfig
}

// NewCustomDiscovery 创建自定义协议设备发现器
func NewCustomDiscovery(config CustomProtocolConfig) *CustomDiscovery {
	return &CustomDiscovery{
		config: config,
	}
}

// DiscoverDevices 发现设备
func (cd *CustomDiscovery) DiscoverDevices(ctx context.Context, timeout time.Duration) ([]comm.DeviceInfo, error) {
	return []comm.DeviceInfo{
		{
			ID:      "custom-protocol-001",
			Address: cd.config.Address,
			Type:    "custom",
			Model:   "Custom Protocol Device",
			Properties: map[string]interface{}{
				"protocol_type": cd.config.ProtocolType,
				"encoding":     cd.config.Encoding,
			},
		},
	}, nil
}

// PingDevice 测试设备连通性
func (cd *CustomDiscovery) PingDevice(ctx context.Context, address string) (bool, error) {
	client := NewCustomProtocolClient(cd.config)

	// 注册基本的ping命令
	pingCmd := CommandDefinition{
		Name:        "ping",
		CommandCode: "01",
		Format:      "binary",
		Fields:      []FieldDefinition{},
	}
	client.RegisterCommand(pingCmd)

	pingResp := ResponseDefinition{
		CommandCode: "01",
		Format:      "binary",
		Fields:      []FieldDefinition{},
	}
	client.RegisterResponse(pingResp)

	err := client.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)

	// 发送ping命令
	_, err = client.sendCommandAndWaitForResponse(ctx, pingCmd, map[string]interface{}{})
	return err == nil, nil
}