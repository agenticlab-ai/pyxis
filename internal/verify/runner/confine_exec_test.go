package runner

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestConfineLinuxRewritesThroughTheCurrentExecutable(t *testing.T) {
	resolve := func() (string, error) { t.Fatal("disabled confinement resolved executable"); return "", nil }
	t.Setenv("CODEGATE_LANDLOCK", "0")
	cmd := &exec.Cmd{Path: "/tool", Args: []string{"tool", "arg"}}
	if active, err := confineWithLandlock(cmd, []string{"/workspace"}, resolve); err != nil || active {
		t.Fatalf("disabled confine = %v, %v", active, err)
	}

	t.Setenv("CODEGATE_LANDLOCK", "1")
	resolve = func() (string, error) { return "/codegate", nil }
	cmd = &exec.Cmd{Path: "/tool", Args: []string{"tool", "arg"}}
	active, err := confineWithLandlock(cmd, []string{"/workspace", "/tmp/cache"}, resolve)
	if err != nil || !active {
		t.Fatalf("enabled confine = %v, %v", active, err)
	}
	wantArgs := []string{"/codegate", ConfineExecArg, "/workspace", "/tmp/cache", "--", "/tool", "tool", "arg"}
	if cmd.Path != "/codegate" || !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("rewritten command = %q %#v, want /codegate %#v", cmd.Path, cmd.Args, wantArgs)
	}

	resolve = func() (string, error) { return "", errors.New("missing self") }
	if _, err := confineWithLandlock(&exec.Cmd{Path: "/tool", Args: []string{"tool"}}, []string{"/workspace"}, resolve); err == nil || !strings.Contains(err.Error(), "missing self") {
		t.Fatalf("resolve failure = %v", err)
	}
}

func TestRunConfinedExecFailsClosedAndPreservesExecArguments(t *testing.T) {
	execProcess := func(string, []string, []string) error { t.Fatal("exec called before restriction"); return nil }
	restrictPaths := func([]string) error { t.Fatal("restriction called for malformed argv"); return nil }
	reported := runConfinedExec([]string{"/workspace", "/tool"}, restrictPaths, execProcess)
	if reported == nil || !strings.Contains(reported.Error(), "malformed internal invocation") {
		t.Fatalf("malformed report = %v", reported)
	}

	wantRestriction := errors.New("restriction failed")
	restrictPaths = func(roots []string) error {
		if !reflect.DeepEqual(roots, []string{"/workspace"}) {
			t.Fatalf("roots = %#v", roots)
		}
		return wantRestriction
	}
	execProcess = func(string, []string, []string) error { t.Fatal("exec called after restriction failure"); return nil }
	reported = runConfinedExec([]string{"/workspace", "--", "/tool", "tool"}, restrictPaths, execProcess)
	if !errors.Is(reported, wantRestriction) {
		t.Fatalf("restriction report = %v", reported)
	}

	wantExec := errors.New("exec failed")
	restrictPaths = func([]string) error { return nil }
	execProcess = func(path string, argv, env []string) error {
		if path != "/tool" || !reflect.DeepEqual(argv, []string{"tool", "arg"}) || len(env) == 0 {
			t.Fatalf("exec = %q %#v env=%d", path, argv, len(env))
		}
		return wantExec
	}
	reported = runConfinedExec([]string{"/workspace", "--", "/tool", "tool", "arg"}, restrictPaths, execProcess)
	if reported == nil || !errors.Is(reported, wantExec) || !strings.Contains(reported.Error(), `exec "/tool"`) {
		t.Fatalf("exec report = %v", reported)
	}

	roots, rest, ok := splitConfineArgv([]string{"one", "--", "two", "--", "three"})
	if !ok || !reflect.DeepEqual(roots, []string{"one"}) || !reflect.DeepEqual(rest, []string{"two", "--", "three"}) {
		t.Fatalf("first delimiter split = %#v %#v %v", roots, rest, ok)
	}
}
