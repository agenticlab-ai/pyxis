package adopt_test

// Hermetic tests for the workflow-filename collision resolution (T6;
// prd:existing-repo-onboarding/FR-5; design:existing-repo-onboarding-v2#3.3,
// #A-5): focused unit tests over ResolveWorkflowEntries/ApplyWorkflowEntry
// (no release fixture needed — they operate on entries/bytes directly), plus
// an end-to-end pair through Inspect proving the collision and no-collision
// legs both land the right entry in the plan. Fixture scaffolding
// (writeFile, newRelease, nonEmptyAdopterRepo, adoptFiles, git, snapshot,
// quiet, the pf type) is inspect_test.go's — same package, reused here.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// ---- DistinctWorkflowPath ---------------------------------------------------

func TestDistinctWorkflowPath(t *testing.T) {
	got := adopt.DistinctWorkflowPath()
	want := ".github/workflows/pyxis-docs-gate.yml"
	if got != want {
		t.Errorf("DistinctWorkflowPath() = %q, want %q", got, want)
	}
	if got == payload.InstalledWorkflowTarget {
		t.Fatalf("DistinctWorkflowPath() = %q, must not equal the fixed target %q", got, payload.InstalledWorkflowTarget)
	}
}

// ---- ResolveWorkflowEntries -------------------------------------------------

// TestResolveWorkflowEntriesNoCollision proves that absent any file at the
// fixed target, the ordinary generated-class entry entriesFromFindings
// already produced passes through unchanged (T3's baseline case).
func TestResolveWorkflowEntriesNoCollision(t *testing.T) {
	root := t.TempDir()
	in := []adopt.Entry{
		{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
		{Path: payload.InstalledWorkflowTarget, Class: adopt.ClassGenerated},
	}

	got, err := adopt.ResolveWorkflowEntries(root, in)
	if err != nil {
		t.Fatalf("ResolveWorkflowEntries: %v", err)
	}
	want := []adopt.Entry{
		{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
		{Path: payload.InstalledWorkflowTarget, Class: adopt.ClassGenerated},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveWorkflowEntries (no collision) = %+v, want unchanged %+v", got, want)
	}
}

// TestResolveWorkflowEntriesCollision proves a file already at the fixed
// target redirects the plan entry to DistinctWorkflowPath, names the
// collision in Detail, leaves every other entry untouched, and — critically
// — leaves no entry anywhere naming the colliding fixed target (T6
// acceptance; design #A-5: apply only ever touches a path an entry names).
func TestResolveWorkflowEntriesCollision(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)), []byte("name: my-own-ci\non: [push]\n"))
	in := []adopt.Entry{
		{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
		{Path: payload.InstalledWorkflowTarget, Class: adopt.ClassGenerated},
	}

	got, err := adopt.ResolveWorkflowEntries(root, in)
	if err != nil {
		t.Fatalf("ResolveWorkflowEntries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ResolveWorkflowEntries = %+v, want 2 entries", got)
	}
	if got[0] != in[0] {
		t.Errorf("unrelated entry mutated: got %+v, want %+v", got[0], in[0])
	}

	distinct := adopt.DistinctWorkflowPath()
	if got[1].Path != distinct {
		t.Errorf("redirected entry path = %q, want %q", got[1].Path, distinct)
	}
	if got[1].Class != adopt.ClassGenerated {
		t.Errorf("redirected entry class = %s, want %s", got[1].Class, adopt.ClassGenerated)
	}
	if !strings.Contains(got[1].Detail, payload.InstalledWorkflowTarget) {
		t.Errorf("redirected entry detail = %q, want it to name the colliding target %q", got[1].Detail, payload.InstalledWorkflowTarget)
	}
	for _, e := range got {
		if e.Path == payload.InstalledWorkflowTarget {
			t.Errorf("entries = %+v, want no entry left at the colliding fixed target %q", got, payload.InstalledWorkflowTarget)
		}
	}
}

// TestResolveWorkflowEntriesInventsNothingWithoutAFinding proves the
// redirect never fabricates an entry: a file at the fixed target with no
// classifier-produced entry naming it (an incoherent manifest that does not
// list the installed-workflow path) leaves entries exactly as given.
func TestResolveWorkflowEntriesInventsNothingWithoutAFinding(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)), []byte("name: my-own-ci\n"))
	in := []adopt.Entry{{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd}}

	got, err := adopt.ResolveWorkflowEntries(root, in)
	if err != nil {
		t.Fatalf("ResolveWorkflowEntries: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Errorf("ResolveWorkflowEntries = %+v, want unchanged %+v", got, in)
	}
}

// ---- ApplyWorkflowEntry -----------------------------------------------------

// TestApplyWorkflowEntryWritesDesiredBytesAndLeavesTheColliderUntouched
// proves apply's landing half: writing the resolved entry's bytes at its
// (distinct) path never touches whatever already sits at the fixed target
// (T6 acceptance; prd:existing-repo-onboarding/FR-5 — "both workflows
// present").
func TestApplyWorkflowEntryWritesDesiredBytesAndLeavesTheColliderUntouched(t *testing.T) {
	root := t.TempDir()
	original := []byte("name: my-own-ci\non: [push]\n")
	writeFile(t, filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)), original)

	distinct := adopt.DistinctWorkflowPath()
	entry := adopt.Entry{Path: distinct, Class: adopt.ClassGenerated}
	desired := []byte("name: docs-gate\non: [push]\njobs:\n  docs-gate: {}\n")

	actions, refusals, err := adopt.ApplyWorkflowEntry(context.Background(), root, entry, desired)
	if err != nil {
		t.Fatalf("ApplyWorkflowEntry: %v", err)
	}
	if len(refusals) != 0 {
		t.Fatalf("refusals = %+v, want none", refusals)
	}
	if len(actions) != 1 || actions[0].Path != distinct || actions[0].Class != relmanifest.Generated {
		t.Fatalf("actions = %+v, want one generated-class action at %q", actions, distinct)
	}

	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(distinct)))
	if err != nil {
		t.Fatalf("read distinct path: %v", err)
	}
	if !bytes.Equal(got, desired) {
		t.Errorf("distinct path content = %q, want %q", got, desired)
	}

	stillThere, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)))
	if err != nil {
		t.Fatalf("read colliding target: %v", err)
	}
	if !bytes.Equal(stillThere, original) {
		t.Errorf("the adopter's colliding workflow changed: got %q, want %q", stillThere, original)
	}
}

// TestApplyWorkflowEntryRejectsNonGeneratedClass proves a caller error (an
// entry of the wrong class) is a named refusal, never a silent no-op —
// converge.WriteGenerated itself would just skip it quietly.
func TestApplyWorkflowEntryRejectsNonGeneratedClass(t *testing.T) {
	root := t.TempDir()
	entry := adopt.Entry{Path: payload.InstalledWorkflowTarget, Class: adopt.ClassNeverTouch}

	_, _, err := adopt.ApplyWorkflowEntry(context.Background(), root, entry, []byte("x"))
	if err == nil {
		t.Fatal("ApplyWorkflowEntry: err = nil, want a named refusal for a non-generated entry")
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(entry.Path))); !os.IsNotExist(statErr) {
		t.Error("ApplyWorkflowEntry wrote a file despite refusing the entry")
	}
}

// ---- End-to-end through Inspect ---------------------------------------------

// TestInspectRedirectsCollidingWorkflowToDistinctFilename is T6's headline
// acceptance at the plan layer: on a fixture whose manifest lists the
// installed-workflow path and whose adopter tree already carries a file
// there, Inspect's plan names the distinct path (never the colliding fixed
// target), and the collider is byte-identical after Inspect runs.
func TestInspectRedirectsCollidingWorkflowToDistinctFilename(t *testing.T) {
	files := append(adoptFiles(), pf{path: payload.InstalledWorkflowTarget, class: relmanifest.Generated})
	r := newRelease(t, files)
	root := nonEmptyAdopterRepo(t)
	writeFile(t, filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)), []byte("name: my-own-ci\non: [push]\n"))
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "add my own CI workflow")
	before := snapshot(t, root)

	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	distinct := adopt.DistinctWorkflowPath()
	var sawDistinct, sawFixed bool
	for _, e := range rep.Plan.Entries {
		switch e.Path {
		case distinct:
			sawDistinct = true
			if e.Class != adopt.ClassGenerated {
				t.Errorf("distinct entry class = %s, want %s", e.Class, adopt.ClassGenerated)
			}
			if !strings.Contains(e.Detail, payload.InstalledWorkflowTarget) {
				t.Errorf("distinct entry detail = %q, want it to name %q", e.Detail, payload.InstalledWorkflowTarget)
			}
		case payload.InstalledWorkflowTarget:
			sawFixed = true
		}
	}
	if !sawDistinct {
		t.Errorf("Entries = %+v, want an entry at the distinct path %q", rep.Plan.Entries, distinct)
	}
	if sawFixed {
		t.Errorf("Entries = %+v, want no entry naming the colliding fixed target %q", rep.Plan.Entries, payload.InstalledWorkflowTarget)
	}

	after := snapshot(t, root)
	if before[payload.InstalledWorkflowTarget] != after[payload.InstalledWorkflowTarget] {
		t.Error("the adopter's colliding workflow changed during Inspect (never-touch violated)")
	}
}

// TestInspectKeepsFixedWorkflowTargetWithoutACollision proves the
// no-collision leg is unregressed: with no file at the fixed target, the
// plan still names it plainly (T3's baseline), with no collision Detail.
func TestInspectKeepsFixedWorkflowTargetWithoutACollision(t *testing.T) {
	files := append(adoptFiles(), pf{path: payload.InstalledWorkflowTarget, class: relmanifest.Generated})
	r := newRelease(t, files)
	root := nonEmptyAdopterRepo(t) // carries no .github/workflows at all

	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	var sawFixed bool
	for _, e := range rep.Plan.Entries {
		if e.Path == payload.InstalledWorkflowTarget {
			sawFixed = true
			if e.Class != adopt.ClassGenerated {
				t.Errorf("entry class = %s, want %s", e.Class, adopt.ClassGenerated)
			}
			if e.Detail != "" {
				t.Errorf("entry detail = %q, want empty (no collision to explain)", e.Detail)
			}
		}
		if e.Path == adopt.DistinctWorkflowPath() {
			t.Errorf("entries = %+v, want no distinct-path entry absent a collision", rep.Plan.Entries)
		}
	}
	if !sawFixed {
		t.Errorf("Entries = %+v, want an entry at the fixed target %q", rep.Plan.Entries, payload.InstalledWorkflowTarget)
	}
}
