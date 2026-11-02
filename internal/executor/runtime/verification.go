package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type verificationAccessKey struct{}

// VerificationContext attaches a host-owned boundary to all repository code
// invoked during verification. Its scratch is shared across checks, is outside
// the checkout, and is removed when the owning supervisor call returns. It is
// not a repository setting and cannot be disabled by a manifest or environment.
func VerificationContext(ctx context.Context) (context.Context, func(), error) {
	if VerificationAccessRequired(ctx) {
		return ctx, func() {}, nil
	}
	state, err := os.MkdirTemp("", "pyxis-verification-*")
	if err != nil {
		return ctx, nil, fmt.Errorf("runtime: verification scratch: %w", err)
	}
	protected := context.WithValue(ctx, verificationAccessKey{}, state)
	var owner *launchBrokerOwner
	if _, inherited, err := currentLaunchCapability(); err != nil {
		_ = os.RemoveAll(state) // scratch has not been exposed to a child
		return ctx, nil, err
	} else if !inherited {
		owner = &launchBrokerOwner{ctx: ctx}
		protected = context.WithValue(protected, launchBrokerKey{}, owner)
	}
	return protected, func() {
		if owner != nil {
			_ = owner.close() // invocation owners surface stop errors before allowing checkpoint
		}
		// Scratch contains only disposable verification data. Cleanup cannot
		// authorize publication or turn a failed verification into success.
		_ = os.RemoveAll(state)
	}, nil
}

// InvocationContext gives one native process its own delegated-command owner.
// Verification scratch can be shared across the task, but a killed model must
// not leave broker-launched commands alive until the whole task returns.
// Closing cancels and joins every delegated group before the caller may touch
// Git. An inherited proxy is already owned by its upstream invocation.
func InvocationContext(ctx context.Context) (context.Context, func() error) {
	ctx, cancel := context.WithCancel(ctx)
	owner := &launchBrokerOwner{ctx: ctx}
	protected := context.WithValue(ctx, launchBrokerKey{}, owner)
	var once sync.Once
	var stopErr error
	return protected, func() error {
		once.Do(func() {
			cancel()
			stopErr = owner.close()
		})
		return stopErr
	}
}

// VerificationAccessRequired also separates protected verdicts from legacy
// cache entries created by an unconfined invocation.
func VerificationAccessRequired(ctx context.Context) bool {
	_, ok := ctx.Value(verificationAccessKey{}).(string)
	return ok
}

// ConfineVerificationCommand applies the native harness's write boundary to
// every build/test subprocess. Repository env entries cannot redirect caches
// into Git metadata or regain the host's Git/code-host environment. Declared
// non-directory settings (including a hermetic battery's perturbations) remain.
func ConfineVerificationCommand(ctx context.Context, cmd *exec.Cmd) error {
	state, ok := ctx.Value(verificationAccessKey{}).(string)
	ref, inherited, err := currentLaunchCapability()
	if err != nil {
		return err
	}
	if !ok {
		if !inherited {
			return nil // standalone codegate retains its separately specified policy
		}
		// Descendants already inherit the OS policy. Declarative verb envs may
		// strip the owned temporary directories; keep those directories and the
		// scoped launch capability available without adding another sandbox.
		state = ref.State
	}
	if err := os.MkdirAll(filepath.Join(state, "home"), 0o700); err != nil {
		return fmt.Errorf("runtime: verification home: %w", err)
	}
	overrides := map[string]string{
		"HOME":   filepath.Join(state, "home"),
		"TMPDIR": filepath.Join(state, "tmp"), "TMP": filepath.Join(state, "tmp"), "TEMP": filepath.Join(state, "tmp"),
		"GOCACHE": filepath.Join(state, "go-build"), "GOPATH": filepath.Join(state, "go"),
		"GOMODCACHE": filepath.Join(state, "go", "pkg", "mod"), "GOTMPDIR": filepath.Join(state, "tmp"),
		"XDG_CACHE_HOME": filepath.Join(state, "cache"), "XDG_CONFIG_HOME": filepath.Join(state, "config"),
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null", "GIT_OPTIONAL_LOCKS": "0",
		"GIT_TERMINAL_PROMPT": "0", "GH_CONFIG_DIR": filepath.Join(state, "no-codehost"),
	}
	env := make([]string, 0, len(cmd.Env)+len(overrides))
	for _, item := range cmd.Env {
		key, _, _ := strings.Cut(item, "=")
		// Loader injection would execute inside the wrapper before it applies
		// the boundary. Repository-controlled env cannot preload host code.
		if _, replaced := overrides[key]; replaced || isHostCredential(ctx, key) || strings.HasPrefix(key, "GIT_") || strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_") || key == "GCONV_PATH" || key == "GH_TOKEN" || key == "GITHUB_TOKEN" || key == "GH_ENTERPRISE_TOKEN" || key == "GITHUB_ENTERPRISE_TOKEN" || key == "SSH_AUTH_SOCK" {
			continue
		}
		env = append(env, item)
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	if err := configureVerificationTools(ctx, cmd, inherited); err != nil {
		return err
	}
	if !ok {
		cmd.Env = withLaunchCapability(cmd.Env, ref)
		return nil
	}
	return ConfineCommand(ctx, cmd, &CommandAccess{Workdir: cmd.Dir, RuntimeStateDir: state, RepositoryOptional: true})
}
