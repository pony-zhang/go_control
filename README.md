# Motion Control System

A comprehensive Go-based motion control core system designed for industrial automation and robotics applications.

## Architecture

The system implements a modular, layered architecture:

```
┌──────────────────────┐
│      GUI Client      │ ←→ IPC ↔
├──────────────────────┤
│      Event Loop      │
├──────────────────────┤
│     Task Trigger     │
├──────────────────────┤
│ Task Dispatch & Scheduler │
├──────────────────────┤
│    Task Decomposer   │
├──────────────────────┤
│    Execution Queue   │
├──────────────────────┤
│   Command Executor   │
├──────────────────────┤
│   Device Interface   │
└──────────────────────┘
```

## Key Features

- **Multi-protocol Device Support**: Mock, Modbus, and extensible protocol adapters
- **Real-time Task Scheduling**: Priority-based, preemptive task scheduling
- **Trajectory Planning**: Linear, circular, and point-to-point motion planning
- **IPC Communication**: TCP-based client-server architecture
- **Configuration Management**: YAML-based configuration with hot-reload
- **Safety Features**: Position limits, velocity constraints, and emergency handling

## Building

```bash
# Build both control and simulator
go build ./cmd/control
go build ./cmd/simulator

# Or build with output directory
mkdir -p bin
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator
```

## Running

### Control System
```bash
# Run with default configuration
./bin/control

# Run with custom configuration
./bin/control -config /path/to/config.yaml
```

### Simulator
```bash
# Run simulator connecting to localhost:8080
./bin/simulator

# Connect to different address/port
./bin/simulator -address 192.168.1.100 -port 8080
```

## Configuration

The system uses YAML configuration files. A default configuration will be created if none exists:

```yaml
event_loop_interval: 10ms
queue_size: 1000
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
  timeout: 5s
  buffer_size: 1024
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

## Protocol Support

### Mock Device
- Simulated device for testing
- No external dependencies
- Supports all basic motion commands

### Modbus Device
- Modbus TCP/RTU support
- Register mapping for position/velocity control
- Configurable endpoint and timeout

### Extending
Add new protocols by implementing the `Device` interface:

```go
type CustomDevice struct {
    // device-specific fields
}

func (d *CustomDevice) Write(cmd types.MotionCommand) error {
    // implementation
}

func (d *CustomDevice) Read(reg string) (interface{}, error) {
    // implementation
}

// Other required methods...
```

## IPC Messages

The system supports various message types:

- `task_request`: Submit new motion tasks
- `status_request`: Query system status
- `config_update`: Update configuration
- `task_response`: Task execution feedback
- `status_response`: System status information

## Safety Features

- **Position Limits**: Configurable min/max positions per axis
- **Velocity Constraints**: Maximum velocity and acceleration limits
- **Emergency Stop**: Immediate halt capability
- **Timeout Handling**: Configurable timeouts for all operations
- **Error Recovery**: Graceful error handling and reporting

## Development

### Project Structure
```
control/
├── cmd/                    # Executable applications
│   ├── control/           # Main control system
│   └── simulator/         # Test simulator
├── internal/              # Internal packages
│   ├── core/              # Core system components
│   ├── device/            # Device implementations
│   ├── ipc/               # Communication layer
│   └── config/            # Configuration management
├── pkg/                   # Public packages
│   └── types/             # Common types and interfaces
├── bin/                   # Compiled binaries
└── config.yaml           # Configuration file
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/core
```

### Dependencies
- Go 1.19+
- gopkg.in/yaml.v3 (for configuration)

## Contributing

1. Follow Go standard formatting
2. Add comprehensive tests for new features
3. Update documentation
4. Ensure all existing tests pass
5. Follow the established architectural patterns

## License

This project is open source and available under the MIT License.