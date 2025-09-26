package comm

import (
	"context"
	"time"
)

// ConnectionStatus 表示连接状态
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

// ConnectionConfig 基础连接配置
type ConnectionConfig struct {
	Timeout       time.Duration `yaml:"timeout"`
	RetryCount    int           `yaml:"retry_count"`
	RetryInterval time.Duration `yaml:"retry_interval"`
	KeepAlive     bool          `yaml:"keep_alive"`
}

// CommunicationInterface 通信接口定义
type CommunicationInterface interface {
	// 连接管理
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Reconnect(ctx context.Context) error

	// 状态查询
	GetStatus() ConnectionStatus
	GetLastError() error
	IsConnected() bool

	// 数据读写
	Read(ctx context.Context, address string, length int) ([]byte, error)
	Write(ctx context.Context, address string, data []byte) error

	// 批量操作
	BulkRead(ctx context.Context, addresses []string) (map[string][]byte, error)
	BulkWrite(ctx context.Context, data map[string][]byte) error

	// 配置
	GetConfig() interface{}
	SetConfig(config interface{}) error

	// 事件处理
	AddEventHandler(handler EventHandler)
	RemoveEventHandler(handler EventHandler)
}

// DiscoveryInterface 设备发现接口
type DiscoveryInterface interface {
	DiscoverDevices(ctx context.Context, timeout time.Duration) ([]DeviceInfo, error)
	PingDevice(ctx context.Context, address string) (bool, error)
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID           string                 `json:"id"`
	Address      string                 `json:"address"`
	Type         string                 `json:"type"`
	Model        string                 `json:"model,omitempty"`
	Version      string                 `json:"version,omitempty"`
	SerialNumber string                 `json:"serial_number,omitempty"`
	Vendor       string                 `json:"vendor,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// ErrorHandler 错误处理接口
type ErrorHandler interface {
	HandleError(err error) error
	ShouldRetry(err error) bool
	GetRetryDelay(err error) time.Duration
}

// EventHandler 事件处理接口
type EventHandler interface {
	OnConnected()
	OnDisconnected()
	OnError(err error)
	OnDataReceived(address string, data []byte)
	OnDataWritten(address string, data []byte)
}