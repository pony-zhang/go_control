package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"control/pkg/types"
)

type CommandStatus int

const (
	StatusPending CommandStatus = iota
	StatusSent
	StatusExecuting
	StatusCompleted
	StatusFailed
	StatusTimeout
)

type QueuedCommand struct {
	Command      types.MotionCommand
	Status       CommandStatus
	AddedAt      time.Time
	SentAt       *time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Error        string
	TaskID       string
	CancelFunc   context.CancelFunc
}

type ExecutionQueue struct {
	commands     map[string]*QueuedCommand
	commandsLock sync.RWMutex
	taskCommands map[string][]string
	taskLock     sync.RWMutex

	sendChan    chan *QueuedCommand
	completeChan chan *QueuedCommand

	maxSize     int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
}

func NewExecutionQueue(maxSize int) *ExecutionQueue {
	return &ExecutionQueue{
		commands:     make(map[string]*QueuedCommand),
		taskCommands: make(map[string][]string),
		sendChan:     make(chan *QueuedCommand, 10),
		completeChan: make(chan *QueuedCommand, 10),
		maxSize:      maxSize,
	}
}

func (eq *ExecutionQueue) Start(ctx context.Context) error {
	if eq.running {
		return fmt.Errorf("execution queue is already running")
	}

	eq.ctx, eq.cancel = context.WithCancel(ctx)
	eq.running = true

	eq.wg.Add(2)
	go eq.processCommands()
	go eq.handleCompletions()

	log.Println("Execution queue started")
	return nil
}

func (eq *ExecutionQueue) Stop() error {
	if !eq.running {
		return fmt.Errorf("execution queue is not running")
	}

	eq.cancel()

	eq.commandsLock.Lock()
	for _, cmd := range eq.commands {
		if cmd.CancelFunc != nil {
			cmd.CancelFunc()
		}
	}
	eq.commandsLock.Unlock()

	close(eq.sendChan)
	close(eq.completeChan)

	eq.wg.Wait()
	eq.running = false

	log.Println("Execution queue stopped")
	return nil
}

func (eq *ExecutionQueue) Push(cmd types.MotionCommand) error {
	if !eq.running {
		return fmt.Errorf("execution queue is not running")
	}

	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	if len(eq.commands) >= eq.maxSize {
		return fmt.Errorf("execution queue is full")
	}

	taskID := eq.extractTaskID(cmd)
	queuedCmd := &QueuedCommand{
		Command: cmd,
		Status:  StatusPending,
		AddedAt: time.Now(),
		TaskID:  taskID,
	}

	eq.commands[cmd.ID] = queuedCmd

	eq.taskLock.Lock()
	eq.taskCommands[taskID] = append(eq.taskCommands[taskID], cmd.ID)
	eq.taskLock.Unlock()

	select {
	case eq.sendChan <- queuedCmd:
		log.Printf("Command queued: %s", cmd.ID)
		return nil
	default:
		eq.commandsLock.Lock()
		delete(eq.commands, cmd.ID)
		eq.commandsLock.Unlock()

		eq.taskLock.Lock()
		if commands, exists := eq.taskCommands[taskID]; exists {
			for i, id := range commands {
				if id == cmd.ID {
					eq.taskCommands[taskID] = append(commands[:i], commands[i+1:]...)
					break
				}
			}
		}
		eq.taskLock.Unlock()

		return fmt.Errorf("send channel full")
	}
}

func (eq *ExecutionQueue) Pop() (*types.MotionCommand, error) {
	eq.commandsLock.RLock()
	defer eq.commandsLock.RUnlock()

	for _, cmd := range eq.commands {
		if cmd.Status == StatusPending {
			return &cmd.Command, nil
		}
	}

	return nil, fmt.Errorf("no pending commands")
}

func (eq *ExecutionQueue) Peek() (*types.MotionCommand, error) {
	eq.commandsLock.RLock()
	defer eq.commandsLock.RUnlock()

	for _, cmd := range eq.commands {
		if cmd.Status == StatusPending {
			return &cmd.Command, nil
		}
	}

	return nil, fmt.Errorf("no pending commands")
}

func (eq *ExecutionQueue) Size() int {
	eq.commandsLock.RLock()
	defer eq.commandsLock.RUnlock()
	return len(eq.commands)
}

func (eq *ExecutionQueue) Clear() error {
	if !eq.running {
		return fmt.Errorf("execution queue is not running")
	}

	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	for _, cmd := range eq.commands {
		if cmd.CancelFunc != nil {
			cmd.CancelFunc()
		}
	}

	eq.commands = make(map[string]*QueuedCommand)
	eq.taskLock.Lock()
	eq.taskCommands = make(map[string][]string)
	eq.taskLock.Unlock()

	log.Println("Execution queue cleared")
	return nil
}

func (eq *ExecutionQueue) RemoveByTaskID(taskID string) error {
	eq.taskLock.Lock()
	commands, exists := eq.taskCommands[taskID]
	if exists {
		for _, cmdID := range commands {
			eq.commandsLock.Lock()
			if cmd, cmdExists := eq.commands[cmdID]; cmdExists {
				if cmd.CancelFunc != nil {
					cmd.CancelFunc()
				}
				delete(eq.commands, cmdID)
			}
			eq.commandsLock.Unlock()
		}
		delete(eq.taskCommands, taskID)
	}
	eq.taskLock.Unlock()

	log.Printf("Removed commands for task: %s", taskID)
	return nil
}

func (eq *ExecutionQueue) GetByTaskID(taskID string) ([]types.MotionCommand, error) {
	eq.taskLock.RLock()
	commands, exists := eq.taskCommands[taskID]
	eq.taskLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no commands found for task: %s", taskID)
	}

	eq.commandsLock.RLock()
	defer eq.commandsLock.RUnlock()

	var result []types.MotionCommand
	for _, cmdID := range commands {
		if cmd, cmdExists := eq.commands[cmdID]; cmdExists {
			result = append(result, cmd.Command)
		}
	}

	return result, nil
}

func (eq *ExecutionQueue) GetSendChannel() <-chan *QueuedCommand {
	return eq.sendChan
}

func (eq *ExecutionQueue) GetCompleteChannel() <-chan *QueuedCommand {
	return eq.completeChan
}

func (eq *ExecutionQueue) MarkCommandSent(cmdID string) error {
	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	cmd, exists := eq.commands[cmdID]
	if !exists {
		return fmt.Errorf("command not found: %s", cmdID)
	}

	if cmd.Status != StatusPending {
		return fmt.Errorf("command %s is not in pending state", cmdID)
	}

	cmd.Status = StatusSent
	now := time.Now()
	cmd.SentAt = &now

	return nil
}

func (eq *ExecutionQueue) MarkCommandStarted(cmdID string) error {
	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	cmd, exists := eq.commands[cmdID]
	if !exists {
		return fmt.Errorf("command not found: %s", cmdID)
	}

	if cmd.Status != StatusSent {
		return fmt.Errorf("command %s is not in sent state", cmdID)
	}

	cmd.Status = StatusExecuting
	now := time.Now()
	cmd.StartedAt = &now

	return nil
}

func (eq *ExecutionQueue) MarkCommandCompleted(cmdID string, err error) error {
	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	cmd, exists := eq.commands[cmdID]
	if !exists {
		return fmt.Errorf("command not found: %s", cmdID)
	}

	now := time.Now()
	cmd.CompletedAt = &now

	if err != nil {
		cmd.Status = StatusFailed
		cmd.Error = err.Error()
	} else {
		cmd.Status = StatusCompleted
	}

	select {
	case eq.completeChan <- cmd:
	default:
		log.Printf("Complete channel full, dropping completion for command: %s", cmdID)
	}

	return nil
}

func (eq *ExecutionQueue) processCommands() {
	defer eq.wg.Done()

	for {
		select {
		case <-eq.ctx.Done():
			return
		case cmd := <-eq.sendChan:
			eq.sendCommandToExecutor(cmd)
		}
	}
}

func (eq *ExecutionQueue) sendCommandToExecutor(cmd *QueuedCommand) {
	ctx, cancel := context.WithTimeout(eq.ctx, cmd.Command.Timeout)
	cmd.CancelFunc = cancel

	go func() {
		defer cancel()

		if err := eq.MarkCommandSent(cmd.Command.ID); err != nil {
			log.Printf("Failed to mark command %s as sent: %v", cmd.Command.ID, err)
			return
		}

		<-ctx.Done()

		if ctx.Err() == context.DeadlineExceeded {
			eq.MarkCommandCompleted(cmd.Command.ID, fmt.Errorf("command timeout"))
		}
	}()
}

func (eq *ExecutionQueue) handleCompletions() {
	defer eq.wg.Done()

	for {
		select {
		case <-eq.ctx.Done():
			return
		case cmd := <-eq.completeChan:
			eq.cleanupCompletedCommand(cmd)
		}
	}
}

func (eq *ExecutionQueue) cleanupCompletedCommand(cmd *QueuedCommand) {
	eq.commandsLock.Lock()
	defer eq.commandsLock.Unlock()

	if completedCmd, exists := eq.commands[cmd.Command.ID]; exists {
		if completedCmd.Status == StatusCompleted || completedCmd.Status == StatusFailed {
			delete(eq.commands, cmd.Command.ID)

			eq.taskLock.Lock()
			if commands, taskExists := eq.taskCommands[cmd.TaskID]; taskExists {
				for i, id := range commands {
					if id == cmd.Command.ID {
						eq.taskCommands[cmd.TaskID] = append(commands[:i], commands[i+1:]...)
						if len(eq.taskCommands[cmd.TaskID]) == 0 {
							delete(eq.taskCommands, cmd.TaskID)
						}
						break
					}
				}
			}
			eq.taskLock.Unlock()

			log.Printf("Command completed and cleaned up: %s", cmd.Command.ID)
		}
	}
}

func (eq *ExecutionQueue) extractTaskID(cmd types.MotionCommand) string {
	parts := []rune(cmd.ID)
	for i, r := range parts {
		if r == '-' {
			return string(parts[:i])
		}
	}
	return cmd.ID
}

func (eq *ExecutionQueue) GetStatus() map[string]interface{} {
	eq.commandsLock.RLock()
	eq.taskLock.RLock()
	defer eq.commandsLock.RUnlock()
	defer eq.taskLock.RUnlock()

	statusCounts := make(map[CommandStatus]int)
	for _, cmd := range eq.commands {
		statusCounts[cmd.Status]++
	}

	return map[string]interface{}{
		"total_commands": len(eq.commands),
		"active_tasks":   len(eq.taskCommands),
		"status_counts":  statusCounts,
		"max_size":       eq.maxSize,
		"running":        eq.running,
	}
}