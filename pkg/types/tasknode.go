package types

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type BaseTaskNode struct {
	id         string
	nodeType   CommandType
	status     TaskStatus
	parameters map[string]interface{}
	children   []TaskNode
	parent     TaskNode
	metadata   map[string]interface{}
	startTime  time.Time
	endTime    *time.Time
	cancelFunc context.CancelFunc
	lock       sync.RWMutex
}

func NewBaseTaskNode(id string, nodeType CommandType, params map[string]interface{}) *BaseTaskNode {
	return &BaseTaskNode{
		id:         id,
		nodeType:   nodeType,
		status:     StatusPending,
		parameters: params,
		children:   make([]TaskNode, 0),
		metadata:   make(map[string]interface{}),
		startTime:  time.Now(),
	}
}

func (btn *BaseTaskNode) GetID() string {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.id
}

func (btn *BaseTaskNode) GetType() CommandType {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.nodeType
}

func (btn *BaseTaskNode) GetParameters() map[string]interface{} {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	params := make(map[string]interface{})
	for k, v := range btn.parameters {
		params[k] = v
	}
	return params
}

func (btn *BaseTaskNode) GetStatus() TaskStatus {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.status
}

func (btn *BaseTaskNode) GetChildren() []TaskNode {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.children
}

func (btn *BaseTaskNode) GetParent() TaskNode {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.parent
}

func (btn *BaseTaskNode) AddChild(node TaskNode) error {
	btn.lock.Lock()
	defer btn.lock.Unlock()

	btn.children = append(btn.children, node)
	if childBase, ok := node.(*BaseTaskNode); ok {
		childBase.parent = btn
	}
	return nil
}

func (btn *BaseTaskNode) RemoveChild(nodeID string) error {
	btn.lock.Lock()
	defer btn.lock.Unlock()

	for i, child := range btn.children {
		if child.GetID() == nodeID {
			btn.children = append(btn.children[:i], btn.children[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("child node %s not found", nodeID)
}

func (btn *BaseTaskNode) Validate() error {
	btn.lock.RLock()
	defer btn.lock.RUnlock()

	if btn.id == "" {
		return fmt.Errorf("task node ID cannot be empty")
	}
	if btn.nodeType == "" {
		return fmt.Errorf("task node type cannot be empty")
	}
	return nil
}

func (btn *BaseTaskNode) Execute(ctx context.Context) error {
	btn.lock.Lock()
	btn.startTime = time.Now()
	btn.status = StatusRunning
	btn.lock.Unlock()

	select {
	case <-ctx.Done():
		btn.SetStatus(StatusCancelled)
		return ctx.Err()
	default:
		err := btn.executeSpecific(ctx)
		if err != nil {
			btn.SetStatus(StatusFailed)
			return err
		}

		btn.lock.Lock()
		btn.status = StatusCompleted
		endTime := time.Now()
		btn.endTime = &endTime
		btn.lock.Unlock()

		return nil
	}
}

func (btn *BaseTaskNode) executeSpecific(ctx context.Context) error {
	return fmt.Errorf("executeSpecific must be implemented by concrete task nodes")
}

func (btn *BaseTaskNode) Cancel() error {
	btn.lock.Lock()
	defer btn.lock.Unlock()

	if btn.cancelFunc != nil {
		btn.cancelFunc()
	}

	if btn.status == StatusRunning {
		btn.status = StatusCancelled
		now := time.Now()
		btn.endTime = &now
	}

	return nil
}

func (btn *BaseTaskNode) GetStartTime() time.Time {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.startTime
}

func (btn *BaseTaskNode) GetEndTime() *time.Time {
	btn.lock.RLock()
	defer btn.lock.RUnlock()
	return btn.endTime
}

func (btn *BaseTaskNode) GetDuration() time.Duration {
	btn.lock.RLock()
	defer btn.lock.RUnlock()

	if btn.endTime == nil {
		return time.Since(btn.startTime)
	}
	return btn.endTime.Sub(btn.startTime)
}

func (btn *BaseTaskNode) SetStatus(status TaskStatus) {
	btn.lock.Lock()
	defer btn.lock.Unlock()

	btn.status = status
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		endTime := time.Now()
		btn.endTime = &endTime
	}
}

func (btn *BaseTaskNode) GetMetadata() map[string]interface{} {
	btn.lock.RLock()
	defer btn.lock.RUnlock()

	metadata := make(map[string]interface{})
	for k, v := range btn.metadata {
		metadata[k] = v
	}
	return metadata
}

func (btn *BaseTaskNode) SetMetadata(key string, value interface{}) {
	btn.lock.Lock()
	defer btn.lock.Unlock()
	btn.metadata[key] = value
}

func (btn *BaseTaskNode) SetCancelFunc(cancelFunc context.CancelFunc) {
	btn.lock.Lock()
	defer btn.lock.Unlock()
	btn.cancelFunc = cancelFunc
}

type IOTaskNode struct {
	*BaseTaskNode
	command IOCommand
}

func NewIOTaskNode(id string, command IOCommand) *IOTaskNode {
	base := NewBaseTaskNode(id, CommandIO, map[string]interface{}{
		"device_id":  command.DeviceID,
		"channel":   command.Channel,
		"action":    command.Action,
		"value":     command.Value,
		"threshold": command.Threshold,
		"timeout":   command.Timeout,
	})

	return &IOTaskNode{
		BaseTaskNode: base,
		command:      command,
	}
}

func (ion *IOTaskNode) executeSpecific(ctx context.Context) error {
	switch ion.command.Action {
	case "set":
		ion.SetMetadata("action_type", "io_set")
	case "read":
		ion.SetMetadata("action_type", "io_read")
	case "toggle":
		ion.SetMetadata("action_type", "io_toggle")
	default:
		return fmt.Errorf("unsupported IO action: %s", ion.command.Action)
	}

	return nil
}

type MotorTaskNode struct {
	*BaseTaskNode
	command MotorCommand
}

func NewMotorTaskNode(id string, command MotorCommand) *MotorTaskNode {
	base := NewBaseTaskNode(id, CommandMotorControl, map[string]interface{}{
		"device_id":     command.DeviceID,
		"motor_id":      command.MotorID,
		"action":        command.Action,
		"speed":         command.Speed,
		"position":      command.Position,
		"acceleration":  command.Acceleration,
		"torque":        command.Torque,
		"direction":     command.Direction,
		"enable":        command.Enable,
	})

	return &MotorTaskNode{
		BaseTaskNode: base,
		command:      command,
	}
}

func (mtn *MotorTaskNode) executeSpecific(ctx context.Context) error {
	switch mtn.command.Action {
	case "start", "stop", "set_speed", "set_position":
		mtn.SetMetadata("action_type", "motor_control")
	case "get_status":
		mtn.SetMetadata("action_type", "motor_status_query")
	default:
		return fmt.Errorf("unsupported motor action: %s", mtn.command.Action)
	}

	return nil
}

type DelayTaskNode struct {
	*BaseTaskNode
	command DelayCommand
}

func NewDelayTaskNode(id string, command DelayCommand) *DelayTaskNode {
	base := NewBaseTaskNode(id, CommandDelay, map[string]interface{}{
		"duration": command.Duration,
		"unit":     command.Unit,
	})

	return &DelayTaskNode{
		BaseTaskNode: base,
		command:      command,
	}
}

func (dtn *DelayTaskNode) executeSpecific(ctx context.Context) error {
	dtn.SetMetadata("action_type", "delay")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(dtn.command.Duration):
		return nil
	}
}

type SequenceTaskNode struct {
	*BaseTaskNode
	command SequenceCommand
}

func NewSequenceTaskNode(id string, command SequenceCommand) *SequenceTaskNode {
	base := NewBaseTaskNode(id, CommandSequence, map[string]interface{}{
		"mode":         command.Mode,
		"stop_on_error": command.StopOnError,
		"node_count":   len(command.Nodes),
	})

	seqNode := &SequenceTaskNode{
		BaseTaskNode: base,
		command:      command,
	}

	for _, node := range command.Nodes {
		seqNode.AddChild(node)
	}

	return seqNode
}

func (stn *SequenceTaskNode) executeSpecific(ctx context.Context) error {
	stn.SetMetadata("action_type", "sequence")

	if stn.command.Mode == "sequential" {
		return stn.executeSequential(ctx)
	} else if stn.command.Mode == "parallel" {
		return stn.executeParallel(ctx)
	}

	return fmt.Errorf("unsupported sequence mode: %s", stn.command.Mode)
}

func (stn *SequenceTaskNode) executeSequential(ctx context.Context) error {
	for _, child := range stn.command.Nodes {
		childCtx, cancel := context.WithCancel(ctx)

		if childBase, ok := child.(*BaseTaskNode); ok {
			childBase.SetCancelFunc(cancel)
		}

		err := child.Execute(childCtx)
		if err != nil && stn.command.StopOnError {
			return fmt.Errorf("sequence execution failed at node %s: %w", child.GetID(), err)
		}
	}
	return nil
}

func (stn *SequenceTaskNode) executeParallel(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(stn.command.Nodes))

	for _, child := range stn.command.Nodes {
		wg.Add(1)
		go func(node TaskNode) {
			defer wg.Done()

			childCtx, cancel := context.WithCancel(ctx)
			if childBase, ok := node.(*BaseTaskNode); ok {
				childBase.SetCancelFunc(cancel)
			}

			if err := node.Execute(childCtx); err != nil {
				errChan <- err
			}
		}(child)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil && stn.command.StopOnError {
			return fmt.Errorf("parallel execution failed: %w", err)
		}
	}

	return nil
}

type ConditionTaskNode struct {
	*BaseTaskNode
	command ConditionCommand
}

func NewConditionTaskNode(id string, command ConditionCommand) *ConditionTaskNode {
	base := NewBaseTaskNode(id, CommandCondition, map[string]interface{}{
		"condition":     command.Condition,
		"device_id":     command.DeviceID,
		"channel":      command.Channel,
		"target_value":  command.TargetValue,
		"tolerance":    command.Tolerance,
		"timeout":       command.Timeout,
		"poll_interval": command.PollInterval,
	})

	return &ConditionTaskNode{
		BaseTaskNode: base,
		command:      command,
	}
}

func (ctn *ConditionTaskNode) executeSpecific(ctx context.Context) error {
	ctn.SetMetadata("action_type", "condition_check")

	ticker := time.NewTicker(ctn.command.PollInterval)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, ctn.command.Timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition timeout: %s", ctn.command.Condition)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if ctn.checkCondition() {
				ctn.SetMetadata("condition_met", true)
				return nil
			}
		}
	}
}

func (ctn *ConditionTaskNode) checkCondition() bool {
	ctn.SetMetadata("last_check", time.Now())

	switch ctn.command.Condition {
	case "position_reached":
		ctn.SetMetadata("condition_result", "simulated_position_check")
		return true
	case "io_state":
		ctn.SetMetadata("condition_result", "simulated_io_check")
		return true
	default:
		return false
	}
}