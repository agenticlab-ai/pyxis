package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/inflight"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func TestDraftNoticeReadsOnlyOpenPinnedDraftPullRequests(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "remote.git")
	digestGit(t, root, "init", "-b", "main")
	digestGit(t, root, "config", "user.name", "Fixture")
	digestGit(t, root, "config", "user.email", "fixture@example.test")
	digestGit(t, root, "init", "--bare", bare)
	digestGit(t, root, "remote", "add", "origin", bare)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digestGit(t, root, "add", "README.md")
	digestGit(t, root, "commit", "-m", "base")
	digestGit(t, root, "push", "-u", "origin", "main")
	prd, err := os.ReadFile("../../internal/draft/compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	makeDraft := func(branch, feature, serves string) string {
		t.Helper()
		digestGit(t, root, "switch", "-c", branch, "main")
		dir := filepath.Join(root, "drafts", feature)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string][]byte{
			"prd.md":       prd,
			"tech-spec.md": []byte("# Tech spec\n\n### D-1 — Build\n\n**Decision:** Build it.\n\n**Serves:** " + serves + "\n"),
		} {
			if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		digestGit(t, root, "add", "--", filepath.ToSlash(filepath.Join("drafts", feature)))
		digestGit(t, root, "commit", "-m", "draft "+feature)
		digestGit(t, root, "push", "-u", "origin", branch)
		return digestGit(t, root, "rev-parse", "HEAD")
	}
	alphaHead := makeDraft("draft/alpha", "alpha", "prd:shared/FR-7")
	betaHead := makeDraft("draft/beta-v10", "beta", "prd:shared/FR-7")
	_ = makeDraft("draft/closed", "closed", "prd:shared/FR-7")
	digestGit(t, root, "switch", "draft/alpha")
	open := fmt.Sprintf("11\tdraft/alpha\t%s\n12\tdraft/beta-v10\t%s\n13\tfeature/unrelated\t%s\n", alphaHead, betaHead, betaHead)
	run := lifecycle.RunCommand(func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			if len(args) >= 2 && args[0] == "api" && args[1] == "--paginate" {
				return []byte(open), nil
			}
			return nil, fmt.Errorf("unexpected gh args %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	})
	report := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", run)
	if len(report.Warnings) != 0 || len(report.Matches) != 1 || report.Matches[0].Pull.Number != 12 || report.Matches[0].Pull.Feature != "beta" || strings.Join(report.Matches[0].Shared, ",") != "prd/shared#FR-7" {
		t.Fatalf("notice = %+v", report)
	}
	open = fmt.Sprintf("11\tdraft/alpha\t%s\n12\tdraft/beta-v10\t%s\n", alphaHead, strings.Repeat("a", 40))
	report = noticeForDraft(t.Context(), root, config.Defaults(), "alpha", run)
	if len(report.Matches) != 0 || len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "moved") {
		t.Fatalf("moved PR must be advisory: %+v", report)
	}
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("offline")
	}); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "mainline") {
		t.Fatalf("offline notice = %+v", got)
	}
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", nil); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "adapter") {
		t.Fatalf("missing adapter = %+v", got)
	}
	if got := noticeForDraft(t.Context(), t.TempDir(), config.Defaults(), "alpha", run); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "repository") {
		t.Fatalf("missing repository = %+v", got)
	}
	digestGit(t, root, "switch", "main")
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", run); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "not a feature draft") {
		t.Fatalf("wrong branch = %+v", got)
	}
	digestGit(t, root, "switch", "draft/alpha")
	badBase := config.Defaults()
	badBase.Branch = "absent"
	baseRun := lifecycle.RunCommand(func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) >= 3 && args[0] == "fetch" {
			return nil, nil
		}
		return run(ctx, dir, env, name, args...)
	})
	if got := noticeForDraft(t.Context(), root, badBase, "alpha", baseRun); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "mainline") {
		t.Fatalf("missing mainline ref = %+v", got)
	}
	ghFail := lifecycle.RunCommand(func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			return nil, errors.New("code host offline")
		}
		return run(ctx, dir, env, name, args...)
	})
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", ghFail); len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "open pull requests") {
		t.Fatalf("code host failure = %+v", got)
	}
	open = fmt.Sprintf("malformed\nnope\tdraft/beta-v10\t%s\n11\tdraft/alpha\t%s\n12\tdraft/beta-v10\t%s\n", betaHead, alphaHead, betaHead)
	fetchFail := lifecycle.RunCommand(func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) >= 3 && args[0] == "fetch" && strings.HasPrefix(args[2], "refs/heads/draft/") {
			return nil, errors.New("branch unavailable")
		}
		return run(ctx, dir, env, name, args...)
	})
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", fetchFail); len(got.Matches) != 0 || len(got.Warnings) != 3 || !strings.Contains(strings.Join(got.Warnings, " "), "fetch") {
		t.Fatalf("malformed observation and fetch failure = %+v", got)
	}
	open = fmt.Sprintf("12\tdraft/beta-v10\t%s\n", betaHead)
	if got := noticeForDraft(t.Context(), root, config.Defaults(), "alpha", run); len(got.Matches) != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "current open draft") {
		t.Fatalf("current PR omitted = %+v", got)
	}
}

func TestDraftNoticeCLIIsAlwaysAdvisory(t *testing.T) {
	root := digestFixture(t)
	oldRoot, oldNotice := draftRepoRoot, draftNotice
	t.Cleanup(func() { draftRepoRoot, draftNotice = oldRoot, oldNotice })
	draftRepoRoot = func() (string, error) { return root, nil }
	draftNotice = func(context.Context, string, config.Config, string, lifecycle.RunCommand) inflight.Report {
		return inflight.Report{Matches: []inflight.Match{{Pull: inflight.OpenPull{Number: 9, Branch: "draft/other"}, Shared: []string{"prd/shared#FR-7"}}}, Warnings: []string{"PR #8 unreadable"}}
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"notice", "widget"}, &out, &errout); code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "PR #9") || !strings.Contains(out.String(), "notice unavailable for PR #8") {
		t.Fatalf("notice exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"notice"}, {"notice", "--bad"}, {"notice", "widget", "extra"}} {
		out.Reset()
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || errout.Len() == 0 {
			t.Fatalf("invalid notice args %v: exit=%d stderr=%q", args, code, errout.String())
		}
	}
	draftNotice = func(context.Context, string, config.Config, string, lifecycle.RunCommand) inflight.Report {
		return inflight.Report{Warnings: []string{"code host unavailable"}}
	}
	out.Reset()
	errout.Reset()
	if code := draftVerb([]string{"notice", "widget"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "no other open drafts") || !strings.Contains(out.String(), "code host unavailable") {
		t.Fatalf("unavailable notice exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
}

func TestDraftNoticeFeatureDetectionRequiresOneContract(t *testing.T) {
	root := digestFixture(t)
	repo, err := gitsrc.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	head := digestGit(t, root, "rev-parse", "HEAD")
	feature, err := draftFeatureAtHead(t.Context(), repo, "drafts", "draft/", inflight.OpenPull{Branch: "draft/widget", Head: head})
	if err != nil || feature != "widget" {
		t.Fatalf("feature=%q err=%v", feature, err)
	}
	if _, err := draftFeatureAtHead(t.Context(), repo, "drafts", "draft/", inflight.OpenPull{Branch: "draft/missing", Head: head}); err == nil || !strings.Contains(err.Error(), "one draft contract") {
		t.Fatalf("missing contract = %v", err)
	}
	if _, err := draftFeatureAtHead(t.Context(), repo, "drafts", "draft/", inflight.OpenPull{Branch: "draft/", Head: head}); err == nil || !strings.Contains(err.Error(), "one draft contract") {
		t.Fatalf("empty branch feature = %v", err)
	}
	if _, err := draftFeatureAtHead(t.Context(), repo, "drafts", "draft/", inflight.OpenPull{Branch: "draft/widget", Head: "missing"}); err == nil {
		t.Fatal("missing pinned head accepted")
	}
}
