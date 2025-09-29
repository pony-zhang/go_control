package core

import (
	"context"
	"time"
)

// Event 定义事件接口
type Event interface {
	// Type 返回事件类型
	Type() EventType
	// Source 返回事件源
	Source() string
	// Timestamp 返回事件时间戳
	Timestamp() time.Time
	// Context 返回事件上下文
	Context() context.Context
}

// EventType 事件类型
type EventType string

const (
	// 系统事件
	EventTypeSystemStart    EventType = "system_start"
	EventTypeSystemStop     EventType = "system_stop"
	EventTypeSystemError    EventType = "system_error"

	// 任务事件
	EventTypeTaskRequest    EventType = "task_request"
	EventTypeTaskComplete   EventType = "task_complete"
	EventTypeTaskError      EventType = "task_error"
	EventTypeTaskCancel     EventType = "task_cancel"

	// 设备事件
	EventTypeDeviceConnect  EventType = "device_connect"
	EventTypeDeviceDisconnect EventType = "device_disconnect"
	EventTypeDeviceError    EventType = "device_error"
	EventTypeDeviceStatus   EventType = "device_status"

	// 网络事件
	EventTypeNetworkConnect    EventType = "network_connect"
	EventTypeNetworkDisconnect EventType = "network_disconnect"
	EventTypeNetworkMessage    EventType = "network_message"
	EventTypeNetworkError      EventType = "network_error"

	// 配置事件
	EventTypeConfigReload   EventType = "config_reload"
	EventTypeConfigChange   EventType = "config_change"

	// 定时器事件
	EventTypeTimerTick      EventType = "timer_tick"

	// 自定义事件
	EventTypeCustom        EventType = "custom"
)

// BaseEvent 基础事件结构，其他事件可以嵌入
type BaseEvent struct {
	eventType  EventType
	source     string
	timestamp  time.Time
	ctx        context.Context
}

func NewBaseEvent(eventType EventType, source string) BaseEvent {
	return BaseEvent{
		eventType: eventType,
		source:    source,
		timestamp: time.Now(),
		ctx:       context.Background(),
	}
}

func (be BaseEvent) Type() EventType     { return be.eventType }
func (be BaseEvent) Source() string       { return be.source }
func (be BaseEvent) Timestamp() time.Time { return be.timestamp }
func (be BaseEvent) Context() context.Context { return be.ctx }

// SystemEvent 系统事件
type SystemEvent struct {
	BaseEvent
	Error error
}

func NewSystemEvent(eventType EventType, source string, err error) *SystemEvent {
	return &SystemEvent{
		BaseEvent: NewBaseEvent(eventType, source),
		Error:     err,
	}
}

// TaskEvent 任务事件
type TaskEvent struct {
	BaseEvent
	TaskID   string
	TaskType string
	Data     interface{}
	Error    error
}

func NewTaskEvent(eventType EventType, source, taskID, taskType string, data interface{}, err error) *TaskEvent {
	return &TaskEvent{
		BaseEvent: NewBaseEvent(eventType, source),
		TaskID:    taskID,
		TaskType:  taskType,
		Data:      data,
		Error:     err,
	}
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	BaseEvent
	DeviceID string
	Status   interface{}
	Error    error
	Data     interface{}
}

func NewDeviceEvent(eventType EventType, source, deviceID string, status interface{}, err error, data interface{}) *DeviceEvent {
	return &DeviceEvent{
		BaseEvent: NewBaseEvent(eventType, source),
		DeviceID:  deviceID,
		Status:    status,
		Error:     err,
		Data:      data,
	}
}

// NetworkEvent 网络事件
type NetworkEvent struct {
	BaseEvent
	ClientID string
	Message  interface{}
	Error    error
}

func NewNetworkEvent(eventType EventType, source, clientID string, message interface{}, err error) *NetworkEvent {
	return &NetworkEvent{
		BaseEvent: NewBaseEvent(eventType, source),
		ClientID:  clientID,
		Message:  message,
		Error:    err,
	}
}

// ConfigEvent 配置事件
type ConfigEvent struct {
	BaseEvent
	ConfigPath string
	Config     interface{}
	Error      error
}

func NewConfigEvent(eventType EventType, source, configPath string, config interface{}, err error) *ConfigEvent {
	return &ConfigEvent{
		BaseEvent:  NewBaseEvent(eventType, source),
		ConfigPath: configPath,
		Config:     config,
		Error:      err,
	}
}

// TimerEvent 定时器事件
type TimerEvent struct {
	BaseEvent
	Interval time.Duration
	Count    int
	Data     interface{}
}

func NewTimerEvent(source string, interval time.Duration, count int, data interface{}) *TimerEvent {
	return &TimerEvent{
		BaseEvent: NewBaseEvent(EventTypeTimerTick, source),
		Interval:  interval,
		Count:     count,
		Data:      data,
	}
}

// EventHandler 事件处理器接口
type EventHandler interface {
	// HandleEvent 处理事件
	HandleEvent(event Event) error
	// GetSubscribedEvents 返回订阅的事件类型
	GetSubscribedEvents() []EventType
	// Name 返回处理器名称
	Name() string
}