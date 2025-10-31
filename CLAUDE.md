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
- Implements CommandHandler interface for event-driven processing

**Event-Driven Command System:**
- **CommandRouter** (`internal/core/command_handler.go`): Centralized command routing with subscription model
- **CommandHandler Interface**: Modular command processing interface for all layers
- **Event-Driven Architecture**: Commands automatically routed to subscribed handlers
- **Decoupled Processing**: Each module handles only its subscribed commands

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

**Event-Driven Command Processing:**
- **CommandHandler Interface**: Standardized interface for command processing modules
- **Subscription-Based Routing**: Modules subscribe to commands they handle
- **Automatic Command Routing**: CommandRouter manages command distribution
- **Publisher-Subscriber Pattern**: Clean separation between command generation and processing

**Command Mapping System:**
- High-level abstract commands (e.g., "home", "emergency_stop") map to low-level command sequences
- Configurable through YAML with templates and custom node definitions
- Supports Chinese descriptions for industrial use cases
- Simplified frontend interface with automatic backend routing

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

This tests the abstract command system with various scenarios including self-check, emergency stop, and multi-command sequences.

## Device Protocol Support

Currently implemented through HAL:
- **Mock Protocol**: Simulated device for testing
- **Modbus Protocol**: Modbus TCP/RTU support
- **Serial Protocol**: Serial communication support
- **Custom Protocol**: Extensible protocol framework

Extensible through the `Protocol` interface in `internal/hal/protocol_manager.go`.

## System Operations

**High-Level Abstract Commands (via Event-Driven System):**
- **System Control**: `self_check`, `initialize_system`, `start_system`, `stop_system`, `reset_system`
- **Safety Operations**: `emergency_stop`, `safety_check`
- **Motion Control**: `home_system`, `move_to`, `move_relative`
- **Configuration**: `get_config`, `set_config`, `list_templates`
- **I/O Operations**: Sensor/actuator control through abstracted interfaces

**Event-Driven Command Flow:**
1. Frontend sends simplified `BusinessCommand` via IPC
2. `CommandRouter` automatically routes to appropriate handler
3. Handlers implement `CommandHandler` interface
4. Each module processes only its subscribed commands
5. Responses routed back to frontend through unified interface

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
- Event-driven architecture eliminates complex routing logic
- Easier debugging and troubleshooting through modular design

**Extensibility:**
- New hardware types can be added through HAL protocol adapters
- Business rules can be modified without affecting hardware layers
- New command handlers can be added by implementing `CommandHandler` interface
- Subscription-based routing allows for easy module addition

**Safety:**
- Safety checks integrated at the business logic level
- Emergency stop bypasses normal validation for immediate response
- Resource allocation prevents hardware conflicts
- Event-driven architecture ensures proper command handling

**Performance:**
- Efficient resource utilization through HAL management
- Optimized task scheduling and execution flow
- Reduced hardware communication overhead
- Event-driven processing minimizes unnecessary routing logic
