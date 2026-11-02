//go:build darwin || linux

package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

// Exercise the installed CLI's ordinary command executor under the production
// process guard, without authentication, inference, or a Pyxis host endpoint.
// Run with PYXIS_TEST_INSTALLED_CODEX=1; normal unit tests remain hermetic.
func TestInstalledReviewCommandAccess(t *testing.T) {
	if os.Getenv("PYXIS_TEST_INSTALLED_CODEX") != "1" {
		t.Skip("set PYXIS_TEST_INSTALLED_CODEX=1 to check the installed CLI")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ctx, stopInvocation := runtime.InvocationContext(ctx)
	defer func() {
		if err := stopInvocation(); err != nil {
			t.Errorf("stop invocation: %v", err)
		}
	}()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work, state := filepath.Join(root, "work"), filepath.Join(root, "state")
	for _, path := range []string{work, state, filepath.Join(state, "tmp"), filepath.Join(state, "home"), filepath.Join(state, "model-auth", "codex"), filepath.Join(state, "xdg", "config"), filepath.Join(state, "xdg", "cache"), filepath.Join(state, "xdg", "state")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", work}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	source := filepath.Join(work, "source.txt")
	const original = "exact review source marker\n"
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "source.txt")
	git("-c", "user.name=Review Fixture", "-c", "user.email=review@example.invalid", "commit", "-qm", "source fixture")
	head := git("rev-parse", "HEAD")
	spec := port.LaunchSpec{Purpose: port.LaunchPurposeReview, Workdir: work, RuntimeStateDir: state}
	cmd := exec.CommandContext(ctx, binary, "--no-daemon", "-a", "never", "-c", `sandbox_mode="danger-full-access"`, "app-server", "--stdio")
	cmd.Dir, cmd.Env, cmd.Stderr = work, runtime.LaunchEnvironment(spec, "CODEX_HOME"), os.Stderr
	procgroup.Setpgid(cmd)
	lifetime := procgroup.NewLifetime(cmd)
	cmd.Cancel = lifetime.Kill
	cmd.WaitDelay = time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.ConfineCommand(ctx, cmd, runtime.LaunchAccess(spec)); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := lifetime.ObserveStart(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	shutdown := func() {
		if stopped {
			return
		}
		stopped = true
		if err := in.Close(); err != nil {
			t.Errorf("close command input: %v", err)
		}
		if err := lifetime.Kill(); err != nil {
			t.Errorf("stop command executor: %v", err)
		}
		if err := stopInvocation(); err != nil {
			t.Errorf("stop delegated commands: %v", err)
		}
		// This test intentionally terminates its long-lived command executor.
		_ = cmd.Wait()
	}
	defer shutdown()
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 256<<10)
	encoder := json.NewEncoder(in)
	requestID := 0
	call := func(method string, params any, result any) error {
		requestID++
		request := struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params any    `json:"params"`
		}{requestID, method, params}
		if err := encoder.Encode(request); err != nil {
			return err
		}
		for scanner.Scan() {
			var response struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
				return err
			}
			if response.ID != requestID {
				continue
			}
			if len(response.Error) != 0 {
				return fmt.Errorf("%s: %s", method, response.Error)
			}
			return json.Unmarshal(response.Result, result)
		}
		return fmt.Errorf("command executor closed: scan=%v context=%v", scanner.Err(), ctx.Err())
	}
	var initialized json.RawMessage
	if err := call("initialize", struct {
		ClientInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}{ClientInfo: struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{"pyxis-review-access-test", "1"}}, &initialized); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		command []string
		allowed bool
		stdout  string
	}{
		{"source read", []string{"/bin/cat", source}, true, original},
		{"exact Git revision", []string{"git", "rev-parse", "HEAD"}, true, head + "\n"},
		{"shell overwrite", []string{"/bin/sh", "-c", `printf forbidden > "$1"`, "sh", source}, false, ""},
		{"delete", []string{"/bin/rm", source}, false, ""},
		{"rename", []string{"/bin/mv", source, filepath.Join(state, "moved-source")}, false, ""},
		{"Git metadata write", []string{"git", "update-ref", "refs/heads/review-forbidden", "HEAD"}, false, ""},
		{"scratch write", []string{"/usr/bin/touch", filepath.Join(state, "tmp", "allowed")}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := struct {
				Command       []string `json:"command"`
				Cwd           string   `json:"cwd"`
				SandboxPolicy struct {
					Type string `json:"type"`
				} `json:"sandboxPolicy"`
				TimeoutMS int `json:"timeoutMs"`
			}{Command: tc.command, Cwd: work, TimeoutMS: 5000}
			params.SandboxPolicy.Type = "dangerFullAccess"
			var result struct {
				ExitCode int    `json:"exitCode"`
				Stdout   string `json:"stdout"`
				Stderr   string `json:"stderr"`
			}
			if err := call("command/exec", params, &result); err != nil {
				t.Fatal(err)
			}
			if (result.ExitCode == 0) != tc.allowed || (tc.allowed && result.Stdout != tc.stdout) {
				t.Fatalf("allowed=%v, result=%+v", tc.allowed, result)
			}
			if got, err := os.ReadFile(source); err != nil || string(got) != original {
				t.Fatalf("source changed: %v: %q", err, got)
			}
		})
	}
	if got := git("rev-parse", "HEAD"); got != head {
		t.Fatalf("review moved HEAD: %s", got)
	}
	if got := git("for-each-ref", "refs/heads/review-forbidden"); got != "" {
		t.Fatalf("review created a reference: %s", got)
	}
	if _, err := os.Stat(filepath.Join(state, "tmp", "allowed")); err != nil {
		t.Fatalf("scratch was not writable: %v", err)
	}
	// Exercise a real native command that itself delegates a verifier into
	// another process group. The old CLI-only kill stranded that verifier.
	requestID++
	request := struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params struct {
			Command       []string `json:"command"`
			Cwd           string   `json:"cwd"`
			SandboxPolicy struct {
				Type string `json:"type"`
			} `json:"sandboxPolicy"`
			TimeoutMS int `json:"timeoutMs"`
		} `json:"params"`
	}{ID: requestID, Method: "command/exec"}
	request.Params.Command = []string{os.Args[0], "-test.run=^TestInstalledCommandLifetimeHelper$", "--", "pyxis-lifetime", "root", state, work}
	request.Params.Cwd, request.Params.TimeoutMS, request.Params.SandboxPolicy.Type = work, 30000, "dangerFullAccess"
	if err := encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	var pids []int
	for _, name := range []string{"root", "leaf"} {
		path := filepath.Join(state, name+".pid")
		for {
			data, err := os.ReadFile(path)
			if err == nil {
				pid, err := strconv.Atoi(string(data))
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, pid)
				break
			}
			if ctx.Err() != nil {
				t.Fatalf("native command did not start %s: %v", name, ctx.Err())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if pgid, err := syscall.Getpgid(pids[1]); err != nil || pgid == cmd.Process.Pid {
		t.Fatalf("verifier did not leave the CLI group: %d, %v", pgid, err)
	}
	shutdown()
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
			t.Fatalf("native command pid %d survived acknowledged cleanup: %v", pid, err)
		}
	}
}

func TestInstalledCommandLifetimeHelper(t *testing.T) {
	if len(os.Args) < 5 || os.Args[len(os.Args)-4] != "pyxis-lifetime" {
		return
	}
	mode, state, work := os.Args[len(os.Args)-3], os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if err := os.WriteFile(filepath.Join(state, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(90)
	}
	if mode == "leaf" {
		for {
			time.Sleep(time.Hour)
		}
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInstalledCommandLifetimeHelper$", "--", "pyxis-lifetime", "leaf", state, work)
	cmd.Dir, cmd.Env = work, os.Environ()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	procgroup.Setpgid(cmd)
	if err := runtime.ConfineCommand(ctx, cmd, &runtime.CommandAccess{Purpose: port.LaunchPurposeReview, Workdir: work, RuntimeStateDir: state}); err != nil {
		os.Exit(91)
	}
	if err := cmd.Run(); err != nil {
		os.Exit(92)
	}
	os.Exit(93)
}
