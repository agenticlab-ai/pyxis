// Package checks holds codegate's self-registered check units — one
// self-contained file per check, init()-registered into the harness's own
// registry (design:codegate-v1#3.7, AP-27).
package checks

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func init() {
	register(schema.Check{
		ID:                "test-delta",
		Rung:              "3",
		AuthorIndependent: true,
		InputSet:          []string{"diff"},
	}, testDeltaCheck)
}

// testDeltaCheck is the ladder-dispatched executor for the test-delta audit: it
// resolves the stack's tables, extracts the test-file deltas across the proposal
// diff (its own extraction — the content floors need the full pre/post blob
// content the shared DiffSet does not carry), and runs the pure TestDelta.
func testDeltaCheck(ctx context.Context, rc RunContext) Outcome {
	const id = "test-delta"
	if rc.Manifest == nil {
		return Outcome{Result: schema.CheckResult{CheckID: id, State: "error", Message: "no manifest available"}}
	}
	tables, err := TablesForStack(rc.Ecosystem, rc.Manifest.Stack, rc.Manifest.TestGlobs)
	if err != nil {
		return Outcome{Result: schema.CheckResult{CheckID: id, State: "error", Message: err.Error()}}
	}
	deltas, err := ExtractFileDeltas(ctx, rc.Workspace, rc.Bindings.DiffBase, rc.Bindings.HeadSHA, tables.TestGlobs)
	if err != nil {
		return Outcome{Result: schema.CheckResult{CheckID: id, State: "error", Message: fmt.Sprintf("extract test-file deltas: %v", err)}}
	}
	findings := TestDelta(deltas, tables)
	result := schema.CheckResult{CheckID: id, State: "pass"}
	if len(findings) > 0 {
		result.State = "fail"
		result.Message = fmt.Sprintf("%d gutted-test finding(s)", len(findings))
	}
	return Outcome{Result: result, Findings: findings}
}

// DeltaStatus is one file's status in a proposal diff, with git's rename
// detection (-M) already resolved: a genuine move/rename is DeltaRenamed, not
// a DeltaDeleted+DeltaAdded pair (design:codegate-v1#3.9 — "-M runs before
// any floor fires").
type DeltaStatus string

const (
	DeltaAdded    DeltaStatus = "added"
	DeltaDeleted  DeltaStatus = "deleted"
	DeltaModified DeltaStatus = "modified"
	DeltaRenamed  DeltaStatus = "renamed"
)

// FileDelta is one file's change in a proposal diff, restricted to what the
// test-delta audit needs: its status, current and (for a rename) prior path,
// and the before/after blob content the skip-marker and assertion-pattern
// floors compare. OldContent is nil for DeltaAdded; NewContent is nil for
// DeltaDeleted.
type FileDelta struct {
	Status     DeltaStatus
	Path       string
	OldPath    string
	OldContent []byte
	NewContent []byte
}

// Tables are the test-delta audit's per-stack ecosystem-table inputs: the
// effective test globs and the skip-marker / assertion-pattern regexp sets.
// Growing a pattern table is a change to the value a caller builds, never to
// TestDelta itself (plan acceptance: pattern-table additions require zero
// changes to the check unit).
type Tables struct {
	TestGlobs         []string
	SkipMarkers       []*regexp.Regexp
	AssertionPatterns []*regexp.Regexp
}

// TablesForStack resolves Tables for one stack from the compiled ecosystem
// defaults, honoring a manifest test-glob override via the existing
// ecosystem.Defaults merge rule. A stack with no registered pattern tables is a
// hard error: returning empty tables would silently disable the skip and
// assertion floors (a hollowed test would pass clean), so an unsupported or
// misspelled stack fails closed rather than fails open.
func TablesForStack(d *ecosystem.Defaults, stack string, manifestTestGlobs []string) (Tables, error) {
	skip, ok := d.SkipMarkers[stack]
	if !ok {
		return Tables{}, fmt.Errorf("test-delta: unsupported stack %q (no skip/assertion pattern tables registered)", stack)
	}
	testGlobs, _ := d.ResolveManifestDefaults(stack, manifestTestGlobs, nil)
	return Tables{
		TestGlobs:         testGlobs,
		SkipMarkers:       skip,
		AssertionPatterns: d.AssertionPatterns[stack],
	}, nil
}

// TestDelta evaluates the three test-delta floors (design:codegate-v1#3.9,
// basis FR-12) over a proposal diff's file changes: a test-glob path deleted,
// a skip/exclusion marker added, or a modified test's assertion-pattern count
// dropping. Rename detection has already run by the time a FileDelta reaches
// here (ExtractFileDeltas resolves it via git's -M), so a pure move never
// carries DeltaDeleted and never fires the deletion floor — but a rename that
// also hollows out its test content still fires the content floors, exactly
// as a same-path modification would. TestDelta is pure: no IO, no git.
func TestDelta(changes []FileDelta, tables Tables) []schema.Finding {
	var findings []schema.Finding
	globs := compileGlobs(tables.TestGlobs)

	for _, d := range changes {
		newInScope := anyGlobMatch(globs, d.Path)

		// A rename that leaves the test set is a disguised deletion: the file no
		// longer compiles under `go test`, so every test in it stops running.
		// Caught here on the old path because neither the deletion floor nor the
		// content floors below would otherwise fire — both key on the new path.
		if d.Status == DeltaRenamed && !newInScope && anyGlobMatch(globs, d.OldPath) {
			findings = append(findings, schema.Finding{
				CheckID: "test-delta",
				Class:   "test-renamed-out",
				Rung:    "3",
				Path:    d.OldPath,
				Message: fmt.Sprintf("test file renamed out of the test set: %s -> %s", d.OldPath, d.Path),
			})
			continue
		}

		if !newInScope {
			continue
		}

		if d.Status == DeltaDeleted {
			findings = append(findings, schema.Finding{
				CheckID: "test-delta",
				Class:   "test-deleted",
				Rung:    "3",
				Path:    d.Path,
				Message: fmt.Sprintf("test file deleted: %s", d.Path),
			})
			continue
		}

		// The content floors compare a test against its own prior version, so
		// they apply only when the file already existed AS A TEST: a
		// modification, or a rename that stayed within the test set. A
		// newly-added test — a plain add, or a rename INTO the set from a
		// non-test path — has no test pre-image, so a legitimate skip guard
		// (`if testing.Short() { t.Skip() }`) or its assertions are original,
		// not a regression, and must not fire the floors.
		existedAsTest := d.Status == DeltaModified || (d.Status == DeltaRenamed && anyGlobMatch(globs, d.OldPath))
		if !existedAsTest {
			continue
		}

		oldSkips, newSkips := countMatches(tables.SkipMarkers, d.OldContent), countMatches(tables.SkipMarkers, d.NewContent)
		if newSkips > oldSkips {
			findings = append(findings, schema.Finding{
				CheckID: "test-delta",
				Class:   "skip-marker-added",
				Rung:    "3",
				Path:    d.Path,
				Line:    firstMatchLine(tables.SkipMarkers, d.NewContent),
				Message: fmt.Sprintf("skip marker added to test: %s", d.Path),
			})
		}

		oldAsserts, newAsserts := countMatches(tables.AssertionPatterns, d.OldContent), countMatches(tables.AssertionPatterns, d.NewContent)
		if newAsserts < oldAsserts {
			findings = append(findings, schema.Finding{
				CheckID: "test-delta",
				Class:   "assertion-count-drop",
				Rung:    "3",
				Path:    d.Path,
				Message: fmt.Sprintf("test assertion count dropped from %d to %d: %s", oldAsserts, newAsserts, d.Path),
			})
		}
	}

	// Deterministic output (AP-10): stable regardless of the diff's own
	// enumeration order.
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Class < findings[j].Class
	})

	return findings
}

// countMatches sums each pattern's match count over content. Patterns are
// deliberately text-level, not AST-aware (R5, design:codegate-v1) — a floor
// tightened by table data, never by harness code.
func countMatches(patterns []*regexp.Regexp, content []byte) int {
	count := 0
	for _, p := range patterns {
		count += len(p.FindAllIndex(content, -1))
	}
	return count
}

// firstMatchLine returns the 1-based line number of the earliest pattern
// match in content, or 0 if none match — a best-effort pointer for the
// skip-marker-added finding (§3.6: "line where the class has one").
func firstMatchLine(patterns []*regexp.Regexp, content []byte) int {
	best := -1
	for _, p := range patterns {
		if loc := p.FindIndex(content); loc != nil && (best == -1 || loc[0] < best) {
			best = loc[0]
		}
	}
	if best == -1 {
		return 0
	}
	return bytes.Count(content[:best], []byte("\n")) + 1
}

// compileGlobs translates each glob to an anchored regexp and compiles it once,
// so the file loops match against precompiled patterns instead of recompiling
// per file (the sibling ecosystem pattern tables follow the same compile-once
// rule). A glob whose translation fails to compile is dropped — globToRegexp
// only ever emits valid regexp syntax, so this is unreachable for real input and
// simply fails safe by never panicking mid-audit.
func compileGlobs(globs []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(globs))
	for _, g := range globs {
		if re, err := regexp.Compile("^" + globToRegexp(g) + "$"); err == nil {
			res = append(res, re)
		}
	}
	return res
}

// anyGlobMatch reports whether path matches any of the precompiled glob regexps.
func anyGlobMatch(res []*regexp.Regexp, path string) bool {
	for _, re := range res {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// globToRegexp maps a glob to a regexp body using double-star semantics: `**`
// spans any characters including `/` (and a `**/` prefix optionally matches zero
// path segments, so `**/*_test.go` reaches a root-level file too); `*` spans a
// run of non-`/` characters; every other byte is literal.
func globToRegexp(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' && i+1 < len(pattern) && pattern[i+1] == '*' {
			if i+2 < len(pattern) && pattern[i+2] == '/' {
				b.WriteString("(?:.*/)?") // **/ matches zero or more path segments
				i += 2
				continue
			}
			b.WriteString(".*")
			i++
			continue
		}
		if pattern[i] == '*' {
			b.WriteString("[^/]*")
			continue
		}
		// Quote the raw byte, not string(pattern[i]): converting a byte to a
		// string widens a high byte into its multi-byte rune encoding (0xC3 ->
		// "Ã"), corrupting any non-ASCII path glob so it matches nothing. A
		// one-byte sub-slice preserves the exact UTF-8 bytes.
		b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
	}
	return b.String()
}

// ExtractFileDeltas runs `git diff --name-status -M` between diffBase and
// head inside repoDir — rename detection resolved before any floor fires,
// exactly as design:codegate-v1#3.9 requires — keeps entries whose current
// path, or (for a rename that leaves the test set) whose prior path, matches
// one of testGlobs, and fetches before/after blob content only for the entries
// whose content a floor actually reads. diffBase and head must already be
// resolved revisions (T2's ResolveBindings resolves and validates both before
// any check runs).
func ExtractFileDeltas(ctx context.Context, repoDir, diffBase, head string, testGlobs []string) ([]FileDelta, error) {
	out, err := runGitBytes(ctx, repoDir, "diff", "--no-color", "--name-status", "-M", "-z", diffBase, head)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	entries, err := parseNameStatusZ(out)
	if err != nil {
		return nil, fmt.Errorf("parse diff status: %w", err)
	}

	globs := compileGlobs(testGlobs)
	var deltas []FileDelta
	for _, e := range entries {
		newInScope := anyGlobMatch(globs, e.path)
		// Keep an entry that touches the test set on EITHER side: the new path
		// (a modification, addition, rename-in, or a deletion keyed on the
		// deleted path), or — for a rename that LEAVES the set — the old path,
		// which TestDelta reports as a disguised deletion.
		renamedOut := e.status == DeltaRenamed && !newInScope && anyGlobMatch(globs, e.oldPath)
		if !newInScope && !renamedOut {
			continue
		}
		d := FileDelta{Status: e.status, Path: e.path, OldPath: e.oldPath}

		// Fetch blob content only for the cases the content floors read: a test
		// that existed before and still exists — a modification, or a rename
		// that stayed within the test set. Deletions and renames-out are
		// path-only; additions and renames-in have no test pre-image. This
		// skips a wasted `git show` (and full-file allocation) per deleted or
		// added test on the bulk-change paths.
		if newInScope && (e.status == DeltaModified || (e.status == DeltaRenamed && anyGlobMatch(globs, e.oldPath))) {
			oldRef := e.path
			if e.status == DeltaRenamed {
				oldRef = e.oldPath
			}
			oldContent, err := runGitBytes(ctx, repoDir, "show", diffBase+":"+oldRef)
			if err != nil {
				return nil, fmt.Errorf("read pre-image of %s at %s: %w", oldRef, diffBase, err)
			}
			newContent, err := runGitBytes(ctx, repoDir, "show", head+":"+e.path)
			if err != nil {
				return nil, fmt.Errorf("read post-image of %s at %s: %w", e.path, head, err)
			}
			d.OldContent = oldContent
			d.NewContent = newContent
		}

		deltas = append(deltas, d)
	}

	return deltas, nil
}

// nameStatusEntry is one parsed record from `git diff --name-status -z`.
type nameStatusEntry struct {
	status  DeltaStatus
	path    string
	oldPath string
}

// parseNameStatusZ parses NUL-delimited `--name-status` output. A plain
// record is "STATUS\0PATH\0"; a rename/copy record is
// "STATUS+score\0OLDPATH\0NEWPATH\0" (verified against real git output — see
// testdelta_test.go).
func parseNameStatusZ(out []byte) ([]nameStatusEntry, error) {
	raw := string(bytes.TrimSuffix(out, []byte{0}))
	var fields []string
	if raw != "" {
		fields = strings.Split(raw, "\x00")
	}

	var entries []nameStatusEntry
	for i := 0; i < len(fields); {
		code := fields[i]
		i++
		if code == "" {
			continue
		}
		switch code[0] {
		case 'R':
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("truncated rename record at field %d", i)
			}
			entries = append(entries, nameStatusEntry{status: DeltaRenamed, oldPath: fields[i], path: fields[i+1]})
			i += 2
		case 'C':
			// A copy record cannot occur without the -C flag, which this
			// extraction never passes; skip its two path fields defensively
			// rather than mis-parse them as a delta.
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("truncated copy record at field %d", i)
			}
			i += 2
		case 'A':
			if i >= len(fields) {
				return nil, fmt.Errorf("truncated add record at field %d", i)
			}
			entries = append(entries, nameStatusEntry{status: DeltaAdded, path: fields[i]})
			i++
		case 'D':
			if i >= len(fields) {
				return nil, fmt.Errorf("truncated delete record at field %d", i)
			}
			entries = append(entries, nameStatusEntry{status: DeltaDeleted, path: fields[i]})
			i++
		default:
			// M, T, U, X, B, etc: treated uniformly as a content
			// modification — never silently dropped.
			if i >= len(fields) {
				return nil, fmt.Errorf("truncated record at field %d for code %q", i, code)
			}
			entries = append(entries, nameStatusEntry{status: DeltaModified, path: fields[i]})
			i++
		}
	}
	return entries, nil
}

// runGitBytes runs a git subcommand under ctx, returning raw stdout bytes
// (untrimmed — a `git show` result is exact file content, and trimming would
// corrupt it) or an error wrapping stderr. Terminal prompts and optional
// locks are disabled — this extraction only ever reads.
//
// Named runGitBytes, not runGit, to avoid colliding with the string-returning
// runGit other files in package checks declare. Both re-roll the same
// credential-safe git posture invoke.go already carries; folding the three onto
// one shared git/diff helper is tracked separately (test-delta review #10).
func runGitBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}
