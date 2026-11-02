package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The T10 survival proof — the work-item store is bucket-1 operand content that
// outlives the factory (docs/tech-design/local-work-tracker.md §3.8, DINV-8,
// A-9; repo-layout-and-system-footprint#DINV-1). Deleting the whole `.pyxis/`
// footprint (what `pyxis remove` deletes) leaves every work item under
// `.workitems/` readable and the store fully operational. The proof drives the
// real Carina CLI both before and after the deletion.

// carina runs the CLI in-process (run is the binary's entrypoint) and returns
// its exit code plus captured stdout/stderr, so a test asserts on both the code
// and the human/JSON output the operator would see.
func carina(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestStoreSurvivesFactoryRemoval(t *testing.T) {
	root := t.TempDir()

	// A realistic factory footprint under .pyxis/ — a config plus some sibling
	// state, exactly what an install leaves and `pyxis remove` deletes.
	pyxisDir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(filepath.Join(pyxisDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pyxisDir, "config.yaml"), "paths:\n  work_items: .workitems\n")
	writeFile(t, filepath.Join(pyxisDir, "state.json"), "{}\n")

	// Build a small backlog through the CLI: an epic, an authorized task with a
	// comment and a decision-record, and a second task blocked on the first.
	mustCarina(t, "create", "--root", root, "--type", "epic", "--title", "The epic")   // #1
	mustCarina(t, "create", "--root", root, "--title", "Ship it", "--body", "do work") // #2
	mustCarina(t, "create", "--root", root, "--title", "Follows")                      // #3
	mustCarina(t, "signal", "add", "--root", root, "2", "agent")
	mustCarina(t, "parent", "set", "--root", root, "2", "1")
	mustCarina(t, "dep", "add", "--root", root, "3", "2")
	mustCarina(t, "comment", "--root", root, "2", "--body", "a durable note")
	mustCarina(t, "record", "--root", root, "2", "triage-disposition", "--body", "plan")

	// The store lives under .workitems/, outside .pyxis/.
	storeDir := filepath.Join(root, ".workitems")
	if _, err := os.Stat(storeDir); err != nil {
		t.Fatalf("store not at bucket-1 .workitems/: %v", err)
	}

	// Delete the entire factory footprint — the survival event.
	if err := os.RemoveAll(pyxisDir); err != nil {
		t.Fatalf("remove .pyxis footprint: %v", err)
	}
	if _, err := os.Stat(pyxisDir); !os.IsNotExist(err) {
		t.Fatalf(".pyxis footprint not gone: %v", err)
	}

	// Every work item is still readable, whole, and correct — no config needed
	// (an absent .pyxis/config.yaml falls back to the default .workitems path —
	// the same root the explicit pin used, so a pinned-default store stays
	// addressable after removal; a pinned NON-default root would not: the named
	// residual in local-work-tracker-v2 §1).
	for _, want := range []struct {
		id    string
		title string
	}{{"1", "The epic"}, {"2", "Ship it"}, {"3", "Follows"}} {
		code, stdout, stderr := carina("show", "--root", root, want.id)
		if code != 0 {
			t.Fatalf("show #%s after removal exited %d: %s", want.id, code, stderr)
		}
		if !strings.Contains(stdout, want.title) {
			t.Fatalf("show #%s lost its title %q after removal:\n%s", want.id, want.title, stdout)
		}
	}

	// The durable log survived: #2 still carries its comment and decision-record.
	if _, stdout, _ := carina("show", "--root", root, "2"); !strings.Contains(stdout, "a durable note") {
		t.Fatalf("comment lost after removal:\n%s", stdout)
	}

	// The kit stays green: the store is fully operational after removal — reads,
	// queries, predicates, and a fresh mutation all still work.
	assertGreen(t, root)
}

// assertGreen exercises a representative slice of the surface post-removal and
// asserts every verb succeeds — the store is not merely readable but live.
func assertGreen(t *testing.T, root string) {
	t.Helper()
	// The ready set still computes and still names the authorized, unblocked #2.
	code, stdout, stderr := carina("ready", "--root", root)
	if code != 0 {
		t.Fatalf("ready after removal exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "#2") {
		t.Fatalf("ready set wrong after removal (want #2):\n%s", stdout)
	}
	// Triage, list, and search all still answer.
	mustCarina(t, "triage", "--root", root)
	mustCarina(t, "list", "--root", root, "--state", "open")
	if code, stdout, _ := carina("search", "--root", root, "durable"); code != 0 || !strings.Contains(stdout, "#2") {
		t.Fatalf("search after removal did not find #2:\n%s", stdout)
	}
	// A fresh write lands and reads back — the store still grows.
	mustCarina(t, "create", "--root", root, "--title", "Born after removal") // #4
	if code, stdout, _ := carina("show", "--root", root, "4"); code != 0 || !strings.Contains(stdout, "Born after removal") {
		t.Fatalf("post-removal create did not round-trip:\n%s", stdout)
	}
	mustCarina(t, "edit", "--root", root, "4", "--title", "Renamed after removal")
	if _, stdout, _ := carina("show", "--root", root, "4"); !strings.Contains(stdout, "Renamed after removal") {
		t.Fatalf("post-removal edit did not take:\n%s", stdout)
	}
}

func mustCarina(t *testing.T, args ...string) string {
	t.Helper()
	code, stdout, stderr := carina(args...)
	if code != 0 {
		t.Fatalf("carina %s exited %d: %s", strings.Join(args, " "), code, stderr)
	}
	return stdout
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
