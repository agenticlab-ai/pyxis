package opencode

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestOSCommanderRunsAndAwaitsRealProcesses(t *testing.T) {
	c := osCommander{}
	env := append(os.Environ(), "PYXIS_EXEC_TEST=env")
	out, code, err := c.run(t.Context(), commandSpec{
		Name: "sh", Args: []string{"-c", `read line; printf '%s:%s' "$PYXIS_EXEC_TEST" "$line"`},
		Dir: t.TempDir(), Env: env, Stdin: "payload\n",
	})
	if err != nil || code != 0 || string(out) != "env:payload" {
		t.Fatalf("run success = %q code=%d err=%v", out, code, err)
	}
	out, code, err = c.run(t.Context(), commandSpec{Name: "sh", Args: []string{"-c", "printf failed; exit 7"}})
	if err != nil || code != 7 || string(out) != "failed" {
		t.Fatalf("run nonzero = %q code=%d err=%v", out, code, err)
	}
	if _, code, err := c.run(t.Context(), commandSpec{Name: "pyxis-command-that-does-not-exist"}); err == nil || code != -1 {
		t.Fatalf("missing run code=%d err=%v", code, err)
	}
	p, err := c.start(t.Context(), commandSpec{
		Name: "sh", Args: []string{"-c", `read line; printf '%s:%s' "$PYXIS_EXEC_TEST" "$line"; exit 3`},
		Dir: t.TempDir(), Env: env, Stdin: "started\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	out, code, err = p.wait()
	if err != nil || code != 3 || string(out) != "env:started" {
		t.Fatalf("wait = %q code=%d err=%v", out, code, err)
	}
	if err := p.kill(); err != nil {
		t.Fatalf("kill completed process: %v", err)
	}
	if _, err := c.start(t.Context(), commandSpec{Name: "pyxis-command-that-does-not-exist"}); err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("missing start error=%v", err)
	}
}

func TestOSProcessReportsSignalTerminationAsOperational(t *testing.T) {
	p, err := (osCommander{}).start(context.Background(), commandSpec{Name: "sh", Args: []string{"-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.kill(); err != nil {
		t.Fatal(err)
	}
	if _, code, err := p.wait(); err == nil || code != -1 {
		t.Fatalf("signalled wait code=%d err=%v", code, err)
	}
}
