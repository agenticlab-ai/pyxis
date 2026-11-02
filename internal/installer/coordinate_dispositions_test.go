package installer_test

// The impl-review #809 disposition evidence (its report on the PR): the
// sink-validation and containment behaviors FND-1 demanded, the
// journalRemote unwind leg CAL-12 found unasserted, and the create-from-
// bare scenario CAL-2 found unevidenced.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// TestCoordinateWiringFreeKnobsIgnoredAtSink: a coordinate beside knobs
// that declare no gh-transport operation is inert — it must neither reach
// git argv (FND-1's unvalidated path) nor fail the run; the install stays
// green and no remote is materialized.
func TestCoordinateWiringFreeKnobsIgnoredAtSink(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target)
	opts.Answers.Tracker = "carina"
	opts.Answers.WiringRepo = "use"
	opts.Answers.WiringProtection = "none"
	opts.Answers.Coordinate = "not a coordinate!!" // hostile; must never reach argv
	opts.Answers.RemoteURL = filepath.Join(r.src, "local.git")

	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(rep.Commits) == 0 {
		t.Fatal("not a green install")
	}
	if url, gerr := gitIn(t, target, "remote", "get-url", "origin"); gerr != nil || strings.TrimSpace(url) != opts.Answers.RemoteURL {
		t.Errorf("publication destination = %q, %v; hostile coordinate must stay inert", url, gerr)
	}
}

// TestCoordinateNestedTargetNotShadowed: a target nested inside another
// repository must resolve its OWN coordinate — the enclosing repository's
// remote (git's upward discovery) may never shadow the supplied one
// (FND-1's second path; the RemoteSlug containment guard).
func TestCoordinateNestedTargetNotShadowed(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)

	parent := t.TempDir()
	mustGit(t, parent, "init", "-b", "main")
	mustGit(t, parent, "remote", "add", "origin", wire.RemoteURL("acme/parent"))
	target := filepath.Join(parent, "child")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	opts := r.baseOptions(target) // promptless GitHub mode, Coordinate=fixtureCoordinate
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(rep.Commits) == 0 {
		t.Fatal("not a green install")
	}
	url, gerr := gitIn(t, target, "remote", "get-url", "origin")
	if gerr != nil {
		t.Fatalf("target remote get-url: %v", gerr)
	}
	if strings.TrimSpace(url) != wire.RemoteURL(fixtureCoordinate) {
		t.Errorf("target remote = %q, want %q — the parent's remote shadowed the supplied coordinate", strings.TrimSpace(url), wire.RemoteURL(fixtureCoordinate))
	}
	purl, _ := gitIn(t, parent, "remote", "get-url", "origin")
	if strings.TrimSpace(purl) != wire.RemoteURL("acme/parent") {
		t.Errorf("parent remote changed to %q — init must never touch the enclosing repository", strings.TrimSpace(purl))
	}
}

// TestRollbackRemovesMaterializedRemote is CAL-12's missing assertion: on a
// PRE-EXISTING (metadata-only) repository, a post-materialization failure
// unwinds the journalRemote entry — the repository survives, the remote
// this run added does not.
func TestRollbackRemovesMaterializedRemote(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	mustGit(t, target, "init", "-b", "main") // pre-existing empty repo, no remote

	restore := installer.SetFailingStageForTest("seed") // the first injectable slot after the remote stage
	defer restore()
	opts := r.baseOptions(target)
	_, err := installer.Init(context.Background(), opts)
	if err == nil {
		t.Fatal("Init = nil error, want the injected converge failure")
	}
	if _, serr := os.Stat(filepath.Join(target, ".git")); serr != nil {
		t.Fatalf("pre-existing repository removed by rollback: %v", serr)
	}
	if out, gerr := gitIn(t, target, "remote", "get-url", "origin"); gerr == nil {
		t.Errorf("materialized remote survived the unwind: %s", strings.TrimSpace(out))
	}
}

// TestCreateFromBareEndsGreen is CAL-2's missing evidence: wiring
// repo=create on a bare directory, promptless coordinate — the whole
// bootstrap this task exists for — ends green with the remote configured
// and the create-repo operation in the wiring audit.
func TestCreateFromBareEndsGreen(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target)
	opts.Answers.WiringRepo = "create"
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(rep.Commits) == 0 {
		t.Fatal("not a green install")
	}
	url, gerr := gitIn(t, target, "remote", "get-url", "origin")
	if gerr != nil || strings.TrimSpace(url) != wire.RemoteURL(fixtureCoordinate) {
		t.Errorf("remote = %q (err %v), want %q", strings.TrimSpace(url), gerr, wire.RemoteURL(fixtureCoordinate))
	}
	if rep.T8.Wiring == nil {
		t.Fatal("no wiring audit on the report")
	}
	var sawCreate bool
	for _, op := range rep.T8.Wiring.Operations {
		if op.Name == wire.OpCreateRepo {
			sawCreate = true
		}
	}
	if !sawCreate {
		t.Errorf("wiring audit %v carries no create-repo op for %s", rep.T8.Wiring.OperationNames(), fixtureCoordinate)
	}
}
