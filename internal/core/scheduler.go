// Package core implements priority-based task scheduling for motion control
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/internal/logging"
	"control/pkg/types"
)

type TaskQueue struct {
	tasks    []*types.Task
	lock     sync.Mutex
	notEmpty *sync.Cond
}

func NewTaskQueue() *TaskQueue {
	q := &TaskQueue{
		tasks: make([]*types.Task, 0),
	}
	q.notEmpty = sync.NewCond(&q.lock)
	return q
}

func (q *TaskQueue) Push(task *types.Task) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.tasks = append(q.tasks, task)
	q.notEmpty.Signal()
}

func (q *TaskQueue) Pop() *types.Task {
	q.lock.Lock()
	defer q.lock.Unlock()

	for len(q.tasks) == 0 {
		q.notEmpty.Wait()
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	return task
}

func (q *TaskQueue) Remove(taskID string) bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	for i, task := range q.tasks {
		if task.ID == taskID {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			return true
		}
	}
	return false
}

func (q *TaskQueue) Size() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.tasks)
}

type TaskScheduler struct {
	highPriorityQueue   *TaskQueue
	mediumPriorityQueue *TaskQueue
	lowPriorityQueue    *TaskQueue
	emergencyQueue      *TaskQueue

	activeTasks   map[string]*types.Task
	activeLock    sync.RWMutex
	taskChan      chan *types.Task
	config        types.SystemConfig
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	logger        *logging.Logger
}

func NewTaskScheduler(config types.SystemConfig) *TaskScheduler {
	return &TaskScheduler{
		highPriorityQueue:   NewTaskQueue(),
		mediumPriorityQueue: NewTaskQueue(),
		lowPriorityQueue:    NewTaskQueue(),
		emergencyQueue:      NewTaskQueue(),
		activeTasks:        make(map[string]*types.Task),
		taskChan:          make(chan *types.Task, 100),
		config:            config,
		logger:            logging.GetLogger("scheduler"),
	}
}

func (ts *TaskScheduler) Start(ctx context.Context) error {
	if ts.running {
		return fmt.Errorf("scheduler is already running")
	}

	ts.ctx, ts.cancel = context.WithCancel(ctx)
	ts.running = true

	ts.wg.Add(1)
	go ts.schedule()

	ts.logger.Info("Task scheduler started")
	return nil
}

func (ts *TaskScheduler) Stop() error {
	if !ts.running {
		return fmt.Errorf("scheduler is not running")
	}

	ts.cancel()
	close(ts.taskChan)

	ts.wg.Wait()
	ts.running = false

	ts.logger.Info("Task scheduler stopped")
	return nil
}

func (ts *TaskScheduler) ScheduleTask(task *types.Task) error {
	if !ts.running {
		return fmt.Errorf("scheduler is not running")
	}

	task.Status = types.StatusPending

	switch task.Priority {
	case types.PriorityEmergency:
		ts.emergencyQueue.Push(task)
	case types.PriorityHigh:
		ts.highPriorityQueue.Push(task)
	case types.PriorityMedium:
		ts.mediumPriorityQueue.Push(task)
	case types.PriorityLow:
		ts.lowPriorityQueue.Push(task)
	default:
		ts.mediumPriorityQueue.Push(task)
	}

	ts.logger.Info("Task scheduled", "task_id", task.ID, "priority", task.Priority)
	return nil
}

func (ts *TaskScheduler) CancelTask(taskID string) error {
	ts.activeLock.Lock()
	defer ts.activeLock.Unlock()

	if task, exists := ts.activeTasks[taskID]; exists {
		if task.CancelFunc != nil {
			task.CancelFunc()
		}
		task.Status = types.StatusCancelled
		now := time.Now()
		task.CompletedAt = &now
		delete(ts.activeTasks, taskID)
		return nil
	}

	if ts.emergencyQueue.Remove(taskID) ||
	   ts.highPriorityQueue.Remove(taskID) ||
	   ts.mediumPriorityQueue.Remove(taskID) ||
	   ts.lowPriorityQueue.Remove(taskID) {
		return nil
	}

	return fmt.Errorf("task %s not found", taskID)
}

func (ts *TaskScheduler) PauseTask(taskID string) error {
	ts.activeLock.Lock()
	defer ts.activeLock.Unlock()

	if task, exists := ts.activeTasks[taskID]; exists {
		task.Status = types.StatusPaused
		return nil
	}

	return fmt.Errorf("task %s not found or not running", taskID)
}

func (ts *TaskScheduler) ResumeTask(taskID string) error {
	ts.activeLock.Lock()
	defer ts.activeLock.Unlock()

	if task, exists := ts.activeTasks[taskID]; exists {
		task.Status = types.StatusRunning
		return nil
	}

	return fmt.Errorf("task %s not found", taskID)
}

func (ts *TaskScheduler) GetTaskStatus(taskID string) (types.TaskStatus, error) {
	ts.activeLock.RLock()
	defer ts.activeLock.RUnlock()

	if task, exists := ts.activeTasks[taskID]; exists {
		return task.Status, nil
	}

	return types.StatusPending, fmt.Errorf("task %s not found", taskID)
}

func (ts *TaskScheduler) GetTaskChannel() <-chan *types.Task {
	return ts.taskChan
}

func (ts *TaskScheduler) schedule() {
	defer ts.wg.Done()

	for {
		select {
		case <-ts.ctx.Done():
			return
		default:
			task := ts.getNextTask()
			if task != nil {
				ts.executeTask(task)
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

func (ts *TaskScheduler) getNextTask() *types.Task {
	if ts.emergencyQueue.Size() > 0 {
		return ts.emergencyQueue.Pop()
	}

	if ts.highPriorityQueue.Size() > 0 {
		return ts.highPriorityQueue.Pop()
	}

	if ts.mediumPriorityQueue.Size() > 0 {
		return ts.mediumPriorityQueue.Pop()
	}

	if ts.lowPriorityQueue.Size() > 0 {
		return ts.lowPriorityQueue.Pop()
	}

	return nil
}

func (ts *TaskScheduler) executeTask(task *types.Task) {
	ts.activeLock.Lock()
	ts.activeTasks[task.ID] = task
	task.Status = types.StatusRunning
	now := time.Now()
	task.StartedAt = &now
	ts.activeLock.Unlock()

	ctx, cancel := context.WithTimeout(ts.ctx, task.Timeout)
	task.CancelFunc = cancel

	select {
	case ts.taskChan <- task:
		ts.logger.Info("Task execution started", "task_id", task.ID)

		go func() {
			<-ctx.Done()
			ts.activeLock.Lock()
			if _, exists := ts.activeTasks[task.ID]; exists {
				if ctx.Err() == context.DeadlineExceeded {
					task.Status = types.StatusFailed
					task.Error = "task timeout"
				}
				now := time.Now()
				task.CompletedAt = &now
				delete(ts.activeTasks, task.ID)
			}
			ts.activeLock.Unlock()
		}()

	default:
		task.Status = types.StatusFailed
		task.Error = "task channel full"
		now := time.Now()
		task.CompletedAt = &now
		ts.activeLock.Lock()
		delete(ts.activeTasks, task.ID)
		ts.activeLock.Unlock()

		ts.logger.Warn("Task channel full, rejecting task", "task_id", task.ID)
	}
}

func (ts *TaskScheduler) Name() string {
	return "task_scheduler"
}

// HandleEvent 实现EventHandler接口
func (ts *TaskScheduler) HandleEvent(event Event) error {
	switch event.Type() {
	case EventTypeSystemStart:
		ts.logger.Info("Task scheduler received system start event")
		return nil
	case EventTypeSystemStop:
		ts.logger.Info("Task scheduler received system stop event")
		return nil
	case EventTypeTaskRequest:
		if taskEvent, ok := event.(*TaskEvent); ok {
			ts.logger.Info("Task scheduler received task request", "task_id", taskEvent.TaskID, "task_type", taskEvent.TaskType)
			// 处理任务请求
			if taskEvent.Data != nil {
				if task, ok := taskEvent.Data.(*types.Task); ok {
					return ts.ScheduleTask(task)
				}
			}
		}
		return fmt.Errorf("invalid task event data")
	case EventTypeTimerTick:
		// 处理定时器事件，检查任务队列
		ts.processQueues()
		return nil
	default:
		ts.logger.Debug("Task scheduler ignoring event", "event_type", event.Type())
		return nil
	}
}

// GetSubscribedEvents 返回订阅的事件类型
func (ts *TaskScheduler) GetSubscribedEvents() []EventType {
	return []EventType{
		EventTypeSystemStart,
		EventTypeSystemStop,
		EventTypeTaskRequest,
		EventTypeTimerTick,
	}
}

func (ts *TaskScheduler) Status() interface{} {
	ts.activeLock.RLock()
	defer ts.activeLock.RUnlock()

	activeCount := len(ts.activeTasks)
	queueSizes := map[string]int{
		"emergency": ts.emergencyQueue.Size(),
		"high":      ts.highPriorityQueue.Size(),
		"medium":    ts.mediumPriorityQueue.Size(),
		"low":       ts.lowPriorityQueue.Size(),
	}

	return map[string]interface{}{
		"active_tasks": activeCount,
		"queue_sizes":  queueSizes,
		"running":      ts.running,
	}
}

// processQueues 处理任务队列
func (ts *TaskScheduler) processQueues() {
	if !ts.running {
		return
	}

	// 检查各个优先级队列并处理任务
	// 这里可以添加队列处理逻辑，例如：
	// - 检查超时任务
	// - 重新排队失败任务
	// - 调整任务优先级
	// - 清理完成的任务

	ts.activeLock.Lock()
	defer ts.activeLock.Unlock()

	// 检查活跃任务的状态
	for _, task := range ts.activeTasks {
		if task.Status == types.StatusFailed || task.Status == types.StatusCompleted {
			if task.CompletedAt != nil && time.Since(*task.CompletedAt) > 5*time.Minute {
				// 清理5分钟前完成的任务
				delete(ts.activeTasks, task.ID)
				ts.logger.Debug("Cleaned up completed task", "task_id", task.ID)
			}
		}
	}
}