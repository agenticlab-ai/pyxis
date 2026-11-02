package dispatch

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// A real tracker leaves Task.HeadBranch empty; the Dispatcher derives it. The
// derived branch must be deterministic (a respawn re-derives the same branch, so
// the re-open stays idempotent — AP-22) and unique per task via the tracker
// Number (two tasks never collide on one branch).
func TestHeadBranchFor(t *testing.T) {
	widget := port.Task{ID: "T1", Number: 2, Title: "Add the widget"}

	got := HeadBranch(widget)
	if want := "feat/t1-add-the-widget-2"; got != want {
		t.Fatalf("headBranchFor = %q, want %q", got, want)
	}

	// Deterministic: same task, same branch.
	if again := headBranchFor(widget); again != got {
		t.Errorf("headBranchFor not deterministic: %q then %q", got, again)
	}

	// Unique per tracker Number even when ID and Title collide.
	twin := widget
	twin.Number = 3
	if headBranchFor(twin) == got {
		t.Errorf("two tasks with the same ID/Title but different Number share a branch %q", got)
	}

	// Every derived branch is a feat/ ref with the number suffixed.
	for _, tk := range []port.Task{widget, twin} {
		b := headBranchFor(tk)
		if !strings.HasPrefix(b, "feat/") {
			t.Errorf("branch %q missing feat/ prefix", b)
		}
		if !strings.HasSuffix(b, "-"+strconv.Itoa(tk.Number)) {
			t.Errorf("branch %q missing -<number> suffix", b)
		}
	}

	// Degenerate charter (no ASCII-alnum in ID+Title) still yields a valid,
	// number-unique branch rather than a bare "feat/-<n>".
	if got := headBranchFor(port.Task{ID: "", Number: 7, Title: "！！！"}); got != "feat/task-7" {
		t.Errorf("degenerate headBranchFor = %q, want feat/task-7", got)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases and hyphenates words", "T1 Add the widget", "t1-add-the-widget"},
		{"drops punctuation", "Hello, World!", "hello-world"},
		{"trims leading and trailing separators", "  /leading-trailing/  ", "leading-trailing"},
		{"collapses runs of non-alnum to one hyphen", "multiple   spaces", "multiple-spaces"},
		{"keeps digits, lowercases letters", "UPPER_case-123", "upper-case-123"},
		{"drops non-ascii letters", "café", "caf"},
		{"all-symbols yields empty", "!!!", ""},
		{"single char", "a", "a"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slugify(tt.in); got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// slugify bounds length so a long title never yields an unwieldy branch, and the
// bounded result is still a clean slug (no trailing hyphen).
func TestSlugify_LengthBounded(t *testing.T) {
	long := strings.Repeat("word ", 40) // 200 chars of "word word word ..."
	got := slugify(long)
	if len(got) > 48 {
		t.Errorf("slug length = %d, want <= 48; got %q", len(got), got)
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("bounded slug %q has a dangling hyphen", got)
	}
}

// The derivation is wired into dispatch, not just a free function: a frontier
// task with an empty HeadBranch (the real-tracker case, #396) reaches the
// Workspace carrying the derived branch — so workspace.Prepare no longer fails
// closed with "no head branch".
func TestRunOne_DerivesHeadBranchForBareTask(t *testing.T) {
	tr := &fakeTracker{}
	ls := &fakeLease{claimErr: map[int]error{}}
	rt := &fakeRuntime{}
	hs := newFakeHost()
	ws := &branchRecordingWorkspace{dir: t.TempDir()}

	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// A frontier task with NO head branch — exactly what ghtracker/carinatracker
	// return today.
	bare := port.Task{ID: "T1", Number: 2, Title: "Add the widget", Brief: "charter", Harness: "claude"}
	tr.setFrontier(bare)

	if _, err := d.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	got := ws.branchFor(2)
	if want := "feat/t1-add-the-widget-2"; got != want {
		t.Fatalf("workspace received branch %q, want the derived %q", got, want)
	}
	if got == "" {
		t.Fatal("workspace received an empty head branch — the derivation did not wire through")
	}
}

// A task that already carries a head branch is left untouched — the derivation
// only fills a gap, never overrides an explicit branch.
func TestRunOne_KeepsExplicitHeadBranch(t *testing.T) {
	tr := &fakeTracker{}
	ls := &fakeLease{claimErr: map[int]error{}}
	rt := &fakeRuntime{}
	hs := newFakeHost()
	ws := &branchRecordingWorkspace{dir: t.TempDir()}

	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pinned := port.Task{ID: "T9", Number: 5, Title: "whatever", Brief: "c", Harness: "claude", HeadBranch: "feat/explicit-branch"}
	if got := HeadBranch(pinned); got != pinned.HeadBranch {
		t.Fatalf("proposal recovery branch %q differs from explicit branch %q", got, pinned.HeadBranch)
	}
	tr.setFrontier(pinned)

	if _, err := d.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := ws.branchFor(5); got != "feat/explicit-branch" {
		t.Fatalf("explicit head branch overwritten: got %q, want feat/explicit-branch", got)
	}
}

// branchRecordingWorkspace is a port.Workspace that records the HeadBranch each
// task reached Prepare with, so a test can assert the Dispatcher's derivation
// flowed all the way into the Worker's workspace step.
type branchRecordingWorkspace struct {
	dir      string
	mu       sync.Mutex
	branches map[int]string
}

func (w *branchRecordingWorkspace) Prepare(_ context.Context, task port.Task) (port.Workdir, error) {
	w.mu.Lock()
	if w.branches == nil {
		w.branches = map[int]string{}
	}
	w.branches[task.Number] = task.HeadBranch
	w.mu.Unlock()
	return &fakeWorkdir{dir: w.dir, branch: task.HeadBranch}, nil
}

func (w *branchRecordingWorkspace) branchFor(number int) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.branches[number]
}
