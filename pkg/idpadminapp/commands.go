package idpadminapp

import (
	"context"
	"errors"
	"strings"
)

type CommandExecutor interface {
	Execute(context.Context, ExecutionRequest, []byte) ([]byte, error)
}

// CommandDispatcher routes only after validating the signed handle. Individual
// command services repeat preflight immediately before preparing and committing
// their domain-specific mutation.
type CommandDispatcher struct {
	executor    *Executor
	users       CommandExecutor
	invitations CommandExecutor
	clients     CommandExecutor
}

func NewCommandDispatcher(
	executor *Executor,
	users CommandExecutor,
	invitations CommandExecutor,
	clients CommandExecutor,
) (*CommandDispatcher, error) {
	if executor == nil || users == nil || invitations == nil || clients == nil {
		return nil, errors.New("executor and all administration command services are required")
	}
	return &CommandDispatcher{
		executor: executor, users: users, invitations: invitations, clients: clients,
	}, nil
}

func (d *CommandDispatcher) Execute(
	ctx context.Context,
	request ExecutionRequest,
	rawInput []byte,
) ([]byte, error) {
	claims, err := d.executor.Preflight(ctx, request)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasPrefix(claims.Command, "users."):
		return d.users.Execute(ctx, request, rawInput)
	case strings.HasPrefix(claims.Command, "invitations."):
		return d.invitations.Execute(ctx, request, rawInput)
	case strings.HasPrefix(claims.Command, "clients."):
		return d.clients.Execute(ctx, request, rawInput)
	default:
		return nil, ErrUnknownCommand
	}
}
