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

func (ts *TaskScheduler) Process() error {
	return nil
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