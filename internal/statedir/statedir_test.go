package statedir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootHonorsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	got, err := Root()
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if want := filepath.Join("/custom/state", "pyxis"); got != want {
		t.Errorf("root = %s, want %s", got, want)
	}
}

func TestRootRejectsRelativeXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "checkout-local-state")
	if _, err := Root(); err == nil {
		t.Fatal("relative XDG_STATE_HOME was accepted — machine-shared state would depend on cwd")
	}
}

func TestRootFallsBackToTheHomeConvention(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	got, err := Root()
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if want := filepath.Join(home, ".local", "state", "pyxis"); got != want {
		t.Errorf("root = %s, want %s", got, want)
	}
}

func TestRootRejectsRelativeHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "relative-home")
	if _, err := Root(); err == nil {
		t.Fatal("relative home was accepted — machine-shared state would depend on cwd")
	}
}

// Sub is the form every named area uses, so no consumer re-derives the root
// and no two of them can disagree about where machine state lives (AP-11).
func TestSubJoinsBeneathTheRoot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	got, err := Sub("content-budget", "github.com", "octocat.json")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	want := filepath.Join("/custom/state", "pyxis", "content-budget", "github.com", "octocat.json")
	if got != want {
		t.Errorf("sub = %s, want %s", got, want)
	}
	root, _ := Root()
	if bare, err := Sub(); err != nil || bare != root {
		t.Errorf("Sub() = %s (err %v), want the root %s", bare, err, root)
	}
}
