// Package core implements task trigger management for motion control
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/pkg/types"
	"control/internal/logging"
)

type TriggerSource struct {
	Name       string
	Trigger    func() *types.Task
	Enabled    bool
	LastTrigger time.Time
}

type TaskTrigger struct {
	triggers     map[string]*TriggerSource
	triggersLock sync.RWMutex
	config       types.SystemConfig
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	taskChan     chan *types.Task
	running      bool
	logger       *logging.Logger
}

func NewTaskTrigger(config types.SystemConfig) *TaskTrigger {
	return &TaskTrigger{
		triggers: make(map[string]*TriggerSource),
		config:   config,
		taskChan: make(chan *types.Task, 100),
		logger:   logging.GetLogger("task_trigger"),
	}
}

func (tt *TaskTrigger) Start(ctx context.Context) error {
	if tt.running {
		return fmt.Errorf("task trigger is already running")
	}

	tt.ctx, tt.cancel = context.WithCancel(ctx)
	tt.running = true

	tt.wg.Add(1)
	go tt.run()

	tt.logger.Info("Task trigger started")
	return nil
}

func (tt *TaskTrigger) Stop() error {
	if !tt.running {
		return fmt.Errorf("task trigger is not running")
	}

	tt.cancel()
	close(tt.taskChan)

	tt.wg.Wait()
	tt.running = false

	tt.logger.Info("Task trigger stopped")
	return nil
}

func (tt *TaskTrigger) AddTrigger(source string, triggerFunc func() *types.Task) error {
	tt.triggersLock.Lock()
	defer tt.triggersLock.Unlock()

	if _, exists := tt.triggers[source]; exists {
		return fmt.Errorf("trigger %s already exists", source)
	}

	tt.triggers[source] = &TriggerSource{
		Name:    source,
		Trigger: triggerFunc,
		Enabled: true,
	}

	tt.logger.Info("Trigger added", "source", source)
	return nil
}

func (tt *TaskTrigger) RemoveTrigger(source string) error {
	tt.triggersLock.Lock()
	defer tt.triggersLock.Unlock()

	if _, exists := tt.triggers[source]; !exists {
		return fmt.Errorf("trigger %s not found", source)
	}

	delete(tt.triggers, source)
	tt.logger.Info("Trigger removed", "source", source)
	return nil
}

func (tt *TaskTrigger) EnableTrigger(source string) error {
	tt.triggersLock.Lock()
	defer tt.triggersLock.Unlock()

	trigger, exists := tt.triggers[source]
	if !exists {
		return fmt.Errorf("trigger %s not found", source)
	}

	trigger.Enabled = true
	return nil
}

func (tt *TaskTrigger) DisableTrigger(source string) error {
	tt.triggersLock.Lock()
	defer tt.triggersLock.Unlock()

	trigger, exists := tt.triggers[source]
	if !exists {
		return fmt.Errorf("trigger %s not found", source)
	}

	trigger.Enabled = false
	return nil
}

func (tt *TaskTrigger) ValidateTrigger(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.Type == "" {
		return fmt.Errorf("task type cannot be empty")
	}

	if len(task.Axes) == 0 {
		return fmt.Errorf("task must specify at least one axis")
	}

	if task.Timeout == 0 {
		task.Timeout = 30 * time.Second
	}

	for _, axisID := range task.Axes {
		axisConfig, exists := tt.config.Axes[types.AxisID(axisID)]
		if !exists {
			return fmt.Errorf("axis %s not found in configuration", axisID)
		}

		if task.Target.X < axisConfig.MinPosition || task.Target.X > axisConfig.MaxPosition {
			return fmt.Errorf("target X position %f is out of bounds for axis %s", task.Target.X, axisID)
		}

		if task.Target.Y < axisConfig.MinPosition || task.Target.Y > axisConfig.MaxPosition {
			return fmt.Errorf("target Y position %f is out of bounds for axis %s", task.Target.Y, axisID)
		}

		if task.Velocity.Linear > axisConfig.MaxVelocity {
			return fmt.Errorf("velocity %f exceeds maximum for axis %s", task.Velocity.Linear, axisID)
		}
	}

	return nil
}

func (tt *TaskTrigger) GetTaskChannel() <-chan *types.Task {
	return tt.taskChan
}

func (tt *TaskTrigger) run() {
	defer tt.wg.Done()

	for {
		select {
		case <-tt.ctx.Done():
			return
		default:
			tt.checkTriggers()
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (tt *TaskTrigger) checkTriggers() {
	tt.triggersLock.RLock()
	defer tt.triggersLock.RUnlock()

	for _, trigger := range tt.triggers {
		if !trigger.Enabled {
			continue
		}

		task := trigger.Trigger()
		if task != nil {
			if err := tt.ValidateTrigger(task); err != nil {
				tt.logger.Error("Trigger validation failed", "trigger_name", trigger.Name, "error", err)
				continue
			}

			task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
			task.CreatedAt = time.Now()

			select {
			case tt.taskChan <- task:
				trigger.LastTrigger = time.Now()
				tt.logger.Info("Task triggered", "trigger_name", trigger.Name, "task_id", task.ID)
			default:
				tt.logger.Warn("Task channel full, dropping task", "trigger_name", trigger.Name)
			}
		}
	}
}

func (tt *TaskTrigger) Name() string {
	return "task_trigger"
}

// HandleEvent 实现EventHandler接口
func (tt *TaskTrigger) HandleEvent(event Event) error {
	switch event.Type() {
	case EventTypeSystemStart:
		tt.logger.Info("Task trigger received system start event")
		return nil
	case EventTypeSystemStop:
		tt.logger.Info("Task trigger received system stop event")
		return nil
	case EventTypeTimerTick:
		// 处理定时器事件，检查并触发任务
		tt.checkAndTriggerTriggers()
		return nil
	default:
		tt.logger.Debug("Task trigger ignoring event", "event_type", event.Type())
		return nil
	}
}

// GetSubscribedEvents 返回订阅的事件类型
func (tt *TaskTrigger) GetSubscribedEvents() []EventType {
	return []EventType{
		EventTypeSystemStart,
		EventTypeSystemStop,
		EventTypeTimerTick,
	}
}

func (tt *TaskTrigger) Status() interface{} {
	return tt.GetTriggerStatus()
}

func (tt *TaskTrigger) GetTriggerStatus() map[string]interface{} {
	tt.triggersLock.RLock()
	defer tt.triggersLock.RUnlock()

	status := make(map[string]interface{})
	for name, trigger := range tt.triggers {
		status[name] = map[string]interface{}{
			"enabled":       trigger.Enabled,
			"last_trigger":  trigger.LastTrigger,
		}
	}
	return status
}

// checkAndTriggerTriggers 检查并触发任务
func (tt *TaskTrigger) checkAndTriggerTriggers() {
	tt.triggersLock.RLock()
	defer tt.triggersLock.RUnlock()

	for name, trigger := range tt.triggers {
		if !trigger.Enabled {
			continue
		}

		// 这里可以添加触发条件检查逻辑
		// 例如：时间条件、设备状态条件等
		if tt.shouldTrigger(trigger) {
			task := trigger.Trigger()
			if task != nil {
				select {
				case tt.taskChan <- task:
					trigger.LastTrigger = time.Now()
					tt.logger.Info("Task triggered", "trigger_name", name, "task_id", task.ID)
				default:
					tt.logger.Warn("Task channel full, dropping task", "trigger_name", name)
				}
			}
		}
	}
}

// shouldTrigger 检查是否应该触发
func (tt *TaskTrigger) shouldTrigger(trigger *TriggerSource) bool {
	// 简单的触发条件检查
	// 可以根据实际需求扩展更复杂的逻辑
	if time.Since(trigger.LastTrigger) < time.Second {
		return false // 防止过于频繁的触发
	}

	return true
}