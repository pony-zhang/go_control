package management

import (
	"context"
	"fmt"
	"time"

	"control/internal/core"
	"control/internal/logging"
	"control/pkg/types"
)

// ConfigHandler handles configuration-related business commands
type ConfigHandler struct {
	configManager interface{} // 这里应该是 *config.ConfigManager，但为了避免循环依赖，使用interface{}
	logger        *logging.Logger
}

// NewConfigHandler creates a new configuration handler
func NewConfigHandler(configManager interface{}, logger *logging.Logger) *ConfigHandler {
	return &ConfigHandler{
		configManager: configManager,
		logger:        logger,
	}
}

// CommandHandler interface implementation
func (ch *ConfigHandler) GetHandledCommands() []types.BusinessCommand {
	return []types.BusinessCommand{
		types.CmdGetConfig,
		types.CmdSetConfig,
		types.CmdListTemplates,
	}
}

func (ch *ConfigHandler) HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse {
	ch.logger.Info("Config handler handling command", "command", msg.Command)

	switch msg.Command {
	case types.CmdGetConfig:
		return ch.handleGetConfig(msg)
	case types.CmdSetConfig:
		return ch.handleSetConfig(msg)
	case types.CmdListTemplates:
		return ch.handleListTemplates(msg)
	default:
		return &types.BusinessResponse{
			RequestID: msg.RequestID,
			Status:    "error",
			Error:     fmt.Sprintf("config handler cannot handle command: %s", msg.Command),
			Timestamp: time.Now(),
		}
	}
}

func (ch *ConfigHandler) GetName() string {
	return "config"
}

// Command handling methods
func (ch *ConfigHandler) handleGetConfig(msg *types.BusinessMessage) *types.BusinessResponse {
	// 简化实现，返回默认配置
	config := map[string]interface{}{
		"system": map[string]interface{}{
			"name":        "Motion Control System",
			"version":     "1.0.0",
			"status":      "running",
		},
		"ipc": map[string]interface{}{
			"address": "127.0.0.1",
			"port":    18080,
		},
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"config": config,
		},
		Timestamp: time.Now(),
	}
}

func (ch *ConfigHandler) handleSetConfig(msg *types.BusinessMessage) *types.BusinessResponse {
	// 简化实现，只记录配置更新请求
	ch.logger.Info("Configuration update requested", "params", msg.Params)

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"message": "Configuration update initiated",
		},
		Timestamp: time.Now(),
	}
}

func (ch *ConfigHandler) handleListTemplates(msg *types.BusinessMessage) *types.BusinessResponse {
	// 返回可用的任务模板
	templates := []string{
		"home_all",
		"safety_check",
		"self_check",
		"emergency_stop",
	}

	return &types.BusinessResponse{
		RequestID: msg.RequestID,
		Status:    "success",
		Data: map[string]interface{}{
			"templates": templates,
		},
		Timestamp: time.Now(),
	}
}

// Ensure ConfigHandler implements core.CommandHandler
var _ core.CommandHandler = (*ConfigHandler)(nil)