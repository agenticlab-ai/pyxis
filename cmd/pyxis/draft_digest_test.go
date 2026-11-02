package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func digestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func digestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	digestGit(t, root, "init", "-b", "main")
	digestGit(t, root, "config", "user.name", "Fixture")
	digestGit(t, root, "config", "user.email", "fixture@example.test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digestGit(t, root, "add", "--", "README.md")
	digestGit(t, root, "commit", "-m", "base")
	digestGit(t, root, "switch", "-c", "draft/widget")
	dir := filepath.Join(root, "drafts", "widget")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"prd.md": "# Widget PRD\n", "tech-spec.md": "# Widget tech spec\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	digestGit(t, root, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
	digestGit(t, root, "commit", "-m", "open widget draft")
	return root
}

func TestDraftDigestPinsCommittedContract(t *testing.T) {
	root := digestFixture(t)
	cfg := config.Defaults()
	head, got, err := draftDigest(t.Context(), root, cfg, "widget")
	want := compile.ContractDigest([]byte("# Widget PRD\n"), []byte("# Widget tech spec\n"))
	if err != nil || head != digestGit(t, root, "rev-parse", "HEAD") || got != want {
		t.Fatalf("digest head=%q value=%q err=%v, want %q", head, got, err, want)
	}
	oldRoot := draftRepoRoot
	t.Cleanup(func() { draftRepoRoot = oldRoot })
	draftRepoRoot = func() (string, error) { return root, nil }
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"digest", "widget"}, &out, &errout); code != 0 || out.String() != head+" "+want+"\n" || errout.Len() != 0 {
		t.Fatalf("digest CLI exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"digest"}, {"digest", "--bad"}, {"digest", "widget", "extra"}} {
		out.Reset()
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || errout.Len() == 0 {
			t.Fatalf("invalid digest args %v: exit=%d stderr=%q", args, code, errout.String())
		}
	}
	if err := os.WriteFile(filepath.Join(root, "drafts/widget/prd.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := draftDigest(t.Context(), root, cfg, "widget"); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("uncommitted contract accepted: %v", err)
	}
	out.Reset()
	errout.Reset()
	if code := draftVerb([]string{"digest", "widget"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "uncommitted") {
		t.Fatalf("dirty digest CLI exit=%d stderr=%q", code, errout.String())
	}
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("unknown: key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errout.Reset()
	if code := draftVerb([]string{"digest", "widget"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "configuration") {
		t.Fatalf("invalid config digest exit=%d stderr=%q", code, errout.String())
	}
}

func TestDraftDigestRefusesWrongBranchAndMissingContract(t *testing.T) {
	root := digestFixture(t)
	cfg := config.Defaults()
	if _, _, err := draftDigest(t.Context(), root, cfg, "../widget"); err == nil || !strings.Contains(err.Error(), "feature slug") {
		t.Fatalf("invalid feature accepted: %v", err)
	}
	if _, _, err := draftDigest(t.Context(), t.TempDir(), cfg, "widget"); err == nil || !strings.Contains(err.Error(), "open draft repository") {
		t.Fatalf("non-Git draft accepted: %v", err)
	}
	digestGit(t, root, "switch", "main")
	if _, _, err := draftDigest(t.Context(), root, cfg, "widget"); err == nil || !strings.Contains(err.Error(), "not the draft") {
		t.Fatalf("wrong branch accepted: %v", err)
	}
	digestGit(t, root, "switch", "draft/widget")
	digestGit(t, root, "rm", "--", "drafts/widget/tech-spec.md")
	digestGit(t, root, "commit", "-m", "remove tech spec")
	if _, _, err := draftDigest(t.Context(), root, cfg, "widget"); err == nil || !strings.Contains(err.Error(), "missing or not a regular file") {
		t.Fatalf("missing tech spec accepted: %v", err)
	}
}

type failingDigestRepository struct{ fail string }

func (r failingDigestRepository) CurrentBranch(context.Context) (string, error) {
	if r.fail == "branch" {
		return "", errors.New("injected branch read")
	}
	return "draft/widget-v10", nil
}
func (r failingDigestRepository) DirtyPaths(context.Context, []string) (bool, error) {
	if r.fail == "dirty" {
		return false, errors.New("injected dirty read")
	}
	return false, nil
}
func (r failingDigestRepository) Head(context.Context) (string, error) {
	if r.fail == "head" {
		return "", errors.New("injected head read")
	}
	return "head", nil
}
func (r failingDigestRepository) ListTree(context.Context, string, string) ([]gitsrc.TreeEntry, error) {
	if r.fail == "tree" {
		return nil, errors.New("injected tree read")
	}
	return []gitsrc.TreeEntry{
		{Path: "drafts/widget/prd.md", Type: "blob", Mode: "100644"},
		{Path: "drafts/widget/tech-spec.md", Type: "blob", Mode: "100644"},
	}, nil
}
func (r failingDigestRepository) Files(context.Context, string, []string) (map[string][]byte, error) {
	if r.fail == "files" {
		return nil, errors.New("injected blob read")
	}
	return map[string][]byte{"drafts/widget/prd.md": []byte("prd"), "drafts/widget/tech-spec.md": []byte("tech")}, nil
}

func TestDraftDigestFailsClosedOnPinnedReadErrors(t *testing.T) {
	old := openDigestRepository
	t.Cleanup(func() { openDigestRepository = old })
	for _, tc := range []struct{ fail, want string }{
		{"branch", "read draft branch"},
		{"dirty", "inspect draft files"},
		{"head", "resolve draft head"},
		{"tree", "inspect committed draft"},
		{"files", "read committed draft contract"},
	} {
		openDigestRepository = func(context.Context, string) (digestRepository, error) {
			return failingDigestRepository{fail: tc.fail}, nil
		}
		if _, _, err := draftDigest(t.Context(), "ignored", config.Defaults(), "widget"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error %v, want %q", tc.fail, err, tc.want)
		}
	}
}
