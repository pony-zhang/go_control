# 开发指南

## 开发环境设置

### 1. 系统要求

- Go 1.21+
- Git
- Make (可选)
- Docker (可选，用于容器化部署)

### 2. 项目结构

```
control/
├── cmd/                    # 应用程序入口
│   ├── control/           # 控制系统主程序
│   └── simulator/         # 模拟器程序
├── internal/              # 内部包
│   ├── core/              # 核心系统组件
│   ├── device/            # 设备实现
│   ├── hardware/          # 硬件抽象层
│   ├── ipc/               # 进程间通信
│   ├── config/            # 配置管理
│   └── logging/           # 日志系统
├── pkg/                   # 公共包
│   └── types/             # 类型定义
├── docs/                  # 文档
├── config.yaml           # 配置文件
├── go.mod                # Go模块定义
└── Makefile              # 构建脚本
```

### 3. 依赖管理

```bash
# 初始化模块
go mod tidy

# 添加依赖
go get github.com/example/package

# 更新依赖
go get -u github.com/example/package
```

## 构建和运行

### 1. 本地构建

```bash
# 构建所有组件
make build

# 或者手动构建
go build ./cmd/control
go build ./cmd/simulator

# 构建到指定目录
mkdir -p bin
go build -o bin/control ./cmd/control
go build -o bin/simulator ./cmd/simulator
```

### 2. 运行系统

```bash
# 运行控制系统
./bin/control

# 运行模拟器
./bin/simulator

# 指定配置文件
./bin/control -config /path/to/config.yaml
```

### 3. 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/core
go test ./internal/device

# 运行测试并显示覆盖率
go test -cover ./...

# 运行测试并生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 核心开发概念

### 1. 模块接口

所有核心模块都必须实现 `Module` 接口：

```go
type Module interface {
    Name() string                    // 模块名称
    Start(ctx context.Context) error // 启动模块
    Stop() error                     // 停止模块
    Process() error                  // 处理逻辑
    Status() interface{}             // 状态信息
}
```

### 2. 设备接口

设备实现必须实现 `Device` 接口：

```go
type Device interface {
    Write(cmd types.MotionCommand) error // 写入命令
    Read(reg string) (interface{}, error) // 读取寄存器
    Connect() error                     // 连接设备
    Disconnect()                        // 断开连接
    Status() types.DeviceStatus         // 设备状态
    ID() types.DeviceID                 // 设备ID
    Type() string                       // 设备类型
}
```

### 3. 任务生命周期

```go
type TaskStatus int

const (
    StatusPending TaskStatus = iota   // 待处理
    StatusScheduled                  // 已调度
    StatusDecomposing                // 分解中
    StatusExecuting                  // 执行中
    StatusCompleted                  // 已完成
    StatusFailed                     // 失败
    StatusCancelled                  // 已取消
)
```

## 添加新功能

### 1. 添加新设备类型

1. **实现设备接口**

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

2. **注册设备类型**

```go
// 在 device_manager.go 中添加设备创建逻辑
case "new_protocol":
    device = &NewDevice{
        id:     deviceID,
        config: config,
    }
```

3. **更新配置**

```yaml
devices:
  new-device:
    type: controller
    protocol: new_protocol
    endpoint: tcp://192.168.1.100:9999
    timeout: 10s
```

### 2. 添加新任务类型

1. **定义任务类型**

```go
// 在 types/types.go 中添加
const (
    CommandTypeNewTask CommandType = "new_task"
    // 其他类型...
)
```

2. **实现任务分解逻辑**

```go
// 在 decomposer.go 中添加分解逻辑
func (td *TaskDecomposer) decomposeNewTask(task *types.Task) ([]types.MotionCommand, error) {
    commands := make([]types.MotionCommand, 0)

    // 实现新任务的分解逻辑
    // ...

    return commands, nil
}
```

3. **注册处理函数**

```go
// 在 task_decomposer.go 中注册
td.decomposers[types.CommandTypeNewTask] = td.decomposeNewTask
```

### 3. 添加新触发器

1. **实现触发器接口**

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
```

2. **注册触发器**

```go
// 在主程序中添加
trigger := &NewTrigger{
    name: "new_trigger",
    triggerFn: generateNewTask,
    interval: 5 * time.Second,
}

taskTrigger.AddTrigger(trigger)
```

### 4. 添加新的IPC消息处理器

1. **实现处理器**

```go
func (mcs *MotionControlSystem) handleNewMessage(message types.IPCMessage) {
    // 处理新消息类型
    response := types.IPCMessage{
        Type:      "new_message_response",
        Source:    "control_system",
        Target:    message.Source,
        Data:      map[string]interface{}{"result": "success"},
        Timestamp: time.Now(),
        ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
    }

    mcs.ipcServer.SendToClient(message.Source, response)
}
```

2. **注册处理器**

```go
// 在 setupEventHandlers 中添加
mcs.ipcServer.RegisterHandler("new_message", mcs.handleNewMessage)
```

## 配置管理

### 1. 配置文件结构

```yaml
# 系统配置
event_loop_interval: 10ms
queue_size: 1000

# IPC配置
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
  timeout: 5s
  buffer_size: 1024

# 设备配置
devices:
  device-1:
    type: controller
    protocol: modbus
    endpoint: tcp://192.168.1.100:502
    timeout: 10s

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

# 安全配置
safety:
  emergency_stop_timeout: 100ms
  max_position_error: 0.1
  max_velocity_error: 1.0

# 日志配置
logging:
  level: info
  format: text
  output: stdout
  add_source: false
  time_format: "2006-01-02T15:04:05.000Z"
```

### 2. 配置热重载

系统支持配置文件的热重载，修改配置文件后会自动应用新配置：

```go
// 配置变更监听
mcs.configManager.WatchChanges(func(config types.SystemConfig) {
    log.Println("Configuration changed, updating system...")
    go mcs.handleConfigUpdate(config)
})
```

## 调试和诊断

### 1. 日志系统

使用结构化日志进行调试：

```go
// 获取模块日志器
logger := logging.GetLogger("module_name")

// 记录不同级别的日志
logger.Debug("Debug information", "key", "value")
logger.Info("General information", "key", "value")
logger.Warn("Warning condition", "key", "value")
logger.Error("Error condition", "error", err)

// 带上下文的日志
logger.WithContext(ctx).Info("Contextual information")
```

### 2. 状态查询

通过IPC接口查询系统状态：

```go
// 发送状态请求
statusRequest := types.IPCMessage{
    Type:      "status_request",
    Source:    "client_id",
    Target:    "control_system",
    Data:      map[string]interface{}{},
    Timestamp: time.Now(),
    ID:        "msg-123",
}

// 接收状态响应
statusResponse := types.IPCMessage{
    Type: "status_response",
    Data: map[string]interface{}{
        "system": map[string]interface{}{
            "running":    true,
            "uptime":     "1h30m",
            "config":     "/path/to/config.yaml",
        },
        "devices": deviceStatuses,
        "scheduler": schedulerStatus,
        "executor": executorStatus,
    },
}
```

### 3. 性能分析

使用Go的性能分析工具：

```bash
# CPU分析
go tool pprof http://localhost:8080/debug/pprof/profile

# 内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# 协程分析
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

## 错误处理最佳实践

### 1. 错误处理模式

```go
// 1. 错误包装
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// 2. 错误日志记录
if err := doSomething(); err != nil {
    logger.Error("Failed to do something", "error", err, "context", "additional_info")
    return err
}

// 3. 错误恢复
defer func() {
    if r := recover(); r != nil {
        logger.Error("Recovered from panic", "error", r)
        // 恢复逻辑
    }
}()
```

### 2. 资源清理

```go
// 使用defer确保资源清理
func process() error {
    resource, err := acquireResource()
    if err != nil {
        return err
    }
    defer resource.Release()

    // 处理逻辑
    return nil
}
```

### 3. 超时处理

```go
// 使用context处理超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

select {
case result := <-doSomething(ctx):
    return result
case <-ctx.Done():
    return ctx.Err()
}
```

## 测试策略

### 1. 单元测试

```go
func TestTaskScheduler_ScheduleTask(t *testing.T) {
    // 准备测试数据
    scheduler := NewTaskScheduler(testConfig)
    task := &types.Task{
        ID:       "test-task",
        Priority: types.PriorityMedium,
    }

    // 执行测试
    err := scheduler.ScheduleTask(task)

    // 验证结果
    if err != nil {
        t.Errorf("Failed to schedule task: %v", err)
    }

    // 验证任务在队列中
    if scheduler.mediumPriorityQueue.Size() != 1 {
        t.Error("Task not found in queue")
    }
}
```

### 2. 集成测试

```go
func TestMotionControlSystem_Integration(t *testing.T) {
    // 创建完整的系统
    system, err := NewMotionControlSystem("test_config.yaml")
    if err != nil {
        t.Fatalf("Failed to create system: %v", err)
    }

    // 启动系统
    err = system.Start()
    if err != nil {
        t.Fatalf("Failed to start system: %v", err)
    }
    defer system.Stop()

    // 测试任务执行
    task := createTestTask()
    // 发送任务并验证结果...
}
```

### 3. 模拟测试

```go
func TestDeviceManager_MockDevice(t *testing.T) {
    // 使用模拟设备进行测试
    device := &MockDevice{id: "test-device"}
    manager := NewDeviceManager(testConfig)

    err := manager.AddDevice(device)
    if err != nil {
        t.Errorf("Failed to add mock device: %v", err)
    }

    // 测试设备操作
    cmd := types.MotionCommand{
        ID:       "test-cmd",
        DeviceID: "test-device",
        Type:     types.CommandTypeMoveTo,
    }

    err = device.Write(cmd)
    if err != nil {
        t.Errorf("Mock device write failed: %v", err)
    }
}
```

## 部署和运维

### 1. 容器化部署

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/control ./cmd/control
RUN go build -o bin/simulator ./cmd/simulator

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/bin/ .
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./control"]
```

### 2. 系统服务

```ini
# /etc/systemd/system/motion-control.service
[Unit]
Description=Motion Control System
After=network.target

[Service]
Type=simple
User=control
WorkingDirectory=/opt/motion-control
ExecStart=/opt/motion-control/bin/control
ExecStop=/opt/motion-control/bin/control --shutdown
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 3. 监控配置

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'motion-control'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
```

## 贡献指南

### 1. 代码风格

- 遵循Go标准代码风格
- 使用gofmt格式化代码
- 添加适当的注释和文档
- 编写单元测试

### 2. 提交规范

```bash
# 提交前检查
make test
make lint
make build

# 提交信息格式
feat: 添加新功能
fix: 修复bug
docs: 文档更新
style: 代码格式化
refactor: 代码重构
test: 测试相关
chore: 构建或工具变动
```

### 3. PR流程

1. Fork项目
2. 创建功能分支
3. 编写代码和测试
4. 提交PR
5. 代码审查
6. 合并到主分支

## 常见问题

### Q: 如何添加新的设备协议？
A: 实现Device接口，在DeviceManager中注册，更新配置文件格式。

### Q: 任务执行失败如何处理？
A: 系统会自动记录错误日志，可以通过状态查询接口获取失败详情。

### Q: 如何优化系统性能？
A: 调整事件循环间隔、队列大小、并发参数，使用性能分析工具定位瓶颈。

### Q: 如何实现设备热插拔？
A: 通过DeviceManager的AddDevice/RemoveDevice方法，配合配置热重载功能。

这个开发指南提供了完整的技术文档，帮助开发者理解系统架构、开发新功能、调试问题和部署系统。