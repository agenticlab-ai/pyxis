// Package diffset extracts the proposal diff (resolved diff base → head)
// once into a shared, in-memory shape: file statuses with rename detection,
// per-file byte size, binary detection, and the exact added lines with their
// new-file line numbers. Rung 0's neutral checks (secret scan,
// forbidden-artifact hygiene, machinery protection — design codegate-v1
// §3.7) all read the same DiffSet rather than each re-running git.
package diffset

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Status classifies how a path changed between the diff base and head.
type Status string

const (
	StatusAdded    Status = "added"
	StatusModified Status = "modified"
	StatusDeleted  Status = "deleted"
	StatusRenamed  Status = "renamed"
)

// AddedLine is one line introduced by the diff, at its line number in the
// file as it exists at head (1-based, matching the finding schema's Line
// field).
type AddedLine struct {
	Line    int
	Content string
}

// FileChange is one file's diff entry.
type FileChange struct {
	Status Status
	// Path is the file's location at head: the new path for an add, modify,
	// or rename, and the (now-gone) path for a delete.
	Path string
	// OldPath is set only when Status == StatusRenamed.
	OldPath string
	// Binary reports whether git treated this file as binary content —
	// unset (false) and meaningless for a deletion.
	Binary bool
	// SizeBytes is the file's size at head — unset (0) for a deletion.
	SizeBytes int64
	// AddedLines are the lines this diff introduced, empty for a deletion or
	// a pure rename with no content change.
	AddedLines []AddedLine
}

// DiffSet is the proposal diff, extracted once and shared across checks.
type DiffSet struct {
	Base  string
	Head  string
	Files []FileChange
}

// TouchedPaths returns every path this diff references at either endpoint —
// the path at head for every change, plus the old path for a rename —
// deduplicated. Machinery protection matches against this set: a rename
// either into or out of a protected path is gate-gaming either direction.
func (ds *DiffSet) TouchedPaths() []string {
	seen := make(map[string]bool, len(ds.Files)*2)
	var paths []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}
	for _, f := range ds.Files {
		add(f.Path)
		add(f.OldPath)
	}
	return paths
}

// ContentIntroduced reports whether this file entry carries new content at
// head. A deletion never does. An addition always does — the whole file is
// new content, even when the content pass parsed no added lines (an empty or
// newline-free blob), so the size and forbidden floors still see it. A modify
// or rename carries new content only when the change actually changed content
// (binary or added lines): a pure rename, and a content-preserving modify
// (git reports status M for a mode-only change like chmod +x, with zero hunks
// and no binary marker), both report none, so diff-scoped checks have nothing
// to say about them.
func (f *FileChange) ContentIntroduced() bool {
	switch f.Status {
	case StatusDeleted:
		return false
	case StatusAdded:
		return true
	default: // modified or renamed
		return f.Binary || len(f.AddedLines) > 0
	}
}

// Extract resolves the diff between base and head inside workspace: an
// authoritative name-status pass (rename detection via -M) followed by a
// per-file content pass (added lines, binary detection) and a single batched
// size lookup. Two to N+2 git invocations total, where N is the number of
// non-deleted changed files. Content passes stay isolated by file so a parser
// bug can never misattribute one file's hunks to another, but a small fixed
// worker pool runs them concurrently so a wide proposal does not pay N
// subprocess latencies in series. The scanner's correctness and bounded
// resource use are both security-relevant: this is an anti-gaming check.
func Extract(ctx context.Context, workspace, base, head string) (*DiffSet, error) {
	files, err := nameStatus(ctx, workspace, base, head)
	if err != nil {
		return nil, fmt.Errorf("diffset: %w", err)
	}

	sizePaths := make([]string, 0, len(files))
	for i := range files {
		if files[i].Status == StatusDeleted {
			continue
		}
		sizePaths = append(sizePaths, files[i].Path)
	}
	if err := populateContent(ctx, workspace, base, head, files, fileDiff); err != nil {
		return nil, fmt.Errorf("diffset: %w", err)
	}

	sizes, err := batchSizes(ctx, workspace, head, sizePaths)
	if err != nil {
		return nil, fmt.Errorf("diffset: %w", err)
	}
	for i := range files {
		if sz, ok := sizes[files[i].Path]; ok {
			files[i].SizeBytes = sz
		}
	}

	return &DiffSet{Base: base, Head: head, Files: files}, nil
}

const maxConcurrentFileDiffs = 8

type fileDiffFunc func(context.Context, string, string, string, []string) (bool, []AddedLine, error)

type fileDiffResult struct {
	binary bool
	added  []AddedLine
}

// populateContent keeps fileDiff's one-path parsing boundary while bounding
// subprocess fan-out. Results are written into their original indexes, so
// completion order cannot affect DiffSet ordering. The first error cancels
// in-flight siblings and prevents queued work from starting; all workers are
// joined before the error is returned.
func populateContent(ctx context.Context, workspace, base, head string, files []FileChange, diff fileDiffFunc) error {
	var indexes []int
	for i := range files {
		if files[i].Status != StatusDeleted {
			indexes = append(indexes, i)
		}
	}
	if len(indexes) == 0 {
		return nil
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int, len(indexes))
	for _, i := range indexes {
		jobs <- i
	}
	close(jobs)

	results := make([]fileDiffResult, len(files))
	workerCount := min(maxConcurrentFileDiffs, len(indexes))
	var workers sync.WaitGroup
	var failOnce sync.Once
	var firstErr error
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for i := range jobs {
				if workCtx.Err() != nil {
					return
				}
				f := files[i]
				pathspecs := []string{f.Path}
				if f.Status == StatusRenamed && f.OldPath != "" && f.OldPath != f.Path {
					pathspecs = []string{f.OldPath, f.Path}
				}
				binary, added, err := diff(workCtx, workspace, base, head, pathspecs)
				if err != nil {
					failOnce.Do(func() {
						firstErr = fmt.Errorf("path %q: %w", f.Path, err)
						cancel()
					})
					return
				}
				results[i] = fileDiffResult{binary: binary, added: added}
			}
		}()
	}
	workers.Wait()

	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, i := range indexes {
		files[i].Binary = results[i].binary
		files[i].AddedLines = results[i].added
	}
	return nil
}

// nameStatus runs the authoritative, NUL-delimited status+rename pass.
func nameStatus(ctx context.Context, workspace, base, head string) ([]FileChange, error) {
	out, err := runGit(ctx, workspace, "diff", "--no-color", "-M", "--name-status", "-z", base, head)
	if err != nil {
		return nil, fmt.Errorf("name-status: %w", err)
	}

	tokens := strings.Split(out, "\x00")
	var files []FileChange
	for i := 0; i < len(tokens); {
		tok := tokens[i]
		if tok == "" {
			i++
			continue
		}
		i++
		if i >= len(tokens) {
			return nil, fmt.Errorf("name-status: status token %q has no following path", tok)
		}
		switch code := tok[0]; code {
		case 'A':
			files = append(files, FileChange{Status: StatusAdded, Path: tokens[i]})
			i++
		case 'D':
			files = append(files, FileChange{Status: StatusDeleted, Path: tokens[i]})
			i++
		case 'R', 'C':
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("name-status: rename/copy token %q missing its path pair", tok)
			}
			old, nw := tokens[i], tokens[i+1]
			st := StatusRenamed
			if code == 'C' {
				// Copy detection is never requested (-M only); defensive only.
				st = StatusAdded
			}
			files = append(files, FileChange{Status: st, OldPath: old, Path: nw})
			i += 2
		case 'M', 'T', 'U', 'X', 'B':
			// M is the common case; T/U/X/B (typechange, unmerged, unknown,
			// pairing-broken) are rare on a clean two-commit diff but are
			// scanned conservatively as content-modified rather than
			// rejected outright — a check that errors out on an unusual but
			// legitimate status is worse than one that just scans it.
			files = append(files, FileChange{Status: StatusModified, Path: tokens[i]})
			i++
		default:
			return nil, fmt.Errorf("name-status: unrecognized status token %q", tok)
		}
	}
	return files, nil
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// fileDiff runs the per-file content pass: binary detection and added lines
// with their new-file line numbers, over --unified=0 output so only changed
// lines appear (no context to filter out).
//
// --text (-a) forces git's textual diff algorithm on every blob, bypassing
// the "diff" gitattribute's binary override. Without it, a proposal diff can
// pair an added `path -diff` .gitattributes entry with genuinely textual
// added content (a live secret, a baked absolute path) in the SAME diff:
// git then prints only "Binary files ... differ" for that path and no
// hunks, leaving secret-scan and forbidden-artifacts' absolute-path floor
// with zero added lines to match against — an attribute-driven blind spot
// in checks whose whole purpose is resisting an adversarial diff, reproduced
// against real git (a `.gitattributes` entry alone, no non-text bytes
// required, forces the binary branch). Binary detection below is done on
// the resulting content instead of trusting git's attribute-aware
// self-report; the "Binary files" branch stays as a defensive fallback for
// content --text still can't render at all (e.g. over core.bigFileThreshold).
func fileDiff(ctx context.Context, workspace, base, head string, pathspecs []string) (binary bool, added []AddedLine, err error) {
	args := []string{"-c", "core.quotePath=false", "diff", "--no-color", "--text", "-M", "--unified=0", base, head, "--"}
	args = append(args, pathspecs...)
	out, err := runGit(ctx, workspace, args...)
	if err != nil {
		return false, nil, fmt.Errorf("content diff: %w", err)
	}

	newLine := 0
	inHunk := false
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			// A new file section begins; leave any prior hunk context so this
			// file's "--- a/…" / "+++ b/…" headers are never read as content.
			inHunk = false
		case strings.HasPrefix(l, "Binary files ") && strings.HasSuffix(l, " differ"):
			return true, nil, nil
		case strings.HasPrefix(l, "@@ "):
			m := hunkHeaderRe.FindStringSubmatch(l)
			if m == nil {
				return false, nil, fmt.Errorf("content diff: unparseable hunk header %q", l)
			}
			n, convErr := strconv.Atoi(m[1])
			if convErr != nil {
				return false, nil, fmt.Errorf("content diff: bad hunk line number %q: %w", m[1], convErr)
			}
			newLine = n
			inHunk = true
		case inHunk && strings.HasPrefix(l, "+"):
			// Inside a hunk every '+' line is added content — including one
			// whose own text starts with '+' (git renders an added "++foo"
			// line as "+++foo"). The file's "+++ b/…" header is excluded by
			// inHunk (it precedes the first @@, and the diff --git reset above
			// clears the flag between files), so there is NO "+++"-prefix guard
			// here: one would silently drop a "++"-prefixed secret line.
			content := strings.TrimSuffix(l[1:], "\r") // tolerate CRLF line endings
			// A NUL byte in the added content is git's own default
			// (attribute-free) binary heuristic, applied to content git already
			// decoded for us — never re-derived from a second blob fetch.
			if strings.IndexByte(content, 0) >= 0 {
				binary = true
			}
			added = append(added, AddedLine{Line: newLine, Content: content})
			newLine++
		case inHunk && strings.HasPrefix(l, "-"):
			// A removed line does not consume a new-file line number.
		}
	}
	if binary {
		// Binary content's "lines" are an artifact of incidental 0x0A bytes,
		// never meaningful text — matches the "Binary files" branch's
		// contract (FileChange.AddedLines empty for binary content) and
		// keeps secret-scan/forbidden-artifacts' absolute-path floor from
		// pattern-matching raw binary noise.
		added = nil
	}
	return binary, added, nil
}

// batchSizes looks up every path's byte size at head in one process via
// `git cat-file --batch-check`, preserving input order to map results back
// to paths without needing %(rest) tricks.
func batchSizes(ctx context.Context, workspace, head string, paths []string) (map[string]int64, error) {
	if len(paths) == 0 {
		return map[string]int64{}, nil
	}

	var stdin bytes.Buffer
	for _, p := range paths {
		stdin.WriteString(head)
		stdin.WriteString(":")
		stdin.WriteString(p)
		stdin.WriteString("\n")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "cat-file", "--batch-check=%(objectsize)")
	cmd.Env = gitBaseEnv()
	cmd.Stdin = &stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cat-file --batch-check: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != len(paths) {
		return nil, fmt.Errorf("cat-file --batch-check: expected %d result(s), got %d", len(paths), len(lines))
	}
	sizes := make(map[string]int64, len(paths))
	for i, line := range lines {
		if strings.HasSuffix(line, " missing") {
			// A gitlink (submodule) pointer resolves head:<path> to a commit
			// object that lives in the submodule's own store, not here, so
			// cat-file reports it missing. A submodule has no file size to
			// floor — skip it (leaving SizeBytes 0) rather than failing the
			// whole extraction on any submodule-touching proposal.
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cat-file --batch-check: unparseable size %q for %q: %w", line, paths[i], err)
		}
		sizes[paths[i]] = n
	}
	return sizes, nil
}

// gitBaseEnv is the hermetic, non-interactive environment every diffset git
// call runs under: no credential prompts, no optional locks, all pathspecs
// literal (a tracked file named ":!x" or ":(glob)" is a path, never a pathspec
// expression that would return an unrelated file's diff), and ambient
// system/global config ignored (so a user's diff.renames=copies — which would
// make a copy FROM a protected path false-fire machinery-protection — or any
// other ambient setting can't alter what the scanners see). The workspace's own
// local config still applies.
func gitBaseEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_LITERAL_PATHSPECS=1",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
	)
}

// runGit runs a git subcommand with credential prompts and optional locks
// disabled, returning raw stdout (not trimmed — callers that split on
// newlines or NUL need the exact byte stream) and wrapping stderr into the
// error.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = gitBaseEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}
