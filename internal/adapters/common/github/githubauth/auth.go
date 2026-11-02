// Package githubauth scopes GitHub CLI credentials to one child process.
package githubauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Environment returns a child environment for one explicitly configured
// credential. An empty name preserves the caller's legacy ambient behavior.
func Environment(tokenEnv string) ([]string, error) {
	return EnvironmentForHost(tokenEnv, "")
}

// EnvironmentForHost also binds an explicit GitHub connection when its
// credential comes from ambient gh authentication. A zero-value pair retains
// legacy behavior. Enterprise servers are not part of the connection contract.
func EnvironmentForHost(tokenEnv, host string) ([]string, error) {
	if tokenEnv == "" && host == "" {
		return nil, nil
	}
	if host != "" && host != "github.com" {
		return nil, fmt.Errorf("configured GitHub host must be github.com")
	}
	token := os.Getenv(tokenEnv)
	if tokenEnv != "" && token == "" {
		return nil, fmt.Errorf("configured GitHub credential is missing: set %s", tokenEnv)
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GH_HOST", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN":
			continue
		case "GH_TOKEN", "GITHUB_TOKEN":
			if tokenEnv != "" {
				continue
			}
		}
		env = append(env, entry)
	}
	if tokenEnv != "" {
		env = append(env, "GH_TOKEN="+token)
	}
	return append(env, "GH_HOST=github.com"), nil
}

// Run executes a single gh command. Credentials travel only in the child
// environment; writes are never retried. stdin may carry a structured body.
func Run(ctx context.Context, tokenEnv string, stdin []byte, args ...string) ([]byte, error) {
	return RunForHost(ctx, tokenEnv, "", stdin, args...)
}

// RunForHost executes a command using an explicit public GitHub connection.
func RunForHost(ctx context.Context, tokenEnv, host string, stdin []byte, args ...string) ([]byte, error) {
	env, err := EnvironmentForHost(tokenEnv, host)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("GitHub request: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		message := strings.TrimSpace(string(exit.Stderr))
		if token := os.Getenv(tokenEnv); token != "" {
			message = strings.ReplaceAll(message, token, "[redacted]")
		}
		return nil, fmt.Errorf("GitHub request: %s", message)
	}
	return nil, fmt.Errorf("GitHub request: %w", err)
}
