package main

import (
	"io"
	"os"
	"testing"
)

const hermeticRootEnv = "WT_HERMETIC_ROOT"

// hermeticLifecycle is the ordered verb sequence the child drives against a real
// store under root — the full lifecycle FR-17 names (create → link → transition
// → search → close) and more, so the trace covers every code path a real tick
// exercises. Each step is `carina <verb> --root <root> …` with no program name,
// matching run's arg contract.
func hermeticLifecycle(root string) [][]string {
	r := func(args ...string) []string {
		return append([]string{args[0], "--root", root}, args[1:]...)
	}
	return [][]string{
		r("create", "--type", "epic", "--title", "The epic"),        // #1
		r("create", "--title", "Task A", "--body", "does a thing"),  // #2
		r("create", "--title", "Task B"),                            // #3
		r("signal", "add", "2", "agent"),                            // authorize #2
		r("parent", "set", "2", "1"),                                // #2 under the epic
		r("dep", "add", "3", "2"),                                   // #3 blocked_by #2 (link)
		r("comment", "2", "--body", "a note", "--author", "kit"),    // append log
		r("record", "2", "triage-disposition", "--body", "plan"),    // decision record
		r("edit", "2", "--title", "Task A (edited)"),                // field edit
		r("close", "3", "--reason", "completed", "--author", "kit"), // transition close
		r("reopen", "3", "--author", "kit"),                         // transition reopen
		r("list", "--state", "open"),                                // query: list/filter
		r("search", "Task"),                                         // query: search
		r("ready"),                                                  // predicate: ready set
		r("triage"),                                                 // predicate: triage set
		r("show", "1"),                                              // read with rollup
		r("close", "2", "--reason", "completed", "--author", "kit"), // final transition
	}
}

// TestHermeticLifecycleChild is shared by Linux's syscall trace and macOS's
// offline sandbox check. A non-zero verb result fails the child independently
// of the parent's network assertion. It is skipped in an ordinary test run.
func TestHermeticLifecycleChild(t *testing.T) {
	root := os.Getenv(hermeticRootEnv)
	if root == "" {
		t.Skip("child entrypoint for the hermetic and offline lifecycle checks")
	}
	for i, args := range hermeticLifecycle(root) {
		if code := run(args, io.Discard, io.Discard); code != 0 {
			t.Fatalf("lifecycle step %d %v exited %d", i, args, code)
		}
	}
}
