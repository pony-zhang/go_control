# Development Guide

## Environment Setup

### Requirements
- Go 1.21+
- Git

### Project Structure
```
control/
├── cmd/                    # Application entry points
│   ├── control/           # Main control system
│   └── simulator/         # Test simulator
├── internal/              # Internal packages
│   ├── core/              # Core system components
│   ├── device/            # Device implementations
│   ├── hal/               # Hardware Abstraction Layer
│   ├── application/       # Application horizontal layers
│   ├── management/        # Layer managers
│   ├── ipc/               # Inter-process communication
│   ├── config/            # Configuration management
│   └── logging/           # Logging system
├── pkg/                   # Public packages
│   └── types/             # Type definitions
└── config.yaml           # Configuration file
```

### Build & Run
```bash
# Build (recommended)
go run build.go

# Manual build
mkdir -p bin
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator

# Run
./bin/control
./bin/simulator

# Test
go test ./...
go test -cover ./...
```

## Core Concepts

### Event-Driven Command Processing

The system uses an event-driven architecture with CommandRouter and CommandHandler interfaces:

```go
// CommandHandler - Modular command processing interface
type CommandHandler interface {
    GetHandledCommands() []types.BusinessCommand
    HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse
    GetName() string
}

// CommandRouter - Intelligent command routing
type CommandRouter struct {
    handlers map[types.BusinessCommand]CommandHandler
}

func (cr *CommandRouter) RouteCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    handler, exists := cr.handlers[msg.Command]
    if !exists {
        return createErrorResponse(msg, "no handler registered")
    }
    return handler.HandleCommand(ctx, msg)
}
```

### Business Commands

High-level abstract commands that the system supports:

```go
const (
    CmdQueryStatus     BusinessCommand = "query"
    CmdEmergencyStop   BusinessCommand = "stop"
    CmdHome            BusinessCommand = "home"
    CmdInitialize      BusinessCommand = "initialize"
    CmdSelfCheck       BusinessCommand = "self_check"
    CmdSafetyCheck     BusinessCommand = "safety_check"
    CmdMove            BusinessCommand = "move"
    CmdGetConfig       BusinessCommand = "get_config"
    CmdSetConfig       BusinessCommand = "set_config"
)
```

### Hardware Abstraction Layer (HAL)

HAL provides unified interface for diverse hardware:

```go
// Protocol interface for hardware communication
type Protocol interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    Connect(deviceID types.DeviceID, config types.DeviceConfig) error
    Write(deviceID types.DeviceID, cmd types.MotionCommand) error
    Read(deviceID types.DeviceID, register string) (interface{}, error)
    Disconnect(deviceID types.DeviceID) error
    Status(deviceID types.DeviceID) types.DeviceStatus
}
```

## Adding New Features

### 1. Adding New Commands

```go
// 1. Define command type
const (
    CmdNewCommand BusinessCommand = "new_command"
)

// 2. Implement CommandHandler
type CustomHandler struct {
    logger *logging.Logger
}

func (h *CustomHandler) GetHandledCommands() []types.BusinessCommand {
    return []types.BusinessCommand{types.CmdNewCommand}
}

func (h *CustomHandler) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    // Handle command logic
    return &types.BusinessResponse{
        RequestID: msg.RequestID,
        Status:    "success",
        Data:      result,
        Timestamp: time.Now(),
    }
}

// 3. Register handler
commandRouter.RegisterHandler(customHandler)
```

### 2. Adding New Protocols

```go
// 1. Implement Protocol interface
type NewProtocol struct {
    devices map[types.DeviceID]*Device
    logger  *logging.Logger
}

func (p *NewProtocol) Write(deviceID types.DeviceID, cmd types.MotionCommand) error {
    // Protocol-specific implementation
    return nil
}

// 2. Register protocol
protocolManager.Register("new_protocol", &NewProtocol{})
```

### 3. Configuration

```yaml
# System configuration
event_loop_interval: 10ms
queue_size: 1000

# IPC configuration
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
  timeout: 5s

# Device configuration
devices:
  device-1:
    type: controller
    protocol: modbus
    endpoint: tcp://192.168.1.100:502

# Axis configuration
axes:
  axis-x:
    device_id: device-1
    max_velocity: 100.0
    max_acceleration: 50.0
    min_position: -1000.0
    max_position: 1000.0
    units: mm
```

## Testing

### Unit Tests
```go
func TestCommandHandler(t *testing.T) {
    handler := NewCustomHandler(testLogger)

    msg := &types.BusinessMessage{
        Command:   types.CmdNewCommand,
        RequestID: "test-123",
        Params:    map[string]interface{}{"param": "value"},
    }

    response := handler.HandleCommand(context.Background(), msg)

    assert.Equal(t, "success", response.Status)
    assert.Equal(t, "test-123", response.RequestID)
}
```

### Integration Tests
```go
func TestSystemIntegration(t *testing.T) {
    // Test complete command flow
    system := NewTestSystem(t)
    defer system.Stop()

    response := system.SendCommand(types.CmdEmergencyStop, nil)
    assert.Equal(t, "success", response.Status)
}
```

## Debugging

### Structured Logging
```go
logger := logging.GetLogger("component_name")
logger.Info("Processing command",
    "command", cmd,
    "device_id", deviceID,
    "request_id", requestID)
```

### Status Monitoring
```bash
# Query system status
echo '{"command": "query"}' | nc localhost 8080

# Monitor logs
./bin/control -log-level debug
```

## Best Practices

### Error Handling
```go
func (h *Handler) ProcessCommand(ctx context.Context, cmd Command) error {
    if err := validateCommand(cmd); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    result, err := h.executeCommand(ctx, cmd)
    if err != nil {
        h.logger.Error("Command execution failed",
            "command", cmd,
            "error", err)
        return fmt.Errorf("execution failed: %w", err)
    }

    return nil
}
```

### Resource Management
```go
func (m *Manager) Start(ctx context.Context) error {
    m.ctx = ctx

    // Start resources
    if err := m.startResources(); err != nil {
        return fmt.Errorf("failed to start resources: %w", err)
    }

    // Handle shutdown
    go func() {
        <-ctx.Done()
        m.cleanup()
    }()

    return nil
}
```

## Contributing

1. **Code Style**: Follow Go standard formatting
2. **Testing**: Add comprehensive tests for new features
3. **Documentation**: Update relevant documentation
4. **PR Process**:
   - Fork the project
   - Create feature branch
   - Add tests and documentation
   - Submit PR for review

## Troubleshooting

### Common Issues
- **Build failures**: Check Go version and dependencies
- **Connection errors**: Verify network configuration and firewall
- **Protocol issues**: Check device configuration and endpoint
- **Performance**: Monitor system metrics and adjust configuration

### Debug Commands
```bash
# Check system status
go run ./cmd/control -status

# Run with debug logging
go run ./cmd/control -log-level debug

# Test configuration
go run ./cmd/control -config-check
```

For detailed architecture information, see [architecture.md](./architecture.md).

## 添加新功能

### 1. 添加新设备类型

#### 1.1 实现设备接口

```go
type NewDevice struct {
    id     types.DeviceID
    config DeviceConfig
    conn   net.Conn
    logger *logging.Logger
}

func (d *NewDevice) Write(cmd types.MotionCommand) error {
    // 实现设备特定的命令写入逻辑
    return nil
}

func (d *NewDevice) Read(reg string) (interface{}, error) {
    // 实现设备特定的读取逻辑
    return nil, nil
}

// 实现其他接口方法...
```

#### 1.2 实现 HAL 协议接口（推荐）

```go
type NewProtocol struct {
    devices map[types.DeviceID]*NewDevice
    logger  *logging.Logger
}

func (p *NewProtocol) Name() string {
    return "new_protocol"
}

func (p *NewProtocol) Write(deviceID types.DeviceID, cmd types.MotionCommand) error {
    if device, exists := p.devices[deviceID]; exists {
        return device.Write(cmd)
    }
    return fmt.Errorf("device not found: %s", deviceID)
}

// 实现其他 Protocol 接口方法...
```

#### 1.3 注册协议类型

```go
// 在 protocol_manager.go 中添加协议注册
case "new_protocol":
    protocol = &NewProtocol{
        devices: make(map[types.DeviceID]*NewDevice),
        logger:  logger,
    }
```

#### 1.4 更新配置

```yaml
devices:
  new-device:
    type: controller
    protocol: new_protocol
    endpoint: tcp://192.168.1.100:9999
    timeout: 10s
    parameters:
      custom_param: value
```

### 2. 添加新业务命令

#### 2.1 定义业务命令类型

```go
// 在 types/types.go 中添加
type BusinessCommand string

const (
    CmdNewCommand BusinessCommand = "new_command"
    // 其他业务命令...
)
```

#### 2.2 实现 CommandHandler 接口

```go
// 在相应的处理器中实现命令处理逻辑
func (handler *CustomHandler) GetHandledCommands() []types.BusinessCommand {
    return []types.BusinessCommand{
        types.CmdNewCommand,
        // 其他该处理器能处理的命令...
    }
}

func (handler *CustomHandler) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    switch msg.Command {
    case types.CmdNewCommand:
        return handler.handleNewCommand(msg)
    // 其他命令处理...
    default:
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     fmt.Sprintf("handler cannot handle command: %s", msg.Command),
            Timestamp: time.Now(),
        }
    }
}

func (handler *CustomHandler) handleNewCommand(msg *types.BusinessMessage) *types.BusinessResponse {
    // 业务规则验证
    if err := handler.validateCommand(msg.Params); err != nil {
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     err.Error(),
            Timestamp: time.Now(),
        }
    }

    // 安全检查
    if err := handler.safetyManager.CheckSafety(msg.Command, msg.Params); err != nil {
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     err.Error(),
            Timestamp: time.Now(),
        }
    }

    // 处理命令并返回响应
    result := handler.processNewCommand(msg.Params)

    return &types.BusinessResponse{
        RequestID: msg.RequestID,
        Status:    "success",
        Data:      result,
        Timestamp: time.Now(),
    }
}
```

#### 2.3 注册命令处理器

```go
// 在 ApplicationManager 中注册新的命令处理器
func (am *ApplicationManager) setupCommandHandlers() {
    // 注册业务逻辑层处理器
    am.commandRouter.RegisterHandler(am.businessLogic)

    // 注册配置管理处理器
    am.commandRouter.RegisterHandler(am.configHandler)

    // 注册自定义命令处理器
    customHandler := NewCustomHandler(am.logger)
    am.commandRouter.RegisterHandler(customHandler)
}
```

### 3. 添加新命令处理器

#### 3.1 创建自定义命令处理器

```go
// 创建自定义命令处理器，实现 CommandHandler 接口
type CustomCommandHandler struct {
    logger *logging.Logger
    // 其他依赖项...
}

func NewCustomCommandHandler(logger *logging.Logger) *CustomCommandHandler {
    return &CustomCommandHandler{
        logger: logger,
    }
}

// 实现 CommandHandler 接口
func (h *CustomCommandHandler) GetHandledCommands() []types.BusinessCommand {
    return []types.BusinessCommand{
        types.CmdNewCommand,
        types.CmdCustomOperation,
    }
}

func (h *CustomCommandHandler) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    h.logger.Info("Custom handler processing command", "command", msg.Command, "request_id", msg.RequestID)

    switch msg.Command {
    case types.CmdNewCommand:
        return h.handleNewCommand(ctx, msg)
    case types.CmdCustomOperation:
        return h.handleCustomOperation(ctx, msg)
    default:
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     fmt.Sprintf("custom handler cannot handle command: %s", msg.Command),
            Timestamp: time.Now(),
        }
    }
}

func (h *CustomCommandHandler) GetName() string {
    return "custom_handler"
}
```

#### 3.2 实现具体命令处理逻辑

```go
func (h *CustomCommandHandler) handleNewCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    // 验证参数
    if err := h.validateNewCommandParams(msg.Params); err != nil {
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     err.Error(),
            Timestamp: time.Now(),
        }
    }

    // 处理业务逻辑
    result, err := h.processNewCommand(ctx, msg.Params)
    if err != nil {
        return &types.BusinessResponse{
            RequestID: msg.RequestID,
            Status:    "error",
            Error:     err.Error(),
            Timestamp: time.Now(),
        }
    }

    return &types.BusinessResponse{
        RequestID: msg.RequestID,
        Status:    "success",
        Data:      result,
        Timestamp: time.Now(),
    }
}
```

### 4. 添加新水平层组件

#### 4.1 创建水平层结构

```go
// 在 application/ 目录下创建新的水平层
package application

import (
    "context"
    "fmt"

    "control/pkg/types"
    "control/internal/hal"
    "control/internal/logging"
)

type NewHorizontalLayer struct {
    lowerLayer interface{}  // 下一层依赖
    logger    *logging.Logger
    ctx       context.Context
}

func NewNewHorizontalLayer(lowerLayer interface{}) *NewHorizontalLayer {
    return &NewHorizontalLayer{
        lowerLayer: lowerLayer,
        logger:     logging.GetLogger("new_horizontal_layer"),
    }
}

func (nhl *NewHorizontalLayer) Start(ctx context.Context) error {
    nhl.ctx = ctx
    nhl.logger.Info("Starting New Horizontal Layer")
    return nil
}

func (nhl *NewHorizontalLayer) Stop() error {
    nhl.logger.Info("Stopping New Horizontal Layer")
    return nil
}
```

#### 4.2 在应用管理器中集成

```go
// 在 application_manager.go 中添加新水平层
type ApplicationManager struct {
    infrastructure      *InfrastructureManager
    hal                 *hal.HardwareAbstractionLayer
    newHorizontalLayer  *application.NewHorizontalLayer  // 新的水平层
    serviceCoordination *application.ServiceCoordinationLayer
    taskOrchestration  *application.TaskOrchestrationLayer
    businessLogic      *application.BusinessLogicLayer
}
```

### 5. 添加新资源类型

#### 5.1 定义资源类型

```go
// 在 resource_manager.go 中添加
const (
    ResourceTypeNew ResourceType = "new_resource"
)

type NewResource struct {
    ID          string
    Type        string
    Allocated   bool
    AllocatedTo string
    Metadata    map[string]interface{}
    Status      ResourceStatus
}

func (rm *ResourceManager) AllocateNewResource(resourceID string, metadata map[string]interface{}) error {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    if _, exists := rm.resources[ResourceTypeNew][resourceID]; exists {
        return fmt.Errorf("new resource %s already exists", resourceID)
    }

    resource := &NewResource{
        ID:        resourceID,
        Type:      ResourceTypeNew,
        Allocated: false,
        Metadata:  metadata,
        Status:    ResourceStatusAvailable,
    }

    rm.resources[ResourceTypeNew][resourceID] = resource
    return nil
}
```

### 5. 添加新触发器

#### 5.1 实现触发器接口

```go
type NewTrigger struct {
    name      string
    triggerFn func() (*types.Task, error)
    interval  time.Duration
    logger    *logging.Logger
}

func (t *NewTrigger) Trigger() (*types.Task, error) {
    return t.triggerFn()
}

func (t *NewTrigger) Name() string {
    return t.name
}

func (t *NewTrigger) Start(ctx context.Context) error {
    t.logger.Info("Starting trigger", "trigger_name", t.name)
    // 实现定时触发逻辑
    go t.runTrigger(ctx)
    return nil
}

func (t *NewTrigger) Stop() error {
    t.logger.Info("Stopping trigger", "trigger_name", t.name)
    // 实现停止逻辑
    return nil
}

func (t *NewTrigger) runTrigger(ctx context.Context) {
    ticker := time.NewTicker(t.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if task, err := t.triggerFn(); err == nil {
                // 发送任务到任务触发器
                t.logger.Info("Trigger generated task", "task_id", task.ID)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

#### 5.2 注册触发器

```go
// 在任务编排层中注册
func (tol *TaskOrchestrationLayer) addCustomTrigger() {
    trigger := &NewTrigger{
        name: "new_trigger",
        triggerFn: generateNewTask,
        interval: 5 * time.Second,
        logger: tol.logger,
    }

    if err := tol.taskTrigger.AddTrigger(trigger); err != nil {
        tol.logger.Error("Failed to add trigger", "error", err)
    }
}
```

### 6. 事件驱动命令处理集成

#### 6.1 IPC消息到业务命令的转换

```go
// 在 ApplicationManager 中实现IPC消息处理
func (am *ApplicationManager) handleBusinessCommand(message types.IPCMessage) {
    am.logger.Info("Received business command", "message", message)

    go func() {
        // 解析业务消息
        businessMsg, err := am.parseBusinessMessage(message)
        if err != nil {
            am.logger.Error("Failed to parse business message", "error", err)
            am.sendBusinessErrorResponse(message.Source, "", err.Error())
            return
        }

        // 通过命令路由器自动路由到相应的处理器
        response := am.routeBusinessCommand(businessMsg)
        am.sendBusinessResponse(message.Source, response)
    }()
}
```

#### 6.2 业务消息解析

```go
func (am *ApplicationManager) parseBusinessMessage(message types.IPCMessage) (*types.BusinessMessage, error) {
    if command, ok := message.Data["command"].(string); ok {
        businessMsg := &types.BusinessMessage{
            Command:   types.BusinessCommand(command),
            Source:    message.Source,
            Target:    message.Target,
            RequestID: message.ID,
            Timestamp: time.Now(),
        }

        if params, ok := message.Data["params"].(map[string]interface{}); ok {
            businessMsg.Params = params
        } else {
            businessMsg.Params = make(map[string]interface{})
        }

        return businessMsg, nil
    }

    return nil, fmt.Errorf("invalid business command format")
}
```

#### 6.3 命令路由和响应

```go
func (am *ApplicationManager) routeBusinessCommand(msg *types.BusinessMessage) *types.BusinessResponse {
    am.logger.Info("Routing business command through command router", "command", msg.Command)
    return am.commandRouter.RouteCommand(am.ctx, msg)
}

func (am *ApplicationManager) sendBusinessResponse(target string, response *types.BusinessResponse) {
    if target == "" {
        return
    }

    ipcServer := am.infrastructure.GetIPCServer()
    if ipcServer == nil {
        am.logger.Error("IPC server not available for sending response")
        return
    }

    // 将BusinessResponse转换为IPCMessage
    ipcMessage := types.IPCMessage{
        Type:   "business_response",
        Source: "control_system",
        Target: target,
        Data: map[string]interface{}{
            "request_id": response.RequestID,
            "status":     response.Status,
            "data":       response.Data,
            "error":      response.Error,
        },
        Timestamp: time.Now(),
        ID:        fmt.Sprintf("resp-%d", time.Now().UnixNano()),
    }

    if err := ipcServer.SendToClient(target, ipcMessage); err != nil {
        am.logger.Error("Failed to send business response", "target", target, "error", err)
    }
}
```

## 配置管理

### 1. 配置文件结构

```yaml
# 系统基础配置
event_loop_interval: 10ms
queue_size: 1000

# IPC配置
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
  timeout: 5s
  buffer_size: 1024

# 设备配置（支持多种协议）
devices:
  device-1:
    type: controller
    protocol: mock
    endpoint: mock://device-1
    parameters:
      custom_param: value
    timeout: 10s
    retry_count: 3
  device-2:
    type: controller
    protocol: modbus
    endpoint: tcp://192.168.1.100:502
    timeout: 10s
    parameters:
      unit_id: 1
      register_base: 0

# 轴配置
axes:
  axis-x:
    device_id: device-1
    max_velocity: 100.0
    max_acceleration: 50.0
    min_position: -1000.0
    max_position: 1000.0
    home_position: 0.0
    units: mm
    invert_direction: false

# 安全配置
safety:
  enable_limits: true
  enable_emergency: true
  max_temperature: 80
  min_voltage: 12
  watchdog_timeout: 10s

# 日志配置
logging:
  level: info
  format: json
  output: stdout
  add_source: true
  time_format: "2006-01-02T15:04:05.000Z"

# 任务模板配置
task_templates:
  home_all:
    name: Home All Axes
    description: Home all axes to their home positions
    type: sequence
    parameters:
      mode: sequential
      stop_on_error: true
    nodes:
      - id: home-x
        type: home
        parameters:
          axes: [axis-x]
    timeout: 30s
    priority: 1

# 事件驱动命令系统配置
command_handlers:
  business_logic:
    handler: "business_logic"
    commands:
      - query
      - self_check
      - emergency_stop
      - home
      - initialize
      - safety_check

  config_handler:
    handler: "config"
    commands:
      - get_config
      - set_config
      - list_templates

# 业务命令处理配置
business_commands:
  emergency_stop:
    description: 急停系统
    priority: 0
    timeout: 5s
    handler: "business_logic"

  home:
    description: 回零所有轴
    priority: 1
    timeout: 30s
    handler: "business_logic"

  query:
    description: 查询系统状态
    priority: 2
    timeout: 10s
    handler: "business_logic"

  get_config:
    description: 获取系统配置
    priority: 3
    timeout: 15s
    handler: "config_handler"
```

### 2. HAL 配置管理

#### 2.1 协议配置

```yaml
# HAL 协议配置
hal:
  protocols:
    mock:
      enabled: true
      parameters:
        simulation_delay: 10ms
    modbus:
      enabled: true
      parameters:
        default_timeout: 5s
        max_retries: 3
    serial:
      enabled: false
      parameters:
        baud_rate: 9600
        data_bits: 8
        stop_bits: 1
        parity: none
```

#### 2.2 资源配置

```yaml
# 资源管理配置
resources:
  devices:
    device-1:
      type: controller
      allocation_policy: exclusive
      timeout: 30s
  axes:
    axis-x:
      device_id: device-1
      allocation_policy: shared
      max_concurrent_tasks: 3
  safety:
    emergency_stop:
      type: system
      allocation_policy: exclusive
      timeout: 1s
```

### 3. 配置热重载

系统支持配置文件的热重载，修改配置文件后会自动应用新配置：

```go
// 在应用管理器中实现配置热重载
func (am *ApplicationManager) setupConfigHotReload() {
    am.configManager.WatchChanges(func(config types.SystemConfig) {
        am.logger.Info("Configuration changed, updating system...")

        // 更新各层配置
        if am.businessLogic != nil {
            am.businessLogic.UpdateConfig(config)
        }

        if am.taskOrchestration != nil {
            am.taskOrchestration.UpdateConfig(config)
        }

        if am.serviceCoordination != nil {
            am.serviceCoordination.UpdateConfig(config)
        }

        if am.hal != nil {
            am.hal.UpdateConfig(config)
        }

        am.logger.Info("Configuration update completed")
    })
}
```

### 4. 分层配置验证

```go
// 配置验证函数
func (am *ApplicationManager) validateConfig(config types.SystemConfig) error {
    // 验证基础配置
    if config.EventLoopInterval <= 0 {
        return fmt.Errorf("event_loop_interval must be positive")
    }

    // 验证HAL配置
    if err := am.hal.ValidateConfig(config); err != nil {
        return fmt.Errorf("HAL config validation failed: %w", err)
    }

    // 验证水平层配置
    if err := am.businessLogic.ValidateConfig(config); err != nil {
        return fmt.Errorf("business logic config validation failed: %w", err)
    }

    return nil
}
```

## 调试和诊断

### 1. 分层日志系统

使用结构化日志进行分层调试：

```go
// 获取各层日志器
businessLogger := logging.GetLogger("business_logic")
orchestrationLogger := logging.GetLogger("task_orchestration")
serviceLogger := logging.GetLogger("service_coordination")
halLogger := logging.GetLogger("hal")

// 记录不同级别的日志
businessLogger.Info("Processing abstract command", "command", cmd, "params", params)
orchestrationLogger.Debug("Task decomposition started", "task_id", task.ID)
serviceLogger.Warn("Device resource allocation timeout", "device_id", deviceID)
halLogger.Error("Protocol communication failed", "protocol", protocol, "error", err)

// 带上下文的日志
logger.WithContext(ctx).Info("Layer processing context", "layer", "business_logic")
```

### 2. 分层状态监控

```go
// 查询各层状态
func (am *ApplicationManager) GetSystemStatus() map[string]interface{} {
    status := make(map[string]interface{})

    // 业务逻辑层状态
    if am.businessLogic != nil {
        status["business_logic"] = am.businessLogic.GetStatus()
    }

    // 任务编排层状态
    if am.taskOrchestration != nil {
        status["task_orchestration"] = am.taskOrchestration.GetStatus()
    }

    // 服务协调层状态
    if am.serviceCoordination != nil {
        status["service_coordination"] = am.serviceCoordination.GetStatus()
    }

    // HAL状态
    if am.hal != nil {
        status["hal"] = am.hal.GetStatus()
    }

    return status
}
```

### 3. 分层状态查询

通过IPC接口查询各层状态：

```go
// 发送分层状态请求
statusRequest := types.IPCMessage{
    Type:      "layered_status_request",
    Source:    "client_id",
    Target:    "control_system",
    Data: map[string]interface{}{
        "layers": []string{
            "business_logic",
            "task_orchestration",
            "service_coordination",
            "hal",
        },
    },
    Timestamp: time.Now(),
    ID:        "msg-123",
}

// 接收分层状态响应
statusResponse := types.IPCMessage{
    Type: "layered_status_response",
    Data: map[string]interface{}{
        "business_logic": map[string]interface{}{
            "running": true,
            "active_tasks": 5,
            "processed_commands": 150,
            "safety_status": "normal",
        },
        "task_orchestration": map[string]interface{}{
            "queue_size": 10,
            "active_tasks": 3,
            "scheduler_status": "running",
            "decomposer_status": "active",
        },
        "service_coordination": map[string]interface{}{
            "allocated_resources": 8,
            "pending_commands": 2,
            "device_connections": 5,
        },
        "hal": map[string]interface{}{
            "connected_devices": 5,
            "active_protocols": 3,
            "resource_utilization": 0.75,
        },
        "system": map[string]interface{}{
            "uptime": "1h30m",
            "config": "/path/to/config.yaml",
            "version": "2.0.0",
        },
    },
}
```

### 4. 分层性能分析

使用Go的性能分析工具进行分层性能分析：

```bash
# CPU分析 - 各层处理时间
go tool pprof http://localhost:8080/debug/pprof/profile
# 分析业务逻辑层、任务编排层、服务协调层、HAL的CPU占用

# 内存分析 - 各层内存使用
go tool pprof http://localhost:8080/debug/pprof/heap
# 分析各层的内存分配和GC情况

# 协程分析 - 并发处理情况
go tool pprof http://localhost:8080/debug/pprof/goroutine
# 分析各层的协程数量和阻塞情况

# 阻塞分析 - 通道和锁竞争
go tool pprof http://localhost:8080/debug/pprof/block
# 分析层间通信的阻塞情况
```

### 5. 分层调试工具

```go
// 分层调试助手
type LayerDebugger struct {
    layers map[string]interface{}
    logger *logging.Logger
}

func (ld *LayerDebugger) DebugLayerFlow(layerName string, operation string, data interface{}) {
    ld.logger.Debug("Layer flow debug",
        "layer", layerName,
        "operation", operation,
        "data", data,
        "timestamp", time.Now(),
    )
}

func (ld *LayerDebugger) TraceExecution(taskID string, layers []string) {
    ld.logger.Info("Execution trace started",
        "task_id", taskID,
        "layers", layers,
    )

    // 跟踪任务在各层的执行情况
    for _, layer := range layers {
        ld.DebugLayerFlow(layer, "process_task", map[string]interface{}{
            "task_id": taskID,
            "status": "started",
        })
    }
}
```

## 错误处理最佳实践

### 1. 分层错误处理模式

```go
// 1. 错误包装和上下文
if err := doSomething(); err != nil {
    return fmt.Errorf("business logic layer: failed to process command: %w", err)
}

// 2. 分层错误日志记录
if err := bll.ExecuteAbstractCommand(cmd, params); err != nil {
    bll.logger.Error("Abstract command execution failed",
        "command", cmd,
        "parameters", params,
        "error", err,
        "layer", "business_logic",
    )
    return fmt.Errorf("business logic: %w", err)
}

// 3. 分层错误恢复
defer func() {
    if r := recover(); r != nil {
        logger.Error("Recovered from panic in layer",
            "layer", layerName,
            "error", r,
            "stack_trace", debug.Stack(),
        )
        // 分层特定的恢复逻辑
        if layerName == "hal" {
            // HAL层恢复：重置设备连接
            hal.recoverFromPanic()
        }
    }
}()
```

### 2. 水平层错误传播

```go
// 业务逻辑层 → 任务编排层
func (bll *BusinessLogicLayer) ExecuteAbstractCommand(cmd AbstractCommand, params map[string]interface{}) (*types.Task, error) {
    // 业务验证
    if err := bll.validateCommand(cmd, params); err != nil {
        return nil, fmt.Errorf("business validation failed: %w", err)
    }

    // 安全检查
    if err := bll.safetyManager.CheckSafety(cmd, params); err != nil {
        return nil, fmt.Errorf("safety check failed: %w", err)
    }

    // 创建任务并传递到下一层
    task, err := bll.taskOrchestration.CreateTask(cmd, params)
    if err != nil {
        return nil, fmt.Errorf("task creation failed: %w", err)
    }

    return task, nil
}

// 任务编排层 → 服务协调层
func (tol *TaskOrchestrationLayer) ExecuteCommand(ctx context.Context, cmd types.MotionCommand) error {
    // 任务调度验证
    if err := tol.validateCommand(cmd); err != nil {
        return fmt.Errorf("task orchestration: command validation failed: %w", err)
    }

    // 传递到服务协调层
    if err := tol.serviceCoordination.ExecuteCommand(ctx, cmd); err != nil {
        return fmt.Errorf("task orchestration: command execution failed: %w", err)
    }

    return nil
}
```

### 3. 分层资源清理

```go
// 分层资源清理模式
func (am *ApplicationManager) Stop() error {
    am.logger.Info("Stopping Application Manager")

    var errs []error

    // 按照依赖关系的逆序停止各层
    // 1. 停止业务逻辑层
    if am.businessLogic != nil {
        if err := am.businessLogic.Stop(); err != nil {
            errs = append(errs, fmt.Errorf("business logic stop error: %w", err))
        }
    }

    // 2. 停止任务编排层
    if am.taskOrchestration != nil {
        if err := am.taskOrchestration.Stop(); err != nil {
            errs = append(errs, fmt.Errorf("task orchestration stop error: %w", err))
        }
    }

    // 3. 停止服务协调层
    if am.serviceCoordination != nil {
        if err := am.serviceCoordination.Stop(); err != nil {
            errs = append(errs, fmt.Errorf("service coordination stop error: %w", err))
        }
    }

    // 4. 停止HAL
    if am.hal != nil {
        if err := am.hal.Stop(); err != nil {
            errs = append(errs, fmt.Errorf("HAL stop error: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("application manager stop errors: %v", errs)
    }

    return nil
}
```

### 4. 分层超时处理

```go
// 分层超时处理
func (bll *BusinessLogicLayer) ExecuteWithTimeout(cmd AbstractCommand, params map[string]interface{}, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // 使用带超时的上下文执行
    resultChan := make(chan error, 1)

    go func() {
        _, err := bll.ExecuteAbstractCommand(cmd, params)
        resultChan <- err
    }()

    select {
    case err := <-resultChan:
        return err
    case <-ctx.Done():
        bll.logger.Warn("Command execution timeout",
            "command", cmd,
            "timeout", timeout,
        )
        return fmt.Errorf("business logic: command execution timeout: %w", ctx.Err())
    }
}

// 层间通信超时
func (tol *TaskOrchestrationLayer) executeWithTimeout(ctx context.Context, cmd types.MotionCommand) error {
    timeout := time.After(30 * time.Second)
    resultChan := make(chan error, 1)

    go func() {
        resultChan <- tol.serviceCoordination.ExecuteCommand(ctx, cmd)
    }()

    select {
    case err := <-resultChan:
        return err
    case <-timeout:
        tol.logger.Error("Service coordination timeout",
            "command_id", cmd.ID,
            "device_id", cmd.DeviceID,
        )
        return fmt.Errorf("task orchestration: service coordination timeout")
    case <-ctx.Done():
        return fmt.Errorf("task orchestration: context cancelled: %w", ctx.Err())
    }
}
```

## 测试策略

### 1. 分层单元测试

```go
func TestBusinessLogicLayer_ExecuteAbstractCommand(t *testing.T) {
    // 准备测试数据
    mockHAL := &MockHAL{}
    serviceCoord := NewServiceCoordinationLayer(mockHAL, testConfig)
    taskOrchestration := NewTaskOrchestrationLayer(serviceCoord, testConfig)
    bll := NewBusinessLogicLayer(taskOrchestration, testConfig)

    abstractCmd := types.AbstractCommand("test_command")
    params := map[string]interface{}{
        "device_id": "test-device",
        "position":  100.0,
    }

    // 执行测试
    task, err := bll.ExecuteAbstractCommand(abstractCmd, params)

    // 验证结果
    if err != nil {
        t.Errorf("Failed to execute abstract command: %v", err)
    }

    if task == nil {
        t.Error("Task should not be nil")
    }

    if task.Type != types.CommandTypeTest {
        t.Errorf("Expected task type CommandTypeTest, got %v", task.Type)
    }
}

func TestHAL_ExecuteCommand(t *testing.T) {
    // 测试HAL层
    protocolManager := NewProtocolManager(testConfig)
    resourceManager := NewResourceManager(testConfig)
    hal := NewHAL(protocolManager, resourceManager, testConfig)

    cmd := types.MotionCommand{
        ID:       "test-cmd",
        DeviceID: "test-device",
        Type:     types.CommandTypeMoveTo,
        Position: 100.0,
    }

    // 执行测试
    err := hal.ExecuteCommand(context.Background(), cmd)

    // 验证结果
    if err != nil {
        t.Errorf("HAL command execution failed: %v", err)
    }
}
```

### 2. 分层集成测试

```go
func TestApplicationLayer_Integration(t *testing.T) {
    // 创建完整的应用层栈
    mockHAL := NewMockHAL()
    serviceCoord := NewServiceCoordinationLayer(mockHAL, testConfig)
    taskOrchestration := NewTaskOrchestrationLayer(serviceCoord, testConfig)
    businessLogic := NewBusinessLogicLayer(taskOrchestration, testConfig)

    ctx := context.Background()

    // 启动各层
    if err := serviceCoord.Start(ctx); err != nil {
        t.Fatalf("Failed to start service coordination: %v", err)
    }
    defer serviceCoord.Stop()

    if err := taskOrchestration.Start(ctx); err != nil {
        t.Fatalf("Failed to start task orchestration: %v", err)
    }
    defer taskOrchestration.Stop()

    if err := businessLogic.Start(ctx); err != nil {
        t.Fatalf("Failed to start business logic: %v", err)
    }
    defer businessLogic.Stop()

    // 测试完整的命令执行流程
    abstractCmd := types.AbstractCommand("move_to")
    params := map[string]interface{}{
        "device_id": "test-device",
        "position":  100.0,
        "velocity":  50.0,
    }

    // 通过业务逻辑层执行
    task, err := businessLogic.ExecuteAbstractCommand(abstractCmd, params)
    if err != nil {
        t.Fatalf("Failed to execute abstract command: %v", err)
    }

    // 验证任务创建和调度
    if task == nil {
        t.Fatal("Task should not be nil")
    }

    // 等待任务完成
    time.Sleep(100 * time.Millisecond)

    // 验证HAL层的执行结果
    status := mockHAL.GetCommandStatus(cmd.ID)
    if status != types.StatusCompleted {
        t.Errorf("Expected command status completed, got %v", status)
    }
}
```

### 3. 分层模拟测试

```go
func TestLayeredSystem_MockDependencies(t *testing.T) {
    // 创建各层的模拟对象
    mockHAL := &MockHAL{
        commands: make(map[string]types.MotionCommand),
    }

    mockServiceCoord := &MockServiceCoordinationLayer{
        hal: mockHAL,
    }

    mockTaskOrchestration := &MockTaskOrchestrationLayer{
        serviceCoord: mockServiceCoord,
    }

    // 测试业务逻辑层
    bll := NewBusinessLogicLayer(mockTaskOrchestration, testConfig)

    abstractCmd := types.AbstractCommand("test_command")
    params := map[string]interface{}{"test": "value"}

    // 执行测试
    task, err := bll.ExecuteAbstractCommand(abstractCmd, params)

    // 验证结果
    if err != nil {
        t.Errorf("Business logic execution failed: %v", err)
    }

    // 验证各层的调用
    if !mockTaskOrchestration.CreateTaskCalled {
        t.Error("Task orchestration CreateTask should have been called")
    }

    if !mockServiceCoord.ExecuteCommandCalled {
        t.Error("Service coordination ExecuteCommand should have been called")
    }

    if !mockHAL.ExecuteCommandCalled {
        t.Error("HAL ExecuteCommand should have been called")
    }
}
```

## 部署和运维

### 1. 分层容器化部署

```dockerfile
# 多阶段构建，支持分层部署
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 分别构建控制组件和模拟器
RUN go build -o bin/control ./cmd/control
RUN go build -o bin/simulator ./cmd/simulator

# 运行时镜像
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /opt/motion-control

# 复制二进制文件和配置
COPY --from=builder /app/bin/ ./bin/
COPY --from=builder /app/config.yaml ./
COPY --from=builder /app/docs/ ./docs/

# 创建分层目录结构
RUN mkdir -p logs config backups

# 环境变量配置
ENV MOTION_CONTROL_CONFIG=/opt/motion-control/config.yaml
ENV MOTION_CONTROL_LOG_LEVEL=info
ENV MOTION_CONTROL_IPC_PORT=8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

EXPOSE 8080

# 启动命令
CMD ["./bin/control"]
```

### 2. 分层系统服务配置

```ini
# /etc/systemd/system/motion-control.service
[Unit]
Description=Motion Control System with HAL and Horizontal Layers
After=network.target
Wants=network.target

[Service]
Type=simple
User=control
Group=control
WorkingDirectory=/opt/motion-control

# 环境变量
Environment="MOTION_CONTROL_CONFIG=/opt/motion-control/config.yaml"
Environment="MOTION_CONTROL_LOG_LEVEL=info"
Environment="MOTION_CONTROL_IPC_PORT=8080"

# 启动命令
ExecStart=/opt/motion-control/bin/control
ExecStop=/opt/motion-control/bin/control --shutdown

# 重启策略
Restart=always
RestartSec=5

# 资源限制
LimitNOFILE=65536
MemoryMax=1G

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/motion-control/logs

[Install]
WantedBy=multi-user.target
```

### 3. 分层监控配置

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'motion-control'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
    scrape_timeout: 3s

  # 分层监控
  - job_name: 'motion-control-layers'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/layer-metrics'
    scrape_interval: 10s
    params:
      layers: ['business_logic', 'task_orchestration', 'service_coordination', 'hal']

# 告警规则
groups:
  - name: motion_control_alerts
    rules:
      - alert: BusinessLogicLayerHighErrorRate
        expr: rate(business_logic_errors_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "业务逻辑层错误率过高"
          description: "业务逻辑层错误率在5分钟内超过10%"

      - alert: HALDeviceConnectionFailure
        expr: hal_device_connections_total{status="failed"} > 5
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "HAL设备连接失败"
          description: "HAL层设备连接失败次数超过5次"

      - alert: TaskOrchestrationQueueFull
        expr: task_orchestration_queue_size / task_orchestration_queue_capacity > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "任务编排层队列接近满载"
          description: "任务编排层队列使用率超过90%"
```

### 4. 分层日志聚合配置

```yaml
# fluentd配置
<source>
  @type tail
  path /opt/motion-control/logs/*.log
  pos_file /var/log/fluentd/motion-control.pos
  tag motion-control.*
  format json
  time_format %Y-%m-%dT%H:%M:%S.%L%z
</source>

# 分层日志过滤
<filter motion-control.business_logic>
  @type grep
  <regexp>
    key layer
    pattern business_logic
  </regexp>
</filter>

<filter motion-control.task_orchestration>
  @type grep
  <regexp>
    key layer
    pattern task_orchestration
  </regexp>
</filter>

# 输出到Elasticsearch
<match motion-control.**>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name motion-control
  type_name _doc
</match>
```

## 贡献指南

### 1. 分层开发规范

#### 1.1 代码风格

- 遵循Go标准代码风格
- 使用gofmt格式化代码
- 添加适当的注释和文档
- 编写分层单元测试和集成测试
- 确保层间接口清晰和一致

#### 1.2 分层开发原则

```go
// 层间依赖原则：上层依赖下层，下层不依赖上层
// 正确的依赖方向
business_logic -> task_orchestration -> service_coordination -> hal -> physical

// 错误的依赖方向（避免）
hal -> business_logic  // 违反分层架构
```

#### 1.3 接口设计规范

```go
// 每个水平层都应该定义清晰的接口
type BusinessLogicLayerInterface interface {
    ExecuteAbstractCommand(cmd AbstractCommand, params map[string]interface{}) (*types.Task, error)
    EmergencyStop() error
    GetSystemStatus() (map[string]interface{}, error)
}

type TaskOrchestrationLayerInterface interface {
    ScheduleTask(task *types.Task) error
    ExecuteCommand(ctx context.Context, cmd types.MotionCommand) error
    GetTaskStatus(taskID string) (types.TaskStatus, error)
}
```

### 2. 提交规范

```bash
# 提交前检查
make test
make lint
make build

# 分层提交信息格式
feat(business-logic): 添加新的抽象命令处理
fix(hal): 修复Modbus协议连接问题
docs(architecture): 更新架构文档
style(task-orchestration): 重构任务调度器代码
refactor(service-coordination): 优化资源分配算法
test(hal): 添加协议管理器单元测试
chore(config): 更新配置文件格式
```

### 3. PR流程

1. Fork项目
2. 创建功能分支（按层命名）
3. 编写代码和分层测试
4. 确保层间接口兼容性
5. 提交PR
6. 代码审查（重点检查分层架构合规性）
7. 合并到主分支

## 常见问题

### Q: 如何添加新的设备协议？
A: 实现HAL的Protocol接口，在ProtocolManager中注册，更新配置文件格式。推荐使用HAL抽象层而不是直接实现Device接口。

### Q: 如何添加新的水平层？
A: 在application包中创建新的水平层，实现标准的Start/Stop接口，在ApplicationManager中集成，确保层间依赖关系正确。

### Q: 任务执行失败如何处理？
A: 系统会自动记录错误日志，可以通过分层状态查询接口获取各层的详细错误信息。错误会按照层级进行包装和传播。

### Q: 如何优化系统性能？
A: 调整事件循环间隔、队列大小、并发参数，使用分层性能分析工具定位瓶颈。重点优化HAL层的设备通信和任务编排层的调度算法。

### Q: 如何实现设备热插拔？
A: 通过HAL的ResourceManager和DeviceManager，配合配置热重载功能。确保资源分配和释放的正确性。

### Q: 如何确保分层架构的正确性？
A: 定期进行依赖分析，确保没有循环依赖，层间通信通过标准接口，避免跨层调用。使用静态分析工具检查架构合规性。

### Q: 如何进行分层调试？
A: 使用分层日志系统，配置不同层级的日志级别，使用分层调试工具跟踪执行流程，利用分层状态监控接口。

## 事件循环使用示例

### 1. 基本事件循环使用

```go
// 创建事件循环（100ms定时器间隔）
eventLoop := NewEventLoop(100 * time.Millisecond)

// 启动事件循环
ctx := context.Background()
if err := eventLoop.Start(ctx); err != nil {
    log.Fatalf("Failed to start event loop: %v", err)
}

// 发送各种类型的事件
eventLoop.EmitEvent(NewSystemEvent(EventTypeSystemStart, "system", nil))
eventLoop.EmitEvent(NewTaskEvent(EventTypeTaskRequest, "scheduler", "task-001", "move", task, nil))
eventLoop.EmitEvent(NewDeviceEvent(EventTypeDeviceConnect, "hal", "motor-1", status, nil, data))

// 停止事件循环
if err := eventLoop.Stop(); err != nil {
    log.Fatalf("Failed to stop event loop: %v", err)
}
```

### 2. 自定义事件处理器

```go
// 自定义事件处理器
type CustomEventHandler struct {
    name string
}

func (h *CustomEventHandler) Name() string {
    return h.name
}

func (h *CustomEventHandler) HandleEvent(event Event) error {
    switch event.Type() {
    case EventTypeSystemStart:
        fmt.Printf("System started: %s\n", event.Source())
    case EventTypeTaskComplete:
        if taskEvent, ok := event.(*TaskEvent); ok {
            fmt.Printf("Task completed: %s\n", taskEvent.TaskID)
        }
    case EventTypeTimerTick:
        // 处理定时任务
        h.processTimedTasks()
    }
    return nil
}

func (h *CustomEventHandler) GetSubscribedEvents() []EventType {
    return []EventType{
        EventTypeSystemStart,
        EventTypeTaskComplete,
        EventTypeTimerTick,
    }
}

// 注册事件处理器
handler := &CustomEventHandler{name: "custom_handler"}
eventLoop.RegisterHandler(EventTypeSystemStart, handler)
eventLoop.RegisterHandler(EventTypeTaskComplete, handler)
eventLoop.RegisterHandler(EventTypeTimerTick, handler)
```

### 3. 模块集成事件循环

```go
// 自定义模块
type CustomModule struct {
    name string
}

func (m *CustomModule) Name() string {
    return m.name
}

func (m *CustomModule) Start(ctx context.Context) error {
    fmt.Printf("Module %s started\n", m.name)
    return nil
}

func (m *CustomModule) Stop() error {
    fmt.Printf("Module %s stopped\n", m.name)
    return nil
}

func (m *CustomModule) HandleEvent(event Event) error {
    switch event.Type() {
    case EventTypeSystemStart:
        // 系统启动时的初始化工作
        m.initialize()
    case EventTypeSystemStop:
        // 系统停止时的清理工作
        m.cleanup()
    case EventTypeTimerTick:
        // 定时任务
        m.periodicTask()
    }
    return nil
}

func (m *CustomModule) GetSubscribedEvents() []EventType {
    return []EventType{
        EventTypeSystemStart,
        EventTypeSystemStop,
        EventTypeTimerTick,
    }
}

func (m *CustomModule) Status() interface{} {
    return map[string]interface{}{
        "name":    m.name,
        "running": true,
    }
}

// 注册模块（自动注册事件处理器）
module := &CustomModule{name: "custom_module"}
if err := eventLoop.RegisterModule("custom_module", module); err != nil {
    log.Fatalf("Failed to register module: %v", err)
}
```

### 4. 事件类型和创建

```go
// 创建系统事件
systemEvent := NewSystemEvent(EventTypeSystemStart, "main", nil)

// 创建任务事件
taskEvent := NewTaskEvent(
    EventTypeTaskRequest,
    "trigger",
    "task-001",
    "move",
    &types.Task{ID: "task-001", Type: "move"},
    nil,
)

// 创建设备事件
deviceEvent := NewDeviceEvent(
    EventTypeDeviceConnect,
    "hal",
    "motor-1",
    map[string]interface{}{"connected": true},
    nil,
    map[string]interface{}{"position": 0},
)

// 创建网络事件
networkEvent := NewNetworkEvent(
    EventTypeNetworkMessage,
    "ipc",
    "client-001",
    "hello world",
    nil,
)

// 发送事件
eventLoop.EmitEvent(systemEvent)
eventLoop.EmitEvent(taskEvent)
eventLoop.EmitEvent(deviceEvent)
eventLoop.EmitEvent(networkEvent)
```

### 5. 事件循环性能监控

```go
// 获取模块状态
status := eventLoop.GetModuleStatus()
for moduleName, moduleStatus := range status {
    fmt.Printf("Module %s: %+v\n", moduleName, moduleStatus)
}

// 获取事件处理器统计
handlers := eventLoop.GetHandlers()
for eventType, count := range handlers {
    fmt.Printf("Event type %s: %d handlers\n", eventType, count)
}
```

这个开发指南已更新以反映新的HAL和水平层架构，提供了完整的分层开发指导、测试策略和运维配置。