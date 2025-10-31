// Package core implements task decomposition and trajectory planning for motion control
package core

import (
	"fmt"
	"math"
	"time"

	"control/pkg/types"
)

type TrajectoryPoint struct {
	Position  types.Point
	Velocity  types.Velocity
	Timestamp time.Time
}

type TrajectoryPlanner interface {
	PlanLinear(start, end types.Point, velocity types.Velocity, accel types.Velocity) ([]TrajectoryPoint, error)
	PlanCircular(center types.Point, radius float64, startAngle, endAngle float64, velocity types.Velocity) ([]TrajectoryPoint, error)
	PlanPointToPoint(start, end types.Point, velocity types.Velocity, accel types.Velocity) ([]TrajectoryPoint, error)
}

type SimpleTrajectoryPlanner struct{}

func (p *SimpleTrajectoryPlanner) PlanLinear(start, end types.Point, velocity types.Velocity, accel types.Velocity) ([]TrajectoryPoint, error) {
	distance := math.Sqrt(
		math.Pow(end.X-start.X, 2) +
		math.Pow(end.Y-start.Y, 2) +
		math.Pow(end.Z-start.Z, 2))

	if distance < 0.001 {
		return []TrajectoryPoint{{
			Position:  end,
			Velocity:  types.Velocity{Linear: 0, Angular: 0},
			Timestamp: time.Now(),
		}}, nil
	}

	steps := int(math.Ceil(distance / (velocity.Linear * 0.01)))
	if steps < 10 {
		steps = 10
	}

	trajectory := make([]TrajectoryPoint, steps+1)

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)

		trajectory[i] = TrajectoryPoint{
			Position: types.Point{
				X: start.X + t*(end.X-start.X),
				Y: start.Y + t*(end.Y-start.Y),
				Z: start.Z + t*(end.Z-start.Z),
			},
			Velocity:  velocity,
			Timestamp: time.Now().Add(time.Duration(t * float64(distance/velocity.Linear) * float64(time.Second))),
		}
	}

	return trajectory, nil
}

func (p *SimpleTrajectoryPlanner) PlanCircular(center types.Point, radius float64, startAngle, endAngle float64, velocity types.Velocity) ([]TrajectoryPoint, error) {
	angleDiff := endAngle - startAngle
	circumference := 2 * math.Pi * radius
	arcLength := circumference * math.Abs(angleDiff) / (2 * math.Pi)

	steps := int(math.Ceil(arcLength / (velocity.Linear * 0.01)))
	if steps < 20 {
		steps = 20
	}

	trajectory := make([]TrajectoryPoint, steps+1)

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := startAngle + t*angleDiff

		trajectory[i] = TrajectoryPoint{
			Position: types.Point{
				X: center.X + radius*math.Cos(angle),
				Y: center.Y + radius*math.Sin(angle),
				Z: center.Z,
			},
			Velocity:  velocity,
			Timestamp: time.Now().Add(time.Duration(t * float64(arcLength/velocity.Linear) * float64(time.Second))),
		}
	}

	return trajectory, nil
}

func (p *SimpleTrajectoryPlanner) PlanPointToPoint(start, end types.Point, velocity types.Velocity, accel types.Velocity) ([]TrajectoryPoint, error) {
	distance := math.Sqrt(
		math.Pow(end.X-start.X, 2) +
		math.Pow(end.Y-start.Y, 2) +
		math.Pow(end.Z-start.Z, 2))

	if distance < 0.001 {
		return []TrajectoryPoint{{
			Position:  end,
			Velocity:  types.Velocity{Linear: 0, Angular: 0},
			Timestamp: time.Now(),
		}}, nil
	}

	t_accel := velocity.Linear / accel.Linear
	d_accel := 0.5 * accel.Linear * t_accel * t_accel

	var trajectory []TrajectoryPoint

	if 2*d_accel >= distance {
		t_half := math.Sqrt(distance / accel.Linear)
		steps := int(t_half / 0.01)
		if steps < 5 {
			steps = 5
		}

		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			dist := 0.5 * accel.Linear * t * t
			progress := dist / distance

			point := types.Point{
				X: start.X + progress*(end.X-start.X),
				Y: start.Y + progress*(end.Y-start.Y),
				Z: start.Z + progress*(end.Z-start.Z),
			}

			vel := types.Velocity{
				Linear:  accel.Linear * t,
				Angular: 0,
			}

			if i > steps/2 {
				vel.Linear = accel.Linear * (1 - t)
			}

			trajectory = append(trajectory, TrajectoryPoint{
				Position:  point,
				Velocity:  vel,
				Timestamp: time.Now().Add(time.Duration(t * 2 * t_half * float64(time.Second))),
			})
		}
	} else {
		const_stages := 3
		steps_per_stage := int(t_accel / 0.01)
		if steps_per_stage < 10 {
			steps_per_stage = 10
		}

		for stage := 0; stage < const_stages; stage++ {
			for i := 0; i <= steps_per_stage; i++ {
				t := float64(i) / float64(steps_per_stage)
				var progress float64
				var vel types.Velocity

				switch stage {
				case 0:
					dist := 0.5 * accel.Linear * t * t_accel * t
					progress = dist / distance
					vel.Linear = accel.Linear * t
				case 1:
					dist := d_accel + velocity.Linear * t * t_accel
					progress = dist / distance
					vel = velocity
				case 2:
					dist := d_accel + velocity.Linear * t_accel + 0.5 * accel.Linear * t * t_accel * (1-t)
					progress = dist / distance
					vel.Linear = velocity.Linear * (1 - t)
				}

				point := types.Point{
					X: start.X + progress*(end.X-start.X),
					Y: start.Y + progress*(end.Y-start.Y),
					Z: start.Z + progress*(end.Z-start.Z),
				}

				trajectory = append(trajectory, TrajectoryPoint{
					Position:  point,
					Velocity:  vel,
					Timestamp: time.Now().Add(time.Duration(float64(stage*steps_per_stage+i) * 0.01 * float64(time.Second))),
				})
			}
		}
	}

	return trajectory, nil
}

// MotionPlan represents an abstract motion plan
type MotionPlan struct {
	Type         types.CommandType
	Dimensions   []string                // Target dimensions
	Trajectories map[string][]TrajectoryPoint // Dimension -> trajectory points
	Constraints  map[string]interface{}  // Motion constraints
	Workspace    string                  // Workspace identifier
}

// TaskDecomposer handles task decomposition with abstract motion planning
type TaskDecomposer struct {
	planner  TrajectoryPlanner
	config   types.SystemConfig
}

func NewTaskDecomposer(config types.SystemConfig) *TaskDecomposer {
	return &TaskDecomposer{
		planner: &SimpleTrajectoryPlanner{},
		config:  config,
	}
}

func (td *TaskDecomposer) Decompose(task *types.Task) ([]types.MotionCommand, error) {
	if err := td.ValidateTask(task); err != nil {
		return nil, fmt.Errorf("task validation failed: %w", err)
	}

	// Get device group configuration
	deviceGroup, err := td.getDeviceGroupConfig(task.DeviceGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get device group config: %w", err)
	}

	// Plan motion based on motion space and device group
	motionPlan, err := td.planMotion(task, deviceGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to plan motion: %w", err)
	}

	// Generate commands from motion plan
	commands, err := td.generateCommands(motionPlan, deviceGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to generate commands: %w", err)
	}

	// Set command metadata
	for i := range commands {
		commands[i].ID = fmt.Sprintf("%s-cmd-%d", task.ID, i)
		commands[i].Timestamp = time.Now()
		commands[i].Timeout = task.Timeout
	}

	return commands, nil
}

// Helper methods for the new decomposition approach

// getDeviceGroupConfig retrieves device group configuration
func (td *TaskDecomposer) getDeviceGroupConfig(groupID string) (types.DeviceGroupConfig, error) {
	if groupID == "" {
		return types.DeviceGroupConfig{}, fmt.Errorf("device group ID cannot be empty")
	}

	deviceGroup, exists := td.config.DeviceGroups[groupID]
	if !exists {
		return types.DeviceGroupConfig{}, fmt.Errorf("device group %s not found", groupID)
	}

	return deviceGroup, nil
}

// planMotion creates an abstract motion plan based on task and device group
func (td *TaskDecomposer) planMotion(task *types.Task, deviceGroup types.DeviceGroupConfig) (*MotionPlan, error) {
	plan := &MotionPlan{
		Type:         task.Type,
		Dimensions:   task.Dimensions,
		Constraints:  deviceGroup.Kinematics.Constraints,
		Workspace:    task.WorkSpace,
		Trajectories: make(map[string][]TrajectoryPoint),
	}

	// Validate dimensions exist in device group
	for _, dimName := range task.Dimensions {
		dimConfig, exists := deviceGroup.Dimensions[dimName]
		if !exists {
			return nil, fmt.Errorf("dimension %s not found in device group %s", dimName, task.DeviceGroup)
		}

		// Plan trajectory for this dimension
		trajectory, err := td.planDimensionTrajectory(task, dimName, dimConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to plan trajectory for dimension %s: %w", dimName, err)
		}

		plan.Trajectories[dimName] = trajectory
	}

	return plan, nil
}

// planDimensionTrajectory plans trajectory for a single dimension
func (td *TaskDecomposer) planDimensionTrajectory(task *types.Task, dimName string, dimConfig types.DimensionConfig) ([]TrajectoryPoint, error) {
	accel := types.Velocity{
		Linear:  dimConfig.MaxAccel,
		Angular: 0, // TODO: Handle angular acceleration for rotary dimensions
	}

	var trajectory []TrajectoryPoint
	var err error

	switch task.Type {
	case types.CommandMoveTo:
		trajectory, err = td.planLinearMove(task, dimName, dimConfig, accel)
	case types.CommandMoveRelative:
		trajectory, err = td.planRelativeMove(task, dimName, dimConfig, accel)
	case types.CommandHome:
		trajectory, err = td.planHomeMove(task, dimName, dimConfig, accel)
	case types.CommandJog:
		trajectory, err = td.planJogMove(task, dimName, dimConfig, accel)
	default:
		return nil, fmt.Errorf("unsupported command type: %s", task.Type)
	}

	if err != nil {
		return nil, err
	}

	return trajectory, nil
}

// planLinearMove plans a linear move to absolute position
func (td *TaskDecomposer) planLinearMove(task *types.Task, dimName string, dimConfig types.DimensionConfig, accel types.Velocity) ([]TrajectoryPoint, error) {
	// Get target value based on dimension
	targetValue := td.getTargetValueForDimension(task.Target, dimName)
	if targetValue < dimConfig.MinValue || targetValue > dimConfig.MaxValue {
		return nil, fmt.Errorf("target value %f for dimension %s is out of range [%f, %f]",
			targetValue, dimName, dimConfig.MinValue, dimConfig.MaxValue)
	}

	start := types.Point{X: 0, Y: 0, Z: 0}
	end := td.createPointForDimension(targetValue, dimName)

	// Use dimension-specific velocity
	dimVelocity := task.Velocity.Linear
	if dimVelocity > dimConfig.MaxVelocity {
		dimVelocity = dimConfig.MaxVelocity
	}

	velocity := types.Velocity{Linear: dimVelocity, Angular: 0}

	return td.planner.PlanPointToPoint(start, end, velocity, accel)
}

// planRelativeMove plans a relative move
func (td *TaskDecomposer) planRelativeMove(task *types.Task, dimName string, dimConfig types.DimensionConfig, accel types.Velocity) ([]TrajectoryPoint, error) {
	relativeValue := td.getTargetValueForDimension(task.Target, dimName)
	if math.Abs(relativeValue) > (dimConfig.MaxValue - dimConfig.MinValue) {
		return nil, fmt.Errorf("relative move %f for dimension %s exceeds range", relativeValue, dimName)
	}

	start := types.Point{X: 0, Y: 0, Z: 0}
	end := td.createPointForDimension(relativeValue, dimName)

	// Use dimension-specific velocity
	dimVelocity := task.Velocity.Linear
	if dimVelocity > dimConfig.MaxVelocity {
		dimVelocity = dimConfig.MaxVelocity
	}

	velocity := types.Velocity{Linear: dimVelocity, Angular: 0}

	return td.planner.PlanLinear(start, end, velocity, accel)
}

// planHomeMove plans a home move
func (td *TaskDecomposer) planHomeMove(task *types.Task, dimName string, dimConfig types.DimensionConfig, accel types.Velocity) ([]TrajectoryPoint, error) {
	end := td.createPointForDimension(dimConfig.HomeValue, dimName)
	start := types.Point{X: 0, Y: 0, Z: 0}

	// Use reduced velocity for homing
	velocity := types.Velocity{
		Linear:  dimConfig.MaxVelocity * 0.5,
		Angular: 0,
	}

	// Use reduced acceleration for homing
	homeAccel := types.Velocity{
		Linear:  dimConfig.MaxAccel * 0.5,
		Angular: 0,
	}

	return td.planner.PlanPointToPoint(start, end, velocity, homeAccel)
}

// planJogMove plans a jog move (continuous movement)
func (td *TaskDecomposer) planJogMove(task *types.Task, dimName string, dimConfig types.DimensionConfig, accel types.Velocity) ([]TrajectoryPoint, error) {
	// For jog, create a single point representing the continuous motion
	jogValue := td.getTargetValueForDimension(task.Target, dimName)
	point := td.createPointForDimension(jogValue, dimName)

	return []TrajectoryPoint{{
		Position:  point,
		Velocity:  task.Velocity,
		Timestamp: time.Now(),
	}}, nil
}

// generateCommands converts motion plan to device-specific commands
func (td *TaskDecomposer) generateCommands(plan *MotionPlan, deviceGroup types.DeviceGroupConfig) ([]types.MotionCommand, error) {
	var commands []types.MotionCommand

	// For each device in the group, generate commands for assigned dimensions
	for _, deviceID := range deviceGroup.DeviceIDs {
		deviceCommands, err := td.generateCommandsForDevice(plan, deviceID, deviceGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to generate commands for device %s: %w", deviceID, err)
		}
		commands = append(commands, deviceCommands...)
	}

	return commands, nil
}

// generateCommandsForDevice generates commands for a specific device
func (td *TaskDecomposer) generateCommandsForDevice(plan *MotionPlan, deviceID types.DeviceID, deviceGroup types.DeviceGroupConfig) ([]types.MotionCommand, error) {
	var commands []types.MotionCommand

	// Find dimensions assigned to this device
	// Note: In a real implementation, you'd need a mapping from dimensions to devices
	// For now, we'll assume all dimensions are handled by all devices in the group
	for dimName, trajectory := range plan.Trajectories {
		dimConfig, exists := deviceGroup.Dimensions[dimName]
		if !exists {
			continue
		}

		// Convert trajectory points to motion commands
		for _, point := range trajectory {
			cmd := types.MotionCommand{
				DeviceID:     deviceID,
				CommandType:  plan.Type,
				Position:     point.Position,
				Velocity:     point.Velocity,
				Acceleration: types.Velocity{Linear: dimConfig.MaxAccel, Angular: 0},
				Timestamp:    point.Timestamp,
			}
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

// Utility methods

// getTargetValueForDimension extracts the target value for a specific dimension
func (td *TaskDecomposer) getTargetValueForDimension(target types.Point, dimName string) float64 {
	switch dimName {
	case "X", "x":
		return target.X
	case "Y", "y":
		return target.Y
	case "Z", "z":
		return target.Z
	default:
		// For custom dimensions, you might need a more sophisticated mapping
		return target.X // Default fallback
	}
}

// createPointForDimension creates a Point with the value set for the specific dimension
func (td *TaskDecomposer) createPointForDimension(value float64, dimName string) types.Point {
	switch dimName {
	case "X", "x":
		return types.Point{X: value, Y: 0, Z: 0}
	case "Y", "y":
		return types.Point{X: 0, Y: value, Z: 0}
	case "Z", "z":
		return types.Point{X: 0, Y: 0, Z: value}
	default:
		// For custom dimensions, use X as default
		return types.Point{X: value, Y: 0, Z: 0}
	}
}


func (td *TaskDecomposer) ValidateTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.Type == "" {
		return fmt.Errorf("task type cannot be empty")
	}

	if task.DeviceGroup == "" {
		return fmt.Errorf("task must specify a device group")
	}

	if len(task.Dimensions) == 0 {
		return fmt.Errorf("task must specify at least one dimension")
	}

	// Validate device group exists
	deviceGroup, err := td.getDeviceGroupConfig(task.DeviceGroup)
	if err != nil {
		return fmt.Errorf("device group validation failed: %w", err)
	}

	// Validate dimensions exist in device group
	for _, dimName := range task.Dimensions {
		dimConfig, exists := deviceGroup.Dimensions[dimName]
		if !exists {
			return fmt.Errorf("dimension %s not found in device group %s", dimName, task.DeviceGroup)
		}

		// Validate velocity constraints
		if task.Velocity.Linear > dimConfig.MaxVelocity {
			return fmt.Errorf("velocity %f exceeds maximum %f for dimension %s in device group %s",
				task.Velocity.Linear, dimConfig.MaxVelocity, dimName, task.DeviceGroup)
		}

		// Validate position constraints for absolute moves
		if task.Type == types.CommandMoveTo {
			targetValue := td.getTargetValueForDimension(task.Target, dimName)
			if targetValue < dimConfig.MinValue || targetValue > dimConfig.MaxValue {
				return fmt.Errorf("target position %f for dimension %s is out of bounds [%f, %f]",
					targetValue, dimName, dimConfig.MinValue, dimConfig.MaxValue)
			}
		}
	}

	return nil
}