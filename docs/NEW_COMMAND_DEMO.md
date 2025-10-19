# 事件驱动命令处理架构演示

## 架构概述

本次重构实现了从复杂的前端命令接口到事件驱动的业务命令处理架构的彻底转型，通过CommandHandler接口和CommandRouter实现了模块化的命令处理机制。

## 重构前后对比

### 🔄 重构前（复杂接口）

前端需要了解后端实现细节：

```javascript
// 前端需要知道多种命令类型
{
  "type": "task_request",          // 任务请求
  "data": { "task": {...} }
}

{
  "type": "task_template_request",  // 模板请求
  "data": { "template_name": "home_all" }
}

{
  "type": "abstract_command_request", // 抽象命令请求
  "data": { "command": "home", "parameters": {...} }
}

{
  "type": "status_request",         // 状态请求
  "data": { "request": "full_status" }
}
```

### ✅ 重构后（事件驱动架构）

前端只关注业务意图，后端通过事件驱动机制处理：

```javascript
// 统一的命令格式
{
  "type": "business_command",
  "data": {
    "command": "move",    // 业务意图
    "params": {           // 参数
      "target": {"x": 100, "y": 50},
      "mode": "precise"
    }
  }
}
```

**后端事件驱动处理**:
```go
// CommandHandler接口实现
type BusinessLogicLayer struct {
    // ...
}

func (bll *BusinessLogicLayer) GetHandledCommands() []types.BusinessCommand {
    return []types.BusinessCommand{
        types.CmdMove,
        types.CmdHome,
        types.CmdEmergencyStop,
        // ...
    }
}

func (bll *BusinessLogicLayer) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    switch msg.Command {
    case types.CmdMove:
        return bll.handleMove(msg)
    case types.CmdHome:
        return bll.handleHome(msg)
    // ...
    }
}
```

## 新的命令类型（BusinessCommand）

### 系统控制命令
- `CmdQueryStatus` - 查询系统状态
- `CmdInitialize` - 初始化系统
- `CmdEmergencyStop` - 紧急停止
- `CmdReset` - 系统复位

### 运动控制命令
- `CmdMove` - 移动到位置
- `CmdMoveRelative` - 相对移动
- `CmdHome` - 系统回零
- `CmdJog` - 手动操作

### 安全检查命令
- `CmdSafetyCheck` - 安全检查
- `CmdSelfCheck` - 系统自检

### 配置管理命令
- `CmdGetConfig` - 获取配置
- `CmdSetConfig` - 设置配置
- `CmdListTemplates` - 列出模板

## 事件驱动命令路由机制

### CommandRouter智能路由

后端通过CommandRouter实现智能命令路由：

```go
// CommandRouter实现
type CommandRouter struct {
    handlers map[types.BusinessCommand]CommandHandler
    logger   *logging.Logger
}

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

### CommandHandler注册机制

模块通过实现CommandHandler接口注册命令处理器：

```go
// ApplicationManager注册所有CommandHandler
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

### 处理器分配示例

| 命令类型 | 处理器 | 职责 |
|---------|---------|------|
| `CmdMove`, `CmdHome` | BusinessLogicLayer | 运动控制和系统操作 |
| `CmdEmergencyStop` | BusinessLogicLayer | 紧急安全处理 |
| `CmdGetConfig`, `CmdSetConfig` | ConfigHandler | 配置管理 |
| `CmdSelfCheck` | BusinessLogicLayer | 系统自检 |

## 事件驱动架构实现过程

### 第一阶段：清理旧架构
✅ 移除所有旧的命令接口
✅ 删除复杂的消息类型
✅ 清理向后兼容代码

### 第二阶段：实现事件驱动
✅ 设计CommandHandler接口
✅ 实现CommandRouter路由机制
✅ 创建BusinessLogicLayer和ConfigHandler

### 第三阶段：系统整合
✅ 重构ApplicationManager使用CommandRouter
✅ 更新simulator使用新接口
✅ 完善测试和验证

## 测试验证

### 构建和运行测试
```bash
# 构建系统
go run build.go

# 运行控制系统
./bin/control

# 运行simulator测试
./bin/simulator
```

### 测试命令示例
```javascript
// 系统状态查询
{
  "type": "business_command",
  "data": {
    "command": "query",
    "params": {}
  }
}

// 系统回零
{
  "type": "business_command",
  "data": {
    "command": "home",
    "params": {
      "axes": ["x", "y", "z"]
    }
  }
}

// 紧急停止
{
  "type": "business_command",
  "data": {
    "command": "stop",
    "params": {
      "reason": "emergency_test"
    }
  }
}
```

### 预期结果
1. **100% 使用新命令** - simulator 只发送 `business_command`
2. **事件驱动路由** - CommandRouter正确路由到相应的CommandHandler
3. **模块化处理** - BusinessLogicLayer和ConfigHandler各司其职
4. **统一响应格式** - 所有命令返回一致的BusinessResponse结构

## 事件驱动架构优势

### 对前端开发者
- **极简接口** - 只需要了解有限的业务命令类型
- **专注业务** - 完全不关心后端实现细节
- **稳定可靠** - 后端架构变化不影响前端接口

### 对后端开发者
- **模块化设计** - 每个CommandHandler专注于特定领域
- **易于扩展** - 新命令类型只需实现CommandHandler接口
- **清晰职责** - 业务逻辑、配置管理等职责分离明确

### 对系统整体
- **高度解耦** - 前后端通过标准化接口完全解耦
- **事件驱动** - 模块间通过事件通信，降低耦合度
- **易于维护** - 清晰的架构和接口设计
- **高性能** - 并发处理和智能路由提升性能

### 架构演进成果
- **彻底转型** - 从复杂接口到事件驱动架构的彻底重构
- **模块化** - 通过CommandHandler接口实现真正的模块化
- **标准化** - 统一的BusinessMessage和BusinessResponse格式
- **可扩展** - 新功能可以轻松添加而不影响现有代码

## 扩展新CommandHandler

### 添加新的命令处理器

```go
// 1. 实现CommandHandler接口
type AlarmHandler struct {
    logger *logging.Logger
}

func (ah *AlarmHandler) GetHandledCommands() []types.BusinessCommand {
    return []types.BusinessCommand{
        types.CmdAlarmQuery,
        types.CmdAlarmClear,
        types.CmdAlarmHistory,
    }
}

func (ah *AlarmHandler) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
    // 处理报警相关命令
}

// 2. 在ApplicationManager中注册
am.commandRouter.RegisterHandler(&AlarmHandler{})
```

## 总结

这次事件驱动架构重构成功地实现了：
- ✅ **彻底简化** - 前端接口极简化，只发送业务命令
- ✅ **事件驱动** - 通过CommandRouter和CommandHandler实现事件驱动
- ✅ **模块化** - 每个模块专注于特定类型的命令处理
- ✅ **高度解耦** - 前后端通过标准化接口完全解耦
- ✅ **易于扩展** - 新功能可以轻松添加而不影响现有代码

这种架构真正实现了"关注点分离"的设计原则，前端开发者专注于业务意图，后端开发者专注于模块化实现，系统具备了更强的扩展能力、更好的维护性和更高的开发效率。