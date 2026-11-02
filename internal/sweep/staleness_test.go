package sweep

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// The staleness fixture is a hermetic temp-git repo built in-test with pinned
// committer dates, exercised through the PRODUCTION runner (maintenance-sweep-v1
// §3.9 — the toolchain reads are integration-covered by the fixture repos). Days
// are measured from a fixed epoch so no test reads the wall clock.
const fixtureEpoch = "2026-01-01T00:00:00 +0000"

// day returns fixtureEpoch + n days as a git date string.
func day(n int) string {
	base, _ := time.Parse("2006-01-02T15:04:05 -0700", fixtureEpoch)
	return base.AddDate(0, 0, n).Format("2006-01-02T15:04:05 -0700")
}

// newFixtureRepo initializes an isolated git repo under a temp dir.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, nil, "init", "-q")
	git(t, dir, nil, "config", "user.email", "sweep@test")
	git(t, dir, nil, "config", "user.name", "sweep")
	return dir
}

// commitAt writes path with content and commits it at the given git date.
func commitAt(t *testing.T, dir, path, content, date string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	git(t, dir, nil, "add", path)
	git(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date},
		"commit", "-q", "-m", "commit "+path+" @ "+date)
}

// git runs one git command under dir with optional extra env, failing the test
// on error.
func git(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// fixtureState builds an evaluated State over a fixture repo using default
// config (docs/prd.md, docs/techspec.md; capability_paths cmd/ internal/ …) and
// the production runner.
func fixtureState(dir string) State {
	return State{Root: dir, Config: config.Defaults(), Runner: ExecRunner{}}
}

// TestStaleness_BeyondBoundReportsWithDelta — structure shipped beyond the bound
// after both rollups' last touch: each rollup is reported, evidence carries the
// age delta, and every finding carries all four contracted fields (A-4, DINV-5).
func TestStaleness_BeyondBoundReportsWithDelta(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(0))
	commitAt(t, dir, "internal/feature/feature.go", "package feature\n", day(31))

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 2 {
		t.Fatalf("want 2 stale rollups, got %d: %+v", len(res.Findings), res.Findings)
	}
	subjects := map[string]bool{}
	for _, f := range res.Findings {
		subjects[f.Subject] = true
		if f.Detector != DetectorRollupStaleness {
			t.Errorf("finding detector = %q, want %q", f.Detector, DetectorRollupStaleness)
		}
		// All four contracted fields present.
		if f.Detector == "" || f.Subject == "" || f.Evidence == "" || f.Remedy == "" {
			t.Errorf("finding missing a contracted field: %+v", f)
		}
		// The age delta is 31 days.
		if !strings.Contains(f.Evidence, "31d") {
			t.Errorf("evidence lacks the age delta %q: %q", "31d", f.Evidence)
		}
		if !strings.Contains(f.Evidence, "bound: 14d") {
			t.Errorf("evidence lacks the policy bound: %q", f.Evidence)
		}
	}
	for _, want := range []string{"docs/prd.md", "docs/techspec.md"} {
		if !subjects[want] {
			t.Errorf("missing finding for rollup %q", want)
		}
	}
}

// TestStaleness_WithinBoundZero — structure shipped within the bound after the
// rollups' last touch: zero findings (A-4).
func TestStaleness_WithinBoundZero(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(0))
	commitAt(t, dir, "internal/feature/feature.go", "package feature\n", day(10))

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("within the bound wants zero findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

// TestStaleness_BoundOverrideFlipsVerdict — the SAME fixture (structure 20 days
// past the rollups) reports under the default 14-day bound and reports zero
// under a policy-overridden 30-day bound: the bound is read from policy (A-4).
func TestStaleness_BoundOverrideFlipsVerdict(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(0))
	commitAt(t, dir, "internal/feature/feature.go", "package feature\n", day(20))
	st := fixtureState(dir)

	def := DetectRollupStaleness(context.Background(), st, Empty()) // 14d bound → stale
	if len(def.Findings) != 2 {
		t.Fatalf("default bound: want 2 findings, got %d", len(def.Findings))
	}

	relaxed := Empty()
	relaxed.StalenessBound = 30 * 24 * time.Hour // policy override → within bound
	over := DetectRollupStaleness(context.Background(), st, relaxed)
	if len(over.Findings) != 0 {
		t.Fatalf("overridden 30d bound: want zero findings, got %d: %+v", len(over.Findings), over.Findings)
	}
}

// TestStaleness_DeterministicAcrossRuns — two runs on the same fixture yield
// byte-identical findings: committed timestamps only, no wall clock (A-5,
// DINV-2).
func TestStaleness_DeterministicAcrossRuns(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(0))
	commitAt(t, dir, "internal/feature/feature.go", "package feature\n", day(40))
	st := fixtureState(dir)

	one := reportOf(t, DetectRollupStaleness(context.Background(), st, Empty()))
	two := reportOf(t, DetectRollupStaleness(context.Background(), st, Empty()))
	if one != two {
		t.Fatalf("nondeterministic findings:\n first: %s\nsecond: %s", one, two)
	}
}

// reportOf marshals a single detector result deterministically for comparison.
func reportOf(t *testing.T, d DetectorResult) string {
	t.Helper()
	r := NewReport(EvaluatedState{Commit: "fixed", Dirty: false}, []DetectorResult{d})
	b, err := r.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestStaleness_NotLivingRollupSkipped — a rollup with no commits is not living
// and yields no finding, no fault (the pre-first-capability state: the live repo
// has no docs/prd.md or docs/techspec.md yet).
func TestStaleness_NotLivingRollupSkipped(t *testing.T) {
	dir := newFixtureRepo(t)
	// Structure shipped, but neither rollup file was ever committed.
	commitAt(t, dir, "internal/feature/feature.go", "package feature\n", day(30))

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("a non-living rollup must not be reported, got %d: %+v", len(res.Findings), res.Findings)
	}
}

// TestStaleness_OneStaleOneFresh — a rollup refreshed after the newest structure
// is not stale while its stale sibling is: only the lagging rollup is reported.
func TestStaleness_OneStaleOneFresh(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0)) // stale
	commitAt(t, dir, "internal/feature/feature.go", "x\n", day(20))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(30)) // refreshed after structure

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("want exactly the stale rollup reported, got %d: %+v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Subject != "docs/prd.md" {
		t.Fatalf("want docs/prd.md stale, got %q", res.Findings[0].Subject)
	}
}

// TestStaleness_NoShippedStructureZero — no capability-path history means nothing
// can be stale, even with living rollups.
func TestStaleness_NoShippedStructureZero(t *testing.T) {
	dir := newFixtureRepo(t)
	commitAt(t, dir, "docs/prd.md", "prd\n", day(0))
	commitAt(t, dir, "docs/techspec.md", "techspec\n", day(0))
	// No cmd/ or internal/ commit at all.

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("no shipped structure wants zero findings, got %d", len(res.Findings))
	}
}

// TestStaleness_GitFaultNamed — an unreadable repository (git errors) is a named
// in-band fault with no finding, never a silent clean pass (DINV-3/DINV-4).
func TestStaleness_GitFaultNamed(t *testing.T) {
	dir := t.TempDir() // NOT a git repo

	res := DetectRollupStaleness(context.Background(), fixtureState(dir), Empty())

	if res.Fault == "" {
		t.Fatal("an unreadable repository must name a fault, not certify clean")
	}
	if len(res.Findings) != 0 {
		t.Fatalf("a fault must carry no finding, got %d", len(res.Findings))
	}
	if !strings.Contains(res.Fault, "rollup-staleness") {
		t.Errorf("fault should name the detector: %q", res.Fault)
	}
}
