// Package gitrefstore provides a small compare-and-swap file store on a
// fetched Git ref. Every successful update creates and pushes a new commit;
// readers always fetch the configured ref before exact-reading a file.
package gitrefstore

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const maxCASAttempts = 5

type Store struct {
	mu                     sync.Mutex
	repo                   string
	remote                 string
	ref                    string
	checkpoint             string
	checkpointHead         string
	lastHead               string
	deletionObserved       bool
	nonFastForwardObserved bool
}

// Commit is the exact object/tree/parent view needed by a consumer to prove
// continuity from a protected genesis commit.
type Commit struct {
	Object  string
	Parents []string
	Tree    string
}

// Snapshot is one fetch and one exact tree read. Files never mix generations.
type Snapshot struct {
	Ref                    string
	Head                   string
	CheckpointHead         string
	Files                  map[string][]byte
	Commits                []Commit
	DeletionObserved       bool
	NonFastForwardObserved bool
}

// Mutation describes paths to add or replace in one tree. The callback is
// replayed from a new fetched snapshot after a losing compare-and-swap.
type Mutation struct {
	Files   map[string][]byte
	Changed bool
}

// ApplyResult exact-reads the changed path set from the winning commit. On a
// no-op, Head is the fetched head and Files contains the callback's path set.
type ApplyResult struct {
	Head    string
	Prior   string
	Files   map[string][]byte
	Changed bool
}

// Genesis is the exact root object returned by the one-time initialization
// ceremony. Consumers bind both objects and the authorization reference in
// reviewed policy before any append is authoritative.
type Genesis struct {
	Authorization string
	Commit        string
	Tree          string
}

var ErrCASExhausted = errors.New("git ref store: compare-and-swap retries exhausted")

func New(repo, remote, ref string) (*Store, error) {
	repo, remote, ref = strings.TrimSpace(repo), strings.TrimSpace(remote), strings.TrimSpace(ref)
	if repo == "" || remote == "" || strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, "\x00\r\n") || !validEvidenceRef(ref) {
		return nil, errors.New("git ref store: repo, remote, and a refs/pyxis/* evidence ref are required")
	}
	return &Store{repo: repo, remote: remote, ref: ref}, nil
}

// NewWithCheckpoint atomically advances a second retained ref with every
// successful write. Fetch returns both advertised heads so a consumer can
// detect rollback or deletion across restarts, without relying on lastHead.
// A host that cannot atomically lease both refs refuses publication.
func NewWithCheckpoint(repo, remote, ref, checkpoint string) (*Store, error) {
	store, err := New(repo, remote, ref)
	if err != nil {
		return nil, err
	}
	if !validEvidenceRef(checkpoint) || checkpoint == store.ref {
		return nil, errors.New("git ref store: a distinct refs/pyxis/* checkpoint ref is required")
	}
	store.checkpoint = checkpoint
	return store, nil
}

func validEvidenceRef(ref string) bool {
	if !strings.HasPrefix(ref, "refs/pyxis/") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") || strings.ContainsAny(ref, "\x00 \t\r\n~^:?*[\\") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for component := range strings.SplitSeq(ref, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func (s *Store) Read(ctx context.Context, path string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validPath(path); err != nil {
		return nil, err
	}
	head, err := s.fetchHead(ctx)
	if err != nil || head == "" {
		return nil, err
	}
	return s.readAt(ctx, head, path)
}

// ReadMany fetches once and reads every requested path from the same commit,
// preventing a reducer from joining files from different ref generations.
func (s *Store) ReadMany(ctx context.Context, paths ...string) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, path := range paths {
		if err := validPath(path); err != nil {
			return nil, err
		}
	}
	head, err := s.fetchHead(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := s.readAt(ctx, head, path)
		if err != nil {
			return nil, err
		}
		out[path] = data
	}
	return out, nil
}

// Fetch reads every regular file below prefix and the complete first-parent
// ancestry from the same fetched head. An absent ref is returned as an empty,
// poisoned snapshot after it has previously been observed.
func (s *Store) Fetch(ctx context.Context, prefix string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/")
		if err := validPath(prefix); err != nil {
			return Snapshot{}, err
		}
	}
	return s.fetchSnapshot(ctx, prefix)
}

// ReadExact reads paths from one named commit without consulting a worktree.
// Callers use Fetch first to establish that the commit belongs to the current
// protected history.
func (s *Store) ReadExact(ctx context.Context, commit string, paths ...string) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validObjectID(commit) {
		return nil, fmt.Errorf("git ref store: invalid commit object %q", commit)
	}
	return s.readManyAt(ctx, commit, paths)
}

// FetchBranch imports one published branch into this store's private ref
// without writing FETCH_HEAD. It is intended for caller-owned control
// repositories, never an enrolled worktree.
func (s *Store) FetchBranch(ctx context.Context, branch string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if branch == "" || strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, "\x00\r\n") {
		return "", errors.New("git ref store: valid source branch is required")
	}
	source := "refs/heads/" + branch
	if _, err := s.git(ctx, nil, nil, "check-ref-format", source); err != nil {
		return "", fmt.Errorf("git ref store: invalid source branch %q: %w", branch, err)
	}
	if _, err := s.git(ctx, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--force", s.remote, source+":"+s.ref); err != nil {
		return "", fmt.Errorf("git ref store: fetch published branch %q: %w", branch, err)
	}
	out, err := s.git(ctx, nil, nil, "rev-parse", "--verify", s.ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("git ref store: resolve fetched branch %q: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SnapshotAt exact-reads a tree already proven to belong to a fetched
// protected history. It performs no fetch and therefore cannot silently move
// the caller's requested generation.
func (s *Store) SnapshotAt(ctx context.Context, commit, prefix string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotAt(ctx, commit, prefix)
}

// SnapshotsAt answers each commit exactly as SnapshotAt would. The commits of
// one history share a few Git processes instead of several per commit.
func (s *Store) SnapshotsAt(ctx context.Context, commits []string, prefix string) ([]Snapshot, []error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var listed map[string][]string
	if prefix == "" {
		listed = s.listTrees(ctx, commits)
	}
	var specs []string
	for commit, paths := range listed {
		for _, path := range paths {
			specs = append(specs, commit+":"+path)
		}
	}
	blobs := s.readBlobs(ctx, specs)
	snapshots, errs := make([]Snapshot, len(commits)), make([]error, len(commits))
	for i, commit := range commits {
		paths, found := listed[commit]
		if !found {
			snapshots[i], errs[i] = s.snapshotAt(ctx, commit, prefix)
			continue
		}
		files, err := s.readManyWith(ctx, commit, paths, blobs)
		if err != nil {
			errs[i] = err
			continue
		}
		snapshots[i] = Snapshot{Ref: s.ref, Head: commit, Files: files}
	}
	return snapshots, errs
}

func (s *Store) snapshotAt(ctx context.Context, commit, prefix string) (Snapshot, error) {
	if !validObjectID(commit) {
		return Snapshot{}, fmt.Errorf("git ref store: invalid commit object %q", commit)
	}
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/")
		if err := validPath(prefix); err != nil {
			return Snapshot{}, err
		}
	}
	paths, err := s.listPathsAt(ctx, commit, prefix)
	if err != nil {
		return Snapshot{}, err
	}
	files, err := s.readManyAt(ctx, commit, paths)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Ref: s.ref, Head: commit, Files: files}, nil
}

// EstablishGenesis creates an empty root commit only when the configured ref
// is absent. Remote ref protection remains responsible for limiting who may
// run the ceremony; reviewed consumer policy exact-binds the returned objects
// and authorization before trusting it.
func (s *Store) EstablishGenesis(ctx context.Context, authorization string) (Genesis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	authorization = strings.TrimSpace(authorization)
	if authorization == "" || strings.ContainsAny(authorization, "\x00\r\n") {
		return Genesis{}, errors.New("git ref store: genesis authorization is required")
	}
	head, err := s.fetchHead(ctx)
	if err != nil {
		return Genesis{}, err
	}
	if head != "" || s.checkpointHead != "" || s.deletionObserved {
		return Genesis{}, errors.New("git ref store: genesis requires a never-initialized ref")
	}
	message := "establish protected evidence genesis\n\nauthorization " + authorization
	commit, err := s.commitFiles(ctx, "", nil, message)
	if err != nil {
		return Genesis{}, err
	}
	if err := s.pushCAS(ctx, "", commit); err != nil {
		return Genesis{}, err
	}
	if _, err := s.git(ctx, nil, nil, "update-ref", s.ref, commit); err != nil {
		return Genesis{}, fmt.Errorf("git ref store: update genesis ref: %w", err)
	}
	treeOut, err := s.git(ctx, nil, nil, "show", "-s", "--format=%T", commit)
	if err != nil {
		return Genesis{}, fmt.Errorf("git ref store: read genesis tree: %w", err)
	}
	tree := strings.TrimSpace(string(treeOut))
	if !validObjectID(tree) {
		return Genesis{}, errors.New("git ref store: genesis tree is malformed")
	}
	s.lastHead = commit
	return Genesis{Authorization: authorization, Commit: commit, Tree: tree}, nil
}

// Apply fetches, exact-reads, revalidates through mutate, writes all returned
// paths into one commit, and publishes with bounded compare-and-swap replay.
func (s *Store) Apply(ctx context.Context, prefix, message string, mutate func(Snapshot) (Mutation, error)) (ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mutate == nil {
		return ApplyResult{}, errors.New("git ref store: nil mutation function")
	}
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/")
		if err := validPath(prefix); err != nil {
			return ApplyResult{}, err
		}
	}
	var lastErr error
	for range maxCASAttempts {
		if err := ctx.Err(); err != nil {
			return ApplyResult{}, err
		}
		snapshot, err := s.fetchSnapshot(ctx, prefix)
		if err != nil {
			return ApplyResult{}, err
		}
		mutation, err := mutate(cloneSnapshot(snapshot))
		if err != nil {
			return ApplyResult{}, err
		}
		if len(mutation.Files) == 0 {
			if mutation.Changed {
				return ApplyResult{}, errors.New("git ref store: changed mutation has no files")
			}
			return ApplyResult{Head: snapshot.Head, Prior: snapshot.Head, Files: map[string][]byte{}}, nil
		}
		for path := range mutation.Files {
			if err := validPath(path); err != nil {
				return ApplyResult{}, err
			}
		}
		if !mutation.Changed {
			exact, err := s.readManyAt(ctx, snapshot.Head, sortedPaths(mutation.Files))
			if err != nil {
				return ApplyResult{}, err
			}
			return ApplyResult{Head: snapshot.Head, Prior: snapshot.Head, Files: exact}, nil
		}
		commit, err := s.commitFiles(ctx, snapshot.Head, mutation.Files, message)
		if err != nil {
			return ApplyResult{}, err
		}
		if err := s.pushCAS(ctx, snapshot.Head, commit); err != nil {
			if errors.Is(err, errCASConflict) {
				lastErr = err
				continue
			}
			return ApplyResult{}, err
		}
		if _, err := s.git(ctx, nil, nil, "update-ref", s.ref, commit); err != nil {
			return ApplyResult{}, fmt.Errorf("git ref store: update fetched evidence ref: %w", err)
		}
		s.lastHead = commit
		exact, err := s.readManyAt(ctx, commit, sortedPaths(mutation.Files))
		if err != nil {
			return ApplyResult{}, err
		}
		return ApplyResult{Head: commit, Prior: snapshot.Head, Files: exact, Changed: true}, nil
	}
	return ApplyResult{}, fmt.Errorf("%w: %v", ErrCASExhausted, lastErr)
}

// Update exact-reads path, applies a deterministic mutation, and publishes a
// fast-forward/CAS commit. A stale lease re-fetches and re-applies; an
// unchanged mutation performs no commit or push.
func (s *Store) Update(ctx context.Context, path, message string, mutate func([]byte) ([]byte, bool, error)) ([]byte, error) {
	if err := validPath(path); err != nil {
		return nil, err
	}
	if mutate == nil {
		return nil, errors.New("git ref store: nil update function")
	}
	result, err := s.Apply(ctx, filepath.Dir(path), message, func(snapshot Snapshot) (Mutation, error) {
		current := snapshot.Files[path]
		next, changed, err := mutate(append([]byte(nil), current...))
		if err != nil {
			return Mutation{}, err
		}
		return Mutation{Files: map[string][]byte{path: next}, Changed: changed}, nil
	})
	if err != nil {
		return nil, err
	}
	return result.Files[path], nil
}

func (s *Store) fetchHead(ctx context.Context) (string, error) {
	head, checkpoint, err := s.remoteHeads(ctx)
	if err != nil {
		return "", fmt.Errorf("git ref store: read remote evidence ref %s: %w", s.ref, err)
	}
	s.checkpointHead = checkpoint
	if head == "" {
		if s.lastHead != "" {
			s.deletionObserved = true
		}
		return "", nil
	}
	// A local ref that already names the advertised head makes the fetch a
	// no-op: every read below uses that exact object, which is then present.
	local, err := s.git(ctx, nil, nil, "rev-parse", "--verify", "--quiet", s.ref)
	if err != nil || strings.TrimSpace(string(local)) != head {
		if _, err := s.git(ctx, nil, nil, "fetch", "--quiet", "--no-tags", s.remote, "+"+s.ref+":"+s.ref); err != nil {
			return "", fmt.Errorf("git ref store: fetch %s: %w", s.ref, err)
		}
	}
	if s.lastHead != "" && s.lastHead != head {
		ancestor, err := s.isAncestor(ctx, s.lastHead, head)
		if err != nil {
			return "", err
		}
		if !ancestor {
			s.nonFastForwardObserved = true
		}
	}
	s.lastHead = head
	return head, nil
}

func (s *Store) fetchSnapshot(ctx context.Context, prefix string) (Snapshot, error) {
	head, err := s.fetchHead(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		Ref: s.ref, Head: head, Files: map[string][]byte{},
		CheckpointHead:   s.checkpointHead,
		DeletionObserved: s.deletionObserved, NonFastForwardObserved: s.nonFastForwardObserved,
	}
	if head == "" {
		return snapshot, nil
	}
	paths, err := s.listPathsAt(ctx, head, prefix)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Files, err = s.readManyAt(ctx, head, paths)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Commits, err = s.history(ctx, head)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) listPathsAt(ctx context.Context, head, prefix string) ([]string, error) {
	args := []string{"ls-tree", "-r", "--name-only", "-z", head}
	if prefix != "" {
		args = append(args, "--", prefix)
	}
	out, err := s.git(ctx, nil, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("git ref store: list evidence tree at %s: %w", head, err)
	}
	parts := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := string(part)
		if err := validPath(path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *Store) readManyAt(ctx context.Context, head string, paths []string) (map[string][]byte, error) {
	if head == "" {
		return make(map[string][]byte), nil
	}
	if !validObjectID(head) {
		return nil, fmt.Errorf("git ref store: invalid commit object %q", head)
	}
	var specs []string
	for _, path := range paths {
		if validPath(path) != nil {
			break
		}
		specs = append(specs, head+":"+path)
	}
	return s.readManyWith(ctx, head, paths, s.readBlobs(ctx, specs))
}

// readManyWith reads paths at head, taking each blob already read in a batch.
// Absent, non-blob, and unreadable entries keep the exact two-step read and
// its diagnostics.
func (s *Store) readManyWith(ctx context.Context, head string, paths []string, blobs map[string][]byte) (map[string][]byte, error) {
	out := make(map[string][]byte, len(paths))
	for _, path := range paths {
		if err := validPath(path); err != nil {
			return nil, err
		}
		data, found := blobs[head+":"+path]
		if !found {
			var err error
			data, err = s.readAt(ctx, head, path)
			if err != nil {
				return nil, err
			}
		}
		if data == nil {
			return nil, fmt.Errorf("git ref store: exact path %s is absent at %s", path, head)
		}
		out[path] = data
	}
	return out, nil
}

// readBlobs reads every "<commit>:<path>" spec that names a blob through one
// cat-file process. Any other answer, or a failed batch, reads nothing.
func (s *Store) readBlobs(ctx context.Context, specs []string) map[string][]byte {
	if len(specs) == 0 {
		return nil
	}
	out, err := s.catFile(ctx, specs)
	if err != nil {
		return nil
	}
	return parseBlobs(out, specs)
}

func (s *Store) catFile(ctx context.Context, specs []string) ([]byte, error) {
	var input bytes.Buffer
	for _, spec := range specs {
		input.WriteString(spec + "\n")
	}
	return s.git(ctx, nil, input.Bytes(), "cat-file", "--batch")
}

// parseBlobs maps each spec naming a blob to its body. Malformed output yields
// no blobs at all.
func parseBlobs(out []byte, specs []string) map[string][]byte {
	objects, ok := parseBatch(out, len(specs))
	if !ok {
		return nil
	}
	blobs := make(map[string][]byte, len(specs))
	for i, object := range objects {
		if object.Type == "blob" {
			// An empty blob stays nil, like the exact read's empty output.
			blobs[specs[i]] = append([]byte(nil), object.Body...)
		}
	}
	return blobs
}

type batchObject struct {
	Object, Type string
	Body         []byte
}

// parseBatch splits cat-file --batch output answering count names in order.
// Git answers an unresolved name with "<name> missing" and no body; that name
// gets an empty answer.
func parseBatch(out []byte, count int) ([]batchObject, bool) {
	objects := make([]batchObject, 0, count)
	for range count {
		header, rest, found := bytes.Cut(out, []byte{'\n'})
		if !found {
			return nil, false
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 || !validObjectID(fields[0]) {
			objects = append(objects, batchObject{})
			out = rest
			continue
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size >= len(rest) || rest[size] != '\n' {
			return nil, false
		}
		objects = append(objects, batchObject{Object: fields[0], Type: fields[1], Body: rest[:size]})
		out = rest[size+1:]
	}
	return objects, len(out) == 0
}

// listTrees lists every non-tree entry below each commit, as "ls-tree -r
// --name-only" does, through one cat-file process per tree depth. A commit
// whose trees hold anything unusual is left out, so that ls-tree itself lists
// or refuses it.
func (s *Store) listTrees(ctx context.Context, commits []string) map[string][]string {
	type pending struct{ commit, dir, object string }
	listed := make(map[string][]string)
	var queue []pending
	for _, commit := range commits {
		if _, seen := listed[commit]; validObjectID(commit) && !seen {
			listed[commit] = []string{}
			queue = append(queue, pending{commit: commit, object: commit + "^{tree}"})
		}
	}
	unusual := make(map[string]bool)
	for len(queue) > 0 {
		specs := make([]string, len(queue))
		for i, p := range queue {
			specs[i] = p.object
		}
		out, err := s.catFile(ctx, specs)
		if err != nil {
			return nil
		}
		objects, ok := parseBatch(out, len(specs))
		if !ok {
			return nil
		}
		var next []pending
		for i, p := range queue {
			entries, ok := parseTree(objects[i])
			if unusual[p.commit] || !ok {
				unusual[p.commit] = true
				continue
			}
			for _, entry := range entries {
				path := entry.name
				if p.dir != "" {
					path = p.dir + "/" + path
				}
				if entry.tree {
					next = append(next, pending{commit: p.commit, dir: path, object: entry.object})
				} else {
					listed[p.commit] = append(listed[p.commit], path)
				}
			}
		}
		queue = next
	}
	for commit, paths := range listed {
		if unusual[commit] || slices.ContainsFunc(paths, func(path string) bool { return validPath(path) != nil }) {
			delete(listed, commit)
			continue
		}
		sort.Strings(paths)
	}
	return listed
}

type treeEntry struct {
	name, object string
	tree         bool
}

// parseTree decodes a tree object, accepting only the canonical modes Git
// writes and plain, unique entry names.
func parseTree(object batchObject) ([]treeEntry, bool) {
	if object.Type != "tree" {
		return nil, false
	}
	size := len(object.Object) / 2 // the repository's binary object ID length
	var entries []treeEntry
	names := make(map[string]bool)
	for body := object.Body; len(body) > 0; {
		mode, rest, found := bytes.Cut(body, []byte{' '})
		if !found {
			return nil, false
		}
		name, rest, found := bytes.Cut(rest, []byte{0})
		if !found || len(rest) < size || len(name) == 0 || string(name) == "." || string(name) == ".." || bytes.ContainsRune(name, '/') || names[string(name)] {
			return nil, false
		}
		entry := treeEntry{name: string(name), object: hex.EncodeToString(rest[:size])}
		switch string(mode) {
		case "40000":
			entry.tree = true
		case "100644", "100755", "120000", "160000":
		default:
			return nil, false
		}
		names[entry.name] = true
		entries = append(entries, entry)
		body = rest[size:]
	}
	return entries, true
}

func (s *Store) history(ctx context.Context, head string) ([]Commit, error) {
	// One walk reports every commit's parents and tree: each commit is
	// headed by "commit <object> <parents...>" and followed by its tree.
	out, err := s.git(ctx, nil, nil, "rev-list", "--reverse", "--parents", "--format=%T", head)
	if err != nil {
		return nil, fmt.Errorf("git ref store: read ancestry at %s: %w", head, err)
	}
	return parseHistory(head, out)
}

func parseHistory(head string, out []byte) ([]Commit, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines)%2 != 0 {
		return nil, fmt.Errorf("git ref store: malformed ancestry at %s", head)
	}
	commits := make([]Commit, 0, len(lines)/2)
	for i := 0; i < len(lines); i += 2 {
		fields := strings.Fields(lines[i])
		if len(fields) < 2 || fields[0] != "commit" || !validObjectID(fields[1]) {
			return nil, fmt.Errorf("git ref store: malformed ancestry at %s", head)
		}
		for _, parent := range fields[2:] {
			if !validObjectID(parent) {
				return nil, fmt.Errorf("git ref store: malformed parent at %s", fields[1])
			}
		}
		tree := strings.TrimSpace(lines[i+1])
		if !validObjectID(tree) {
			return nil, fmt.Errorf("git ref store: malformed tree for %s", fields[1])
		}
		commits = append(commits, Commit{Object: fields[1], Parents: append([]string(nil), fields[2:]...), Tree: tree})
	}
	return commits, nil
}

func (s *Store) isAncestor(ctx context.Context, ancestor, head string) (bool, error) {
	_, err := s.git(ctx, nil, nil, "merge-base", "--is-ancestor", ancestor, head)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git ref store: compare observed ancestry: %w", err)
}

func (s *Store) readAt(ctx context.Context, head, path string) ([]byte, error) {
	if head == "" {
		return nil, nil
	}
	listed, err := s.git(ctx, nil, nil, "ls-tree", head, "--", path)
	if err != nil {
		return nil, fmt.Errorf("git ref store: inspect %s at %s: %w", path, head, err)
	}
	if len(bytes.TrimSpace(listed)) == 0 {
		return nil, nil
	}
	out, err := s.git(ctx, nil, nil, "show", head+":"+path)
	if err != nil {
		return nil, fmt.Errorf("git ref store: exact-read %s at %s: %w", path, head, err)
	}
	return out, nil
}

func (s *Store) commitFile(ctx context.Context, parent, path string, content []byte, message string) (string, error) {
	return s.commitFiles(ctx, parent, map[string][]byte{path: content}, message)
}

func (s *Store) commitFiles(ctx context.Context, parent string, files map[string][]byte, message string) (string, error) {
	if len(files) == 0 && parent != "" {
		return "", errors.New("git ref store: cannot commit an empty path set")
	}
	if parent != "" && !validObjectID(parent) {
		return "", fmt.Errorf("git ref store: invalid parent object %q", parent)
	}
	f, err := os.CreateTemp(os.TempDir(), "pyxis-gitref-index-*")
	if err != nil {
		return "", fmt.Errorf("git ref store: create temporary index: %w", err)
	}
	indexPath := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(indexPath); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(indexPath) }() // Best-effort cleanup of the private temporary index.
	env := []string{"GIT_INDEX_FILE=" + indexPath, "GIT_AUTHOR_NAME=pyxis-drive", "GIT_AUTHOR_EMAIL=pyxis-drive@invalid", "GIT_COMMITTER_NAME=pyxis-drive", "GIT_COMMITTER_EMAIL=pyxis-drive@invalid"}
	if parent == "" {
		if _, err := s.git(ctx, env, nil, "read-tree", "--empty"); err != nil {
			return "", fmt.Errorf("git ref store: initialize tree: %w", err)
		}
	} else if _, err := s.git(ctx, env, nil, "read-tree", parent); err != nil {
		return "", fmt.Errorf("git ref store: read evidence tree: %w", err)
	}
	for _, path := range sortedPaths(files) {
		if err := validPath(path); err != nil {
			return "", err
		}
		blobOut, err := s.git(ctx, env, files[path], "hash-object", "-w", "--stdin")
		if err != nil {
			return "", fmt.Errorf("git ref store: write evidence blob: %w", err)
		}
		blob := strings.TrimSpace(string(blobOut))
		if _, err := s.git(ctx, env, nil, "update-index", "--add", "--cacheinfo", "100644", blob, path); err != nil {
			return "", fmt.Errorf("git ref store: stage evidence blob: %w", err)
		}
	}
	treeOut, err := s.git(ctx, env, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("git ref store: write evidence tree: %w", err)
	}
	args := []string{"commit-tree", strings.TrimSpace(string(treeOut))}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", strings.TrimSpace(message))
	commitOut, err := s.git(ctx, env, nil, args...)
	if err != nil {
		return "", fmt.Errorf("git ref store: create evidence commit: %w", err)
	}
	return strings.TrimSpace(string(commitOut)), nil
}

var errCASConflict = errors.New("git ref store: compare-and-swap conflict")

func (s *Store) pushCAS(ctx context.Context, prior, commit string) error {
	lease := "--force-with-lease=" + s.ref + ":" + prior
	args := []string{"push", "--porcelain", lease}
	if s.checkpoint != "" {
		args = append(args, "--atomic", "--force-with-lease="+s.checkpoint+":"+s.checkpointHead)
	}
	args = append(args, s.remote, commit+":"+s.ref)
	if s.checkpoint != "" {
		args = append(args, commit+":"+s.checkpoint)
	}
	out, err := s.git(ctx, nil, nil, args...)
	if err != nil {
		remote, checkpoint, readErr := s.remoteHeads(ctx)
		if readErr == nil && (remote != prior || checkpoint != s.checkpointHead) {
			return fmt.Errorf("%w: expected %s, found %s", errCASConflict, prior, remote)
		}
		return fmt.Errorf("publish %s: %w: %s", s.ref, err, strings.TrimSpace(string(out)))
	}
	// Git reports success without checking the stale lease when another writer
	// has already published the exact same commit. Porcelain marks that no-op
	// with '='; it did not move the ref, so this caller did not win the CAS.
	for line := range bytes.SplitSeq(out, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("=\t")) {
			return fmt.Errorf("%w: expected %s, found %s", errCASConflict, prior, commit)
		}
	}
	if s.checkpoint != "" {
		s.checkpointHead = commit
	}
	return nil
}

func (s *Store) remoteHead(ctx context.Context) (string, error) {
	head, _, err := s.remoteHeads(ctx)
	return head, err
}

func (s *Store) remoteHeads(ctx context.Context) (string, string, error) {
	args := []string{"ls-remote", "--exit-code", s.remote, s.ref}
	if s.checkpoint != "" {
		args = append(args, s.checkpoint)
	}
	out, err := s.git(ctx, nil, nil, args...)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 2 {
			return "", "", nil
		}
		return "", "", err
	}
	heads := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !validObjectID(fields[0]) ||
			(fields[1] != s.ref && (s.checkpoint == "" || fields[1] != s.checkpoint)) || heads[fields[1]] != "" {
			return "", "", fmt.Errorf("git ref store: malformed remote heads for %s", s.ref)
		}
		heads[fields[1]] = fields[0]
	}
	return heads[s.ref], heads[s.checkpoint], nil
}

func (s *Store) git(ctx context.Context, extraEnv []string, stdin []byte, args ...string) ([]byte, error) {
	// This store accepts only refs/pyxis/* evidence. Its supervisor-owned CAS
	// commits must not execute repository hooks outside verification or undergo
	// branch content gates. Keep the override scoped to this subprocess.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", s.repo}, args...)...)
	cmd.Env = append(gitEnvironment(), extraEnv...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	// Evidence bytes and object identities come from stdout only. Git may write
	// harmless advice or warnings to stderr even on exit 0; CombinedOutput would
	// splice those bytes into JSONL evidence reads and hashes.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return stdout.Bytes(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, message)
		}
		return stdout.Bytes(), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// gitEnvironment preserves transport authentication and non-interactive
// controls while removing only variables that can redirect the repository,
// object database, index, namespace, or configuration used by canonical
// evidence reads and writes.
func gitEnvironment() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if gitRedirectVariable(upper) {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0")
	return env
}

func gitRedirectVariable(key string) bool {
	switch key {
	case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_NAMESPACE",
		"GIT_CEILING_DIRECTORIES", "GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
		"GIT_CONFIG_PARAMETERS", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR", "GIT_REPLACE_REF_BASE",
		"GIT_NO_REPLACE_OBJECTS", "GIT_OPTIONAL_LOCKS":
		return true
	default:
		return strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_")
	}
}

func cloneSnapshot(in Snapshot) Snapshot {
	out := in
	out.Files = make(map[string][]byte, len(in.Files))
	for path, content := range in.Files {
		out.Files[path] = append([]byte(nil), content...)
	}
	out.Commits = append([]Commit(nil), in.Commits...)
	for i := range out.Commits {
		out.Commits[i].Parents = append([]string(nil), in.Commits[i].Parents...)
	}
	return out
}

func sortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func validObjectID(object string) bool {
	if len(object) != 40 && len(object) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(object)
	return err == nil && hex.EncodeToString(decoded) == object
}

func validPath(path string) error {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "../") || strings.ContainsAny(path, "\x00\r\n") {
		return fmt.Errorf("git ref store: invalid evidence path %q", path)
	}
	return nil
}
