# 数据流和状态管理

## 数据流总览

运动控制系统的数据流遵循分层处理模式，从外部请求到物理设备执行，经过多个处理阶段。每个阶段都有明确的职责边界和数据转换规则。

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

### 3. IPC消息 (IPCMessage)

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

### 1. 任务请求流程

```
外部客户端 → IPC服务器 → 任务处理
```

**步骤详解**:

1. **客户端连接** (IPC Server)
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

2. **消息接收** (IPC Server)
   ```go
   // 消息解码和路由
   for message := range client.ReceiveChan {
       if handler, exists := s.handlers[message.Type]; exists {
           go handler(message)
       }
   }
   ```

3. **任务创建** (Task Handler)
   ```go
   // 根据消息类型创建任务
   task := &types.Task{
       ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
       Type:      types.CommandType(taskData["type"].(string)),
       Priority:  types.PriorityMedium,
       Status:    types.StatusPending,
       CreatedAt: time.Now(),
   }
   ```

### 2. 任务验证和调度流程

```
任务创建 → 验证 → 优先级调度 → 执行队列
```

**步骤详解**:

1. **任务验证** (Task Trigger)
   ```go
   // 验证任务参数
   if err := validateTaskConstraints(task, systemConfig); err != nil {
       return fmt.Errorf("task validation failed: %w", err)
   }
   ```

2. **优先级分配** (Task Scheduler)
   ```go
   // 根据任务类型分配优先级
   switch task.Type {
   case types.CommandTypeEmergencyStop:
       task.Priority = types.PriorityEmergency
   case types.CommandTypeMoveTo:
       task.Priority = types.PriorityHigh
   default:
       task.Priority = types.PriorityMedium
   }
   ```

3. **队列分配** (Task Scheduler)
   ```go
   // 根据优先级放入相应队列
   switch task.Priority {
   case types.PriorityEmergency:
       ts.emergencyQueue.Push(task)
   case types.PriorityHigh:
       ts.highPriorityQueue.Push(task)
   case types.PriorityMedium:
       ts.mediumPriorityQueue.Push(task)
   case types.PriorityLow:
       ts.lowPriorityQueue.Push(task)
   }
   ```

### 3. 任务分解流程

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

运动控制系统的数据流设计体现了现代工业控制系统的核心特征：

1. **分层处理**: 从高层任务到底层命令的清晰分层
2. **异步处理**: 通过通道实现高效的异步通信
3. **状态管理**: 完善的状态跟踪和转换机制
4. **错误处理**: 多层次的错误处理和恢复机制
5. **性能监控**: 全面的系统性能指标收集
6. **数据一致性**: 并发控制和事务性操作保证

这种设计确保了系统在高负载、高可靠性要求下的稳定运行，为工业自动化应用提供了坚实的技术基础。