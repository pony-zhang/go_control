// Command simulator implements a test client for the motion control system
package main

import (
	"context"
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

	// 注册业务响应处理器
	s.ipcClient.RegisterHandler("business_response", s.handleBusinessResponse)

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
	// 只使用新的简化业务命令
	s.simulateBusinessCommands()
}

// simulateBusinessCommands 使用新的简化业务命令
func (s *Simulator) simulateBusinessCommands() {
	commands := []types.BusinessCommand{
		types.CmdQueryStatus,
		types.CmdMove,
		types.CmdHome,
		types.CmdEmergencyStop,
		types.CmdSafetyCheck,
		types.CmdSelfCheck,
		types.CmdListTemplates,
	}

	command := commands[rand.Intn(len(commands))]

	switch command {
	case types.CmdQueryStatus:
		s.sendBusinessCommand(command, map[string]interface{}{
			"query_type": "system_status",
		})
	case types.CmdMove:
		target := types.Point{
			X: rand.Float64()*200 - 100,
			Y: rand.Float64()*200 - 100,
			Z: rand.Float64()*100 - 50,
		}
		velocity := types.Velocity{
			Linear:  rand.Float64()*50 + 10,
			Angular: rand.Float64() * 10,
		}
		s.sendBusinessCommand(command, map[string]interface{}{
			"target":   target,
			"velocity": velocity,
			"axes":     []string{"axis-x", "axis-y"},
			"mode":     []string{"precise", "rapid", "simple"}[rand.Intn(3)],
		})
	case types.CmdHome:
		s.sendBusinessCommand(command, map[string]interface{}{
			"axes": []string{"axis-x", "axis-y", "axis-z"},
			"mode": "standard",
		})
	case types.CmdEmergencyStop:
		s.sendBusinessCommand(command, map[string]interface{}{
			"reason": "simulated emergency",
		})
	case types.CmdSafetyCheck:
		s.sendBusinessCommand(command, map[string]interface{}{
			"check_type": "full_system",
		})
	case types.CmdSelfCheck:
		s.sendBusinessCommand(command, map[string]interface{}{
			"check_level": "comprehensive",
		})
	case types.CmdListTemplates:
		s.sendBusinessCommand(command, map[string]interface{}{})
	}
}

// handleBusinessResponse 处理新的业务命令响应
func (s *Simulator) handleBusinessResponse(message types.IPCMessage) {
	if requestID, ok := message.Data["request_id"].(string); ok {
		if status, ok := message.Data["status"].(string); ok {
			if data, ok := message.Data["data"].(map[string]interface{}); ok {
				log.Printf("Business response: %s -> %s", requestID, status)
				log.Printf("Response data: %+v", data)
			} else if errorMsg, ok := message.Data["error"].(string); ok {
				log.Printf("Business error response: %s -> %s", requestID, errorMsg)
			} else {
				log.Printf("Business response: %s -> %s (no additional data)", requestID, status)
			}
		}
	}
}

// sendBusinessCommand 发送新的简化业务命令
func (s *Simulator) sendBusinessCommand(command types.BusinessCommand, params map[string]interface{}) {
	message := types.IPCMessage{
		Type:   "business_command",
		Source: s.clientID,
		Target: "control_system",
		Data: map[string]interface{}{
			"command": string(command),
			"params":  params,
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}

	if err := s.ipcClient.Send(message); err != nil {
		log.Printf("Failed to send business command %s: %v", command, err)
	} else {
		log.Printf("Sent business command: %s", command)
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
