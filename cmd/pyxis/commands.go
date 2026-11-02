package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var (
	errInvalidRootCommand   = errors.New("invalid root command descriptor")
	errDuplicateRootCommand = errors.New("duplicate root command")
	errLegacyRootCommand    = errors.New("root command collides with legacy verb")
)

type rootCommand struct {
	key     string
	usage   string
	handler func(context.Context, []string, io.Writer, io.Writer) int
}

type rootCommandRegistry struct {
	commands map[string]rootCommand
}

func newRootCommandRegistry() *rootCommandRegistry {
	return &rootCommandRegistry{commands: make(map[string]rootCommand)}
}

func (registry *rootCommandRegistry) register(command rootCommand) error {
	if command.key == "" || strings.TrimSpace(command.key) != command.key || strings.ContainsAny(command.key, " \t\r\n") || strings.TrimSpace(command.usage) == "" || command.handler == nil {
		return fmt.Errorf("%w: key, usage, and handler are required", errInvalidRootCommand)
	}
	if legacyRootCommands[command.key] {
		return fmt.Errorf("%w: %q", errLegacyRootCommand, command.key)
	}
	if _, exists := registry.commands[command.key]; exists {
		return fmt.Errorf("%w: %q", errDuplicateRootCommand, command.key)
	}
	registry.commands[command.key] = command
	return nil
}

func (registry *rootCommandRegistry) dispatch(ctx context.Context, key string, args []string, stdout, stderr io.Writer) (int, bool) {
	command, ok := registry.commands[key]
	if !ok {
		return 0, false
	}
	return command.handler(ctx, args, stdout, stderr), true
}

func (registry *rootCommandRegistry) usages() []string {
	keys := make([]string, 0, len(registry.commands))
	for key := range registry.commands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, registry.commands[key].usage)
	}
	return result
}

var rootCommands = newRootCommandRegistry()

func mustRegisterRootCommand(command rootCommand) {
	if err := rootCommands.register(command); err != nil {
		panic(err)
	}
}

var legacyRootCommands = map[string]bool{
	"credential": true, "init": true, "surfaces": true, "doctor": true,
	"converge": true, "repair": true, "adopt": true, "apply": true,
	"undo": true, "watch": true, "status": true, "draft": true,
	"approve": true, "review-rearm": true, "exec": true, "upgrade": true,
	"version": true, "shell": true, "help": true, "-h": true, "--help": true,
}
