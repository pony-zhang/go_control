package core

import (
	"context"
	"fmt"
	"log"
	"sync"

	"control/pkg/types"
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
}

func NewCommandExecutor(config types.SystemConfig) *CommandExecutor {
	return &CommandExecutor{
		devices: make(map[types.DeviceID]Device),
		queue:   NewExecutionQueue(config.QueueSize),
		config:  config,
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

	log.Println("Command executor started")
	return nil
}

func (ce *CommandExecutor) Stop() error {
	if !ce.running {
		return fmt.Errorf("command executor is not running")
	}

	ce.cancel()

	if err := ce.queue.Stop(); err != nil {
		log.Printf("Error stopping execution queue: %v", err)
	}

	ce.devicesLock.Lock()
	for _, device := range ce.devices {
		device.Disconnect()
	}
	ce.devicesLock.Unlock()

	ce.wg.Wait()
	ce.running = false

	log.Println("Command executor stopped")
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
	log.Printf("Device added: %s (%s)", device.ID(), device.Type())

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

	log.Printf("Device removed: %s", deviceID)
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

	for {
		select {
		case <-ce.ctx.Done():
			return
		case queuedCmd := <-sendChan:
			go ce.processCommand(queuedCmd)
		}
	}

	completeChan := ce.queue.GetCompleteChannel()
	go ce.handleCompletions(completeChan)
}

func (ce *CommandExecutor) processCommand(queuedCmd *QueuedCommand) {

	if err := ce.queue.MarkCommandStarted(queuedCmd.Command.ID); err != nil {
		log.Printf("Failed to mark command %s as started: %v", queuedCmd.Command.ID, err)
		return
	}

	ce.devicesLock.RLock()
	device, exists := ce.devices[queuedCmd.Command.DeviceID]
	ce.devicesLock.RUnlock()

	if !exists {
		err := fmt.Errorf("device %s not found", queuedCmd.Command.DeviceID)
		ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)
		log.Printf("Device not found for command %s: %v", queuedCmd.Command.ID, err)
		return
	}

	if !device.Status().Connected {
		err := fmt.Errorf("device %s is not connected", queuedCmd.Command.DeviceID)
		ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)
		log.Printf("Device not connected for command %s: %v", queuedCmd.Command.ID, err)
		return
	}

	log.Printf("Executing command %s on device %s", queuedCmd.Command.ID, queuedCmd.Command.DeviceID)

	err := device.Write(queuedCmd.Command)
	ce.queue.MarkCommandCompleted(queuedCmd.Command.ID, err)

	if err != nil {
		log.Printf("Command %s execution failed: %v", queuedCmd.Command.ID, err)
	} else {
		log.Printf("Command %s execution completed successfully", queuedCmd.Command.ID)
	}
}

func (ce *CommandExecutor) handleCommandCompletion(completedCmd *QueuedCommand) {
	log.Printf("Command %s completed with status: %d", completedCmd.Command.ID, completedCmd.Status)
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