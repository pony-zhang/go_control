# Motion Control System

A comprehensive Go-based motion control core system designed for industrial automation and robotics applications.

## Quick Start

### Build
```bash
# Recommended: Use build script
go run build.go

# Manual build
mkdir -p bin
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator
```

### Run
```bash
# Control system
./bin/control

# Simulator (connects to localhost:8080)
./bin/simulator

# With custom config
./bin/control -config /path/to/config.yaml
```

## Architecture

Four-layer modular architecture with event-driven command processing:

```
┌─────────────────────────────────┐
│        External Clients         │
└─────────────┬───────────────────┘
              │ TCP (Business Commands)
              ▼
┌─────────────────────────────────┐
│      Coordination Layer        │
│   Event Loop + Management       │
└─────────────┬───────────────────┘
              │ System Events
              ▼
┌─────────────────────────────────┐
│     Infrastructure Layer       │
│ Config + Device + IPC Manager  │
└─────────────┬───────────────────┘
              │ Event-Driven Commands
              ▼
┌─────────────────────────────────┐
│      Application Layer         │
│  Business → Task → Service → HAL│
└─────────────┬───────────────────┘
              │ Hardware Commands
              ▼
┌─────────────────────────────────┐
│        Physical Devices         │
└─────────────────────────────────┘
```

## Key Features

- **Event-Driven Architecture**: CommandRouter with subscription-based command handling
- **Multi-Protocol Support**: Mock, Modbus, and extensible protocol adapters via HAL
- **Real-Time Scheduling**: Priority-based preemptive task scheduling
- **Hardware Abstraction**: Unified HAL interface for diverse hardware devices
- **Safety-First Design**: Multi-layer safety checks and emergency handling
- **Hot Configuration**: YAML-based configuration with runtime updates

## Configuration

Basic `config.yaml` structure:

```yaml
event_loop_interval: 10ms
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
  timeout: 5s

devices:
  device-1:
    type: controller
    protocol: mock
    endpoint: mock://device-1
    timeout: 10s

axes:
  axis-x:
    device_id: device-1
    max_velocity: 100.0
    max_acceleration: 50.0
    min_position: -1000.0
    max_position: 1000.0
    home_position: 0.0
    units: mm
```

## High-Level Commands

Event-driven system supports abstract commands:

- **System Control**: `self_check`, `initialize_system`, `start_system`, `stop_system`
- **Safety Operations**: `emergency_stop`, `safety_check`
- **Motion Control**: `home_system`, `move_to`, `move_relative`
- **Configuration**: `get_config`, `set_config`, `list_templates`

## Documentation

- **[Architecture](docs/architecture.md)** - Complete system architecture and design patterns
- **[Development Guide](docs/development.md)** - Development setup and contribution guidelines
- **[Data Flow](docs/data-flow.md)** - Event-driven data flow and state management

## Dependencies

- Go 1.21+
- gopkg.in/yaml.v3

## License

MIT License