package application

import (
	"context"
	"fmt"

	"control/pkg/types"
	"control/internal/core"
	"control/internal/logging"
)

// TaskOrchestrationLayer manages task scheduling, decomposition, and flow control
type TaskOrchestrationLayer struct {
	serviceCoordination *ServiceCoordinationLayer
	taskScheduler       *core.TaskScheduler
	taskDecomposer      *core.TaskNodeDecomposer
		taskTrigger         *core.TaskTrigger
	logger              *logging.Logger
	ctx                 context.Context
}

// NewTaskOrchestrationLayer creates a new task orchestration layer
func NewTaskOrchestrationLayer(serviceCoord *ServiceCoordinationLayer, config types.SystemConfig) *TaskOrchestrationLayer {
	return &TaskOrchestrationLayer{
		serviceCoordination: serviceCoord,
		taskScheduler:       core.NewTaskScheduler(config),
		taskDecomposer:      core.NewTaskNodeDecomposer(config),
		taskTrigger:         core.NewTaskTrigger(config),
		logger:              logging.GetLogger("task_orchestration"),
	}
}

// Start initializes the task orchestration layer
func (tol *TaskOrchestrationLayer) Start(ctx context.Context) error {
	tol.ctx = ctx
	tol.logger.Info("Starting Task Orchestration Layer")

	// Start task trigger first
	if err := tol.taskTrigger.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task trigger: %w", err)
	}

	// Start task scheduler
	if err := tol.taskScheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task scheduler: %w", err)
	}

	// Set up task processing channels
	tol.setupTaskChannels()

	tol.logger.Info("Task Orchestration Layer started successfully")
	return nil
}

// Stop gracefully shuts down the task orchestration layer
func (tol *TaskOrchestrationLayer) Stop() error {
	tol.logger.Info("Stopping Task Orchestration Layer")

	var errs []error

	// Stop task trigger
	if tol.taskTrigger != nil {
		if err := tol.taskTrigger.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("task trigger stop error: %w", err))
		}
	}

	// Stop task scheduler
	if tol.taskScheduler != nil {
		if err := tol.taskScheduler.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("task scheduler stop error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("task orchestration stop errors: %v", errs)
	}

	tol.logger.Info("Task Orchestration Layer stopped successfully")
	return nil
}

// setupTaskChannels sets up the communication channels between task components
func (tol *TaskOrchestrationLayer) setupTaskChannels() {
	tol.logger.Info("Setting up task processing channels")

	// Task trigger -> Task scheduler
	taskChan := tol.taskTrigger.GetTaskChannel()
	go func() {
		for task := range taskChan {
			if err := tol.taskScheduler.ScheduleTask(task); err != nil {
				tol.logger.Error("Failed to schedule task", "task_id", task.ID, "error", err)
			}
		}
	}()

	// Task scheduler -> Task processing
	scheduledTaskChan := tol.taskScheduler.GetTaskChannel()
	go func() {
		for task := range scheduledTaskChan {
			go tol.processTask(task)
		}
	}()
}

// processTask processes a task through decomposition and execution
func (tol *TaskOrchestrationLayer) processTask(task *types.Task) {
	tol.logger.Info("Processing task", "task_id", task.ID, "task_type", task.Type)

	// Decompose task into commands
	commands, err := tol.taskDecomposer.DecomposeTask(task)
	if err != nil {
		tol.logger.Error("Failed to decompose task", "task_id", task.ID, "error", err)
		return
	}

	tol.logger.Info("Task decomposed into commands", "task_id", task.ID, "command_count", len(commands))

	// Execute commands through service coordination
	for _, cmd := range commands {
		if err := tol.serviceCoordination.ExecuteCommand(tol.ctx, cmd); err != nil {
			tol.logger.Error("Failed to execute command", "command_id", cmd.ID, "error", err)
			// Continue with other commands or handle error appropriately
		}
	}

	tol.logger.Info("Task processing completed", "task_id", task.ID)
}

// ScheduleTask schedules a task for execution
func (tol *TaskOrchestrationLayer) ScheduleTask(task *types.Task) error {
	tol.logger.Info("Scheduling task", "task_id", task.ID, "priority", task.Priority)
	return tol.taskScheduler.ScheduleTask(task)
}

// CancelTask cancels a scheduled or executing task
func (tol *TaskOrchestrationLayer) CancelTask(taskID string) error {
	tol.logger.Info("Canceling task", "task_id", taskID)
	return tol.taskScheduler.CancelTask(taskID)
}

// PauseTask pauses a executing task
func (tol *TaskOrchestrationLayer) PauseTask(taskID string) error {
	tol.logger.Info("Pausing task", "task_id", taskID)
	return tol.taskScheduler.PauseTask(taskID)
}

// ResumeTask resumes a paused task
func (tol *TaskOrchestrationLayer) ResumeTask(taskID string) error {
	tol.logger.Info("Resuming task", "task_id", taskID)
	return tol.taskScheduler.ResumeTask(taskID)
}

// GetTaskStatus returns the status of a task
func (tol *TaskOrchestrationLayer) GetTaskStatus(taskID string) (types.TaskStatus, error) {
	return tol.taskScheduler.GetTaskStatus(taskID)
}


// TriggerTask triggers a new task by directly scheduling it
func (tol *TaskOrchestrationLayer) TriggerTask(task *types.Task) error {
	tol.logger.Info("Triggering task", "task_id", task.ID)
	// For now, directly schedule the task since TaskTrigger manages internal triggers
	return tol.taskScheduler.ScheduleTask(task)
}

// GetTaskScheduler returns the task scheduler
func (tol *TaskOrchestrationLayer) GetTaskScheduler() *core.TaskScheduler {
	return tol.taskScheduler
}

// GetTaskTrigger returns the task trigger
func (tol *TaskOrchestrationLayer) GetTaskTrigger() *core.TaskTrigger {
	return tol.taskTrigger
}

// GetTaskDecomposer returns the task decomposer
func (tol *TaskOrchestrationLayer) GetTaskDecomposer() *core.TaskNodeDecomposer {
	return tol.taskDecomposer
}


// GetServiceCoordination returns the service coordination layer
func (tol *TaskOrchestrationLayer) GetServiceCoordination() *ServiceCoordinationLayer {
	return tol.serviceCoordination
}