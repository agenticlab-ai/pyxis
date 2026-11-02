package reconcile

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestBuildSnapshot_ReadsOnlyThePinnedCommit(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeSnapshotFile(t, dir, "README.md", []byte("old source\n"), 0o644)
	writeSnapshotFile(t, dir, "internal/widget/widget.go", []byte("package widget\n\nconst Version = 1\n"), 0o644)
	oldCommit := commitSnapshot(t, dir, "old")

	writeSnapshotFile(t, dir, "README.md", []byte("new source\n"), 0o644)
	writeSnapshotFile(t, dir, "internal/widget/widget.go", []byte("package widget\n\nconst Version = 2\n"), 0o644)
	newCommit := commitSnapshot(t, dir, "new")
	writeSnapshotFile(t, dir, "README.md", []byte("dirty working tree\n"), 0o644)
	writeSnapshotFile(t, dir, "docs/wiki/untracked.md", []byte("untracked\n"), 0o644)

	repo := openSnapshotRepo(t, dir)
	old, err := BuildSnapshot(context.Background(), repo, oldCommit, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot(old): %v", err)
	}
	if old.Commit != oldCommit || string(old.Files["README.md"].Body) != "old source\n" {
		t.Fatalf("old snapshot identity/body = %s %q", old.Commit, old.Files["README.md"].Body)
	}
	if _, ok := old.Files["docs/wiki/untracked.md"]; ok {
		t.Fatal("snapshot read an untracked working-tree file")
	}
	wantOldInventory, err := codeinventory.Build(
		[]codeinventory.Entry{{Path: "internal/widget/widget.go", Mode: "100644", Type: "blob"}},
		map[string][]byte{"internal/widget/widget.go": []byte("package widget\n\nconst Version = 1\n")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if old.CodeInventorySHA256 != wantOldInventory.AggregateSHA256 {
		t.Fatalf("old code inventory = %s, want %s", old.CodeInventorySHA256, wantOldInventory.AggregateSHA256)
	}

	current, err := BuildSnapshot(context.Background(), repo, "HEAD", Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot(HEAD): %v", err)
	}
	if current.Commit != newCommit || string(current.Files["README.md"].Body) != "new source\n" {
		t.Fatalf("HEAD snapshot identity/body = %s %q", current.Commit, current.Files["README.md"].Body)
	}
	if bytes.Equal(old.Files["README.md"].Body, current.Files["README.md"].Body) || old.CodeInventorySHA256 == current.CodeInventorySHA256 {
		t.Fatal("distinct pinned commits collapsed to the same source/code identities")
	}
}

func TestBuildSnapshot_DeterministicIndependentClosures(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeSnapshotFile(t, dir, "docs/wiki/z.md", []byte("z\n"), 0o644)
	writeSnapshotFile(t, dir, "README.md", []byte("readme\n"), 0o644)
	writeSnapshotFile(t, dir, "docs/wiki/a.md", []byte("a\n"), 0o644)
	writeSnapshotFile(t, dir, "internal/z/z.go", []byte("package z\n"), 0o644)
	writeSnapshotFile(t, dir, "internal/a/a.go", []byte("package a\n"), 0o644)
	writeSnapshotFile(t, dir, ".pyxis/scripts/tool.sh", []byte("#!/bin/sh\nexit 0\n"), 0o755)
	baseCommit := commitSnapshot(t, dir, "base")
	repo := openSnapshotRepo(t, dir)

	first := mustBuildSnapshot(t, repo, baseCommit, Options{})
	second := mustBuildSnapshot(t, repo, baseCommit, Options{})
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical pinned inputs produced different snapshots")
	}
	if !slices.IsSorted(first.AuthoredSources) || !slices.IsSortedFunc(first.Code, func(a, b codeinventory.Unit) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		t.Fatalf("snapshot outputs are not canonical: sources %v code %#v", first.AuthoredSources, first.Code)
	}

	writeSnapshotFile(t, dir, "notes.txt", []byte("not authored or executable\n"), 0o644)
	unrelated := mustBuildSnapshot(t, repo, commitSnapshot(t, dir, "unrelated"), Options{})
	if unrelated.SourceClosureSHA256 != first.SourceClosureSHA256 || unrelated.CodeInventorySHA256 != first.CodeInventorySHA256 {
		t.Fatal("an unrelated file changed a source or code closure")
	}

	writeSnapshotFile(t, dir, "README.md", []byte("source changed\n"), 0o644)
	sourceChanged := mustBuildSnapshot(t, repo, commitSnapshot(t, dir, "source"), Options{})
	if sourceChanged.SourceClosureSHA256 == unrelated.SourceClosureSHA256 || sourceChanged.CodeInventorySHA256 != unrelated.CodeInventorySHA256 {
		t.Fatal("source edit did not change only the source closure")
	}

	writeSnapshotFile(t, dir, "internal/a/a.go", []byte("package a\n\nconst Changed = true\n"), 0o644)
	codeChanged := mustBuildSnapshot(t, repo, commitSnapshot(t, dir, "code"), Options{})
	if codeChanged.SourceClosureSHA256 != sourceChanged.SourceClosureSHA256 || codeChanged.CodeInventorySHA256 == sourceChanged.CodeInventorySHA256 {
		t.Fatal("code edit did not change only the code closure")
	}
}

func TestBuildSnapshot_IncludesSourcedZsh(t *testing.T) {
	dir := newSnapshotRepo(t)
	const name = ".pyxis/scripts/startup/codex-worktree.zsh"
	oldBody := []byte("codex() { command codex \"$@\"; }\n")
	writeSnapshotFile(t, dir, name, oldBody, 0o644)
	oldCommit := commitSnapshot(t, dir, "add sourced zsh")
	repo := openSnapshotRepo(t, dir)

	old := mustBuildSnapshot(t, repo, oldCommit, Options{})
	if len(old.Code) != 1 || old.Code[0].Kind != codeinventory.ExecutableScript || old.Code[0].Path != name || !reflect.DeepEqual(old.Code[0].Files, []string{name}) {
		t.Fatalf("sourced zsh inventory = %#v, want one executable-script unit", old.Code)
	}
	if got := old.Files[name].Body; !bytes.Equal(got, oldBody) {
		t.Fatalf("sourced zsh snapshot body = %q, want %q", got, oldBody)
	}

	writeSnapshotFile(t, dir, name, []byte("codex() { command codex --search \"$@\"; }\n"), 0o644)
	changed := mustBuildSnapshot(t, repo, commitSnapshot(t, dir, "change sourced zsh"), Options{})
	if changed.CodeInventorySHA256 == old.CodeInventorySHA256 || changed.Code[0].SHA256 == old.Code[0].SHA256 {
		t.Fatal("sourced zsh edit did not move its unit and aggregate digests")
	}
}

func TestBuildSnapshot_LegacyV2ExcludesSourcedZsh(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeSnapshotFile(t, dir, "README.md", []byte("fixture\n"), 0o644)
	baseCommit := commitSnapshot(t, dir, "base")
	repo := openSnapshotRepo(t, dir)
	before := mustBuildSnapshot(t, repo, baseCommit, Options{})
	const name = ".pyxis/scripts/startup/codex-worktree.zsh"
	writeSnapshotFile(t, dir, name, []byte("codex() { command codex \"$@\"; }\n"), 0o644)
	commit := commitSnapshot(t, dir, "add sourced zsh")

	legacy := mustBuildSnapshot(t, repo, commit, Options{LegacyCodeInventory: true})
	if legacy.CodeInventorySHA256 != before.CodeInventorySHA256 || !reflect.DeepEqual(legacy.Code, before.Code) {
		t.Fatalf("legacy v2 snapshot did not reproduce pre-zsh code closure:\nbefore: %#v\nlegacy: %#v", before.Code, legacy.Code)
	}
	current := mustBuildSnapshot(t, repo, commit, Options{})
	if current.CodeInventorySHA256 == legacy.CodeInventorySHA256 || len(current.Code) != len(legacy.Code)+1 || current.Code[0].Path != name {
		t.Fatalf("current snapshot did not retain sourced zsh:\ncurrent: %#v\nlegacy: %#v", current.Code, legacy.Code)
	}
}

func TestBuildSnapshot_RejectsGeneratedProjectionDrift(t *testing.T) {
	t.Run("PRD projection", func(t *testing.T) {
		dir := newSnapshotRepo(t)
		writeSnapshotPRD(t, dir, "docs/prds", "docgate")
		writeSnapshotFile(t, dir, "docs/prds/docgate/effective.md", []byte("hand edited\n"), 0o644)
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "stale PRD projection"), Options{})
		if err == nil || !strings.Contains(err.Error(), "differs from the pinned fold") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("technical-design index", func(t *testing.T) {
		dir := newSnapshotRepo(t)
		writeSnapshotFile(t, dir, "docs/tech-design/index.json", []byte("{}\n"), 0o644)
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "stale design index"), Options{})
		if err == nil || !strings.Contains(err.Error(), "differs from the graph recomputed") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestBuildSnapshot_RejectsMissingOrIrregularCompilerInputs(t *testing.T) {
	t.Run("missing generated PRD source map", func(t *testing.T) {
		dir := newSnapshotRepo(t)
		writeSnapshotPRD(t, dir, "docs/prds", "docgate")
		if err := os.Remove(filepath.Join(dir, "docs", "prds", "docgate", "effective.source.json")); err != nil {
			t.Fatal(err)
		}
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "missing projection"), Options{})
		if err == nil || !strings.Contains(err.Error(), "generated projection") || !strings.Contains(err.Error(), "is missing") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing technical-design index", func(t *testing.T) {
		dir := newSnapshotRepo(t)
		design, _ := snapshotDesignArtifacts(t, "widget")
		writeSnapshotFile(t, dir, "docs/tech-design/widget.md", design, 0o644)
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "missing index"), Options{})
		if err == nil || !strings.Contains(err.Error(), "index.json is missing") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("symlink compiler input", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Git symlink mode is not portable on Windows")
		}
		dir := newSnapshotRepo(t)
		full := filepath.Join(dir, "docs", "tech-design", "index.json")
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("elsewhere.json", full); err != nil {
			t.Fatal(err)
		}
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "irregular compiler"), Options{})
		if err == nil || !strings.Contains(err.Error(), "compiler input") || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("symlink production code", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Git symlink mode is not portable on Windows")
		}
		dir := newSnapshotRepo(t)
		full := filepath.Join(dir, "internal", "widget", "widget.go")
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../../outside.go", full); err != nil {
			t.Fatal(err)
		}
		repo := openSnapshotRepo(t, dir)
		_, err := BuildSnapshot(context.Background(), repo, commitSnapshot(t, dir, "irregular code"), Options{})
		if err == nil || !strings.Contains(err.Error(), "invalid code inventory tree") || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestBuildSnapshot_HonorsCustomSourceRoots(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeSnapshotPRD(t, dir, "spec/requirements", "docgate")
	design, index := snapshotDesignArtifacts(t, "widget")
	writeSnapshotFile(t, dir, "architecture/widget.md", design, 0o644)
	writeSnapshotFile(t, dir, "architecture/index.json", index, 0o644)
	writeSnapshotFile(t, dir, "roadmaps/docgate/plan.md", []byte("# Document pipeline plan\n"), 0o644)
	writeSnapshotFile(t, dir, "roadmaps/widget/plan.md", []byte("# Draft-born plan\n"), 0o644)
	writeSnapshotFile(t, dir, "notes/context.txt", []byte("extra context\n"), 0o644)
	writeSnapshotFile(t, dir, "pkg/widget/widget.go", []byte("package widget\n"), 0o644)
	commit := commitSnapshot(t, dir, "custom roots")
	repo := openSnapshotRepo(t, dir)

	opts := Options{
		PRDs: "spec/requirements", TechDesigns: "architecture", Plans: "roadmaps",
		ExtraSources: []string{"notes/context.txt", "roadmaps/widget/plan.md"},
	}
	snapshot, err := BuildSnapshot(context.Background(), repo, commit, opts)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	wantSources := []string{
		"architecture/widget.md",
		"notes/context.txt",
		"roadmaps/docgate/plan.md",
		"spec/requirements/docgate/000-base.md",
	}
	if !reflect.DeepEqual(snapshot.AuthoredSources, wantSources) {
		t.Fatalf("authored sources = %v, want %v", snapshot.AuthoredSources, wantSources)
	}
	if len(snapshot.PRDs) != 1 || snapshot.PRDs[0].Source != "spec/requirements/docgate/effective.md" {
		t.Fatalf("custom PRD projection = %#v", snapshot.PRDs)
	}
	if len(snapshot.Designs) != 1 || snapshot.Designs[0].Source != "architecture/widget.md" {
		t.Fatalf("custom design projection = %#v", snapshot.Designs)
	}
	if len(snapshot.Code) != 1 || snapshot.Code[0].Path != "pkg/widget" {
		t.Fatalf("custom-root code inventory = %#v", snapshot.Code)
	}
	for _, generated := range []string{"spec/requirements/docgate/effective.md", "spec/requirements/docgate/effective.source.json", "architecture/index.json"} {
		if slices.Contains(snapshot.AuthoredSources, generated) {
			t.Errorf("generated artifact %s entered authored source closure", generated)
		}
	}
	if _, present := snapshot.Files["roadmaps/widget/plan.md"]; present {
		t.Fatal("draft-born plan entered the pinned source snapshot")
	}
	set, err := store.Load(t.Context(), store.MapSource{})
	if err != nil {
		t.Fatal(err)
	}
	report := Analyze(snapshot, set, resolve.New(set), opts)
	capturedDocumentPlan := false
	for _, action := range report.Actions {
		if strings.HasPrefix(action.Scope, "roadmaps/widget/") {
			t.Fatalf("draft-born plan has a memory capture action: %+v", action)
		}
		capturedDocumentPlan = capturedDocumentPlan || action.Code == "capture-document" && action.Scope == "roadmaps/docgate/plan.md"
	}
	if !capturedDocumentPlan {
		t.Fatal("document-pipeline plan lost its memory capture action")
	}
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.Scope, "roadmaps/widget/") {
			t.Fatalf("draft-born plan has a reconciliation finding: %+v", finding)
		}
	}
}

func TestBuildSnapshot_RequiresRepositoryAndResolvableCommit(t *testing.T) {
	if _, err := BuildSnapshot(context.Background(), nil, "HEAD", Options{}); err == nil {
		t.Fatal("nil repository succeeded")
	}
	dir := newSnapshotRepo(t)
	writeSnapshotFile(t, dir, "README.md", []byte("base\n"), 0o644)
	commitSnapshot(t, dir, "base")
	repo := openSnapshotRepo(t, dir)
	if _, err := BuildSnapshot(context.Background(), repo, "does-not-exist", Options{}); err == nil || !strings.Contains(err.Error(), "resolve reconciliation commit") {
		t.Fatalf("unresolvable ref error = %v", err)
	}
}

const snapshotPRDBase = `# t — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v1 |

## 5. Requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When asked, the system shall answer. | required | Must | med | answer returned | Test | G1 |
`

const snapshotDesign = `# widget

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @o |
| **Approver(s)** | @o |
| **Date** | 2026-09-08 |
| **Relevant-when** | testing snapshots |
| **Scope** | fixture |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | none — fixture |
| **Template version** | tech-design-template/v1 |

## 3. The design

### 3.4 Observability

none — fixture.

### 3.5 Security & privacy

none — fixture.
`

func writeSnapshotPRD(t *testing.T, dir, root, feature string) {
	t.Helper()
	entry := ledger.Entry{Name: "000-base.md", Bytes: []byte(snapshotPRDBase)}
	result := fold.Fold([]ledger.Entry{entry})
	if len(result.Defects) > 0 {
		t.Fatalf("fold snapshot fixture: %v", result.Defects)
	}
	base := filepath.ToSlash(filepath.Join(root, feature))
	writeSnapshotFile(t, dir, base+"/000-base.md", entry.Bytes, 0o644)
	writeSnapshotFile(t, dir, base+"/effective.md", result.EffectiveMD, 0o644)
	writeSnapshotFile(t, dir, base+"/effective.source.json", result.SourceJSON, 0o644)
}

func snapshotDesignArtifacts(t *testing.T, slug string) ([]byte, []byte) {
	t.Helper()
	body := []byte(snapshotDesign)
	design, defects := techdesigns.Parse(slug, body)
	if len(defects) > 0 || design == nil {
		t.Fatalf("parse snapshot design fixture: %v", defects)
	}
	env := techdesigns.Env{
		Grandfathered: map[string]bool{}, PrecedenceGrandfathered: map[string]bool{},
		PRDUnits: map[string]map[string]string{}, CorpusIDs: map[string]bool{}, NewDesigns: map[string]bool{},
		DocExists: func(string) bool { return true },
	}
	_, index, defects := techdesigns.GenIndex([]*techdesigns.Design{design}, env)
	if len(defects) > 0 {
		t.Fatalf("generate snapshot design fixture: %v", defects)
	}
	return body, index
}

func newSnapshotRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runSnapshotGit(t, dir, "init", "-q", "-b", "main")
	return dir
}

func openSnapshotRepo(t *testing.T, dir string) *gitsrc.Repo {
	t.Helper()
	repo, err := gitsrc.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeSnapshotFile(t *testing.T, dir, name string, body []byte, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, body, mode); err != nil {
		t.Fatal(err)
	}
}

func commitSnapshot(t *testing.T, dir, message string) string {
	t.Helper()
	runSnapshotGit(t, dir, "add", "--all")
	runSnapshotGit(t, dir, "commit", "-q", "-m", message)
	return runSnapshotGit(t, dir, "rev-parse", "HEAD")
}

func runSnapshotGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=snapshot-test", "GIT_AUTHOR_EMAIL=snapshot@example.com",
		"GIT_COMMITTER_NAME=snapshot-test", "GIT_COMMITTER_EMAIL=snapshot@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func mustBuildSnapshot(t *testing.T, repo *gitsrc.Repo, commit string, opts Options) Snapshot {
	t.Helper()
	snapshot, err := BuildSnapshot(context.Background(), repo, commit, opts)
	if err != nil {
		t.Fatalf("BuildSnapshot(%s): %v", commit, err)
	}
	return snapshot
}
