# Motion Control System Architecture

## 概述

运动控制系统是一个基于Go语言的工业自动化运动控制核心系统，采用分层、事件驱动的架构设计。系统通过模块化组件实现任务调度、设备管理、进程间通信等核心功能，为工业机器人和自动化设备提供可靠的运动控制解决方案。

## 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        External Clients                         │
│                      (GUI, API, 其他系统)                        │
└─────────────────────────┬───────────────────────────────────────┘
                          │ TCP/IP
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      IPC Server Layer                          │
│                 (进程间通信服务器)                              │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Task Request
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Task Trigger                                │
│                 (任务触发器)                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Validated Tasks
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Task Scheduler                               │
│                (优先级任务调度器)                                │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Scheduled Tasks
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Task Decomposer                               │
│                (任务分解器)                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Motion Commands
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Execution Queue                               │
│                 (命令执行队列)                                   │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Commands to Execute
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Command Executor                               │
│                 (命令执行器)                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Device Commands
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Device Manager                               │
│                 (设备管理器)                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Protocol Commands
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                Hardware Abstraction Layer                       │
│                 (硬件抽象层)                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Physical Signals
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Physical Devices                              │
│                  (物理设备: 电机, 传感器等)                       │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. 事件循环 (Event Loop)

**位置**: `internal/core/eventloop.go`

**职责**: 系统中央协调器，负责驱动所有模块的周期性处理

**关键特性**:
- 可配置的处理间隔 (默认10ms)
- 模块生命周期管理
- 基于Context的取消机制
- 模块状态监控

**数据流**:
```
Ticker → EventLoop.processCycle() → module.Process() for each registered module
```

### 2. 任务触发器 (Task Trigger)

**位置**: `internal/core/trigger.go`

**职责**: 管理多个任务触发源，验证和发布任务

**关键特性**:
- 支持多种触发源 (IPC, 定时器, 外部事件)
- 任务验证机制
- 缓冲通道防止系统过载
- 动态触发源管理

**数据流**:
```
Trigger Sources → TaskTrigger.checkTriggers() → Validate → taskChan → Scheduler
```

### 3. 任务调度器 (Task Scheduler)

**位置**: `internal/core/scheduler.go`

**职责**: 基于优先级的任务调度和管理

**关键特性**:
- 四级优先级队列 (Emergency, High, Medium, Low)
- 抢占式调度
- 任务生命周期管理
- 上下文取消支持

**优先级定义**:
- **Emergency (0)**: 紧急停止等安全相关任务
- **High (1)**: 高优先级运动任务
- **Medium (2)**: 普通运动任务
- **Low (3)**: 后台维护任务

### 4. 任务分解器 (Task Decomposer)

**位置**: `internal/core/decomposer.go`, `tasknode_decomposer.go`

**职责**: 将高级任务分解为设备可执行的命令序列

**关键特性**:
- 任务节点树结构
- 轨迹规划算法
- 系统约束验证
- 抽象命令映射

**分解流程**:
```
High-level Task → Validate Constraints →
Trajectory Planning → Motion Commands[] → Device Commands
```

### 5. 命令执行队列 (Execution Queue)

**位置**: `internal/core/queue.go`

**职责**: 管理命令执行生命周期，提供超时和取消机制

**关键特性**:
- 命令状态管理 (pending→sent→executing→completed/failed)
- 任务级命令分组
- 超时和取消机制
- 资源清理

**状态转换**:
```go
type CommandStatus int
const (
    StatusPending CommandStatus = iota
    StatusSent
    StatusExecuting
    StatusCompleted
    StatusFailed
)
```

### 6. 命令执行器 (Command Executor)

**位置**: `internal/core/executor.go`

**职责**: 协调命令在设备上的执行

**关键特性**:
- 设备生命周期管理
- 命令路由和分发
- 执行状态监控
- 错误处理和报告

**数据流**:
```
Queue.sendChan → executeCommands() → processCommand() →
device.Write() → Queue.completion
```

### 7. 设备管理器 (Device Manager)

**位置**: `internal/device/device.go`

**职责**: 统一管理各种设备，提供协议抽象

**关键特性**:
- 多协议支持 (Mock, Modbus, 自定义协议)
- 设备生命周期管理
- 硬件抽象层集成
- 设备状态聚合

**支持的协议**:
- **Mock Device**: 模拟设备，用于测试
- **Modbus Device**: Modbus TCP/RTU 协议
- **Hardware Device**: 新硬件抽象层包装
- **Custom Protocol**: 可扩展的自定义协议

### 8. IPC 服务器 (IPC Server)

**位置**: `internal/ipc/server.go`

**职责**: 提供进程间通信能力，处理外部客户端请求

**关键特性**:
- TCP服务器实现
- 消息路由和处理
- 客户端连接管理
- 广播和单播通信

**支持的消息类型**:
- `task_request`: 任务请求
- `task_template_request`: 任务模板请求
- `abstract_command_request`: 抽象命令请求
- `status_request`: 状态查询
- `config_update`: 配置更新

## 数据流分析

### 1. 任务执行完整流程

```
外部请求 → IPC服务器 → 任务触发器 → 任务调度器 → 任务分解器 →
执行队列 → 命令执行器 → 设备管理器 → 硬件设备 → 物理执行
```

**详细步骤**:

1. **请求接收**: IPC服务器接收外部客户端的TCP连接
2. **任务验证**: 任务触发器验证任务参数和系统约束
3. **任务调度**: 调度器根据优先级将任务放入相应队列
4. **任务分解**: 分解器将高级任务转换为设备命令序列
5. **命令排队**: 执行队列管理命令的生命周期和状态
6. **命令执行**: 执行器将命令发送到相应设备
7. **设备执行**: 设备管理器通过协议与硬件通信
8. **状态反馈**: 执行结果通过队列返回给调度器

### 2. 事件循环协调机制

```
事件循环定时器 → 模块.Process()调用:
- 任务触发器检查触发源
- 命令执行器处理待执行命令
- IPC服务器管理客户端连接
- 设备管理器更新设备状态
```

### 3. 配置管理流程

```
配置文件 → ConfigManager → 系统组件初始化 →
运行时监控 → 热重载 → 配置更新通知
```

## 关键设计模式

### 1. 接口驱动架构

所有核心组件都基于标准化接口实现:

```go
type Module interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    Process() error
    Status() interface{}
}

type Device interface {
    Write(cmd types.MotionCommand) error
    Read(reg string) (interface{}, error)
    Connect() error
    Disconnect()
    Status() types.DeviceStatus
    ID() types.DeviceID
    Type() string
}
```

### 2. 通道通信模式

模块间通过通道进行异步通信:

```go
// 任务触发器到调度器
taskChan chan *types.Task

// 执行队列到执行器
sendChan chan *QueuedCommand
completeChan chan *CompletedCommand

// IPC消息处理
messageChan chan types.IPCMessage
```

### 3. 事件驱动处理

- 中央事件循环驱动所有模块
- 基于Context的取消机制
- 周期性处理和状态更新

### 4. 分层抽象

- **应用层**: IPC通信和任务管理
- **逻辑层**: 任务调度和命令执行
- **设备层**: 协议抽象和硬件接口
- **物理层**: 实际设备控制

### 5. 配置驱动

系统行为通过YAML配置文件定义:

```yaml
event_loop_interval: 10ms
queue_size: 1000
ipc:
  type: tcp
  address: 127.0.0.1
  port: 8080
devices:
  device-1:
    type: controller
    protocol: modbus
    endpoint: tcp://192.168.1.100:502
```

## 错误处理和恢复机制

### 1. 命令级错误处理

- 每个命令都有独立的超时和取消机制
- 失败命令标记错误详情并记录
- 队列自动清理防止资源泄漏

### 2. 设备级恢复

- 连接状态监控和自动重连
- 优雅降级处理断连设备
- 设备状态缓存和同步

### 3. 系统级安全

- 任务验证防止不安全操作
- 边界检查确保系统约束
- 优雅关闭和资源清理

### 4. 日志和监控

- 结构化日志记录关键事件
- 模块状态聚合和报告
- 性能指标收集

## 扩展性设计

### 1. 模块化扩展

- 新模块可通过事件循环注册添加
- 设备协议通过Device接口扩展
- 触发源可动态注册

### 2. 性能优化

- 并发命令执行提高吞吐量
- 缓冲通道平衡处理速度
- 设备特定优化策略

### 3. 配置灵活性

- 运行时配置热重载
- 环境特定配置支持
- 约束验证防止错误配置

## 部署和运行

### 1. 系统启动

```bash
# 启动控制系统
./bin/control -config config.yaml

# 启动模拟器
./bin/simulator -address 127.0.0.1 -port 8080
```

### 2. 配置要求

- Go 1.21+ 运行环境
- YAML配置文件
- 网络端口权限 (默认8080)
- 设备访问权限

### 3. 监控和维护

- 通过IPC接口查询系统状态
- 结构化日志分析系统行为
- 配置热重载无需重启

## 总结

该运动控制系统采用现代软件工程最佳实践，提供了工业级运动控制所需的可靠性、可扩展性和可维护性。通过分层架构、接口驱动设计和事件驱动处理，系统能够高效管理复杂的运动控制任务，同时保持良好的扩展性和维护性。