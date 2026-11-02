package tracker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCarina compiles a carina binary once for the test and returns its path.
func buildCarina(t *testing.T) string {
	t.Helper()
	root := repoRootForTest(t)
	bin := filepath.Join(t.TempDir(), "carina")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/carina")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build carina: %v\n%s", err, out)
	}
	return bin
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test dir")
		}
		dir = parent
	}
}

// A-1: the Carina adapter passes the shared Tracker contract (hermetic — a
// real carina store, no network). The gh adapter satisfies the same contract
// against a live repo; the contract is adapter-agnostic by construction.
func TestCarina_TrackerContract(t *testing.T) {
	c := NewCarina(t.TempDir(), buildCarina(t))
	runTrackerContract(t, c)
}

// A-4 / A-5 / DINV-3 / DINV-4: the label→signal/type mapping and item identity.
func TestCarina_LabelMappingAndIdentity(t *testing.T) {
	c := NewCarina(t.TempDir(), buildCarina(t))

	id, err := c.CreateIssue("X", "body", []string{"epic", "local-agent", "kind:feat", "mechanical"}, "epic")
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	v, err := c.show(id)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	sigs := "," + strings.Join(v.Signals, ",") + ","

	if strings.Contains(sigs, ",epic,") {
		t.Errorf("epic label not elided (the type carries it): %v", v.Signals)
	}
	if v.Type != "epic" {
		t.Errorf("type = %q, want epic", v.Type)
	}
	if !strings.Contains(sigs, ",agent,") {
		t.Errorf("local-agent not mapped to agent: %v", v.Signals)
	}
	if !strings.Contains(sigs, ",kind:feat,") {
		t.Errorf("kind:feat not passed through: %v", v.Signals)
	}
	if !strings.Contains(sigs, ",kind:mechanical,") {
		t.Errorf("unmapped label mechanical not applied as a namespaced classification signal: %v", v.Signals)
	}
	if v.ID != id {
		t.Errorf("identity: show id %d != created id %d", v.ID, id)
	}

	// DINV-3: mechanical was defined on the classification axis, never widening
	// the engine-recognized set.
	vocab, err := c.vocabulary()
	if err != nil {
		t.Fatalf("vocabulary: %v", err)
	}
	if !vocab["kind:mechanical"] {
		t.Errorf("kind:mechanical not defined in the vocabulary")
	}
}

// A plan changed in place withdraws a routing signal and a dependency edge.
func TestCarina_WithdrawsASignalAndAnEdge(t *testing.T) {
	c := NewCarina(t.TempDir(), buildCarina(t))
	blocker, err := c.CreateIssue("blocker", "body", nil, "task")
	if err != nil {
		t.Fatal(err)
	}
	task, err := c.CreateIssue("task", "body", []string{"local-agent"}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddBlockedBy(task, blocker); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveBlockedBy(task, blocker); err != nil {
		t.Fatal(err)
	}
	if edges, err := c.BlockedBy(task); err != nil || len(edges) != 0 {
		t.Fatalf("edges after removal = %v, %v", edges, err)
	}
	if err := c.RemoveLabel(task, "local-agent"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveLabel(task, "epic"); err != nil {
		t.Fatalf("an elided label was not a no-op: %v", err)
	}
	v, err := c.show(task)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range v.Signals {
		if s == "agent" {
			t.Fatalf("routing signal kept: %v", v.Signals)
		}
	}
}
