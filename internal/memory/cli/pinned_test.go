package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

// retiredSourcesFixture lands n standalone sources, seals them in one batch,
// lands the seals, and leaves a branch that deletes every sealed source. It
// returns the trusted commit that branch is checked against.
func retiredSourcesFixture(t *testing.T, n int) (*harness, string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "# valid defaults\n", 0o644)
	sources := make([]string, n)
	seal := []string{"retire-source", "--reason", "Retire approved standalone sources", "--write", "--json"}
	for i := range sources {
		sources[i] = fmt.Sprintf(".pyxis/skills/example-%d/SKILL.md", i)
		writeFixtureFile(t, h.dir, sources[i], fmt.Sprintf("# Exact historical source %d\n", i), 0o644)
		seal = append(seal, "--source", sources[i])
	}
	h.commit("introduce standalone sources")
	h.ok("reconcile", "--write")
	h.commit("capture sources")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("exact legacy inventory")
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
	h.commit("partitioned policy")
	h.git("checkout", "-qb", "seal")
	h.ok(seal...)
	h.commit("seal every source without deleting any")
	h.git("checkout", "-q", "main")
	h.git("merge", "-q", "--squash", "seal")
	h.commit("land exact retirement evidence")
	h.git("branch", "-qD", "seal")
	trusted := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "retire")
	h.git(append([]string{"rm", "-q", "--"}, sources...)...)
	h.commit("retire sources using landed seals")
	return h, trusted
}

// gitProcesses runs fn while this test binary stands in for git, and returns
// how many processes each Git command started. Every one still reaches the
// installed Git.
func gitProcesses(t *testing.T, fn func()) map[string]int {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "operations.jsonl")
	t.Setenv("PYXIS_TEST_REAL_GIT", gitPath)
	t.Setenv("PYXIS_TEST_GIT_OPERATIONS", log)
	t.Setenv("PYXIS_TEST_GIT_PROXY", "1")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	fn()
	f, err := os.Open(log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }() // Closing a read-only log has no durable effect.
	counts := map[string]int{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var args []string
		if err := json.Unmarshal(scan.Bytes(), &args); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < len(args); i++ {
			if args[i] == "-c" || args[i] == "-C" {
				i++ // the option's value is not the command
			} else if !strings.HasPrefix(args[i], "-") {
				counts[args[i]]++
				break
			}
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	return counts
}

// A-9 of memory-durable-evidence-v1 asks for deterministic Git-operation counts
// at two corpus sizes. Every seal in a batch is checked against the same few
// pinned commits, so the trees, blobs and ancestry the gate reads must not
// depend on how many seals there are.
func TestMemoryGateTreeAndBlobReadsDoNotGrowWithSeals(t *testing.T) {
	const fewer, more = 1, 4
	var counts []map[string]int
	for _, seals := range []int{fewer, more} {
		h, trusted := retiredSourcesFixture(t, seals)
		t.Run(fmt.Sprint(seals), func(t *testing.T) {
			counts = append(counts, gitProcesses(t, func() { h.gateOK("--base", trusted) }))
		})
	}
	small, large := counts[0], counts[1]
	for _, command := range []string{"ls-tree", "cat-file", "merge-base", "rev-list"} {
		if small[command] == 0 || small[command] != large[command] {
			t.Fatalf("%s processes grew with the number of seals: %d for %d, %d for %d", command, small[command], fewer, large[command], more)
		}
	}
	// A sealed source has its own history: its introduction as seen from the
	// seal, trusted and candidate commits, and its deletion from the candidate.
	// Each is looked up once, however many reconciliation passes need it.
	if added := large["log"] - small["log"]; added != 4*(more-fewer) {
		t.Fatalf("%d more seals added %d history lookups, want four each: %v", more-fewer, added, counts)
	}
}

// The gate runs its evidence, packet and reconciliation steps over one shared
// pinned state. Each step run as its own verb starts with nothing kept, so
// equal output shows that sharing changes what is read, never what is decided.
func TestMemoryGateOutputEqualsItsStepsRunAlone(t *testing.T) {
	t.Parallel()
	h, trusted := retiredSourcesFixture(t, 2)
	alone := h.ok("evidence", "--check", "--base", trusted) +
		"memory gate: trusted base " + trusted + "\n" +
		h.ok("check", "--base", trusted) +
		h.ok("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted, "--baseline-replay-ref", initialBaselineReplay)
	if together := h.gateOK("--base", trusted); together != alone {
		t.Fatalf("the gate and its steps disagree:\n--- gate\n%s\n--- steps\n%s", together, alone)
	}
}

// sameMap reports whether two map values are one map rather than equal copies.
func sameMap(a, b any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func TestPinnedStateDerivesEachCommitOnce(t *testing.T) {
	t.Parallel()
	h, trusted := retiredSourcesFixture(t, 1)
	ctx := t.Context()
	env, _, _ := h.env("")
	m, err := open(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	head := h.git("rev-parse", "HEAD")
	unknown := strings.Repeat("e", 40)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bare := &memory{repo: m.repo} // a memory built by hand keeps nothing

	set, err := m.baseSet(ctx, trusted)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := m.baseSet(ctx, trusted); err != nil || again != set {
		t.Fatalf("a pinned commit's memory tree was loaded twice: %v", err)
	}
	// main names the trusted commit today, yet a branch can move: never kept.
	byName, err := m.baseSet(ctx, "main")
	if err != nil || byName == set || !reflect.DeepEqual(byName.RawBytes, set.RawBytes) {
		t.Fatalf("a moving ref must be loaded afresh with the same bytes: %v", err)
	}
	if fresh, err := bare.baseSet(ctx, trusted); err != nil || fresh == set {
		t.Fatalf("a hand-built memory shared a kept tree: %v", err)
	}
	if _, err := m.baseSet(cancelled, trusted); err == nil {
		t.Fatal("a finished context was answered from a kept tree")
	}
	for range 2 {
		if _, err := m.baseSet(ctx, unknown); err == nil {
			t.Fatal("an unknown commit produced a memory tree")
		}
	}
	if len(m.pinned.sets) != 1 {
		t.Fatalf("kept %d memory trees, want only the one that loaded", len(m.pinned.sets))
	}

	if m.projection(m.set) != m.proj {
		t.Fatal("the working tree was projected twice")
	}
	projection := m.projection(set)
	if projection.Set != set || m.projection(set) != projection {
		t.Fatal("a pinned commit's memory tree was projected twice")
	}
	if p := m.projection(byName); p.Set != byName || p == m.projection(byName) {
		t.Fatal("an unpinned memory tree must be projected afresh")
	}
	if p := bare.projection(set); p.Set != set || p == projection {
		t.Fatal("a hand-built memory shared a kept projection")
	}

	opts := reconciliationOptions(m)
	snapshot, err := m.snapshotAt(ctx, head, opts)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := m.snapshotAt(ctx, head, opts); err != nil || !sameMap(again.Files, snapshot.Files) {
		t.Fatalf("a pinned commit's snapshot was built twice: %v", err)
	}
	legacy := opts
	legacy.LegacySessionLogs = true
	if other, err := m.snapshotAt(ctx, head, legacy); err != nil || sameMap(other.Files, snapshot.Files) || !other.SourceOptions.LegacySessionLogs {
		t.Fatalf("different options were answered with another snapshot: %v", err)
	}
	if byRef, err := m.snapshotAt(ctx, "HEAD", opts); err != nil || sameMap(byRef.Files, snapshot.Files) || byRef.Commit != head {
		t.Fatalf("a moving ref must be built afresh: %v", err)
	}
	if fresh, err := bare.snapshotAt(ctx, head, opts); err != nil || sameMap(fresh.Files, snapshot.Files) {
		t.Fatalf("a hand-built memory shared a kept snapshot: %v", err)
	}
	if _, err := m.snapshotAt(cancelled, head, opts); err == nil {
		t.Fatal("a finished context was answered from a kept snapshot")
	}
	for range 2 {
		if _, err := m.snapshotAt(ctx, unknown, opts); err == nil {
			t.Fatal("an unknown commit produced a snapshot")
		}
	}
	if len(m.pinned.snapshots) != 2 {
		t.Fatalf("kept %d snapshots, want only the two that were built", len(m.pinned.snapshots))
	}

	declared := []string{"README.md"}
	m.pinned.evidenceSnapshots(declared)[head] = reconcile.EvidenceSnapshot{Commit: head}
	if kept := m.pinned.evidenceSnapshots(declared); kept[head].Commit != head {
		t.Fatal("sessions with the same declared paths must share evidence snapshots")
	}
	if kept := m.pinned.evidenceSnapshots([]string{"README.md", "other.md"}); len(kept) != 0 {
		t.Fatal("a different declared path set was answered with another set's snapshots")
	}
	if kept := bare.pinned.evidenceSnapshots(declared); kept == nil || len(kept) != 0 {
		t.Fatal("a hand-built memory must start with no evidence snapshots")
	}

	// A step the gate runs opens the memory again and inherits this state.
	env.pinned = m.pinned
	step, err := open(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if step.pinned != m.pinned || step.repo != m.repo || step.set == m.set {
		t.Fatal("a gate step must share pinned state and still load its own working tree")
	}
	if again, err := step.baseSet(ctx, trusted); err != nil || again != set {
		t.Fatalf("a gate step loaded a kept memory tree again: %v", err)
	}
}
