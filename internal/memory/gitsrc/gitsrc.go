// Package gitsrc is the git adapter: it reads the memory tree at any ref,
// lists the branches that carry records not yet on the base, diffs a branch
// against its base, and answers who the actor is. Every operation shells out
// to the git binary, and nothing is kept between runs, because the index is
// rebuilt from files on every run by design. Within one run a caller may opt
// in to KeepPinnedReads, which answers each read of a pinned commit once.
package gitsrc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// ErrNotRepo is returned when the directory is not inside a git work tree.
var ErrNotRepo = errors.New("not inside a git repository")

// ErrGitUnavailable is returned when git itself cannot be run: missing from
// PATH, or failing for a reason other than the directory not being a
// repository. Callers fail closed on it rather than treating the tree as a
// standalone memory.
var ErrGitUnavailable = errors.New("git is unavailable")

// Repo is one git work tree.
type Repo struct {
	Root   string
	run    func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)
	pinned *pinnedReads // nil unless the caller opted in with KeepPinnedReads
}

// Open locates the repository containing dir. Only "not a git repository"
// reads as ErrNotRepo; a missing binary (the first command fails with
// exec.ErrNotFound) or any other git failure is ErrGitUnavailable, so a
// broken environment is never mistaken for a plain directory. The adapter
// probes nothing up front: git's own answer to the first command is the
// classification.
func Open(ctx context.Context, dir string) (*Repo, error) {
	r := &Repo{run: func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		return runGit(ctx, dir, stdin, args...)
	}}
	out, err := r.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && strings.Contains(err.Error(), "not a git repository") {
			return nil, fmt.Errorf("%w: %s", ErrNotRepo, dir)
		}
		return nil, fmt.Errorf("%w: %v", ErrGitUnavailable, err)
	}
	r.Root = strings.TrimSpace(string(out))
	r.run = func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
		return runGit(ctx, r.Root, stdin, args...)
	}
	return r, nil
}

func runGit(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error) {
	// Git is a local object reader here. Do not let inherited repository
	// overrides, replacements, hooks, fsmonitor, or promisor fetches change it.
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "--literal-pathspecs", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "protocol.allow=never"}, args...)...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	// An empty allowlist overrides every protocol-specific repository setting,
	// including on Git versions predating the no-lazy-fetch switch.
	cmd.Env = append(cmd.Env, "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_GRAFT_FILE=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	if len(args) == 2 && args[0] == "config" && args[1] == "user.email" {
		// Authorship keeps Git's ordinary local/global/system identity cascade.
		// This exact read-only key lookup cannot execute configured extensions;
		// object reads below never inherit those configuration files or overrides.
		for _, key := range []string{"GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_GLOBAL"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
	} else {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	}
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// ResolveCommit resolves a ref to the commit id it names, so a base is pinned
// once and every later read uses the same tree.
func (r *Repo) ResolveCommit(ctx context.Context, ref string) (string, error) {
	out, err := r.read(ctx, pinnedRevision(ref), "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("ref %q does not name a commit", ref)
	}
	return strings.TrimSpace(string(out)), nil
}

// Dirty compares raw working bytes with HEAD without running Git's clean
// filters. Staged and untracked paths are included. See dirty.go.
func (r *Repo) Dirty(ctx context.Context, prefix string) (bool, error) {
	return r.DirtyPaths(ctx, []string{prefix})
}

// RefExists reports whether ref resolves to a commit.
func (r *Repo) RefExists(ctx context.Context, ref string) bool {
	_, err := r.run(ctx, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// DefaultBase picks the base ref a proposal is measured against: the
// remote-tracking mainline "<remote>/<branch>", then the local "<branch>". The
// remote and branch names come from the repository's config (the single
// source of every target-facing value, DINV-1), never from this package. The
// empty string means neither exists yet.
func (r *Repo) DefaultBase(ctx context.Context, remote, branch string) string {
	for _, ref := range []string{remote + "/" + branch, branch} {
		if r.RefExists(ctx, ref) {
			return ref
		}
	}
	return ""
}

// CurrentBranch returns the checked-out branch, or "HEAD" when detached.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	out, err := r.run(ctx, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Branch is a local or remote-tracking branch and the commit it points at.
type Branch struct {
	Name string // short name: feature/a, origin/feature/a
	OID  string
}

// Branches lists local and remote branches with their commits, excluding
// symbolic remote heads such as origin/HEAD.
func (r *Repo) Branches(ctx context.Context) ([]Branch, error) {
	out, err := r.run(ctx, nil, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var branches []Branch
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		name, oid, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || name == "" || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		branches = append(branches, Branch{Name: name, OID: oid})
	}
	return branches, nil
}

// Remotes lists the configured remote names.
func (r *Repo) Remotes(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, nil, "remote")
	if err != nil {
		return nil, err
	}
	var names []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// AddedFiles lists the files under prefix that ref adds relative to the merge
// base with base (git's base...ref), so a branch is measured against what it
// branched from, not against everything that landed since.
func (r *Repo) AddedFiles(ctx context.Context, base, ref, prefix string) ([]string, error) {
	out, err := r.run(ctx, nil, "diff", "--name-only", "--diff-filter=A", base+"..."+ref, "--", prefix)
	if err != nil {
		return nil, err
	}
	var files []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// MergeBase returns the merge base commit of two refs.
func (r *Repo) MergeBase(ctx context.Context, a, b string) (string, error) {
	out, err := r.read(ctx, record.ValidCommitID(a) && record.ValidCommitID(b), "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// TreeEntry is one entry of a git tree: its mode, object type and path.
type TreeEntry struct {
	Mode string // 100644, 100755, 120000 (symlink), 160000 (submodule) …
	Type string // blob, commit
	Path string
}

// Regular reports whether the entry is a plain file whose blob is its content.
func (e TreeEntry) Regular() bool {
	return e.Type == "blob" && (e.Mode == "100644" || e.Mode == "100755")
}

// ListTree lists every entry under prefix at ref (paths relative to the repo
// root) with its mode, so a symlink or a submodule is never mistaken for a
// file.
func (r *Repo) ListTree(ctx context.Context, ref, prefix string) ([]TreeEntry, error) {
	out, err := r.read(ctx, record.ValidCommitID(ref), "ls-tree", "-r", ref, "--", prefix)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		meta, path, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("ls-tree line %q", line)
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Type: fields[1], Path: path})
	}
	return entries, nil
}

// Files reads the given paths at ref in one git process (cat-file --batch).
// Missing paths are simply absent from the result. The returned bytes may be
// shared with other reads of the same blob, so callers must not modify them.
func (r *Repo) Files(ctx context.Context, ref string, paths []string) (map[string][]byte, error) {
	if len(paths) == 0 {
		return map[string][]byte{}, nil
	}
	if r.keeps(ctx) && record.ValidCommitID(ref) {
		return r.pinnedFiles(ctx, ref, paths)
	}
	return r.readFiles(ctx, ref, paths)
}

func (r *Repo) readFiles(ctx context.Context, ref string, paths []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	var in bytes.Buffer
	for _, p := range paths {
		in.WriteString(ref + ":" + p + "\n")
	}
	raw, err := r.run(ctx, in.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(raw))
	for _, p := range paths {
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("cat-file output ended early at %s: %w", p, err)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			continue
		}
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("cat-file header %q for %s", strings.TrimSpace(header), p)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("cat-file size %q for %s: %w", fields[2], p, err)
		}
		if size < 0 || size >= len(raw) {
			return nil, fmt.Errorf("cat-file size %d is outside the available output for %s", size, p)
		}
		body := make([]byte, size+1) // content plus the trailing newline cat-file appends
		if _, err := io.ReadFull(rd, body); err != nil {
			return nil, fmt.Errorf("cat-file body for %s: %w", p, err)
		}
		out[p] = body[:size:size] // capped: an append can never write into shared bytes
	}
	return out, nil
}

// TreeSource reads the whole memory tree at a ref, keyed relative to prefix,
// so it can stand in for the working directory as a store source.
type TreeSource struct {
	Repo   *Repo
	Ref    string
	Prefix string // repo-relative memory root, normally "memory"
}

// Files implements the store's Source port: regular blobs are content, a
// symlink or a submodule is irregular.
func (t TreeSource) Files(ctx context.Context) (map[string][]byte, map[string]string, error) {
	entries, err := t.Repo.ListTree(ctx, t.Ref, t.Prefix)
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	irregular := map[string]string{}
	for _, e := range entries {
		rel := strings.TrimPrefix(e.Path, t.Prefix+"/")
		switch {
		case e.Regular():
			paths = append(paths, e.Path)
		case e.Mode == "120000":
			irregular[rel] = "a symlink in git (mode 120000); the memory holds regular files only"
		default:
			irregular[rel] = fmt.Sprintf("git mode %s %s; the memory holds regular files only", e.Mode, e.Type)
		}
	}
	files, err := t.Repo.Files(ctx, t.Ref, paths)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range paths {
		if _, present := files[name]; !present {
			return nil, nil, fmt.Errorf("memory tree at %s lists unavailable blob %s; restore the retained input before checking", t.Ref, name)
		}
	}
	out := make(map[string][]byte, len(files))
	for p, b := range files {
		out[strings.TrimPrefix(p, t.Prefix+"/")] = b
	}
	return out, irregular, nil
}

// UserEmail returns git's configured user.email, or "" when none is set.
func (r *Repo) UserEmail(ctx context.Context) string {
	out, err := r.run(ctx, nil, "config", "user.email")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CommonDir returns the repository's shared git directory (the one worktrees
// of the same repository share), as an absolute path.
func (r *Repo) CommonDir(ctx context.Context) (string, error) {
	out, err := r.run(ctx, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.Root, dir)
	}
	return dir, nil
}

// Head returns the commit HEAD points at.
func (r *Repo) Head(ctx context.Context) (string, error) {
	out, err := r.run(ctx, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
