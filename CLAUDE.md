# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build control and simulator binaries
go build ./cmd/control
go build ./cmd/simulator

# Build with output directory
mkdir -p bin
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator
```

## Testing Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific packages
go test ./internal/core
go test ./internal/device
go test ./internal/ipc
```

## Running Applications

```bash
# Run control system with default config
./bin/control

# Run control system with custom config
./bin/control -config /path/to/config.yaml

# Run simulator connecting to localhost:18080
./bin/simulator

# Run simulator with custom connection
./bin/simulator -address 192.168.1.100 -port 18080
```

## Architecture Overview

This is a Go-based motion control system for industrial automation with a layered architecture:

**Core Components:**
- **Event Loop** (`internal/core/eventloop.go`): Central processing loop with configurable interval
- **Task Trigger** (`internal/core/trigger.go`): Manages incoming task requests
- **Task Scheduler** (`internal/core/scheduler.go`): Priority-based task scheduling
- **Task Decomposer** (`internal/core/decomposer.go`, `tasknode_decomposer.go`): Breaks down high-level tasks into executable commands
- **Execution Queue** (`internal/core/queue.go`): Manages command execution queue
- **Command Executor** (`internal/core/executor.go`): Executes commands on devices
- **Device Manager** (`internal/device/device.go`): Manages device connections and protocols

**Communication Layer:**
- **IPC Server** (`internal/ipc/server.go`): TCP-based server for client communication
- **IPC Client** (`internal/ipc/client.go`): Client interface for connecting to control system

**Configuration Management:**
- **Config Manager** (`internal/config/manager.go`): YAML-based configuration with hot-reload capability

## Key Design Patterns

**Interface-Based Architecture:**
- All major components implement interfaces defined in `internal/core/interfaces.go`
- Device abstraction supports multiple protocols (Mock, Modbus, extensible)
- Task nodes form a tree structure for complex workflows

**Command Mapping System:**
- High-level abstract commands (e.g., "home", "emergency_stop") map to low-level command sequences
- Configurable through YAML with templates and custom node definitions
- Supports Chinese descriptions for industrial use cases

**Priority-Based Scheduling:**
- Task priorities: Emergency (0), High (1), Medium (2), Low (3)
- Preemptive scheduling with context-based cancellation

## Configuration Structure

The system uses `config.yaml` for configuration:
- **Devices**: Protocol configuration (mock, modbus, etc.)
- **Axes**: Physical axis constraints and parameters
- **Safety**: System safety limits and emergency handling
- **IPC**: Network communication settings
- **Task Templates**: Reusable task sequences
- **Command Mappings**: Abstract to concrete command mappings

## Testing

Use the provided test script:
```bash
./test_abstract_commands.sh
```

This tests the abstract command system with various scenarios including self-check, emergency stop, and multi-command sequences.

## Device Protocol Support

Currently implemented:
- **Mock Device**: Simulated device for testing
- **Modbus Device**: Modbus TCP/RTU support

Extensible through the `Device` interface in `internal/core/interfaces.go`.

## Task Node Types

Supported command types:
- Basic motion: move_to, move_relative, home, stop, jog
- I/O operations: io commands for sensor/actuator control
- Motor control: motor_control, motor_status
- Flow control: sequence, parallel, condition, delay
- Safety: emergency_stop, safety_check

## Chinese Language Support

The system includes Chinese descriptions and comments for industrial automation use cases, particularly in command mappings and configuration descriptions.