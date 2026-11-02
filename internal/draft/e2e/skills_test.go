package e2e

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// The corpus is retained output from actual authoring and reviewer skill runs.
// This test executes their contract through the lifecycle; code-host responses
// and the child memory-gate process use the existing hermetic host double.
func TestSkillAuthoredDraftRequiresCurrentReviewsAndOperatorApproval(t *testing.T) {
	w := newWorld(t)
	w.host.prNumber = 0
	const feature = "widget-summary"
	const prefix = "drafts/" + feature + "/"
	opened, err := lifecycle.Open(t.Context(), w.root, w.cfg, feature, w.host.run)
	if err != nil {
		t.Fatal(err)
	}
	files := skillFixture(t)
	read := func(name string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("testdata", "skill-authored", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	commit := func(message string) string {
		t.Helper()
		git(t, w.root, "add", "--", strings.TrimSuffix(prefix, "/"))
		git(t, w.root, "commit", "-qm", message)
		git(t, w.root, "push", "-q", "origin", opened.Branch)
		return git(t, w.root, "rev-parse", "HEAD")
	}
	put := func(name string, body []byte) { write(t, w.root, prefix+name, string(body)) }
	for name, body := range files {
		if strings.HasPrefix(name, "brainstorm/") {
			put(name, body)
		}
	}
	defective := read("snapshots/prd-defect.md")
	put("prd.md", defective)
	put("tech-spec.md", read("snapshots/first-review/tech-spec.md"))
	put("problems.md", []byte("# Requirement problems\n"))
	commit("seed the design-discovered product defect")

	p1, _, found := strings.Cut(string(files["problems.md"]), "### P-2")
	if !found {
		t.Fatal("fixture lacks the second product problem")
	}
	pendingP1, _, found := strings.Cut(p1, "**Ruling:**")
	if !found {
		t.Fatal("fixture lacks the first product ruling")
	}
	put("problems.md", []byte(pendingP1))
	commit("record design problem without editing the requirement")
	unchanged, err := os.ReadFile(filepath.Join(w.root, prefix, "prd.md"))
	if err != nil || !bytes.Equal(unchanged, defective) {
		t.Fatalf("design discovery changed PRD bytes: %v", err)
	}
	put("problems.md", []byte(p1))
	rulingHead := commit("attribute the product ruling before applying it")
	put("prd.md", read("snapshots/first-review/prd.md"))
	commit("apply the ruled requirement sentence")
	put("problems.md", files["problems.md"])
	commit("record review error-precedence ruling before applying it")
	put("prd.md", files["prd.md"])
	put("tech-spec.md", files["tech-spec.md"])
	put("reviews/001.md", files["reviews/001.md"])
	put("reviews/002.md", files["reviews/002.md"])
	before := commit("revise contract while retaining the old reviewed digests")

	refused, err := lock.Prepare(t.Context(), w.root, w.cfg, feature, w.host.run)
	if err == nil || !strings.Contains(err.Error(), "readiness refused") {
		t.Fatalf("stale skill reports admitted preparation: %+v %v", refused, err)
	}
	stale := map[string]bool{}
	for _, unmet := range refused.Verdict.Unmet {
		if unmet.Condition == "FR-16" && strings.Contains(unmet.Remedy, "re-run") {
			stale[unmet.Item] = true
		} else {
			t.Fatalf("unexpected readiness defect: %+v", unmet)
		}
	}
	if len(stale) != 2 || !stale["reviews/001.md"] || !stale["reviews/002.md"] {
		t.Fatalf("both skill reviews must become stale: %+v", stale)
	}
	if git(t, w.root, "rev-parse", "HEAD") != before || git(t, w.root, "status", "--porcelain") != "" || w.host.request != "" {
		t.Fatal("refusal changed the draft or requested approval")
	}
	put("reviews/003.md", files["reviews/003.md"])
	put("reviews/004.md", files["reviews/004.md"])
	head := commit("retain the current-digest second-pass verdicts")
	repo, err := gitsrc.Open(t.Context(), w.root)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBranch(t.Context(), repo, w.base, head, w.cfg.Drafts, feature)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Digest != "sha256:943d43dbd5da94884f517ad453dfbdcb74895218e56b708e0c10d1ebdb2b3c64" {
		t.Fatalf("retained contract bytes drifted: %s", compiled.Digest)
	}
	ruled := false
	for _, change := range compiled.Changes {
		if change.ID == "FR-1" {
			ruled = !change.Unruled && change.Ruling != nil && change.Ruling.Problem == "P-1" && change.Ruling.Author == "operator" && change.Ruling.Reason != "" && change.RulingCommit == rulingHead
		}
	}
	if !ruled {
		t.Fatalf("the applied change lost its attributed ruling: %+v", compiled.Changes)
	}
	for _, name := range []string{"prd.md", "tech-spec.md"} {
		literal := false
		for _, item := range compiled.Records {
			literal = literal || item.Draft.Kind == record.Document && item.Draft.Statement == "Draft "+name && bytes.Equal(item.Draft.Body, files[name])
		}
		if !literal {
			t.Fatalf("compiled %s lost its literal body", name)
		}
	}
	prepared, err := lock.Prepare(t.Context(), w.root, w.cfg, feature, w.host.run)
	if err != nil || !prepared.Verdict.Ready || prepared.Verdict.DraftDigest != compiled.Digest {
		t.Fatalf("prepare authored fixture: %+v %v", prepared, err)
	}
	request := w.host.request
	if !strings.Contains(request, prepared.Head) || !strings.Contains(request, "P-1") || !strings.Contains(request, "P-2") {
		t.Fatalf("operator request omitted its exact head or product rulings: %s", request)
	}
	for _, rel := range []string{strings.TrimSuffix(prefix, "/"), "docs/prds/" + feature, "docs/tech-design/" + feature + ".md", "docs/solution-shapes/" + feature + ".md", "docs/plans/" + feature} {
		if tree := git(t, w.root, "ls-tree", "-r", "--name-only", prepared.Head, "--", rel); tree != "" {
			t.Fatalf("prepared lock retained or created %s: %s", rel, tree)
		}
	}
	reader := approvalFixture(t, prepared.PullRequest, prepared.Head, []approval.Review{{ID: 1, Author: "argus", State: "APPROVED", CommitID: prepared.Head, Body: "operator approved"}})
	if _, err := reader.Approval(t.Context(), prepared.PullRequest, prepared.Head); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("Argus approved its own prepared head: %v", err)
	}
	if w.host.request != request || git(t, w.root, "rev-parse", "origin/main") != w.base || git(t, w.root, "tag", "--list", "lock/"+feature+"/*") != "" {
		t.Fatal("agent refusal altered the operator request or bound a lock")
	}
}

func skillFixture(t *testing.T) map[string][]byte {
	t.Helper()
	root := filepath.Join("testdata", "skill-authored", "draft")
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = body
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
