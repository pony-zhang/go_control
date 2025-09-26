package core

import (
	"context"
	"control/pkg/types"
)

type Device interface {
	Write(cmd types.MotionCommand) error
	Read(reg string) (interface{}, error)
	Connect() error
	Disconnect()
	Status() types.DeviceStatus
	ID() types.DeviceID
	Type() string
}

type Scheduler interface {
	ScheduleTask(task *types.Task) error
	CancelTask(taskID string) error
	PauseTask(taskID string) error
	ResumeTask(taskID string) error
	GetTaskStatus(taskID string) (types.TaskStatus, error)
}

type IPCServer interface {
	Start() error
	Stop() error
	Broadcast(message types.IPCMessage) error
	SendToClient(clientID string, message types.IPCMessage) error
	RegisterHandler(messageType string, handler func(types.IPCMessage))
}

type IPCClient interface {
	Connect() error
	Disconnect() error
	Send(message types.IPCMessage) error
	Receive() <-chan types.IPCMessage
	RegisterHandler(messageType string, handler func(types.IPCMessage))
}

type Module interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Process() error
	Status() interface{}
}




type ConfigManager interface {
	LoadConfig(path string) error
	Reload() error
	GetConfig() types.SystemConfig
	SetConfig(config types.SystemConfig) error
	WatchChanges(callback func(types.SystemConfig)) error
}