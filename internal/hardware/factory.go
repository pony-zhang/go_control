package hardware

import (
	"fmt"
	"strconv"
	"time"

	"control/internal/hardware/comm"
	"control/internal/hardware/devices"
	"control/internal/hardware/protocols/custom"
	"control/internal/hardware/protocols/modbus"
	"control/internal/hardware/protocols/serial"
	"control/pkg/types"
)

// HardwareFactory 硬件抽象层工厂
type HardwareFactory struct {
	deviceManager *devices.DeviceManager
}

// NewHardwareFactory 创建硬件工厂
func NewHardwareFactory() *HardwareFactory {
	return &HardwareFactory{
		deviceManager: devices.NewDeviceManager(),
	}
}

// GetDeviceManager 获取设备管理器
func (hf *HardwareFactory) GetDeviceManager() *devices.DeviceManager {
	return hf.deviceManager
}

// convertStringParamsToInterface converts string parameters to interface parameters
func convertStringParamsToInterface(params map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range params {
		result[k] = v
	}
	return result
}

// convertInterfaceParamsToString converts interface parameters to string parameters
func convertInterfaceParamsToString(params map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range params {
		switch val := v.(type) {
		case string:
			result[k] = val
		case int:
			result[k] = strconv.Itoa(val)
		case float64:
			result[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			result[k] = strconv.FormatBool(val)
		default:
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// CreateModbusConfig 创建Modbus配置
func (hf *HardwareFactory) CreateModbusConfig(deviceConfig types.DeviceConfig) (modbus.ModbusConfig, error) {
	// 从map中提取Modbus配置
	configMap := convertStringParamsToInterface(deviceConfig.Parameters)

	modbusConfig := modbus.ModbusConfig{
		ConnectionConfig: comm.ConnectionConfig{
			Timeout:       10 * time.Second,
			RetryCount:    3,
			RetryInterval: 1 * time.Second,
			KeepAlive:     true,
		},
	}

	// 提取基本配置
	if typ, ok := configMap["type"].(string); ok {
		modbusConfig.Type = typ
	}
	if address, ok := configMap["address"].(string); ok {
		modbusConfig.Address = address
	}
	if port, ok := configMap["port"].(int); ok {
		modbusConfig.Port = port
	}
	if baudRate, ok := configMap["baud_rate"].(int); ok {
		modbusConfig.BaudRate = baudRate
	}
	if dataBits, ok := configMap["data_bits"].(int); ok {
		modbusConfig.DataBits = dataBits
	}
	if stopBits, ok := configMap["stop_bits"].(int); ok {
		modbusConfig.StopBits = stopBits
	}
	if parity, ok := configMap["parity"].(string); ok {
		modbusConfig.Parity = parity
	}
	if slaveID, ok := configMap["slave_id"].(int); ok {
		modbusConfig.SlaveID = byte(slaveID)
	}
	if timeout, ok := configMap["timeout"].(string); ok {
		modbusConfig.Timeout = timeout
	}

	return modbusConfig, nil
}

// CreateSerialConfig 创建串口配置
func (hf *HardwareFactory) CreateSerialConfig(deviceConfig types.DeviceConfig) (serial.SerialConfig, error) {
	configMap := convertStringParamsToInterface(deviceConfig.Parameters)

	serialConfig := serial.SerialConfig{
		ConnectionConfig: comm.ConnectionConfig{
			Timeout:       5 * time.Second,
			RetryCount:    3,
			RetryInterval: 1 * time.Second,
			KeepAlive:     false,
		},
	}

	// 提取串口配置
	if portName, ok := configMap["port_name"].(string); ok {
		serialConfig.PortName = portName
	}
	if baudRate, ok := configMap["baud_rate"].(int); ok {
		serialConfig.BaudRate = baudRate
	}
	if dataBits, ok := configMap["data_bits"].(int); ok {
		serialConfig.DataBits = dataBits
	}
	if stopBits, ok := configMap["stop_bits"].(int); ok {
		serialConfig.StopBits = stopBits
	}
	if parity, ok := configMap["parity"].(string); ok {
		serialConfig.Parity = parity
	}
	if timeout, ok := configMap["timeout"].(string); ok {
		serialConfig.Timeout = timeout
	}
	if flowControl, ok := configMap["flow_control"].(bool); ok {
		serialConfig.FlowControl = flowControl
	}
	if rs485, ok := configMap["rs485"].(bool); ok {
		serialConfig.RS485 = rs485
	}

	return serialConfig, nil
}

// CreateCustomConfig 创建自定义协议配置
func (hf *HardwareFactory) CreateCustomConfig(deviceConfig types.DeviceConfig) (custom.CustomProtocolConfig, error) {
	configMap := convertStringParamsToInterface(deviceConfig.Parameters)

	customConfig := custom.CustomProtocolConfig{
		ConnectionConfig: comm.ConnectionConfig{
			Timeout:       5 * time.Second,
			RetryCount:    3,
			RetryInterval: 1 * time.Second,
			KeepAlive:     true,
		},
	}

	// 提取自定义协议配置
	if protocolType, ok := configMap["protocol_type"].(string); ok {
		customConfig.ProtocolType = protocolType
	}
	if address, ok := configMap["address"].(string); ok {
		customConfig.Address = address
	}
	if port, ok := configMap["port"].(int); ok {
		customConfig.Port = port
	}
	if timeout, ok := configMap["timeout"].(string); ok {
		customConfig.Timeout = timeout
	}
	if frameDelimiter, ok := configMap["frame_delimiter"].(string); ok {
		customConfig.FrameDelimiter = frameDelimiter
	}
	if header, ok := configMap["header"].(string); ok {
		customConfig.Header = header
	}
	if trailer, ok := configMap["trailer"].(string); ok {
		customConfig.Trailer = trailer
	}
	if checksumMethod, ok := configMap["checksum_method"].(string); ok {
		customConfig.ChecksumMethod = checksumMethod
	}
	if bigEndian, ok := configMap["big_endian"].(bool); ok {
		customConfig.BigEndian = bigEndian
	}
	if maxFrameSize, ok := configMap["max_frame_size"].(int); ok {
		customConfig.MaxFrameSize = maxFrameSize
	}
	if encoding, ok := configMap["encoding"].(string); ok {
		customConfig.Encoding = encoding
	}

	return customConfig, nil
}

// ConvertDeviceType 转换设备类型
func (hf *HardwareFactory) ConvertDeviceType(deviceConfig types.DeviceConfig) (devices.DeviceType, error) {
	protocol := deviceConfig.Protocol

	switch protocol {
	case "modbus_tcp":
		return devices.DeviceTypeModbusTCP, nil
	case "modbus_rtu":
		return devices.DeviceTypeModbusRTU, nil
	case "modbus_ascii":
		return devices.DeviceTypeModbusASCII, nil
	case "serial":
		return devices.DeviceTypeSerial, nil
	case "custom":
		return devices.DeviceTypeCustom, nil
	default:
		// 尝试从参数中推断类型
		params := convertStringParamsToInterface(deviceConfig.Parameters)
		if typ, ok := params["type"].(string); ok {
			switch typ {
			case "tcp":
				return devices.DeviceTypeModbusTCP, nil
			case "rtu":
				return devices.DeviceTypeModbusRTU, nil
			case "ascii":
				return devices.DeviceTypeModbusASCII, nil
			}
		}
		return devices.DeviceTypeUnknown, fmt.Errorf("unknown protocol type: %s", protocol)
	}
}

// CreateDeviceConfig 创建设备配置
func (hf *HardwareFactory) CreateDeviceConfig(deviceID string, deviceConfig types.DeviceConfig) (devices.DeviceConfig, error) {
	deviceType, err := hf.ConvertDeviceType(deviceConfig)
	if err != nil {
		return devices.DeviceConfig{}, err
	}

	// 创建基础配置
	config := devices.DeviceConfig{
		ID:          deviceID,
		Name:        deviceConfig.Type,
		Type:        deviceType,
		Protocol:    deviceConfig.Protocol,
		Enabled:     true,
		Timeout:     deviceConfig.Timeout,
		RetryCount:  deviceConfig.RetryCount,
		AutoConnect: true,
		Properties:  make(map[string]interface{}),
		Parameters:  convertStringParamsToInterface(deviceConfig.Parameters),
	}

	// 根据设备类型创建具体配置
	switch deviceType {
	case devices.DeviceTypeModbusTCP, devices.DeviceTypeModbusRTU, devices.DeviceTypeModbusASCII:
		modbusConfig, err := hf.CreateModbusConfig(deviceConfig)
		if err != nil {
			return devices.DeviceConfig{}, err
		}
		config.Config = modbusConfig

	case devices.DeviceTypeSerial:
		serialConfig, err := hf.CreateSerialConfig(deviceConfig)
		if err != nil {
			return devices.DeviceConfig{}, err
		}
		config.Config = serialConfig

	case devices.DeviceTypeCustom:
		customConfig, err := hf.CreateCustomConfig(deviceConfig)
		if err != nil {
			return devices.DeviceConfig{}, err
		}
		config.Config = customConfig

		// 提取命令和响应定义
		params := convertStringParamsToInterface(deviceConfig.Parameters)
		if commands, ok := params["commands"].([]interface{}); ok {
			parsedCommands := make([]custom.CommandDefinition, 0, len(commands))
			for _, cmd := range commands {
				if cmdMap, ok := cmd.(map[string]interface{}); ok {
					parsedCmd := custom.CommandDefinition{
						Name:        getString(cmdMap, "name"),
						CommandCode: getString(cmdMap, "command_code"),
						Format:      getString(cmdMap, "format"),
					}
					parsedCommands = append(parsedCommands, parsedCmd)
				}
			}
			config.Properties["commands"] = parsedCommands
		}

		if responses, ok := params["responses"].([]interface{}); ok {
			parsedResponses := make([]custom.ResponseDefinition, 0, len(responses))
			for _, resp := range responses {
				if respMap, ok := resp.(map[string]interface{}); ok {
					parsedResp := custom.ResponseDefinition{
						CommandCode: getString(respMap, "command_code"),
						Format:      getString(respMap, "format"),
					}
					parsedResponses = append(parsedResponses, parsedResp)
				}
			}
			config.Properties["responses"] = parsedResponses
		}

	default:
		return devices.DeviceConfig{}, fmt.Errorf("unsupported device type: %v", deviceType)
	}

	return config, nil
}

// AddDeviceFromConfig 从配置添加设备
func (hf *HardwareFactory) AddDeviceFromConfig(deviceID string, deviceConfig types.DeviceConfig) error {
	config, err := hf.CreateDeviceConfig(deviceID, deviceConfig)
	if err != nil {
		return fmt.Errorf("failed to create device config: %w", err)
	}

	return hf.deviceManager.AddDevice(config)
}

// AddDeviceFromConfigMap 从配置map添加设备
func (hf *HardwareFactory) AddDeviceFromConfigMap(configMap map[string]interface{}) error {
	deviceID := getString(configMap, "id")
	if deviceID == "" {
		return fmt.Errorf("device ID is required")
	}

	// 将参数从interface{}转换为string
	paramsMap := getMap(configMap, "parameters")
	stringParams := make(map[string]string)
	for k, v := range paramsMap {
		if str, ok := v.(string); ok {
			stringParams[k] = str
		} else {
			stringParams[k] = fmt.Sprintf("%v", v)
		}
	}

	deviceConfig := types.DeviceConfig{
		Type:        getString(configMap, "type"),
		Protocol:    getString(configMap, "protocol"),
		Endpoint:    getString(configMap, "endpoint"),
		Timeout:     10 * time.Second,
		RetryCount:  3,
		Parameters:  stringParams,
	}

	if timeoutStr, ok := configMap["timeout"].(string); ok {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			deviceConfig.Timeout = timeout
		}
	}

	return hf.AddDeviceFromConfig(deviceID, deviceConfig)
}

// getString 从map中获取字符串值
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getMap 从map中获取子map
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if subMap, ok := val.(map[string]interface{}); ok {
			return subMap
		}
	}
	return make(map[string]interface{})
}

// Start 启动硬件抽象层
func (hf *HardwareFactory) Start() error {
	// 启动自动扫描（可选）
	hf.deviceManager.StartAutoScan(30 * time.Second)
	return nil
}

// Stop 停止硬件抽象层
func (hf *HardwareFactory) Stop() error {
	return hf.deviceManager.Stop()
}

// GetDeviceEvents 获取设备事件
func (hf *HardwareFactory) GetDeviceEvents() <-chan devices.DeviceEvent {
	return hf.deviceManager.Events()
}

// GetDeviceStatus 获取设备状态（兼容旧接口）
func (hf *HardwareFactory) GetDeviceStatus(deviceID string) types.DeviceStatus {
	return hf.deviceManager.GetDeviceStatus(deviceID)
}

// ListDevices 列出所有设备
func (hf *HardwareFactory) ListDevices() []types.DeviceStatus {
	deviceConfigs := hf.deviceManager.ListDeviceConfigs()
	statuses := make([]types.DeviceStatus, 0, len(deviceConfigs))

	for _, config := range deviceConfigs {
		statuses = append(statuses, hf.deviceManager.GetDeviceStatus(config.ID))
	}

	return statuses
}

// ReadDevice 读取设备数据
func (hf *HardwareFactory) ReadDevice(deviceID string, address string, length int) ([]byte, error) {
	return hf.deviceManager.ReadDevice(deviceID, address, length)
}

// WriteDevice 写入设备数据
func (hf *HardwareFactory) WriteDevice(deviceID string, address string, data []byte) error {
	return hf.deviceManager.WriteDevice(deviceID, address, data)
}

// ConnectDevice 连接设备
func (hf *HardwareFactory) ConnectDevice(deviceID string) error {
	return hf.deviceManager.ConnectDevice(deviceID)
}

// DisconnectDevice 断开设备
func (hf *HardwareFactory) DisconnectDevice(deviceID string) error {
	return hf.deviceManager.DisconnectDevice(deviceID)
}