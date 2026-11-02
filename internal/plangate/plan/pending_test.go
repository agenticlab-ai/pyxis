package plan

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestFoldFileAtReadsAnUntaggedPendingLock(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	lockedGit(t, root, "tag", "-d", "lock/widget/v1")
	rel := "docs/plans/widget/001-plan.md"
	if got := lockedReasons(lockedFold(t, root, rel)); !lockedDefectContains(got, "cannot resolve Basis") {
		t.Fatalf("untagged Basis folded without a pending lock: %v", got)
	}
	res := FoldFileAt(root, config.Defaults(), rel, PendingBasis{Basis: "lock/widget/v1", Commit: head})
	if len(res.Defects) != 0 || len(res.JSON) == 0 {
		t.Fatalf("pending lock did not fold: %v", res.Defects)
	}
	other := FoldFileAt(root, config.Defaults(), rel, PendingBasis{Basis: "lock/widget/v9", Commit: head})
	if got := lockedReasons(other); !lockedDefectContains(got, "cannot resolve Basis") {
		t.Fatalf("a pending lock for another version resolved this plan's Basis: %v", got)
	}
}

func TestFoldFileAtReadsThePreparedCommitOverAStaleLocalTag(t *testing.T) {
	root := lockedPlanFixture(t)
	tagged := lockedGit(t, root, "rev-parse", "HEAD")
	lockedGit(t, root, "rm", "-r", "-q", "--", "memory/design/widget")
	lockedGit(t, root, "commit", "-q", "-m", "drop the tech spec")
	prepared := lockedGit(t, root, "rev-parse", "HEAD")
	if prepared == tagged {
		t.Fatal("fixture did not advance")
	}
	rel := "docs/plans/widget/001-plan.md"
	// The local tag still names the old, complete records; the prepared
	// commit, which the merge will lock, has no tech spec.
	if got := lockedReasons(FoldFileAt(root, config.Defaults(), rel, PendingBasis{Basis: "lock/widget/v1", Commit: prepared})); len(got) == 0 {
		t.Fatal("a stale local tag stood in for the prepared lock")
	}
	if res := FoldFileAt(root, config.Defaults(), rel, PendingBasis{Basis: "lock/widget/v9", Commit: prepared}); len(res.Defects) != 0 {
		t.Fatalf("a plan on a published lock read another pending lock: %v", res.Defects)
	}
}

func TestRebaseLiveMovesThePlanOntoThePendingLock(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	rel := "docs/plans/widget/001-plan.md"
	changed, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
	if err != nil || len(defects) != 0 {
		t.Fatalf("rebase failed: err=%v defects=%v", err, defects)
	}
	if !slices.Equal(changed, []string{rel, JSONPath(rel)}) {
		t.Fatalf("changed = %v", changed)
	}
	source, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil || !strings.Contains(string(source), "| **Basis** | lock/widget/v2 |") || strings.Contains(string(source), "lock/widget/v1") {
		t.Fatalf("Basis row not moved: err=%v\n%s", err, source)
	}
	face, err := os.ReadFile(filepath.Join(root, JSONPath(rel)))
	if err != nil || !strings.Contains(string(face), `"basis": "lock/widget/v2"`) {
		t.Fatalf("machine face not refolded: err=%v\n%s", err, face)
	}
	again, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
	if err != nil || len(defects) != 0 || len(again) != 0 {
		t.Fatalf("a second rebase changed files: changed=%v defects=%v err=%v", again, defects, err)
	}
}

func TestRebaseLiveRefusesAPlanTheContractNoLongerCarries(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	rel := "docs/plans/widget/001-plan.md"
	stale := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "design:widget#D-1", "design:widget#D-404", 1)
	lockedWrite(t, root, rel, stale)
	changed, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
	if err != nil || len(changed) != 0 || !lockedDefectContains(lockedReasons(Result{Defects: defects}), "D-404") {
		t.Fatalf("stale plan accepted: changed=%v defects=%v err=%v", changed, defects, err)
	}
	after, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil || string(after) != stale {
		t.Fatalf("refused rebase changed the plan: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, JSONPath(rel))); !os.IsNotExist(err) {
		t.Fatalf("refused rebase wrote a machine face: %v", err)
	}
}

func TestRebaseLiveIgnoresFeaturesWithoutALockedPlan(t *testing.T) {
	root := lockedPlanFixture(t)
	changed, defects, err := RebaseLive(root, config.Defaults(), "gadget", PendingBasis{Basis: "lock/gadget/v2", Commit: "HEAD"})
	if err != nil || len(defects) != 0 || len(changed) != 0 {
		t.Fatalf("feature without a plan: changed=%v defects=%v err=%v", changed, defects, err)
	}
	if _, ok := withBasis([]byte("# Plan\n\n| Field | Value |\n|---|---|\n| **PRD** | docs/prds/widget |\n\n## Tasks\n\n| **Basis** | lock/x/v1 |\n"), "lock/widget/v2"); ok {
		t.Fatal("a Basis row below the header was rewritten")
	}
}

func TestFoldFileAtReportsUnreadableAndInvalidSources(t *testing.T) {
	root := t.TempDir()
	pending := PendingBasis{Basis: "lock/widget/v2", Commit: "HEAD"}
	if res := FoldFileAt(root, config.Defaults(), "docs/plans/widget/001-plan.md", pending); len(res.Defects) == 0 {
		t.Fatal("missing plan folded")
	}
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", "not a plan\n")
	if res := FoldFileAt(root, config.Defaults(), "docs/plans/widget/001-plan.md", pending); len(res.Defects) == 0 || res.Doc != nil {
		t.Fatalf("invalid plan folded: %+v", res)
	}
}

func TestRebaseLiveSkipsAndRefusesWhatItCannotMove(t *testing.T) {
	cfg := config.Defaults()
	pending := PendingBasis{Basis: "lock/widget/v2", Commit: "HEAD"}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs/plans/widget"), 0o755); err != nil {
		t.Fatal(err)
	}
	if changed, defects, err := RebaseLive(root, cfg, "widget", pending); err != nil || len(defects) != 0 || len(changed) != 0 {
		t.Fatalf("empty plan directory: changed=%v defects=%v err=%v", changed, defects, err)
	}
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", "# Plan\n\n| Field | Value |\n|---|---|\n| **PRD** | docs/prds/widget |\n\n## Tasks\n")
	if changed, defects, err := RebaseLive(root, cfg, "widget", pending); err != nil || len(defects) != 0 || len(changed) != 0 {
		t.Fatalf("plan without a lock Basis was moved: changed=%v defects=%v err=%v", changed, defects, err)
	}
	notDir := t.TempDir()
	lockedWrite(t, notDir, "docs/plans/widget", "a file, not a directory\n")
	if _, _, err := RebaseLive(notDir, cfg, "widget", pending); err == nil {
		t.Fatal("an unreadable plan directory was skipped")
	}
}

func TestRebaseLiveReportsFilesItCannotWrite(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	pending := PendingBasis{Basis: "lock/widget/v2", Commit: head}
	rel := "docs/plans/widget/001-plan.md"
	if err := os.MkdirAll(filepath.Join(root, JSONPath(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if changed, defects, err := RebaseLive(root, config.Defaults(), "widget", pending); err != nil || len(changed) != 0 || !lockedDefectContains(lockedReasons(Result{Defects: defects}), "not a regular file") {
		t.Fatalf("a directory face was not refused: changed=%v defects=%v err=%v", changed, defects, err)
	}
	if err := os.Remove(filepath.Join(root, JSONPath(rel))); err != nil {
		t.Fatal(err)
	}
	md := filepath.Join(root, rel)
	if err := os.Chmod(md, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(md, 0o644) }) // let the temp directory clean up
	if _, _, err := RebaseLive(root, config.Defaults(), "widget", pending); err == nil || !strings.Contains(err.Error(), "write live plan") {
		t.Fatalf("read-only plan: %v", err)
	}
}

func TestRebaseLiveRefusesADraftPlanWithoutItsBasis(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	rel := "docs/plans/widget/001-plan.md"
	for _, tc := range []struct{ name, from, to string }{
		{"removed", "| **Basis** | lock/widget/v1 |\n", ""},
		{"broken", "| **Basis** | lock/widget/v1 |", "| Basis | lock/widget/v1 |"},
		{"removed beside a compact PRD row", "| **Basis** | lock/widget/v1 |\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), tc.from, tc.to, 1)
			if strings.Contains(tc.name, "compact") {
				source = strings.Replace(source, "| **PRD** | prd/widget |", "|**PRD**|prd/widget|", 1)
				if !strings.Contains(source, "|**PRD**|prd/widget|") {
					t.Fatal("fixture lacks its PRD row")
				}
			}
			if source == lockedPlanSource("001", "lock/widget/v1") {
				t.Fatal("fixture lacks its Basis row")
			}
			lockedWrite(t, root, rel, source)
			changed, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
			if err != nil || len(changed) != 0 || len(defects) == 0 {
				t.Fatalf("plan without its Basis: changed=%v defects=%v err=%v", changed, defects, err)
			}
		})
	}
}

func TestRebaseLiveRefusesPlanFacesAWriteCouldFollowOut(t *testing.T) {
	head := ""
	rel := "docs/plans/widget/001-plan.md"
	outside := filepath.Join(t.TempDir(), "victim.json")
	if err := os.WriteFile(outside, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{"linked folded face", func(t *testing.T, root string) {
			if err := os.Symlink(outside, filepath.Join(root, JSONPath(rel))); err != nil {
				t.Fatal(err)
			}
		}, "symbolic link"},
		{"linked plan directory", func(t *testing.T, root string) {
			elsewhere := t.TempDir()
			src := filepath.Join(root, "docs/plans/widget")
			if err := os.Rename(src, filepath.Join(elsewhere, "widget")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(elsewhere, "widget"), src); err != nil {
				t.Fatal(err)
			}
		}, "outside the repository"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := lockedPlanFixture(t)
			head = lockedGit(t, root, "rev-parse", "HEAD")
			tc.setup(t, root)
			changed, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
			if err != nil || len(changed) != 0 || !lockedDefectContains(lockedReasons(Result{Defects: defects}), tc.want) {
				t.Fatalf("changed=%v defects=%v err=%v", changed, defects, err)
			}
			if body, _ := os.ReadFile(outside); string(body) != "keep\n" {
				t.Fatalf("a write followed a link out of the worktree: %q", body)
			}
		})
	}
	if defects := regularFaces(filepath.Join(t.TempDir(), "missing"), rel); len(defects) == 0 {
		t.Fatal("an unresolvable repository was accepted")
	}
	if defects := regularFaces(t.TempDir(), rel); len(defects) == 0 {
		t.Fatal("an unresolvable plan directory was accepted")
	}
}

func TestRebaseLiveMovesABasisRowInAnySpacingTheParserAccepts(t *testing.T) {
	root := lockedPlanFixture(t)
	head := lockedGit(t, root, "rev-parse", "HEAD")
	rel := "docs/plans/widget/001-plan.md"
	source := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "| **Basis** | lock/widget/v1 |", "|**Basis**|  lock/widget/v1 |", 1)
	if source == lockedPlanSource("001", "lock/widget/v1") {
		t.Fatal("fixture lacks its Basis row")
	}
	lockedWrite(t, root, rel, source)
	if res := FoldFileAt(root, config.Defaults(), rel, PendingBasis{}); len(res.Defects) != 0 {
		t.Fatalf("the parser does not accept the spacing: %v", res.Defects)
	}
	changed, defects, err := RebaseLive(root, config.Defaults(), "widget", PendingBasis{Basis: "lock/widget/v2", Commit: head})
	if err != nil || len(defects) != 0 || len(changed) == 0 {
		t.Fatalf("changed=%v defects=%v err=%v", changed, defects, err)
	}
	if body, err := os.ReadFile(filepath.Join(root, rel)); err != nil || !strings.Contains(string(body), "| **Basis** | lock/widget/v2 |") || strings.Contains(string(body), "lock/widget/v1") {
		t.Fatalf("Basis not moved: err=%v\n%s", err, body)
	}
}
