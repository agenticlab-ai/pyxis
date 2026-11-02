package gitsrc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "m@example.com")
	git(t, dir, "config", "user.name", "m")
	write(t, dir, "memory/policy.json", `{"schema":1}`)
	write(t, dir, "memory/prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV.json", "{\"a\":1}\n")
	write(t, dir, "README.md", "hi")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func TestOpenAndIdentity(t *testing.T) {
	ctx := context.Background()
	dir := newRepo(t)
	sub := filepath.Join(dir, "memory", "prd")
	r, err := Open(ctx, sub)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(dir); r.Root != want && r.Root != dir {
		t.Fatalf("root %q, want %q", r.Root, dir)
	}
	if r.UserEmail(ctx) != "m@example.com" {
		t.Fatalf("email %q", r.UserEmail(ctx))
	}
	if b, err := r.CurrentBranch(ctx); err != nil || b != "main" {
		t.Fatalf("branch %q %v", b, err)
	}
	if h, err := r.Head(ctx); err != nil || len(h) != 40 {
		t.Fatalf("head %q %v", h, err)
	}
	if !r.RefExists(ctx, "main") || r.RefExists(ctx, "nope") {
		t.Fatal("RefExists")
	}
	if dirty, err := r.Dirty(ctx, "memory"); err != nil || dirty {
		t.Fatalf("clean tree reads clean: %v %v", dirty, err)
	}
	write(t, dir, "memory/prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAX.json", "{}")
	if dirty, err := r.Dirty(ctx, "memory"); err != nil || !dirty {
		t.Fatalf("an untracked record makes the tree dirty: %v %v", dirty, err)
	}
	if dirty, err := r.Dirty(ctx, "docs"); err != nil || dirty {
		t.Fatalf("dirtiness is scoped to the prefix: %v %v", dirty, err)
	}
	if oid, err := r.ResolveCommit(ctx, "main"); err != nil || len(oid) != 40 {
		t.Fatalf("ResolveCommit %q %v", oid, err)
	}
	if _, err := r.ResolveCommit(ctx, "nope"); err == nil {
		t.Fatal("ResolveCommit of nothing")
	}
	if r.DefaultBase(ctx, "origin", "main") != "main" {
		t.Fatalf("default base %q", r.DefaultBase(ctx, "origin", "main"))
	}
	git(t, dir, "remote", "add", "origin", dir)
	git(t, dir, "fetch", "-q", "origin")
	if r.DefaultBase(ctx, "origin", "main") != "origin/main" || r.DefaultBase(ctx, "origin", "trunk") != "" {
		t.Fatalf("default base prefers the remote-tracking mainline: %q", r.DefaultBase(ctx, "origin", "main"))
	}
	if _, err := Open(ctx, t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("plain dir must not open: %v", err)
	}
	if cd, err := r.CommonDir(ctx); err != nil || !filepath.IsAbs(cd) || filepath.Base(cd) != ".git" {
		t.Fatalf("common dir %q %v", cd, err)
	}
	if _, err := Open(ctx, filepath.Join(dir, "README.md")); !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("a git failure that is not 'not a repository' is unavailability: %v", err)
	}
	// without git on PATH, opening is unavailability, never a plain directory
	t.Setenv("PATH", t.TempDir())
	if _, err := Open(ctx, dir); !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("missing git must fail closed: %v", err)
	}
}

func TestTreesBranchesAndDiffs(t *testing.T) {
	ctx := context.Background()
	dir := newRepo(t)
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	// a feature branch adds one record; main moves on with an unrelated file
	git(t, dir, "checkout", "-q", "-b", "feature/a")
	write(t, dir, "memory/prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAW.json", "{\"b\":2}\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "feature")
	git(t, dir, "checkout", "-q", "main")
	write(t, dir, "other.txt", "x")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "main moves")

	branches, err := r.Branches(ctx)
	if err != nil || len(branches) != 2 || branches[0].Name != "feature/a" || branches[1].Name != "main" || len(branches[0].OID) != 40 || branches[0].OID == branches[1].OID {
		t.Fatalf("branches %v %v", branches, err)
	}
	if remotes, err := r.Remotes(ctx); err != nil || len(remotes) != 0 {
		t.Fatalf("remotes %v %v", remotes, err)
	}
	git(t, dir, "remote", "add", "origin", dir)
	if remotes, err := r.Remotes(ctx); err != nil || strings.Join(remotes, ",") != "origin" {
		t.Fatalf("remotes %v %v", remotes, err)
	}
	added, err := r.AddedFiles(ctx, "main", "feature/a", "memory")
	if err != nil || strings.Join(added, ",") != "memory/prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAW.json" {
		t.Fatalf("added %v %v", added, err)
	}
	if mb, err := r.MergeBase(ctx, "main", "feature/a"); err != nil || len(mb) != 40 {
		t.Fatalf("merge base %q %v", mb, err)
	}
	entries, err := r.ListTree(ctx, "feature/a", "memory")
	if err != nil || len(entries) != 3 || !entries[0].Regular() || entries[0].Mode != "100644" {
		t.Fatalf("ls-tree %v %v", entries, err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Path)
	}
	got, err := r.Files(ctx, "feature/a", append(files, "memory/missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || string(got["memory/policy.json"]) != `{"schema":1}` || string(got[files[2]]) != "{\"b\":2}\n" {
		t.Fatalf("files %v", got)
	}
	if empty, err := r.Files(ctx, "main", nil); err != nil || len(empty) != 0 {
		t.Fatalf("no paths → no files: %v %v", empty, err)
	}
	src := TreeSource{Repo: r, Ref: "main", Prefix: "memory"}
	tree, irregular, err := src.Files(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || len(irregular) != 0 || string(tree["policy.json"]) != `{"schema":1}` || tree["prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV.json"] == nil {
		t.Fatalf("tree source %v %v", tree, irregular)
	}
	if _, _, err := (TreeSource{Repo: r, Ref: "nope", Prefix: "memory"}).Files(ctx); err == nil {
		t.Fatal("bad ref must fail")
	}
	// a symlink committed under memory/ is irregular, never content
	git(t, dir, "checkout", "-q", "main")
	write(t, dir, "payload.json", "{\"smuggled\":true}\n")
	if err := os.Symlink("../../../payload.json", filepath.Join(dir, "memory", "prd", "x", "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAY.json")); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "link")
	tree, irregular, err = TreeSource{Repo: r, Ref: "main", Prefix: "memory"}.Files(ctx)
	if err != nil || len(tree) != 2 || irregular["prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAY.json"] == "" || !strings.Contains(irregular["prd/x/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAY.json"], "120000") {
		t.Fatalf("symlink in tree: %v %v %v", tree, irregular, err)
	}
	if e := (TreeEntry{Mode: "160000", Type: "commit"}); e.Regular() {
		t.Fatal("a submodule is not regular")
	}
	if _, irr, _ := (TreeSource{Repo: &Repo{run: func(context.Context, []byte, ...string) ([]byte, error) {
		return []byte("160000 commit abc\tmemory/sub\n"), nil
	}}, Ref: "x", Prefix: "memory"}).Files(ctx); irr["sub"] == "" || !strings.Contains(irr["sub"], "160000") {
		t.Fatalf("submodule irregular: %v", irr)
	}
	if _, err := (&Repo{run: func(context.Context, []byte, ...string) ([]byte, error) { return []byte("garbage\n"), nil }}).ListTree(ctx, "x", "memory"); err == nil {
		t.Fatal("malformed ls-tree line must fail")
	}
	if _, err := r.AddedFiles(ctx, "nope", "main", "memory"); err == nil {
		t.Fatal("bad base must fail")
	}
	if _, err := r.MergeBase(ctx, "nope", "main"); err == nil {
		t.Fatal("bad merge base must fail")
	}
	if _, err := r.Files(ctx, "main", []string{"memory/policy.json"}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Branches(cancelled); err == nil {
		t.Fatal("cancelled context must fail")
	}
	git(t, dir, "checkout", "-q", "--detach")
	if b, err := r.CurrentBranch(ctx); err != nil || b != "HEAD" {
		t.Fatalf("detached branch %q %v", b, err)
	}
}

func TestFilesParsesBatchOutputStrictly(t *testing.T) {
	r := &Repo{run: func(context.Context, []byte, ...string) ([]byte, error) { return []byte("abc blob x\n"), nil }}
	if _, err := r.Files(context.Background(), "main", []string{"p"}); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("bad size must fail: %v", err)
	}
	r.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte("abc blob\n"), nil }
	if _, err := r.Files(context.Background(), "main", []string{"p"}); err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("bad header must fail: %v", err)
	}
	r.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte("abc blob 10\nshort\n"), nil }
	if _, err := r.Files(context.Background(), "main", []string{"p"}); err == nil || !strings.Contains(err.Error(), "body") {
		t.Fatalf("short body must fail: %v", err)
	}
	r.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, nil }
	if _, err := r.Files(context.Background(), "main", []string{"p"}); err == nil || !strings.Contains(err.Error(), "ended early") {
		t.Fatalf("empty output must fail: %v", err)
	}
	r.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := r.Files(context.Background(), "main", []string{"p"}); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.ListTree(context.Background(), "main", "memory"); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.Remotes(context.Background()); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.Dirty(context.Background(), "memory"); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.CommonDir(context.Background()); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.CurrentBranch(context.Background()); err == nil {
		t.Fatal("git failure must propagate")
	}
	if _, err := r.Head(context.Background()); err == nil {
		t.Fatal("git failure must propagate")
	}
	if r.UserEmail(context.Background()) != "" || r.DefaultBase(context.Background(), "origin", "main") != "" {
		t.Fatal("failures read as empty identity and no base")
	}
}
