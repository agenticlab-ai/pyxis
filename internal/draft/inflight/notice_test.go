package inflight

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func TestOpenDraftNoticeSharesRequirementsAndDecisionsWithoutChangingReadiness(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-qb", "main")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	prd, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	makeDraft := func(number int, feature, serves, buildsOn string) OpenPull {
		t.Helper()
		git("checkout", "-qb", "draft/"+feature, base)
		root := filepath.Join(dir, "drafts", feature)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "prd.md"), prd, 0o644); err != nil {
			t.Fatal(err)
		}
		tech := "# Tech spec\n\n### D-1 — Build\n\n**Decision:** Build the feature.\n\n**Serves:** " + serves + "\n"
		if buildsOn != "" {
			tech += "**Builds-on:** " + buildsOn + "\n"
		}
		if err := os.WriteFile(filepath.Join(root, "tech-spec.md"), []byte(tech), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", filepath.ToSlash(filepath.Join("drafts", feature)))
		git("commit", "-qm", "draft "+feature)
		return OpenPull{Number: number, Branch: "draft/" + feature, Head: git("rev-parse", "HEAD"), Feature: feature}
	}
	alpha := makeDraft(11, "alpha", "FR-1 · prd:shared/FR-7", "design:shared#D-3")
	beta := makeDraft(12, "beta", "prd:shared/FR-7", "")
	gamma := makeDraft(13, "gamma", "FR-1", "design:shared#D-3")
	_ = makeDraft(14, "closed", "prd:shared/FR-7", "design:shared#D-3")
	repo, err := gitsrc.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	open := []OpenPull{gamma, beta, alpha}
	report := Build(t.Context(), repo, base, "drafts", alpha, open)
	if len(report.Warnings) != 0 || len(report.Matches) != 2 || report.Matches[0].Pull.Number != 12 || !reflect.DeepEqual(report.Matches[0].Shared, []string{"prd/shared#FR-7"}) || report.Matches[1].Pull.Number != 13 || !reflect.DeepEqual(report.Matches[1].Shared, []string{"design/shared#D-3"}) {
		t.Fatalf("alpha notice = %+v", report)
	}
	for _, tc := range []struct {
		current OpenPull
		other   int
		shared  string
	}{{beta, 11, "prd/shared#FR-7"}, {gamma, 11, "design/shared#D-3"}} {
		got := Build(t.Context(), repo, base, "drafts", tc.current, open)
		if len(got.Matches) != 1 || got.Matches[0].Pull.Number != tc.other || !reflect.DeepEqual(got.Matches[0].Shared, []string{tc.shared}) {
			t.Fatalf("%s reciprocal notice = %+v", tc.current.Feature, got)
		}
	}
	result, err := compile.CompileBranch(t.Context(), repo, base, alpha.Head, "drafts", alpha.Feature)
	if err != nil {
		t.Fatal(err)
	}
	before := readiness.Judge(result, readiness.Sources{})
	_ = Build(t.Context(), repo, base, "drafts", alpha, open)
	after := readiness.Judge(result, readiness.Sources{})
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("notice changed readiness: before=%+v after=%+v", before, after)
	}
	if got := Build(t.Context(), repo, base, "drafts", alpha, []OpenPull{beta}); len(got.Matches) != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "no open") {
		t.Fatalf("unlisted current draft = %+v", got)
	}
	badCurrent := alpha
	badCurrent.Head = "missing"
	if got := Build(t.Context(), repo, base, "drafts", badCurrent, open); len(got.Matches) != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "current draft") {
		t.Fatalf("unreadable current draft = %+v", got)
	}
	badOther := beta
	badOther.Head = "missing"
	if got := Build(t.Context(), repo, base, "drafts", alpha, []OpenPull{alpha, badOther, badOther, {Number: 0, Branch: "draft/invalid"}}); len(got.Matches) != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "PR #12") {
		t.Fatalf("unreadable peer must be advisory once = %+v", got)
	}
}

func TestNoticeIsAdvisoryForMissingOrInvalidOpenDraft(t *testing.T) {
	current := OpenPull{Number: 1, Branch: "draft/one", Head: "missing", Feature: "one"}
	if got := Build(t.Context(), nil, "main", "drafts", current, []OpenPull{current}); len(got.Matches) != 0 || len(got.Warnings) != 1 {
		t.Fatalf("missing repository = %+v", got)
	}
	if got := Build(t.Context(), nil, "main", "drafts", OpenPull{}, nil); len(got.Matches) != 0 || len(got.Warnings) != 1 {
		t.Fatalf("invalid current draft = %+v", got)
	}
}
