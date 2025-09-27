// Package core implements command execution for motion control
package core

import (
	"context"
	"fmt"
	"sync"

	"control/pkg/types"
	"control/internal/logging"
)

type CommandExecutor struct {
	devices      map[types.DeviceID]Device
	devicesLock  sync.RWMutex
	queue        *ExecutionQueue
	config       types.SystemConfig
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	running      bool
	logger       *logging.Logger
}

func NewCommandExecutor(config types.SystemConfig) *CommandExecutor {
	return &CommandExecutor{
		devices: make(map[types.DeviceID]Device),
		queue:   NewExecutionQueue(config.QueueSize),
		config:  config,
		logger:  logging.GetLogger("command_executor"),
	}
}

func (ce *CommandExecutor) Start(ctx context.Context) error {
	if ce.running {
		return fmt.Errorf("command executor is already running")
	}

	ce.ctx, ce.cancel = context.WithCancel(ctx)
	ce.running = true

	if err := ce.queue.Start(ce.ctx); err != nil {
		return fmt.Errorf("failed to start execution queue: %w", err)
	}

	ce.wg.Add(1)
	go ce.executeCommands()

	ce.logger.Info("Command executor started")
	return nil
}

func (ce *CommandExecutor) Stop() error {
	if !ce.running {
		return fmt.Errorf("command executor is not running")
	}

	ce.cancel()

	if err := ce.queue.Stop(); err != nil {
		ce.logger.Error("Error stopping execution queue", "error", err)
	}

	ce.devicesLock.Lock()
	for _, device := range ce.devices {
		device.Disconnect()
	}
	ce.devicesLock.Unlock()

	ce.wg.Wait()
	ce.running = false

	ce.logger.Info("Command executor stopped")
	return nil
}

func (ce *CommandExecutor) ExecuteCommand(ctx context.Context, cmd types.MotionCommand) error {
	if !ce.running {
		return fmt.Errorf("command executor is not running")
	}

	return ce.queue.Push(cmd)
}

func (ce *CommandExecutor) Name() string {
	return "command_executor"
}

func (ce *CommandExecutor) Process() error {
	return nil
}

func (ce *CommandExecutor) Status() interface{} {
	status := map[string]interface{}{
		"running":        ce.running,
		"queue_status":   ce.queue.GetStatus(),
		"devices_count":  len(ce.devices),
		"devices_status": ce.getDevicesStatus(),
	}

	return status
}

func (ce *CommandExecutor) AddDevice(device Device) error {
	ce.devicesLock.Lock()
	defer ce.devicesLock.Unlock()

	if _, exists := ce.devices[device.ID()]; exists {
		return fmt.Errorf("device %s already exists", device.ID())
	}

	if err := device.Connect(); err != nil {
		return fmt.Errorf("failed to connect device %s: %w", device.ID(), err)
	}

	ce.devices[device.ID()] = device
	ce.logger.Info("Device added", "device_id", device.ID(), "device_type", device.Type())

	return nil
}

func (ce *CommandExecutor) RemoveDevice(deviceID types.DeviceID) error {
	ce.devicesLock.Lock()
	defer ce.devicesLock.Unlock()

	device, exists := ce.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Disconnect()
	delete(ce.devices, deviceID)

	ce.logger.Info("Device removed", "device_id", deviceID)
	return nil
}

func (ce *CommandExecutor) GetDevice(deviceID types.DeviceID) (Device, error) {
	ce.devicesLock.RLock()
	defer ce.devicesLock.RUnlock()

	device, exists := ce.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

func (ce *CommandExecutor) executeCommands() {
	defer ce.wg.Done()

	sendChan := ce.queue.GetSendChannel()
	completeChan := ce.queue.GetCompleteChannel()
	go ce.handleCompletions(completeChan)

	for {
		select {
		case <-ce.ctx.Done():
			return
		case queuedCmd := <-sendChan:
			go ce.processCommand(queuedCmd)
		}
	}
}

func (ce *CommandExecutor) processCommand(queuedCmd *QueuedCommand) {

	if err := ce.queue.MarkCommandStarted(queuedCmd.Command.ID); err != nil {
		ce.logger.Error("Failed to mark command as started", "command_id", queuedCmd.Command.ID, "error", err)
		return
	}

	ce.devicesLock.RLock()
	device, exists := ce.devices[queuedCmd.Command.DeviceID]
	ce.devicesLock.RUnlock()

	if !exists {
		err := fmt.Errorf("device %s not found", queuedCmd.Command.DeviceID)
		ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)
		ce.logger.Error("Device not found for command", "command_id", queuedCmd.Command.ID, "device_id", queuedCmd.Command.DeviceID, "error", err)
		return
	}

	if !device.Status().Connected {
		err := fmt.Errorf("device %s is not connected", queuedCmd.Command.DeviceID)
		ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)
		ce.logger.Error("Device not connected for command", "command_id", queuedCmd.Command.ID, "device_id", queuedCmd.Command.DeviceID, "error", err)
		return
	}

	ce.logger.Info("Executing command", "command_id", queuedCmd.Command.ID, "device_id", queuedCmd.Command.DeviceID)

	err := device.Write(queuedCmd.Command)
	ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)

	if err != nil {
		ce.logger.Error("Command execution failed", "command_id", queuedCmd.Command.ID, "error", err)
	} else {
		ce.logger.Info("Command execution completed successfully", "command_id", queuedCmd.Command.ID)
	}
}

func (ce *CommandExecutor) handleCommandCompletion(completedCmd *QueuedCommand) {
	ce.logger.Info("Command completed", "command_id", completedCmd.Command.ID, "status", completedCmd.Status)
}

func (ce *CommandExecutor) handleCompletions(completeChan <-chan *QueuedCommand) {
	for completedCmd := range completeChan {
		ce.handleCommandCompletion(completedCmd)
	}
}

func (ce *CommandExecutor) getDevicesStatus() map[string]interface{} {
	ce.devicesLock.RLock()
	defer ce.devicesLock.RUnlock()

	status := make(map[string]interface{})
	for deviceID, device := range ce.devices {
		status[string(deviceID)] = map[string]interface{}{
			"type":      device.Type(),
			"connected": device.Status().Connected,
			"position":  device.Status().Position,
			"velocity":  device.Status().Velocity,
			"error":     device.Status().Error,
		}
	}

	return status
}

func (ce *CommandExecutor) GetExecutionQueue() *ExecutionQueue {
	return ce.queue
}