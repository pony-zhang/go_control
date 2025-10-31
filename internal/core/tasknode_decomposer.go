// Package core implements task node decomposition for complex motion sequences
package core

import (
	"context"
	"fmt"
	"time"

	"control/pkg/types"
)

type TaskNodeDecomposer struct {
	config   types.SystemConfig
	devices  map[types.DeviceID]Device
}

func NewTaskNodeDecomposer(config types.SystemConfig) *TaskNodeDecomposer {
	return &TaskNodeDecomposer{
		config:  config,
		devices: make(map[types.DeviceID]Device),
	}
}

func (tnd *TaskNodeDecomposer) SetDevice(device Device) {
	tnd.devices[device.ID()] = device
}

func (tnd *TaskNodeDecomposer) DecomposeTask(task *types.Task) ([]types.MotionCommand, error) {
	if task.RootNode == nil {
		return tnd.decomposeLegacyTask(task)
	}

	return tnd.decomposeTaskNode(context.Background(), task.RootNode)
}

func (tnd *TaskNodeDecomposer) decomposeLegacyTask(task *types.Task) ([]types.MotionCommand, error) {
	var commands []types.MotionCommand

	switch task.Type {
	case types.CommandMoveTo:
		commands = tnd.decomposeMoveTo(task)
	case types.CommandMoveRelative:
		commands = tnd.decomposeMoveRelative(task)
	case types.CommandHome:
		commands = tnd.decomposeHome(task)
	case types.CommandStop:
		commands = tnd.decomposeStop(task)
	case types.CommandJog:
		commands = tnd.decomposeJog(task)
	default:
		return nil, fmt.Errorf("unsupported legacy task type: %s", task.Type)
	}

	for i := range commands {
		commands[i].ID = fmt.Sprintf("%s-cmd-%d", task.ID, i)
		commands[i].Timestamp = time.Now()
		commands[i].Timeout = task.Timeout
	}

	return commands, nil
}

func (tnd *TaskNodeDecomposer) decomposeTaskNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {

	switch node.GetType() {
	case types.CommandMoveTo, types.CommandMoveRelative, types.CommandHome, types.CommandStop, types.CommandJog:
		return tnd.decomposeMotionNode(ctx, node)
	case types.CommandIO:
		return tnd.decomposeIONode(ctx, node)
	case types.CommandMotorControl:
		return tnd.decomposeMotorNode(ctx, node)
	case types.CommandMotorStatus:
		return tnd.decomposeMotorStatusNode(ctx, node)
	case types.CommandDelay:
		return tnd.decomposeDelayNode(ctx, node)
	case types.CommandSequence, types.CommandParallel:
		return tnd.decomposeSequenceNode(ctx, node)
	case types.CommandCondition:
		return tnd.decomposeConditionNode(ctx, node)
	default:
		return nil, fmt.Errorf("unsupported task node type: %s", node.GetType())
	}
}

func (tnd *TaskNodeDecomposer) decomposeMotionNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()
	task := &types.Task{
		ID:         node.GetID(),
		Type:       node.GetType(),
		Parameters: params,
		Timeout:    30 * time.Second,
	}

	if target, ok := params["target"].(map[string]interface{}); ok {
		var x, y, z float64
		if xVal, exists := target["x"]; exists && xVal != nil {
			x = xVal.(float64)
		}
		if yVal, exists := target["y"]; exists && yVal != nil {
			y = yVal.(float64)
		}
		if zVal, exists := target["z"]; exists && zVal != nil {
			z = zVal.(float64)
		}
		task.Target = types.Point{X: x, Y: y, Z: z}
	}

	if velocity, ok := params["velocity"].(map[string]interface{}); ok {
		var linear, angular float64
		if linearVal, exists := velocity["linear"]; exists && linearVal != nil {
			linear = linearVal.(float64)
		}
		if angularVal, exists := velocity["angular"]; exists && angularVal != nil {
			angular = angularVal.(float64)
		}
		task.Velocity = types.Velocity{Linear: linear, Angular: angular}
	}

	// Get device group from parameters or use default
	if deviceGroup, ok := params["device_group"].(string); ok && deviceGroup != "" {
		task.DeviceGroup = deviceGroup
	} else {
		task.DeviceGroup = "main_workspace" // Default device group
	}

	// Get dimensions from parameters or use default
	if dimensions, ok := params["dimensions"].([]interface{}); ok {
		for _, dim := range dimensions {
			if dim != nil {
				task.Dimensions = append(task.Dimensions, dim.(string))
			}
		}
	} else {
		// Default to X, Y, Z dimensions for backward compatibility
		task.Dimensions = []string{"X", "Y", "Z"}
	}

	// Get workspace from parameters or use default
	if workspace, ok := params["workspace"].(string); ok && workspace != "" {
		task.WorkSpace = workspace
	} else {
		task.WorkSpace = "default" // Default workspace
	}

	switch node.GetType() {
	case types.CommandMoveTo:
		return tnd.decomposeMoveTo(task), nil
	case types.CommandMoveRelative:
		return tnd.decomposeMoveRelative(task), nil
	case types.CommandHome:
		return tnd.decomposeHome(task), nil
	case types.CommandStop:
		return tnd.decomposeStop(task), nil
	case types.CommandJog:
		return tnd.decomposeJog(task), nil
	default:
		return nil, fmt.Errorf("unsupported motion node type: %s", node.GetType())
	}
}

func (tnd *TaskNodeDecomposer) decomposeIONode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()

	var deviceID types.DeviceID
	switch id := params["device_id"].(type) {
	case string:
		deviceID = types.DeviceID(id)
	case types.DeviceID:
		deviceID = id
	default:
		deviceID = "device-1"
	}

	var channel string
	if ch, ok := params["channel"]; ok && ch != nil {
		channel = ch.(string)
	}

	var action string
	if act, ok := params["action"]; ok && act != nil {
		action = act.(string)
	}

	if _, exists := tnd.devices[deviceID]; !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	cmd := types.MotionCommand{
		ID:          fmt.Sprintf("%s-io", node.GetID()),
		DeviceID:    deviceID,
		CommandType: types.CommandIO,
		Timestamp:   time.Now(),
		Timeout:     5 * time.Second,
	}

	switch action {
	case "set":
		cmd.Data = []byte(fmt.Sprintf("IO_SET:%s:%.2f", channel, params["value"]))
	case "read":
		cmd.Data = []byte(fmt.Sprintf("IO_READ:%s", channel))
	case "toggle":
		cmd.Data = []byte(fmt.Sprintf("IO_TOGGLE:%s", channel))
	}

	return []types.MotionCommand{cmd}, nil
}

func (tnd *TaskNodeDecomposer) decomposeMotorNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()

	var deviceID types.DeviceID
	switch id := params["device_id"].(type) {
	case string:
		deviceID = types.DeviceID(id)
	case types.DeviceID:
		deviceID = id
	default:
		deviceID = "device-1"
	}

	var motorID string
	if mid, ok := params["motor_id"]; ok && mid != nil {
		motorID = mid.(string)
	}

	var action string
	if act, ok := params["action"]; ok && act != nil {
		action = act.(string)
	}

	if _, exists := tnd.devices[deviceID]; !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	cmd := types.MotionCommand{
		ID:          fmt.Sprintf("%s-motor", node.GetID()),
		DeviceID:    deviceID,
		CommandType: types.CommandMotorControl,
		Timestamp:   time.Now(),
		Timeout:     10 * time.Second,
	}

	switch action {
	case "start":
		cmd.Data = []byte(fmt.Sprintf("MOTOR_START:%s", motorID))
	case "stop":
		cmd.Data = []byte(fmt.Sprintf("MOTOR_STOP:%s", motorID))
	case "set_speed":
		var speed float64
		if speedVal, ok := params["speed"]; ok && speedVal != nil {
			speed = speedVal.(float64)
		}
		cmd.Data = []byte(fmt.Sprintf("MOTOR_SPEED:%s:%.2f", motorID, speed))
	case "set_position":
		if pos, ok := params["position"].(map[string]interface{}); ok {
			var x, y, z float64
			if xVal, exists := pos["x"]; exists && xVal != nil {
				x = xVal.(float64)
			}
			if yVal, exists := pos["y"]; exists && yVal != nil {
				y = yVal.(float64)
			}
			if zVal, exists := pos["z"]; exists && zVal != nil {
				z = zVal.(float64)
			}
			cmd.Position = types.Point{X: x, Y: y, Z: z}
			cmd.Data = []byte(fmt.Sprintf("MOTOR_POS:%s:%.2f,%.2f,%.2f", motorID, x, y, z))
		}
	}

	return []types.MotionCommand{cmd}, nil
}

func (tnd *TaskNodeDecomposer) decomposeMotorStatusNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()

	var deviceID types.DeviceID
	switch id := params["device_id"].(type) {
	case string:
		deviceID = types.DeviceID(id)
	case types.DeviceID:
		deviceID = id
	default:
		deviceID = "device-1"
	}

	var motorID string
	if mid, ok := params["motor_id"]; ok && mid != nil {
		motorID = mid.(string)
	}

	if _, exists := tnd.devices[deviceID]; !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	cmd := types.MotionCommand{
		ID:          fmt.Sprintf("%s-status", node.GetID()),
		DeviceID:    deviceID,
		CommandType: types.CommandMotorStatus,
		Timestamp:   time.Now(),
		Timeout:     3 * time.Second,
		Data:        []byte(fmt.Sprintf("MOTOR_STATUS:%s", motorID)),
	}

	return []types.MotionCommand{cmd}, nil
}

func (tnd *TaskNodeDecomposer) decomposeDelayNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()
	duration := params["duration"].(time.Duration)

	cmd := types.MotionCommand{
		ID:          fmt.Sprintf("%s-delay", node.GetID()),
		DeviceID:    "system",
		CommandType: types.CommandDelay,
		Timestamp:   time.Now(),
		Timeout:     duration,
		Data:        []byte(fmt.Sprintf("DELAY:%v", duration)),
	}

	return []types.MotionCommand{cmd}, nil
}

func (tnd *TaskNodeDecomposer) decomposeSequenceNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	var allCommands []types.MotionCommand

	for _, child := range node.GetChildren() {
		childCommands, err := tnd.decomposeTaskNode(ctx, child)
		if err != nil {
			return nil, fmt.Errorf("failed to decompose child node %s: %w", child.GetID(), err)
		}
		allCommands = append(allCommands, childCommands...)
	}

	return allCommands, nil
}

func (tnd *TaskNodeDecomposer) decomposeConditionNode(ctx context.Context, node types.TaskNode) ([]types.MotionCommand, error) {
	params := node.GetParameters()

	var condition string
	if cond, ok := params["condition"]; ok && cond != nil {
		condition = cond.(string)
	}

	var deviceID types.DeviceID
	switch id := params["device_id"].(type) {
	case string:
		deviceID = types.DeviceID(id)
	case types.DeviceID:
		deviceID = id
	default:
		deviceID = "device-1"
	}

	var timeout time.Duration
	if t, ok := params["timeout"]; ok && t != nil {
		switch timeoutVal := t.(type) {
		case time.Duration:
			timeout = timeoutVal
		case float64:
			timeout = time.Duration(timeoutVal) * time.Second
		case int64:
			timeout = time.Duration(timeoutVal) * time.Second
		default:
			timeout = 5 * time.Second
		}
	} else {
		timeout = 5 * time.Second
	}

	cmd := types.MotionCommand{
		ID:          fmt.Sprintf("%s-condition", node.GetID()),
		DeviceID:    deviceID,
		CommandType: types.CommandCondition,
		Timestamp:   time.Now(),
		Timeout:     timeout,
		Data:        []byte(fmt.Sprintf("CONDITION:%s", condition)),
	}

	return []types.MotionCommand{cmd}, nil
}

func (tnd *TaskNodeDecomposer) decomposeMoveTo(task *types.Task) []types.MotionCommand {
	return tnd.decomposeMotionCommand(task)
}

func (tnd *TaskNodeDecomposer) decomposeMoveRelative(task *types.Task) []types.MotionCommand {
	return tnd.decomposeMotionCommand(task)
}

func (tnd *TaskNodeDecomposer) decomposeHome(task *types.Task) []types.MotionCommand {
	return tnd.decomposeMotionCommand(task)
}

func (tnd *TaskNodeDecomposer) decomposeStop(task *types.Task) []types.MotionCommand {
	return tnd.decomposeMotionCommand(task)
}

func (tnd *TaskNodeDecomposer) decomposeJog(task *types.Task) []types.MotionCommand {
	return tnd.decomposeMotionCommand(task)
}

// decomposeMotionCommand is the unified method for decomposing motion commands
func (tnd *TaskNodeDecomposer) decomposeMotionCommand(task *types.Task) []types.MotionCommand {
	commands := make([]types.MotionCommand, 0)

	// Get device group configuration
	deviceGroup, exists := tnd.config.DeviceGroups[task.DeviceGroup]
	if !exists {
		// Return empty commands if device group not found
		return commands
	}

	// Generate commands for each dimension
	for _, dimName := range task.Dimensions {
		dimConfig, exists := deviceGroup.Dimensions[dimName]
		if !exists {
			continue
		}

		// For each device in the device group, generate commands
		for _, deviceID := range deviceGroup.DeviceIDs {
			if _, deviceExists := tnd.devices[deviceID]; deviceExists {
				cmd := tnd.createMotionCommand(task, deviceID, dimName, dimConfig)
				commands = append(commands, cmd)
			}
		}
	}

	return commands
}

// createMotionCommand creates a motion command for a specific device and dimension
func (tnd *TaskNodeDecomposer) createMotionCommand(task *types.Task, deviceID types.DeviceID, dimName string, dimConfig types.DimensionConfig) types.MotionCommand {
	cmd := types.MotionCommand{
		DeviceID:     deviceID,
		CommandType:  task.Type,
		Position:     tnd.createTargetPosition(task, dimName),
		Velocity:     tnd.createTargetVelocity(task, dimConfig),
		Acceleration: types.Velocity{Linear: dimConfig.MaxAccel, Angular: 0},
		Timestamp:    time.Now(),
		Timeout:     task.Timeout,
	}

	// Special handling for home command
	if task.Type == types.CommandHome {
		cmd.Position = tnd.createHomePosition(dimName, dimConfig)
		cmd.Velocity = types.Velocity{
			Linear:  dimConfig.MaxVelocity * 0.5,
			Angular: 0,
		}
		cmd.Acceleration = types.Velocity{
			Linear:  dimConfig.MaxAccel * 0.5,
			Angular: 0,
		}
	}

	// Special handling for stop command
	if task.Type == types.CommandStop {
		cmd.Position = types.Point{}
		cmd.Velocity = types.Velocity{Linear: 0, Angular: 0}
		cmd.Acceleration = types.Velocity{Linear: 0, Angular: 0}
	}

	return cmd
}

// createTargetPosition creates the target position for a specific dimension
func (tnd *TaskNodeDecomposer) createTargetPosition(task *types.Task, dimName string) types.Point {
	switch dimName {
	case "X", "x":
		return types.Point{X: task.Target.X, Y: 0, Z: 0}
	case "Y", "y":
		return types.Point{X: 0, Y: task.Target.Y, Z: 0}
	case "Z", "z":
		return types.Point{X: 0, Y: 0, Z: task.Target.Z}
	default:
		// For custom dimensions, use X as default with dimension-specific mapping
		return tnd.mapDimensionToPoint(task.Target, dimName)
	}
}

// createHomePosition creates the home position for a specific dimension
func (tnd *TaskNodeDecomposer) createHomePosition(dimName string, dimConfig types.DimensionConfig) types.Point {
	switch dimName {
	case "X", "x":
		return types.Point{X: dimConfig.HomeValue, Y: 0, Z: 0}
	case "Y", "y":
		return types.Point{X: 0, Y: dimConfig.HomeValue, Z: 0}
	case "Z", "z":
		return types.Point{X: 0, Y: 0, Z: dimConfig.HomeValue}
	default:
		// For custom dimensions, use X as default
		return types.Point{X: dimConfig.HomeValue, Y: 0, Z: 0}
	}
}

// createTargetVelocity creates the target velocity respecting dimension constraints
func (tnd *TaskNodeDecomposer) createTargetVelocity(task *types.Task, dimConfig types.DimensionConfig) types.Velocity {
	// Clamp velocity to dimension maximum
	maxVelocity := task.Velocity.Linear
	if maxVelocity > dimConfig.MaxVelocity {
		maxVelocity = dimConfig.MaxVelocity
	}

	return types.Velocity{
		Linear:  maxVelocity,
		Angular: task.Velocity.Angular, // TODO: Handle angular velocity for rotary dimensions
	}
}

// mapDimensionToPoint maps a custom dimension to a point
func (tnd *TaskNodeDecomposer) mapDimensionToPoint(target types.Point, dimName string) types.Point {
	// This is a simplified mapping - in a real implementation,
	// you might have a more sophisticated mapping based on dimension type
	return types.Point{X: target.X, Y: 0, Z: 0}
}

func (tnd *TaskNodeDecomposer) ValidateTaskNode(node types.TaskNode) error {
	if node == nil {
		return fmt.Errorf("task node cannot be nil")
	}

	if err := node.Validate(); err != nil {
		return fmt.Errorf("task node validation failed: %w", err)
	}

	for _, child := range node.GetChildren() {
		if err := tnd.ValidateTaskNode(child); err != nil {
			return fmt.Errorf("child node %s validation failed: %w", child.GetID(), err)
		}
	}

	return nil
}

func (tnd *TaskNodeDecomposer) CreateTaskNodeFromConfig(nodeType string, params map[string]interface{}) (types.TaskNode, error) {
	cmdType := types.CommandType(nodeType)

	switch cmdType {
	case types.CommandIO:
		var deviceID types.DeviceID
		switch id := params["device_id"].(type) {
		case string:
			deviceID = types.DeviceID(id)
		case types.DeviceID:
			deviceID = id
		default:
			deviceID = "device-1" // default device
		}
		var value float64
		if val, ok := params["value"]; ok && val != nil {
			value = val.(float64)
		}
		var channel string
		if ch, ok := params["channel"]; ok && ch != nil {
			channel = ch.(string)
		}

		var action string
		if act, ok := params["action"]; ok && act != nil {
			action = act.(string)
		}

		ioCmd := types.IOCommand{
			DeviceID:  deviceID,
			Channel:   channel,
			Action:    action,
			Value:     value,
			Timeout:   5 * time.Second,
		}
		return types.NewIOTaskNode(fmt.Sprintf("io-%d", time.Now().UnixNano()), ioCmd), nil

	case types.CommandMotorControl:
		var deviceID types.DeviceID
		switch id := params["device_id"].(type) {
		case string:
			deviceID = types.DeviceID(id)
		case types.DeviceID:
			deviceID = id
		default:
			deviceID = "device-1" // default device
		}
		var speed float64
		if val, ok := params["speed"]; ok && val != nil {
			speed = val.(float64)
		}

		var motorID string
		if mid, ok := params["motor_id"]; ok && mid != nil {
			motorID = mid.(string)
		}

		var action string
		if act, ok := params["action"]; ok && act != nil {
			action = act.(string)
		}

		motorCmd := types.MotorCommand{
			DeviceID: deviceID,
			MotorID:  motorID,
			Action:   action,
			Speed:    speed,
		}
		return types.NewMotorTaskNode(fmt.Sprintf("motor-%d", time.Now().UnixNano()), motorCmd), nil

	case types.CommandDelay:
		var duration time.Duration
		switch d := params["duration"].(type) {
		case time.Duration:
			duration = d
		case string:
			// Parse duration from string (e.g., "1s", "500ms")
			var err error
			duration, err = time.ParseDuration(d)
			if err != nil {
				// Default to 1 second if parsing fails
				duration = 1 * time.Second
			}
		case float64:
			duration = time.Duration(d) * time.Second
		case int64:
			duration = time.Duration(d) * time.Second
		default:
			duration = 1 * time.Second
		}
		unit := "s" // default unit
		if u, ok := params["unit"]; ok && u != nil {
			unit = u.(string)
		}
		delayCmd := types.DelayCommand{
			Duration: duration,
			Unit:     unit,
		}
		return types.NewDelayTaskNode(fmt.Sprintf("delay-%d", time.Now().UnixNano()), delayCmd), nil

	case types.CommandSequence:
		var children []types.TaskNode
		if childNodes, ok := params["children"].([]interface{}); ok {
			for _, child := range childNodes {
				if childMap, ok := child.(map[string]interface{}); ok {
					var nodeType string
					if t, exists := childMap["type"]; exists && t != nil {
						nodeType = t.(string)
					}
					if nodeType != "" {
						childNode, err := tnd.CreateTaskNodeFromConfig(nodeType, childMap)
						if err != nil {
							return nil, err
						}
						children = append(children, childNode)
					}
				}
			}
		}

		seqCmd := types.SequenceCommand{
			Nodes:        children,
			Mode:         "sequential",
			StopOnError:  true,
		}
		return types.NewSequenceTaskNode(fmt.Sprintf("seq-%d", time.Now().UnixNano()), seqCmd), nil

	default:
		return nil, fmt.Errorf("unsupported task node type: %s", nodeType)
	}
}