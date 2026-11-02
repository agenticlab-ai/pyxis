package gitcredential

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// Change captures exact local Git config values so init can undo a late
// failure without disturbing the operator's previous credential helpers.
type Change struct {
	Key    string
	Before []string
	After  []string
}

// Executable returns a durable installed pyxis path. A source-mode go-run
// executable vanishes when the command exits and must never enter Git config.
func Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("credential: locate pyxis executable: %w", err)
	}
	if transientExecutable(path) {
		path, err = exec.LookPath("pyxis")
		if err != nil || transientExecutable(path) {
			return "", fmt.Errorf("credential: install a durable pyxis executable before configuring Git authentication")
		}
	}
	return filepath.Abs(path)
}

func transientExecutable(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/go-build") || strings.HasSuffix(path, ".test")
}

// Plan reads the selected binding and local Git config but performs no writes.
// Helpers are URL scoped, and their first empty entry suppresses any inherited
// helper for that selected URL. Other URLs retain their original configuration.
func Plan(ctx context.Context, root, executable string) ([]Change, error) {
	profile, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	return PlanForProfile(ctx, root, executable, profile)
}

// PlanForProfile supports init preview before its new config exists on disk.
func PlanForProfile(ctx context.Context, root, executable string, profile integrations.Config) ([]Change, error) {
	if profile.Bindings.CodeHost == (integrations.Binding{}) {
		return nil, nil
	}
	bound, err := profile.CodeHost("")
	if err != nil {
		return nil, err
	}
	if !usesHelper(bound) {
		return nil, nil
	}
	if executable == "" {
		executable, err = Executable()
		if err != nil {
			return nil, err
		}
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("credential: resolve executable: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, fmt.Errorf("credential: trusted pyxis executable is missing or not executable")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("credential: resolve repository root: %w", err)
	}
	command := "!" + quote(executable) + " credential --repo " + quote(root)
	gitRoot, err := isRepositoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, suffix := range []string{"", ".git"} {
		prefix := "credential." + bound.BaseURL + "/" + bound.Repository + suffix + "."
		for _, desired := range []struct {
			key    string
			values []string
		}{{prefix + "helper", []string{"", command}}, {prefix + "useHttpPath", []string{"true"}}} {
			var before []string
			if gitRoot {
				before, err = readValues(ctx, root, desired.key)
				if err != nil {
					return nil, err
				}
			}
			if !reflect.DeepEqual(before, desired.values) {
				changes = append(changes, Change{Key: desired.key, Before: before, After: desired.values})
			}
		}
	}
	return changes, nil
}

// Configure applies the read-only plan. Any partial failure restores its own
// writes; a successful caller retains Change records for wider rollback.
func Configure(ctx context.Context, root, executable string) ([]Change, error) {
	profile, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	return ConfigureForProfile(ctx, root, executable, profile)
}

// ConfigureForProfile is Configure for an already-resolved capability profile.
func ConfigureForProfile(ctx context.Context, root, executable string, profile integrations.Config) ([]Change, error) {
	changes, err := PlanForProfile(ctx, root, executable, profile)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return nil, nil
	}
	gitRoot, err := isRepositoryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	if !gitRoot {
		return nil, fmt.Errorf("credential: configuration requires the selected repository root")
	}
	for i, change := range changes {
		if err := writeValues(ctx, root, change.Key, change.After); err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			restoreErr := Restore(cleanup, root, changes[:i+1])
			cancel()
			if restoreErr != nil {
				return changes[:i+1], fmt.Errorf("%w; restore credential configuration: %v", err, restoreErr)
			}
			return nil, err
		}
	}
	return changes, nil
}

func isRepositoryRoot(ctx context.Context, root string) (bool, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 {
			return false, nil
		}
		return false, fmt.Errorf("credential: inspect Git repository root: %w", err)
	}
	actual, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return false, err
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	expected, err = filepath.Abs(expected)
	return actual == expected, err
}

func Restore(ctx context.Context, root string, changes []Change) error {
	for _, change := range slices.Backward(changes) {
		if err := writeValues(ctx, root, change.Key, change.Before); err != nil {
			return err
		}
	}
	return nil
}

func readValues(ctx context.Context, root, key string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--null", "--get-all", key).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("credential: read local Git configuration: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00"), nil
}

func writeValues(ctx context.Context, root, key string, values []string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--unset-all", key)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 5 {
			return fmt.Errorf("credential: reset local Git configuration: %w", err)
		}
	}
	for _, value := range values {
		if err := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--add", key, value).Run(); err != nil {
			return fmt.Errorf("credential: set local Git configuration: %w", err)
		}
	}
	return nil
}

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
