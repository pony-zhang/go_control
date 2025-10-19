package core

import (
	"context"
	"fmt"
	"time"

	"control/pkg/types"
)

// Logger interface for logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// CommandHandler defines the interface for handling business commands
type CommandHandler interface {
	// GetHandledCommands returns the list of commands this handler can process
	GetHandledCommands() []types.BusinessCommand

	// HandleCommand processes a business command and returns a response
	HandleCommand(ctx context.Context, msg *types.BusinessMessage) *types.BusinessResponse

	// GetName returns the handler name for logging
	GetName() string
}

// CommandRouter manages command routing to registered handlers
type CommandRouter struct {
	handlers map[types.BusinessCommand]CommandHandler
	logger   Logger
}

// NewCommandRouter creates a new command router
func NewCommandRouter(logger Logger) *CommandRouter {
	return &CommandRouter{
		handlers: make(map[types.BusinessCommand]CommandHandler),
		logger:   logger,
	}
}

// RegisterHandler registers a command handler for specific commands
func (cr *CommandRouter) RegisterHandler(handler CommandHandler) {
	for _, cmd := range handler.GetHandledCommands() {
		if existing, exists := cr.handlers[cmd]; exists {
			cr.logger.Warn("Command handler conflict", "command", cmd, "existing_handler", existing.GetName(), "new_handler", handler.GetName())
		}
		cr.handlers[cmd] = handler
		cr.logger.Info("Registered command handler", "command", cmd, "handler", handler.GetName())
	}
}

// UnregisterHandler removes a command handler
func (cr *CommandRouter) UnregisterHandler(handler CommandHandler) {
	for _, cmd := range handler.GetHandledCommands() {
		if cr.handlers[cmd] == handler {
			delete(cr.handlers, cmd)
			cr.logger.Info("Unregistered command handler", "command", cmd, "handler", handler.GetName())
		}
	}
}

// RouteCommand routes a command to the appropriate handler
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

	cr.logger.Info("Routing command to handler", "command", msg.Command, "handler", handler.GetName())
	return handler.HandleCommand(ctx, msg)
}

// GetRegisteredHandlers returns information about registered handlers
func (cr *CommandRouter) GetRegisteredHandlers() map[string][]types.BusinessCommand {
	result := make(map[string][]types.BusinessCommand)

	for cmd, handler := range cr.handlers {
		handlerName := handler.GetName()
		result[handlerName] = append(result[handlerName], cmd)
	}

	return result
}