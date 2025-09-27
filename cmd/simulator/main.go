// Command simulator implements a test client for the motion control system
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"control/internal/ipc"
	"control/pkg/types"
)

type Simulator struct {
	ipcClient *ipc.IPCClient
	clientID  string
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewSimulator(config types.IPCConfig) *Simulator {
	return &Simulator{
		ipcClient: ipc.NewIPCClient(config),
		clientID:  fmt.Sprintf("simulator-%d", time.Now().UnixNano()),
	}
}

func (s *Simulator) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	if err := s.ipcClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to IPC server: %w", err)
	}

	s.ipcClient.RegisterHandler("status_response", s.handleStatusResponse)
	s.ipcClient.RegisterHandler("task_response", s.handleTaskResponse)
	s.ipcClient.RegisterHandler("task_template_response", s.handleTaskTemplateResponse)
	s.ipcClient.RegisterHandler("task_node_response", s.handleTaskNodeResponse)
	s.ipcClient.RegisterHandler("abstract_command_response", s.handleAbstractCommandResponse)
	s.ipcClient.RegisterHandler("error_response", s.handleErrorResponse)

	s.running = true
	go s.runSimulation()

	log.Printf("Simulator %s started", s.clientID)
	return nil
}

func (s *Simulator) Stop() error {
	if !s.running {
		return fmt.Errorf("simulator is not running")
	}

	s.cancel()
	s.ipcClient.Disconnect()
	s.running = false

	log.Printf("Simulator %s stopped", s.clientID)
	return nil
}

func (s *Simulator) runSimulation() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.simulateActivity()
		}
	}
}

func (s *Simulator) simulateActivity() {
	action := rand.Intn(8)

	switch action {
	case 0:
		s.sendStatusRequest()
	case 1:
		s.sendMoveTask()
	case 2:
		s.sendHomeTask()
	case 3:
		s.sendTaskTemplateRequest()
	case 4:
		s.sendTaskNodeRequest()
	case 5:
		s.sendTemplateExecutionRequest()
	case 6:
		s.sendAbstractCommandRequest()
	case 7:
		s.sendCommandListRequest()
	}
}

func (s *Simulator) sendStatusRequest() {
	message := types.IPCMessage{
		Type:      "status_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data:      map[string]interface{}{"request": "full_status"},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send status request: %v", err)
	}
}

func (s *Simulator) sendMoveTask() {
	target := types.Point{
		X: rand.Float64()*200 - 100,
		Y: rand.Float64()*200 - 100,
		Z: rand.Float64()*100 - 50,
	}

	velocity := types.Velocity{
		Linear:  rand.Float64()*50 + 10,
		Angular: rand.Float64() * 10,
	}

	taskData := map[string]interface{}{
		"type":     "move_to",
		"target":   target,
		"velocity": velocity,
		"axes":     []string{"axis-x", "axis-y"},
	}

	message := types.IPCMessage{
		Type:      "task_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data:      map[string]interface{}{"task": taskData},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send move task: %v", err)
	} else {
		log.Printf("Sent move task: target=%v, velocity=%v", target, velocity)
	}
}

func (s *Simulator) sendHomeTask() {
	taskData := map[string]interface{}{
		"type": "home",
		"axes": []string{"axis-x", "axis-y", "axis-z"},
	}

	message := types.IPCMessage{
		Type:      "task_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data:      map[string]interface{}{"task": taskData},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send home task: %v", err)
	} else {
		log.Println("Sent home task")
	}
}

func (s *Simulator) handleStatusResponse(message types.IPCMessage) {
	data, err := json.MarshalIndent(message.Data, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal status response: %v", err)
		return
	}

	log.Printf("Status received from %s:\n%s", message.Source, string(data))
}

func (s *Simulator) handleTaskResponse(message types.IPCMessage) {
	taskID := message.Data["task_id"].(string)
	status := message.Data["status"].(string)
	log.Printf("Task response: %s -> %s", taskID, status)
}

func (s *Simulator) handleTaskTemplateResponse(message types.IPCMessage) {
	if templates, ok := message.Data["templates"].([]interface{}); ok {
		log.Printf("Available task templates: %v", templates)
	} else if template, ok := message.Data["template"].(map[string]interface{}); ok {
		if name, ok := template["name"].(string); ok {
			log.Printf("Task template details: %s", name)
		}
	} else if taskID, ok := message.Data["task_id"].(string); ok {
		if status, ok := message.Data["status"].(string); ok {
			log.Printf("Template execution response: %s -> %s", taskID, status)
		}
	}
}

func (s *Simulator) handleTaskNodeResponse(message types.IPCMessage) {
	if taskID, ok := message.Data["task_id"].(string); ok {
		if status, ok := message.Data["status"].(string); ok {
			log.Printf("Task node response: %s -> %s", taskID, status)
		}
	}
}

func (s *Simulator) handleErrorResponse(message types.IPCMessage) {
	if errorType, ok := message.Data["error_type"].(string); ok {
		if errorMsg, ok := message.Data["message"].(string); ok {
			log.Printf("Error response: %s - %s", errorType, errorMsg)
		}
	}
}

func (s *Simulator) handleAbstractCommandResponse(message types.IPCMessage) {
	if command, ok := message.Data["command"].(string); ok {
		if taskID, ok := message.Data["task_id"].(string); ok {
			if status, ok := message.Data["status"].(string); ok {
				log.Printf("Abstract command response: %s -> %s (task: %s)", command, status, taskID)
			}
		}
	}
}

func (s *Simulator) sendTaskTemplateRequest() {
	actions := []string{"list", "get"}
	action := actions[rand.Intn(len(actions))]

	message := types.IPCMessage{
		Type:      "task_template_request",
		Source:    s.clientID,
		Target:    "control_system",
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	switch action {
	case "list":
		message.Data = map[string]interface{}{"action": "list"}
	case "get":
		templates := []string{"home_all", "safety_check", "motor_test"}
		templateName := templates[rand.Intn(len(templates))]
		message.Data = map[string]interface{}{
			"action":        "get",
			"template_name": templateName,
		}
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send task template request: %v", err)
	} else {
		log.Printf("Sent task template request: action=%s", action)
	}
}

func (s *Simulator) sendTaskNodeRequest() {
	nodeTypes := []string{"io", "motor_control", "delay", "motor_status"}
	nodeType := nodeTypes[rand.Intn(len(nodeTypes))]

	var nodeData map[string]interface{}

	switch nodeType {
	case "io":
		nodeData = map[string]interface{}{
			"type": "io",
			"parameters": map[string]interface{}{
				"device_id": "device-1",
				"channel":   fmt.Sprintf("sensor-%d", rand.Intn(5)+1),
				"action":    []string{"read", "set", "toggle"}[rand.Intn(3)],
				"value":     rand.Float64() * 100,
			},
		}
	case "motor_control":
		nodeData = map[string]interface{}{
			"type": "motor_control",
			"parameters": map[string]interface{}{
				"device_id": "device-1",
				"motor_id":  fmt.Sprintf("motor-%d", rand.Intn(3)+1),
				"action":    []string{"start", "stop", "set_speed"}[rand.Intn(3)],
				"speed":     rand.Float64()*200 + 50,
			},
		}
	case "delay":
		nodeData = map[string]interface{}{
			"type": "delay",
			"parameters": map[string]interface{}{
				"duration": time.Duration(rand.Intn(5)+1) * time.Second,
				"unit":     "s",
			},
		}
	case "motor_status":
		nodeData = map[string]interface{}{
			"type": "motor_status",
			"parameters": map[string]interface{}{
				"device_id": "device-1",
				"motor_id":  fmt.Sprintf("motor-%d", rand.Intn(3)+1),
			},
		}
	}

	message := types.IPCMessage{
		Type:      "task_node_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data:      map[string]interface{}{"node": nodeData},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send task node request: %v", err)
	} else {
		log.Printf("Sent task node request: type=%s", nodeType)
	}
}

func (s *Simulator) sendTemplateExecutionRequest() {
	templates := []string{"home_all", "safety_check", "motor_test"}
	templateName := templates[rand.Intn(len(templates))]

	message := types.IPCMessage{
		Type:      "task_template_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data: map[string]interface{}{
			"action":        "execute",
			"template_name": templateName,
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send template execution request: %v", err)
	} else {
		log.Printf("Sent template execution request: template=%s", templateName)
	}
}

func (s *Simulator) sendAbstractCommandRequest() {
	commands := []string{"self_check", "reset", "start", "stop", "emergency_stop", "home", "initialize", "safety_check"}
	command := commands[rand.Intn(len(commands))]

	message := types.IPCMessage{
		Type:      "abstract_command_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data: map[string]interface{}{
			"command": command,
			"parameters": map[string]interface{}{
				"speed":     rand.Float64()*200 + 50,
				"timeout":   time.Duration(rand.Intn(30)+5) * time.Second,
			},
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send abstract command request: %v", err)
	} else {
		log.Printf("Sent abstract command request: command=%s", command)
	}
}

func (s *Simulator) sendCommandListRequest() {
	message := types.IPCMessage{
		Type:      "abstract_command_request",
		Source:    s.clientID,
		Target:    "control_system",
		Data: map[string]interface{}{
			"command": "list",
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send command list request: %v", err)
	} else {
		log.Println("Sent command list request")
	}
}

func main() {
	var (
		address  = flag.String("address", "127.0.0.1", "IPC server address")
		port     = flag.Int("port", 18080, "IPC server port")
		duration = flag.Duration("duration", 30*time.Second, "Simulation duration")
	)

	flag.Parse()

	fmt.Printf("Motion Control Simulator - Starting up...\n")
	fmt.Printf("Connecting to %s:%d\n", *address, *port)

	config := types.IPCConfig{
		Type:       "tcp",
		Address:    *address,
		Port:       *port,
		Timeout:    5 * time.Second,
		BufferSize: 1024,
	}

	simulator := NewSimulator(config)

	if err := simulator.Start(); err != nil {
		log.Fatalf("Failed to start simulator: %v", err)
	}

	fmt.Printf("Simulator running for %v...\n", *duration)
	time.Sleep(*duration)

	if err := simulator.Stop(); err != nil {
		log.Printf("Error stopping simulator: %v", err)
	}

	fmt.Println("Simulator shutdown complete")
}
