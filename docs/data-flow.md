# 数据流和状态管理

## 数据流总览

运动控制系统的数据流遵循先进的分层处理模式和事件驱动架构，从外部请求到物理设备执行，经过协调层、基础设施层、应用层的多个水平子层处理阶段。系统采用命令路由器和订阅分发机制，实现了模块化、可扩展的数据处理流程。每个层次都有明确的职责边界和数据转换规则，确保系统的可靠性和可维护性。

## 主要数据结构

### 1. 任务 (Task)

```go
type Task struct {
    ID          string                 // 任务唯一标识
    Type        CommandType           // 任务类型
    Priority    Priority              // 任务优先级
    Status      TaskStatus            // 任务状态
    Parameters  map[string]interface{} // 任务参数
    RootNode    TaskNode              // 任务根节点
    CreatedAt   time.Time             // 创建时间
    Timeout     time.Duration         // 超时时间
    Context     context.Context       // 任务上下文
}
```

### 2. 命令 (MotionCommand)

```go
type MotionCommand struct {
    ID        string        // 命令唯一标识
    DeviceID  DeviceID      // 目标设备ID
    Type      CommandType   // 命令类型
    Axis      string        // 目标轴
    Position  float64       // 目标位置
    Velocity  float64       // 目标速度
    Accel     float64       // 加速度
    Duration  time.Duration // 执行时长
    Status    CommandStatus // 命令状态
    Timestamp time.Time     // 时间戳
}
```

### 3. 业务消息 (BusinessMessage)

```go
type BusinessMessage struct {
    Command   BusinessCommand         // 业务命令类型
    Source    string                 // 消息源
    Target    string                 // 目标地址
    RequestID string                 // 请求ID
    Params    map[string]interface{} // 命令参数
    Timestamp time.Time               // 时间戳
}

type BusinessResponse struct {
    RequestID string                 // 请求ID
    Status    string                 // 响应状态
    Data      map[string]interface{} // 响应数据
    Error     string                 // 错误信息
    Timestamp time.Time               // 时间戳
}
```

### 4. IPC消息 (IPCMessage)

```go
type IPCMessage struct {
    Type      string                 // 消息类型
    Source    string                 // 消息源
    Target    string                 // 目标地址
    Data      map[string]interface{} // 消息数据
    Timestamp time.Time               // 时间戳
    ID        string                 // 消息ID
}
```

## 数据流详细分析

### 1. 事件驱动命令流程 (新架构)

```
外部客户端 → IPC服务器 → ApplicationManager → CommandRouter → CommandHandlers → 各层处理
```

**步骤详解**:

1. **客户端连接** (IPC Server - 基础设施层)
   ```go
   // TCP连接建立
   conn, err := ln.Accept()
   client := &Client{
       ID:        generateClientID(),
       Conn:      conn,
       SendChan:  make(chan types.IPCMessage, 100),
       ReceiveChan: make(chan types.IPCMessage, 100),
   }
   ```

2. **消息接收和路由** (IPC Server → ApplicationManager)
   ```go
   // 消息解码和路由到ApplicationManager
   for message := range client.ReceiveChan {
       if handler, exists := s.handlers[message.Type]; exists {
           go handler(message)  // 委托给ApplicationManager处理
       }
   }
   ```

3. **业务消息处理** (ApplicationManager)
   ```go
   // ApplicationManager处理业务命令
   func (am *ApplicationManager) handleBusinessCommand(message types.IPCMessage) {
       businessMsg, err := am.parseBusinessMessage(message)
       if err != nil {
           am.sendBusinessErrorResponse(message.Source, "", err.Error())
           return
       }

       // 通过命令路由器自动路由到相应的处理器
       response := am.routeBusinessCommand(businessMsg)
       am.sendBusinessResponse(message.Source, response)
   }
   ```

4. **命令路由和处理** (CommandRouter → CommandHandlers)
   ```go
   // CommandRouter自动路由命令到相应的处理器
   func (cr *CommandRouter) RouteCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
       handler, exists := cr.handlers[msg.Command]
       if !exists {
           return &types.BusinessResponse{
               RequestID: msg.RequestID,
               Status:    "error",
               Error:     fmt.Sprintf("no handler registered for command: %s", msg.Command),
               Timestamp: time.Now(),
           }
       }
       return handler.HandleCommand(ctx, msg)
   }
   ```

5. **业务逻辑层处理** (BusinessLogicLayer)
   ```go
   // BusinessLogicLayer实现CommandHandler接口
   func (bll *BusinessLogicLayer) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
       switch msg.Command {
       case types.CmdQueryStatus:
           return bll.handleQueryStatus(msg)
       case types.CmdSelfCheck:
           return bll.handleSelfCheck(msg)
       case types.CmdEmergencyStop:
           return bll.handleEmergencyStop(msg)
       // ... 其他命令处理
       }
   }
   ```

6. **任务创建和调度** (Task Orchestration Layer)
   ```go
   // 任务调度和分解
   func (bll *BusinessLogicLayer) ExecuteBusinessCommand(command types.BusinessCommand, params map[string]interface{}) (*types.Task, error) {
       task := &types.Task{
           ID:         fmt.Sprintf("business-%s-%d", command, time.Now().UnixNano()),
           Type:       bll.mapBusinessToCommandType(command),
           Priority:   bll.mapBusinessToPriority(command),
           Status:     types.StatusPending,
           Parameters: params,
           CreatedAt:  time.Now(),
           Timeout:    bll.mapBusinessToTimeout(command),
       }

       return bll.taskOrchestration.ScheduleTask(task)
   }
   ```

### 2. 命令处理器注册和管理

```
CommandHandler实现 → 注册到CommandRouter → 命令映射 → 自动路由
```

**步骤详解**:

1. **CommandHandler实现** (各层模块)
   ```go
   // BusinessLogicLayer实现CommandHandler接口
   func (bll *BusinessLogicLayer) GetHandledCommands() []types.BusinessCommand {
       return []types.BusinessCommand{
           types.CmdQueryStatus,
           types.CmdSelfCheck,
           types.CmdEmergencyStop,
           types.CmdHome,
           types.CmdInitialize,
           types.CmdSafetyCheck,
       }
   }

   // ConfigHandler实现CommandHandler接口
   func (ch *ConfigHandler) GetHandledCommands() []types.BusinessCommand {
       return []types.BusinessCommand{
           types.CmdGetConfig,
           types.CmdSetConfig,
           types.CmdListTemplates,
       }
   }
   ```

2. **处理器注册** (ApplicationManager)
   ```go
   // 在ApplicationManager中注册所有命令处理器
   func (am *ApplicationManager) setupCommandHandlers() {
       // 注册业务逻辑层处理器
       am.commandRouter.RegisterHandler(am.businessLogic)

       // 注册配置管理处理器
       am.commandRouter.RegisterHandler(am.configHandler)

       // 记录注册的处理器
       registeredHandlers := am.commandRouter.GetRegisteredHandlers()
       for handlerName, commands := range registeredHandlers {
           am.logger.Info("Registered handler", "handler", handlerName, "commands", commands)
       }
   }
   ```

3. **命令映射管理** (CommandRouter)
   ```go
   // CommandRouter维护命令到处理器的映射
   func (cr *CommandRouter) RegisterHandler(handler CommandHandler) {
       for _, cmd := range handler.GetHandledCommands() {
           if existing, exists := cr.handlers[cmd]; exists {
               cr.logger.Warn("Command handler conflict", "command", cmd, "existing_handler", existing.GetName(), "new_handler", handler.GetName())
           }
           cr.handlers[cmd] = handler
           cr.logger.Info("Registered command handler", "command", cmd, "handler", handler.GetName())
       }
   }
   ```

### 3. 任务处理流程 (保持不变)

```
高级任务 → 轨迹规划 → 命令序列 → 执行队列
```

**步骤详解**:

1. **任务解析** (Task Decomposer)
   ```go
   // 解析任务参数
   if rootNode, ok := task.Parameters["root_node"]; ok {
       task.RootNode = createTaskNode(rootNode)
   }
   ```

2. **轨迹规划** (Trajectory Planner)
   ```go
   // 生成运动轨迹点
   trajectory := []types.TrajectoryPoint{
       {Position: start, Velocity: 0, Time: 0},
       {Position: end, Velocity: 0, Time: duration},
   }

   // 插值计算中间点
   for t := step; t < duration; t += step {
       point := interpolate(trajectory, t)
       trajectory = append(trajectory, point)
   }
   ```

3. **命令生成** (Task Decomposer)
   ```go
   // 生成设备命令
   for i, point := range trajectory {
       cmd := types.MotionCommand{
           ID:       fmt.Sprintf("cmd-%s-%d", task.ID, i),
           DeviceID: deviceID,
           Type:     types.CommandTypeMoveTo,
           Axis:     axis,
           Position: point.Position,
           Velocity: point.Velocity,
           Duration: time.Duration(float64(i) * float64(duration) / float64(len(trajectory))),
       }
       commands = append(commands, cmd)
   }
   ```

### 4. 命令执行流程

```
命令队列 → 设备路由 → 协议转换 → 物理执行
```

**步骤详解**:

1. **命令排队** (Execution Queue)
   ```go
   // 命令状态管理
   queuedCmd := &QueuedCommand{
       Command:    cmd,
       Status:     types.StatusPending,
       CreatedAt:  time.Now(),
       Timeout:    cmd.Timeout,
       Context:    context.WithTimeout(ctx, cmd.Timeout),
   }

   q.commands[cmd.ID] = queuedCmd
   ```

2. **命令发送** (Command Executor)
   ```go
   // 发送到执行通道
   select {
   case q.sendChan <- queuedCmd:
       queuedCmd.Status = types.StatusSent
   default:
       return fmt.Errorf("send channel full")
   }
   ```

3. **设备路由** (Command Executor)
   ```go
   // 根据设备ID路由命令
   if device, exists := ce.devices[cmd.DeviceID]; exists {
       if device.Status() == types.DeviceStatusConnected {
           err := device.Write(cmd)
           // 处理执行结果
       }
   }
   ```

### 5. 设备通信流程

```
设备命令 → 协议转换 → 硬件通信 → 状态反馈
```

**步骤详解**:

1. **协议选择** (Device Manager)
   ```go
   // 根据设备配置选择协议
   switch config.Protocol {
   case "mock":
       device = &MockDevice{id: deviceID, config: config}
   case "modbus":
       device = &ModbusDevice{id: deviceID, config: config}
   case "hardware":
       device = &HardwareDevice{id: deviceID, config: config}
   }
   ```

2. **协议转换** (Device Implementation)
   ```go
   // Modbus协议转换示例
   func (md *ModbusDevice) Write(cmd types.MotionCommand) error {
       // 位置寄存器写入
       _, err := md.client.WriteSingleRegister(
           uint16(md.config.PositionRegister),
           uint16(cmd.Position*md.config.PositionScale),
       )

       // 速度寄存器写入
       _, err = md.client.WriteSingleRegister(
           uint16(md.config.VelocityRegister),
           uint16(cmd.Velocity*md.config.VelocityScale),
       )

       return err
   }
   ```

3. **硬件通信** (Hardware Abstraction Layer)
   ```go
   // 硬件抽象层调用
   func (hd *HardwareDevice) Write(cmd types.MotionCommand) error {
       // 通过硬件抽象层执行命令
       return hd.hardwareLayer.ExecuteCommand(cmd)
   }
   ```

## 状态管理

### 1. 任务状态转换

```
Pending → Scheduled → Decomposing → Executing → Completed
                    ↓
                  Failed/Cancelled
```

**状态转换条件**:
- **Pending**: 任务已创建，等待调度
- **Scheduled**: 任务已进入调度队列
- **Decomposing**: 任务正在分解为命令
- **Executing**: 命令正在执行
- **Completed**: 任务执行完成
- **Failed**: 任务执行失败
- **Cancelled**: 任务被取消

### 2. 命令状态转换

```
Pending → Sent → Executing → Completed
           ↓
         Failed/Timeout
```

**状态管理代码**:
```go
// 命令状态更新
func (q *ExecutionQueue) MarkCommandSent(cmdID string) error {
    q.mu.Lock()
    defer q.mu.Unlock()

    if cmd, exists := q.commands[cmdID]; exists {
        cmd.Status = types.StatusSent
        cmd.SentAt = time.Now()
        return nil
    }

    return fmt.Errorf("command not found: %s", cmdID)
}
```

### 3. 设备状态管理

```
Disconnected → Connecting → Connected → Error
                                   ↓
                              Reconnecting
```

**设备状态监控**:
```go
// 定期检查设备状态
func (dm *DeviceManager) monitorDevices() {
    for _, device := range dm.devices {
        status := device.Status()
        if status != types.DeviceStatusConnected {
            // 尝试重新连接
            go dm.reconnectDevice(device.ID())
        }
    }
}
```

## 错误处理流程

### 1. 命令级错误处理

```go
// 命令执行错误处理
func (ce *CommandExecutor) processCommand(queuedCmd *QueuedCommand) {
    defer func() {
        if r := recover(); r != nil {
            ce.logger.Error("Command execution panic", "error", r)
            queuedCmd.Status = types.StatusFailed
            queuedCmd.Error = fmt.Sprintf("panic: %v", r)
        }
    }()

    err := device.Write(queuedCmd.Command)
    if err != nil {
        queuedCmd.Status = types.StatusFailed
        queuedCmd.Error = err.Error()
    }
}
```

### 2. 任务级错误处理

```go
// 任务执行错误处理
func (ts *TaskScheduler) executeTask(task *types.Task) {
    defer func() {
        if r := recover(); r != nil {
            ts.logger.Error("Task execution panic", "task_id", task.ID, "error", r)
            task.Status = types.StatusFailed
        }
    }()

    commands, err := ts.taskDecomposer.DecomposeTask(task)
    if err != nil {
        task.Status = types.StatusFailed
        return
    }

    // 执行命令序列
    for _, cmd := range commands {
        if err := ts.commandExecutor.ExecuteCommand(task.Context, cmd); err != nil {
            task.Status = types.StatusFailed
            break
        }
    }
}
```

### 3. 系统级错误处理

```go
// 系统级错误恢复
func (el *EventLoop) handleModuleError(moduleName string, err error) {
    el.logger.Error("Module processing error", "module", moduleName, "error", err)

    // 尝试重启模块
    if module, exists := el.modules[moduleName]; exists {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if stopErr := module.Stop(); stopErr != nil {
            el.logger.Error("Failed to stop module", "module", moduleName, "error", stopErr)
        }

        if startErr := module.Start(ctx); startErr != nil {
            el.logger.Error("Failed to restart module", "module", moduleName, "error", startErr)
        }
    }
}
```

## 性能监控指标

### 1. 系统指标

```go
type SystemMetrics struct {
    Uptime           time.Duration `json:"uptime"`
    TaskCount        int64         `json:"task_count"`
    CommandCount     int64         `json:"command_count"`
    ErrorCount       int64         `json:"error_count"`
    ActiveTasks      int64         `json:"active_tasks"`
    QueueSize        int64         `json:"queue_size"`
    DeviceCount      int64         `json:"device_count"`
    ConnectedClients int64         `json:"connected_clients"`
}
```

### 2. 任务指标

```go
type TaskMetrics struct {
    TotalTasks      int64         `json:"total_tasks"`
    CompletedTasks  int64         `json:"completed_tasks"`
    FailedTasks     int64         `json:"failed_tasks"`
    AverageDuration time.Duration `json:"average_duration"`
    MaxDuration     time.Duration `json:"max_duration"`
    MinDuration     time.Duration `json:"min_duration"`
}
```

### 3. 设备指标

```go
type DeviceMetrics struct {
    DeviceID        string        `json:"device_id"`
    CommandsSent    int64         `json:"commands_sent"`
    CommandsFailed  int64         `json:"commands_failed"`
    AverageLatency  time.Duration `json:"average_latency"`
    ConnectionCount int64         `json:"connection_count"`
    Uptime          time.Duration `json:"uptime"`
}
```

## 数据一致性保证

### 1. 并发控制

```go
// 使用互斥锁保护共享状态
type ExecutionQueue struct {
    mu         sync.RWMutex
    commands   map[string]*QueuedCommand
    taskCommands map[string][]string
    sendChan   chan *QueuedCommand
    completeChan chan *CompletedCommand
}
```

### 2. 事务性操作

```go
// 原子性的命令状态更新
func (q *ExecutionQueue) updateCommandStatus(cmdID string, newStatus CommandStatus) error {
    q.mu.Lock()
    defer q.mu.Unlock()

    cmd, exists := q.commands[cmdID]
    if !exists {
        return fmt.Errorf("command not found: %s", cmdID)
    }

    cmd.Status = newStatus
    cmd.UpdatedAt = time.Now()

    return nil
}
```

### 3. 数据持久化

```go
// 关键数据持久化
func (cm *ConfigManager) saveSystemState(state *SystemState) error {
    data, err := json.Marshal(state)
    if err != nil {
        return err
    }

    return os.WriteFile("system_state.json", data, 0644)
}
```

## 总结

运动控制系统的数据流设计体现了现代工业控制系统的核心特征，并集成了事件驱动的命令处理架构：

1. **分层处理**: 从高层任务到底层命令的清晰分层
2. **事件驱动**: 命令路由器和订阅分发机制实现了模块化处理
3. **异步处理**: 通过通道实现高效的异步通信
4. **状态管理**: 完善的状态跟踪和转换机制
5. **错误处理**: 多层次的错误处理和恢复机制
6. **性能监控**: 全面的系统性能指标收集
7. **数据一致性**: 并发控制和事务性操作保证
8. **可扩展性**: 通过CommandHandler接口轻松添加新的命令处理器

这种设计确保了系统在高负载、高可靠性要求下的稳定运行，为工业自动化应用提供了坚实的技术基础。事件驱动的架构使得系统更加模块化、可维护和可扩展。