# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

### Using build.go (Recommended)
```bash
# Build all binaries to bin/ directory
go run build.go

# Manual build
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator
```

### Testing Commands

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

This is a Go-based motion control system for industrial automation with a sophisticated layered architecture:

### Layered Architecture

**Coordination Layer:**
- **Event Loop** (`internal/core/eventloop.go`): Central processing loop with configurable interval
- **Management Layer** (`internal/management/`): Infrastructure and application managers

**Infrastructure Layer:**
- **Config Manager** (`internal/config/manager.go`): YAML-based configuration with hot-reload capability
- **Device Manager** (`internal/device/device.go`): Physical device management
- **IPC Server** (`internal/ipc/server.go`): TCP-based server for client communication
- **IPC Client** (`internal/ipc/client.go`): Client interface for connecting to control system

**Application Layer (Horizontal Sub-layers):**

**Hardware Abstraction Layer (HAL)** (`internal/hal/`):
- **HAL Coordinator** (`hal.go`): Unified hardware interface coordination
- **Device Manager** (`device_manager.go`): Device lifecycle and connection management
- **Protocol Manager** (`protocol_manager.go`): Multi-protocol support (Modbus, Serial, Custom)
- **Resource Manager** (`resource_manager.go`): Hardware resource allocation and monitoring

**Service Coordination Layer** (`internal/application/service_coordination.go`):
- Coordinates hardware resources through HAL
- Manages command execution queues
- Provides device access abstraction
- Handles resource allocation and deallocation

**Task Orchestration Layer** (`internal/application/task_orchestration.go`):
- Task scheduling and prioritization
- Command mapping and transformation
- Task decomposition into executable commands
- Flow control and task lifecycle management

**Business Logic Layer** (`internal/application/business_logic.go`):
- High-level abstract command handling
- Business rule enforcement and validation
- Safety management and emergency handling
- System status monitoring and reporting

**Core Components:**
- **Task Trigger** (`internal/core/trigger.go`): Manages incoming task requests
- **Task Scheduler** (`internal/core/scheduler.go`): Priority-based task scheduling
- **Task Decomposer** (`internal/core/decomposer.go`, `tasknode_decomposer.go`): Breaks down high-level tasks into executable commands
- **Execution Queue** (`internal/core/queue.go`): Manages command execution queue
- **Command Executor** (`internal/core/executor.go`): Executes commands on devices

## Key Design Patterns

**Layered Architecture Pattern:**
- Clear separation between coordination, infrastructure, and application layers
- Horizontal sub-layers within application for specialized responsibilities
- Dependency injection and manager pattern for component coordination

**Hardware Abstraction Layer (HAL) Pattern:**
- Unified interface for diverse hardware devices and protocols
- Protocol adapters for Modbus, Serial, and custom implementations
- Resource management for hardware allocation and monitoring
- Device lifecycle management and status monitoring

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

**Safety-First Design:**
- Emergency stop bypasses normal validation for immediate response
- Safety checks integrated into business logic layer
- Resource allocation prevents conflicts and ensures safe operation

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

Currently implemented through HAL:
- **Mock Protocol**: Simulated device for testing
- **Modbus Protocol**: Modbus TCP/RTU support
- **Serial Protocol**: Serial communication support
- **Custom Protocol**: Extensible protocol framework

Extensible through the `Protocol` interface in `internal/hal/protocol_manager.go`.

## System Operations

**High-Level Abstract Commands (via Business Logic Layer):**
- **System Control**: `self_check`, `initialize_system`, `start_system`, `stop_system`, `reset_system`
- **Safety Operations**: `emergency_stop`, `safety_check`
- **Motion Control**: `home_system`, `move_to`, `move_relative`
- **I/O Operations**: Sensor/actuator control through abstracted interfaces

**Task Node Types:**
- Basic motion: move_to, move_relative, home, stop, jog
- I/O operations: io commands for sensor/actuator control
- Motor control: motor_control, motor_status
- Flow control: sequence, parallel, condition, delay
- Safety: emergency_stop, safety_check

## Resource Management

**Hardware Resources Managed by HAL:**
- **Device Resources**: Individual device allocation and monitoring
- **Axis Resources**: Physical axis constraints and parameter management
- **Safety Resources**: Emergency stop, safety door, limit switches
- **Protocol Resources**: Communication protocol instances and connections

**Resource Allocation Features:**
- Dynamic resource allocation with conflict prevention
- Resource lifecycle management (allocate, use, release)
- Resource status monitoring and reporting
- Automatic cleanup on system shutdown

## Chinese Language Support

The system includes Chinese descriptions and comments for industrial automation use cases, particularly in command mappings and configuration descriptions.

## Application Layer Benefits

**Maintainability:**
- Clear separation of concerns across horizontal layers
- Each layer has well-defined responsibilities
- Easier debugging and troubleshooting

**Extensibility:**
- New hardware types can be added through HAL protocol adapters
- Business rules can be modified without affecting hardware layers
- New command types can be added through the mapping system

**Safety:**
- Safety checks integrated at the business logic level
- Emergency stop bypasses normal validation for immediate response
- Resource allocation prevents hardware conflicts

**Performance:**
- Efficient resource utilization through HAL management
- Optimized task scheduling and execution flow
- Reduced hardware communication overhead