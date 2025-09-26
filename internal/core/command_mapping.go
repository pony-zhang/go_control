package core

import (
	"fmt"
	"time"

	"control/pkg/types"
)

type CommandMappingManager struct {
	config       types.SystemConfig
	decomposer   *TaskNodeDecomposer
}

func NewCommandMappingManager(config types.SystemConfig, decomposer *TaskNodeDecomposer) *CommandMappingManager {
	return &CommandMappingManager{
		config:     config,
		decomposer: decomposer,
	}
}

func (cmm *CommandMappingManager) ExecuteAbstractCommand(abstractCmd types.AbstractCommand, params map[string]interface{}) (*types.Task, error) {
	mapping, exists := cmm.config.CommandMappings[abstractCmd]
	if !exists {
		return nil, fmt.Errorf("abstract command '%s' not found in mapping", abstractCmd)
	}

	// Merge provided parameters with mapping parameters
	finalParams := make(map[string]interface{})
	for k, v := range mapping.Parameters {
		finalParams[k] = v
	}
	for k, v := range params {
		finalParams[k] = v
	}

	// Create task from abstract command
	task := &types.Task{
		ID:         fmt.Sprintf("abstract-%s-%d", abstractCmd, time.Now().UnixNano()),
		Type:       types.CommandType(abstractCmd),
		Priority:   mapping.Priority,
		Status:     types.StatusPending,
		Parameters: finalParams,
		CreatedAt:  time.Now(),
		Timeout:    mapping.Timeout,
	}

	// If mapping references a template, use that
	if mapping.Template != "" {
		template, exists := cmm.config.TaskTemplates[mapping.Template]
		if !exists {
			return nil, fmt.Errorf("template '%s' not found for command '%s'", mapping.Template, abstractCmd)
		}

		// Create task node from template
		if len(template.Nodes) > 0 {
			task.RootNode = cmm.createTaskNodeFromConfigs(template.Nodes)
		}
		task.Type = template.Type
		task.Timeout = template.Timeout
		task.Priority = template.Priority
	}

	// If mapping has direct nodes, use those
	if len(mapping.Nodes) > 0 {
		task.RootNode = cmm.createTaskNodeFromConfigs(mapping.Nodes)
	}

	// If no template or nodes, create a default mapping
	if task.RootNode == nil {
		task.RootNode = cmm.createDefaultTaskNode(abstractCmd, finalParams)
	}

	return task, nil
}

func (cmm *CommandMappingManager) createTaskNodeFromConfigs(nodeConfigs []types.TaskNodeConfig) types.TaskNode {
	if len(nodeConfigs) == 0 {
		return nil
	}

	// Create root node
	rootConfig := nodeConfigs[0]
	rootNode, err := cmm.decomposer.CreateTaskNodeFromConfig(string(rootConfig.Type), rootConfig.Parameters)
	if err != nil {
		return nil
	}

	// Add child nodes
	for _, childConfig := range nodeConfigs[1:] {
		childNode, err := cmm.decomposer.CreateTaskNodeFromConfig(string(childConfig.Type), childConfig.Parameters)
		if err != nil {
			continue
		}
		rootNode.AddChild(childNode)
	}

	return rootNode
}

func (cmm *CommandMappingManager) createDefaultTaskNode(abstractCmd types.AbstractCommand, params map[string]interface{}) types.TaskNode {
	switch abstractCmd {
	case types.AbstractSelfCheck:
		return cmm.createSelfCheckNode(params)
	case types.AbstractReset:
		return cmm.createResetNode(params)
	case types.AbstractStart:
		return cmm.createStartNode(params)
	case types.AbstractStop:
		return cmm.createStopNode(params)
	case types.AbstractEmergencyStop:
		return cmm.createEmergencyStopNode(params)
	case types.AbstractHome:
		return cmm.createHomeNode(params)
	case types.AbstractInitialize:
		return cmm.createInitializeNode(params)
	case types.AbstractSafetyCheck:
		return cmm.createSafetyCheckNode(params)
	default:
		// Create a simple delay node as fallback
		delayCmd := types.DelayCommand{
			Duration: 1 * time.Second,
			Unit:     "s",
		}
		return types.NewDelayTaskNode(fmt.Sprintf("abstract-%s", abstractCmd), delayCmd)
	}
}

func (cmm *CommandMappingManager) createSelfCheckNode(params map[string]interface{}) types.TaskNode {
	// Create a sequence of self-check operations
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("self-check-sequence", sequenceCmd)

	// Add IO check nodes
	ioCmd := types.IOCommand{
		DeviceID:  "device-1",
		Channel:   "system-status",
		Action:    "read",
		Timeout:   5 * time.Second,
	}
	ioNode := types.NewIOTaskNode("self-check-io", ioCmd)
	sequenceNode.AddChild(ioNode)

	// Add delay
	delayCmd := types.DelayCommand{
		Duration: 2 * time.Second,
		Unit:     "s",
	}
	delayNode := types.NewDelayTaskNode("self-check-delay", delayCmd)
	sequenceNode.AddChild(delayNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) createResetNode(params map[string]interface{}) types.TaskNode {
	// Create reset sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("reset-sequence", sequenceCmd)

	// Stop all motors first
	motorCmd := types.MotorCommand{
		DeviceID: "device-1",
		MotorID:  "all",
		Action:   "stop",
	}
	motorNode := types.NewMotorTaskNode("reset-stop-motors", motorCmd)
	sequenceNode.AddChild(motorNode)

	// Add delay
	delayCmd := types.DelayCommand{
		Duration: 1 * time.Second,
		Unit:     "s",
	}
	delayNode := types.NewDelayTaskNode("reset-delay", delayCmd)
	sequenceNode.AddChild(delayNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) createStartNode(params map[string]interface{}) types.TaskNode {
	// Create start sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("start-sequence", sequenceCmd)

	// Safety check first
	safetyNode := cmm.createSafetyCheckNode(params)
	sequenceNode.AddChild(safetyNode)

	// Start motors
	motorCmd := types.MotorCommand{
		DeviceID: "device-1",
		MotorID:  "main",
		Action:   "start",
		Speed:    100.0,
	}
	motorNode := types.NewMotorTaskNode("start-motor", motorCmd)
	sequenceNode.AddChild(motorNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) createStopNode(params map[string]interface{}) types.TaskNode {
	// Create stop sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("stop-sequence", sequenceCmd)

	// Stop all motors
	motorCmd := types.MotorCommand{
		DeviceID: "device-1",
		MotorID:  "all",
		Action:   "stop",
	}
	motorNode := types.NewMotorTaskNode("stop-motor", motorCmd)
	sequenceNode.AddChild(motorNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) createEmergencyStopNode(params map[string]interface{}) types.TaskNode {
	// Emergency stop - immediate stop without sequence
	motorCmd := types.MotorCommand{
		DeviceID: "device-1",
		MotorID:  "all",
		Action:   "stop",
	}
	return types.NewMotorTaskNode("emergency-stop", motorCmd)
}

func (cmm *CommandMappingManager) createHomeNode(params map[string]interface{}) types.TaskNode {
	// Create homing sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("home-sequence", sequenceCmd)

	// Home each axis
	axes := []string{"axis-x", "axis-y", "axis-z"}
	for _, axis := range axes {
		homeCmd := types.IOCommand{
			DeviceID:  "device-1",
			Channel:   axis,
			Action:    "home",
			Timeout:   30 * time.Second,
		}
		homeNode := types.NewIOTaskNode(fmt.Sprintf("home-%s", axis), homeCmd)
		sequenceNode.AddChild(homeNode)
	}

	return sequenceNode
}

func (cmm *CommandMappingManager) createInitializeNode(params map[string]interface{}) types.TaskNode {
	// Create initialization sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("initialize-sequence", sequenceCmd)

	// Self check
	selfCheckNode := cmm.createSelfCheckNode(params)
	sequenceNode.AddChild(selfCheckNode)

	// Home sequence
	homeNode := cmm.createHomeNode(params)
	sequenceNode.AddChild(homeNode)

	// Safety check
	safetyNode := cmm.createSafetyCheckNode(params)
	sequenceNode.AddChild(safetyNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) createSafetyCheckNode(params map[string]interface{}) types.TaskNode {
	// Create safety check sequence
	sequenceCmd := types.SequenceCommand{
		Mode:         "sequential",
		StopOnError:  true,
	}

	sequenceNode := types.NewSequenceTaskNode("safety-check-sequence", sequenceCmd)

	// Check emergency stop
	emergencyCmd := types.IOCommand{
		DeviceID:  "device-1",
		Channel:   "emergency-stop",
		Action:    "read",
		Timeout:   5 * time.Second,
	}
	emergencyNode := types.NewIOTaskNode("safety-check-emergency", emergencyCmd)
	sequenceNode.AddChild(emergencyNode)

	// Check safety door
	doorCmd := types.IOCommand{
		DeviceID:  "device-1",
		Channel:   "safety-door",
		Action:    "read",
		Timeout:   5 * time.Second,
	}
	doorNode := types.NewIOTaskNode("safety-check-door", doorCmd)
	sequenceNode.AddChild(doorNode)

	return sequenceNode
}

func (cmm *CommandMappingManager) GetAvailableCommands() []types.AbstractCommand {
	commands := make([]types.AbstractCommand, 0, len(cmm.config.CommandMappings))
	for cmd := range cmm.config.CommandMappings {
		commands = append(commands, cmd)
	}
	return commands
}

func (cmm *CommandMappingManager) GetCommandDescription(abstractCmd types.AbstractCommand) (string, error) {
	mapping, exists := cmm.config.CommandMappings[abstractCmd]
	if !exists {
		return "", fmt.Errorf("abstract command '%s' not found", abstractCmd)
	}
	return mapping.Description, nil
}