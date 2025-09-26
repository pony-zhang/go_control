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

	var commands []types.MotionCommand

	switch task.Type {
	case types.CommandMoveTo:
		commands = td.decomposeMoveTo(task)
	case types.CommandMoveRelative:
		commands = td.decomposeMoveRelative(task)
	case types.CommandHome:
		commands = td.decomposeHome(task)
	case types.CommandStop:
		commands = td.decomposeStop(task)
	case types.CommandJog:
		commands = td.decomposeJog(task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}

	for i := range commands {
		commands[i].ID = fmt.Sprintf("%s-cmd-%d", task.ID, i)
		commands[i].Timestamp = time.Now()
		commands[i].Timeout = task.Timeout
	}

	return commands, nil
}

func (td *TaskDecomposer) decomposeMoveTo(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			continue
		}

		accel := types.Velocity{
			Linear:  axisConfig.MaxAcceleration,
			Angular: 0,
		}

		trajectory, err := td.plannerPlanForAxis(task, axisID, accel)
		if err != nil {
			continue
		}

		for _, point := range trajectory {
			cmd := types.MotionCommand{
				DeviceID:    axisConfig.DeviceID,
				CommandType: task.Type,
				Position:    point.Position,
				Velocity:    point.Velocity,
				Acceleration: accel,
				Timestamp:   point.Timestamp,
			}
			commands = append(commands, cmd)
		}
	}

	return commands
}

func (td *TaskDecomposer) decomposeMoveRelative(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			continue
		}

		accel := types.Velocity{
			Linear:  axisConfig.MaxAcceleration,
			Angular: 0,
		}

		trajectory, err := td.plannerPlanRelativeForAxis(task, axisID, accel)
		if err != nil {
			continue
		}

		for _, point := range trajectory {
			cmd := types.MotionCommand{
				DeviceID:    axisConfig.DeviceID,
				CommandType: task.Type,
				Position:    point.Position,
				Velocity:    point.Velocity,
				Acceleration: accel,
				Timestamp:   point.Timestamp,
			}
			commands = append(commands, cmd)
		}
	}

	return commands
}

func (td *TaskDecomposer) decomposeHome(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			continue
		}

		cmd := types.MotionCommand{
			DeviceID:    axisConfig.DeviceID,
			CommandType: types.CommandHome,
			Position:    types.Point{X: axisConfig.HomePosition, Y: 0, Z: 0},
			Velocity:    types.Velocity{Linear: axisConfig.MaxVelocity * 0.5, Angular: 0},
			Acceleration: types.Velocity{Linear: axisConfig.MaxAcceleration * 0.5, Angular: 0},
		}
		commands = append(commands, cmd)
	}

	return commands
}

func (td *TaskDecomposer) decomposeStop(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			continue
		}

		cmd := types.MotionCommand{
			DeviceID:    axisConfig.DeviceID,
			CommandType: types.CommandStop,
			Position:    types.Point{},
			Velocity:    types.Velocity{Linear: 0, Angular: 0},
			Acceleration: types.Velocity{Linear: 0, Angular: 0},
		}
		commands = append(commands, cmd)
	}

	return commands
}

func (td *TaskDecomposer) decomposeJog(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			continue
		}

		cmd := types.MotionCommand{
			DeviceID:    axisConfig.DeviceID,
			CommandType: types.CommandJog,
			Position:    task.Target,
			Velocity:    task.Velocity,
			Acceleration: types.Velocity{Linear: axisConfig.MaxAcceleration, Angular: 0},
		}
		commands = append(commands, cmd)
	}

	return commands
}

func (td *TaskDecomposer) plannerPlanForAxis(task *types.Task, axisID types.AxisID, accel types.Velocity) ([]TrajectoryPoint, error) {
	start := types.Point{X: 0, Y: 0, Z: 0}
	end := task.Target
	return td.planner.PlanPointToPoint(start, end, task.Velocity, accel)
}

func (td *TaskDecomposer) plannerPlanRelativeForAxis(task *types.Task, axisID types.AxisID, accel types.Velocity) ([]TrajectoryPoint, error) {
	start := types.Point{X: 0, Y: 0, Z: 0}
	end := task.Target
	return td.planner.PlanLinear(start, end, task.Velocity, accel)
}

func (td *TaskDecomposer) ValidateTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.Type == "" {
		return fmt.Errorf("task type cannot be empty")
	}

	if len(task.Axes) == 0 {
		return fmt.Errorf("task must specify at least one axis")
	}

	for _, axisID := range task.Axes {
		axisConfig, exists := td.config.Axes[axisID]
		if !exists {
			return fmt.Errorf("axis %s not found in configuration", axisID)
		}

		if task.Velocity.Linear > axisConfig.MaxVelocity {
			return fmt.Errorf("velocity %f exceeds maximum for axis %s", task.Velocity.Linear, axisID)
		}

		if task.Type == types.CommandMoveTo {
			if task.Target.X < axisConfig.MinPosition || task.Target.X > axisConfig.MaxPosition {
				return fmt.Errorf("target X position %f is out of bounds for axis %s", task.Target.X, axisID)
			}
		}
	}

	return nil
}