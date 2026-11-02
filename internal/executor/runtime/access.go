package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// CommandAccess is host-owned process policy. It is separate from model routing
// and cannot be relaxed by a charter, repository config, or native tool call.
// The OS boundary applies to the whole CLI, including file tools and children.
type CommandAccess struct {
	Purpose         port.LaunchPurpose
	Workdir         string
	RuntimeStateDir string
	// RepositoryOptional is reserved for supervisor verification of disposable
	// source copies. Native launches always require a Git worktree.
	RepositoryOptional bool
}

// LaunchAccess confines every implementation, review, and prototype process.
func LaunchAccess(spec port.LaunchSpec) *CommandAccess {
	return &CommandAccess{Purpose: spec.Purpose, Workdir: spec.Workdir, RuntimeStateDir: spec.RuntimeStateDir}
}

var ErrAccessUnsupported = errors.New("runtime: OS process confinement unavailable")

type accessPaths struct {
	work      string
	state     string
	protected []string
	readOnly  bool
	ancestors []accessPaths
	launchMounts
}

// ConfineCommand wraps an already configured command before Start. Failure is
// terminal: there is no unsandboxed retry. Native harness permissions remain an
// additional restriction, never the authority granting Git writes.
func ConfineCommand(ctx context.Context, cmd *exec.Cmd, access *CommandAccess) error {
	return confineCommand(ctx, cmd, access, confineProcess)
}

func confineCommand(ctx context.Context, cmd *exec.Cmd, access *CommandAccess, wrap func(*exec.Cmd, accessPaths) error) error {
	if access == nil {
		return nil
	}
	paths, err := resolveAccessPaths(ctx, *access)
	if err != nil {
		return fmt.Errorf("runtime: resolve process access: %w", err)
	}
	if owner, ok := ctx.Value(launchBrokerKey{}).(*launchBrokerOwner); ok {
		broker, err := owner.get(ctx)
		if err != nil {
			return err
		}
		ref, err := broker.grant(paths)
		if err != nil {
			return err
		}
		cmd.Env = withLaunchCapability(cmd.Env, ref)
	}
	if err := os.MkdirAll(filepath.Join(paths.state, "tmp"), 0o700); err != nil {
		return fmt.Errorf("runtime: prepare confined temporary directory: %w", err)
	}
	// Prove the OS can apply this exact policy before starting a model CLI.
	// A wrapper executable existing on disk is not proof that its kernel
	// facilities (or nested sandbox permissions) are available.
	probe := exec.CommandContext(ctx, "/usr/bin/true")
	probe.Dir, probe.Env = paths.work, cmd.Env
	if err := wrap(probe, paths); err != nil {
		return err
	}
	if out, err := probe.CombinedOutput(); err != nil {
		diagnostic := strings.TrimSpace(string(out))
		if len(diagnostic) > 1024 {
			diagnostic = diagnostic[:1024]
		}
		return fmt.Errorf("%w: process policy probe: %v: %s", ErrAccessUnsupported, err, diagnostic)
	}
	return wrap(cmd, paths)
}

func resolveAccessPaths(ctx context.Context, access CommandAccess) (accessPaths, error) {
	paths := accessPaths{readOnly: access.Purpose == port.LaunchPurposeReview}
	if access.Purpose != "" && access.Purpose != port.LaunchPurposeImplementation && access.Purpose != port.LaunchPurposePrototype && !paths.readOnly {
		return paths, fmt.Errorf("unsupported confined purpose %q", access.Purpose)
	}
	var err error
	paths.work, err = accessDirectory(access.Workdir)
	if err != nil {
		return paths, fmt.Errorf("worktree: %w", err)
	}
	paths.state, err = accessDirectory(access.RuntimeStateDir)
	if err != nil {
		return paths, fmt.Errorf("runtime state: %w", err)
	}
	if pathContains(paths.work, paths.state) || pathContains(paths.state, paths.work) {
		return paths, errors.New("runtime state and worktree must be separate directories")
	}
	gitEntry := filepath.Join(paths.work, ".git")
	entry, err := os.Lstat(gitEntry)
	if access.RepositoryOptional && os.IsNotExist(err) {
		return paths, nil // the source copy is writable; every other path remains read-only
	}
	if err != nil {
		return paths, fmt.Errorf("inspect worktree Git entry: %w", err)
	}
	if !entry.IsDir() && !entry.Mode().IsRegular() {
		return paths, errors.New("worktree Git entry must be a directory or regular gitfile")
	}
	// Never inherit a Git directory/index/config override from the host shell.
	probe := exec.CommandContext(ctx, "git", "-C", paths.work, "rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir")
	probe.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_OPTIONAL_LOCKS=0"}
	out, err := probe.Output()
	if err != nil {
		return paths, fmt.Errorf("resolve worktree Git metadata: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 3 {
		return paths, errors.New("ambiguous worktree Git metadata paths")
	}
	root, err := accessDirectory(lines[0])
	if err != nil || root != paths.work {
		return paths, errors.New("process directory must be the exact Git worktree root")
	}
	paths.protected = []string{gitEntry}
	for _, line := range lines[1:] {
		dir, err := accessDirectory(line)
		if err != nil {
			return paths, fmt.Errorf("git metadata: %w", err)
		}
		if pathContains(dir, paths.work) || pathContains(dir, paths.state) || pathContains(paths.state, dir) {
			return paths, errors.New("writable runtime paths overlap Git administration")
		}
		paths.protected = append(paths.protected, dir)
	}
	return paths, nil
}

func accessDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("expected an absolute, unambiguous directory path")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || resolved == string(filepath.Separator) {
		return "", errors.New("expected a non-root directory")
	}
	return resolved, nil
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
