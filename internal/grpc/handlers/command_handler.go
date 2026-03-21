package handlers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gameap/gameap/pkg/proto"
)

type CommandResult struct {
	CommandID string
	ExitCode  int32
	Output    []byte
	Error     string
}

type CommandHandler struct {
	logger *slog.Logger

	mu              sync.RWMutex
	pendingCommands map[string]chan *CommandResult
}

func NewCommandHandler(logger *slog.Logger) *CommandHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &CommandHandler{
		logger:          logger,
		pendingCommands: make(map[string]chan *CommandResult),
	}
}

func (h *CommandHandler) HandleCommandOutput(_ context.Context, nodeID uint64, output *proto.CommandOutput) error {
	h.logger.Debug("received command output",
		"node_id", nodeID,
		"command_id", output.CommandId,
		"bytes", len(output.OutputChunk),
	)

	return nil
}

func (h *CommandHandler) HandleCommandResult(_ context.Context, nodeID uint64, result *proto.CommandResult) error {
	h.logger.Debug("received command result",
		"node_id", nodeID,
		"command_id", result.CommandId,
		"exit_code", result.ExitCode,
		"error", result.Error,
	)

	h.mu.RLock()
	ch, ok := h.pendingCommands[result.CommandId]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- &CommandResult{
			CommandID: result.CommandId,
			ExitCode:  result.ExitCode,
			Output:    result.Output,
			Error:     result.Error,
		}:
		default:
		}
	}

	return nil
}

func (h *CommandHandler) RegisterPendingCommand(commandID string) <-chan *CommandResult {
	ch := make(chan *CommandResult, 1)
	h.mu.Lock()
	h.pendingCommands[commandID] = ch
	h.mu.Unlock()

	return ch
}

func (h *CommandHandler) UnregisterPendingCommand(commandID string) {
	h.mu.Lock()
	if ch, ok := h.pendingCommands[commandID]; ok {
		close(ch)
		delete(h.pendingCommands, commandID)
	}
	h.mu.Unlock()
}
