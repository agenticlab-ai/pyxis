package compile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func TestCompileBranchReadsPinnedPRDHistory(t *testing.T) {
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
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.com")
	write := func(name string, body []byte) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", []byte("# Fixture\n"))
	git("add", "README.md")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-qb", "draft/widget")
	prd := readFixture(t, "testdata/prd/working.md")
	first := []byte(strings.Replace(string(prd), "The widget shall open.", "The widget should open.", 1))
	write("drafts/widget/prd.md", first)
	write("drafts/widget/tech-spec.md", []byte("# Widget — tech spec\n"))
	write("drafts/widget/brainstorm/.keep", nil)
	write("drafts/widget/reviews/.keep", nil)
	git("add", "drafts/widget/prd.md", "drafts/widget/tech-spec.md", "drafts/widget/brainstorm/.keep", "drafts/widget/reviews/.keep")
	git("commit", "-qm", "open draft")
	write("drafts/widget/prd.md", prd)
	git("add", "drafts/widget/prd.md")
	git("commit", "-qm", "revise requirement")
	write("drafts/widget/problems.md", []byte("# Problems\n\n### P-1 — FR-1 — Clarify wording\n**Problem:** The opening verb was unclear.\n**Ruling:** operator · Name the opening action. · Use the shall wording.\n"))
	git("add", "drafts/widget/problems.md")
	git("commit", "-qm", "rule requirement")
	head := git("rev-parse", "HEAD")
	repo, err := gitsrc.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CompileBranch(t.Context(), repo, base, head, "drafts", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mainline != base || len(result.Changes) != 1 || result.Changes[0].ID != "FR-1" || result.Changes[0].Unruled || result.Changes[0].RulingCommit != head || result.Changes[0].FromCommit == "" || result.Changes[0].ToCommit == head {
		t.Fatalf("branch change list = %#v", result.Changes)
	}
	changeEvents, err := ChangeEvents(result, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC))
	if err != nil || len(changeEvents) != 1 || changeEvents[0].Requirement != "FR-1" || changeEvents[0].RulingAuthor != "operator" || changeEvents[0].Problem != "P-1" || !changeEvents[0].ReasonPresent || !strings.Contains(changeEvents[0].Key, head) {
		t.Fatalf("change events = %+v, err = %v", changeEvents, err)
	}
	unruled := result
	unruled.Changes = append([]RequirementChange(nil), result.Changes...)
	unruled.Changes[0].Ruling = nil
	if skipped, err := ChangeEvents(unruled, time.Now()); err != nil || len(skipped) != 0 {
		t.Fatalf("unruled change emitted events = %+v, err = %v", skipped, err)
	}
	withoutCommit := result
	withoutCommit.Changes = append([]RequirementChange(nil), result.Changes...)
	withoutCommit.Changes[0].RulingCommit = ""
	if _, err := ChangeEvents(withoutCommit, time.Now()); err == nil || !strings.Contains(err.Error(), "no ruling commit") {
		t.Fatalf("missing ruling provenance = %v", err)
	}
	withoutRuling := []byte("# Problems\n\n### P-1 — FR-1 — Clarify wording\n**Problem:** The opening verb was unclear.\n")
	write("drafts/widget/problems.md", withoutRuling)
	git("add", "drafts/widget/problems.md")
	git("commit", "-qm", "withdraw ruling")
	write("drafts/widget/problems.md", []byte("# Problems\n\n### P-1 — FR-1 — Clarify wording\n**Problem:** The opening verb was unclear.\n**Ruling:** operator · Name the opening action. · Use the shall wording.\n"))
	git("add", "drafts/widget/problems.md")
	git("commit", "-qm", "restore ruling")
	reintroduced := git("rev-parse", "HEAD")
	relisted, err := CompileBranch(t.Context(), repo, base, reintroduced, "drafts", "widget")
	if err != nil || len(relisted.Changes) != 1 || relisted.Changes[0].RulingCommit != reintroduced {
		t.Fatalf("reintroduced ruling commit = %+v, err = %v", relisted.Changes, err)
	}
	for _, tc := range []struct {
		name                            string
		repo                            *gitsrc.Repo
		base, head, root, feature, want string
	}{
		{"nil repository", nil, base, head, "drafts", "widget", "repository and feature"},
		{"invalid root", repo, base, head, "../drafts", "widget", "invalid drafts root"},
		{"missing base", repo, "missing-base", head, "drafts", "widget", "missing-base"},
		{"missing head", repo, base, "missing-head", "drafts", "widget", "missing-head"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileBranch(t.Context(), tc.repo, tc.base, tc.head, tc.root, tc.feature)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CompileBranch() error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := os.MkdirAll(filepath.Join(dir, "drafts/widget/brainstorm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../README.md", filepath.Join(dir, "drafts/widget/brainstorm/001.md")); err != nil {
		t.Fatal(err)
	}
	git("add", "drafts/widget/brainstorm/001.md")
	git("commit", "-qm", "add unreadable draft link")
	_, err = CompileBranch(t.Context(), repo, base, "HEAD", "drafts", "widget")
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("symlinked part error = %v", err)
	}
}

func TestCompileBranchRecoversUnruledProblemFormatting(t *testing.T) {
	const ruling = "**Ruling:** operator · Name the opening action. · Use the shall wording.\n"
	const question = "**Problem:** The opening verb was unclear.\n"
	valid := "# Problems\n\n### P-1 — FR-1 — Clarify wording\n" + question
	malformed := "# Problems\n\n### P-1 — FR-1, UB-1 — Clarify wording\n" + question
	quoted := "**Problem:** Rename the **Ruling:** label.\n"
	for _, tc := range []struct {
		name, initial, historical, current, wantError string
		wantRuling                                    bool
	}{
		{name: "repaired unanswered problem", historical: malformed, current: valid},
		{name: "ruling label in problem prose", historical: strings.Replace(malformed, question, quoted, 1), current: strings.Replace(valid, question, quoted, 1)},
		{name: "ruling after formatting repair", historical: malformed, current: valid + ruling, wantRuling: true},
		{name: "reintroduced ruling after malformed unruled round", initial: valid + ruling, historical: malformed, current: valid + ruling, wantRuling: true},
		{name: "malformed historical ruling", historical: malformed + ruling, current: valid + ruling, wantError: "compile problems at"},
		{name: "indented malformed historical ruling", historical: malformed + " \t" + ruling, current: valid + ruling, wantError: "compile problems at"},
		{name: "malformed current problem", historical: malformed, current: malformed, wantError: "invalid or repeated problem heading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			write := func(name string, body []byte) {
				t.Helper()
				full := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			git("init", "-q")
			git("config", "user.name", "Fixture")
			git("config", "user.email", "fixture@example.com")
			write("README.md", []byte("# Fixture\n"))
			git("add", "README.md")
			git("commit", "-qm", "base")
			base := git("rev-parse", "HEAD")
			prd := readFixture(t, "testdata/prd/working.md")
			first := []byte(strings.Replace(string(prd), "The widget shall open.", "The widget should open.", 1))
			write("drafts/widget/prd.md", first)
			write("drafts/widget/tech-spec.md", []byte("# Widget — tech spec\n"))
			write("drafts/widget/problems.md", []byte(tc.initial))
			git("add", "drafts/widget/prd.md", "drafts/widget/tech-spec.md", "drafts/widget/problems.md")
			git("commit", "-qm", "open draft")
			write("drafts/widget/prd.md", prd)
			write("drafts/widget/problems.md", []byte(tc.historical))
			git("add", "drafts/widget/prd.md", "drafts/widget/problems.md")
			git("commit", "-qm", "revise draft with malformed problem")
			if tc.current != tc.historical {
				write("drafts/widget/problems.md", []byte(tc.current))
				git("add", "drafts/widget/problems.md")
				git("commit", "-qm", "repair problem formatting")
			}
			head := git("rev-parse", "HEAD")
			repo, err := gitsrc.Open(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CompileBranch(t.Context(), repo, base, head, "drafts", "widget")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("CompileBranch() error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Problems) != 1 || len(result.Changes) != 1 {
				t.Fatalf("draft lost its question or requirement change: %+v", result)
			}
			change := result.Changes[0]
			if tc.wantRuling {
				if change.Ruling == nil || change.Unruled || change.RulingCommit != head || change.Ruling.Author != "operator" {
					t.Fatalf("repaired ruling provenance = %+v, want current head %s", change, head)
				}
			} else if change.Ruling != nil || !change.Unruled || change.RulingCommit != "" || result.Problems[0].Ruling != nil {
				t.Fatalf("unanswered problem invented ruling authority: %+v", change)
			}
		})
	}
}
