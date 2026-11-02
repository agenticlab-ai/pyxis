package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

type memoryBinaries struct {
	t   *testing.T
	dir string
}

func buildMemoryBinaries(t *testing.T) memoryBinaries {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", dir, "./cmd/memory", "./cmd/memory-gate")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build installed memory flow: %v\n%s", err, out)
	}
	return memoryBinaries{t: t, dir: dir}
}

func (b memoryBinaries) ok(dir, bin string, args ...string) string {
	b.t.Helper()
	cmd := exec.CommandContext(b.t.Context(), filepath.Join(b.dir, bin), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MEMORY_ACTOR=test:parallel", "PYXIS_CI_EVENT=", "PYXIS_CI_BASE=", "GITHUB_ACTIONS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.t.Fatalf("%s %v: %v\n%s", bin, args, err, out)
	}
	return string(out)
}

func TestBuiltCLIParallelMemoryTenSiblings(t *testing.T) {
	t.Parallel()
	b := buildMemoryBinaries(t)
	h := initRepo(t)
	b.ok(h.dir, "memory", "init")
	for i := range 10 {
		writeFixtureFile(t, h.dir, fmt.Sprintf("docs/unit-%d.md", i), fmt.Sprintf("# Unit %d\nInitial behavior.\n", i), 0644)
		writeFixtureFile(t, h.dir, fmt.Sprintf("internal/unit%d/unit.go", i), fmt.Sprintf("package unit%d\nconst Value = 1\n", i), 0644)
	}
	h.commit("ten independent authored and code units")
	for i := range 10 {
		h.ok("add", "--kind", "Learning", "--subject", fmt.Sprintf("meta/unit-%d", i), "--statement", "Initial behavior", "--tag", fmt.Sprintf("code:internal/unit%d", i))
	}
	b.ok(h.dir, "memory", "reconcile", "--write")
	h.commit("capture exact initial source and code evidence")
	writeFixtureFile(t, h.dir, reconcile.PolicyPath, b.ok(h.dir, "memory", "reconcile", "--baseline-generate"), 0644)
	h.commit("establish exact v1 inventory")
	legacy := h.git("rev-parse", "HEAD")
	b.ok(h.dir, "memory", "reconcile", "--baseline-sync", "--baseline-ref", legacy, "--migrate-v2")
	h.commit("data-only migration")
	b.ok(h.dir, "memory-gate", "--base", legacy)
	base := h.git("rev-parse", "HEAD")
	baseline := policyFilesOnDisk(t, h)
	if len(baseline) != 1 {
		t.Fatalf("aligned fixture unexpectedly carries %d policy members", len(baseline)-1)
	}
	branches := make([]string, 10)
	memories := make([]map[string][]byte, 10)
	for i := range 10 {
		branch := fmt.Sprintf("sibling-%d", i)
		h.git("checkout", "-q", "-b", branch, base)
		writeFixtureFile(t, h.dir, fmt.Sprintf("docs/unit-%d.md", i), fmt.Sprintf("# Unit %d\nChanged behavior.\n", i), 0644)
		writeFixtureFile(t, h.dir, fmt.Sprintf("internal/unit%d/unit.go", i), fmt.Sprintf("package unit%d\nconst Value = 2\n", i), 0644)
		h.commit("change one source and code unit")
		// Controlled writer clocks deliberately precede newer records already
		// captured on the base. Independent history must not require reissuance.
		h.clock = t0.Add(time.Duration(i+1) * time.Minute)
		h.ok("add", "--kind", "Promise", "--subject", fmt.Sprintf("prd/unit-%d", i), "--family", "FR-1", "--statement", "The unit shall return 2.", "--tag", fmt.Sprintf("code:internal/unit%d", i))
		b.ok(h.dir, "memory", "reconcile", "--write")
		h.commit("record exact evidence and one independent claim")
		b.ok(h.dir, "memory-gate", "--base", base)
		var sync policySyncOutput
		decodeJSON(t, b.ok(h.dir, "memory", "reconcile", "--baseline-sync", "--baseline-ref", base, "--json"), &sync)
		if len(sync.Planned) != 0 {
			t.Fatalf("sibling %d rewrites policy: %+v", i, sync.Planned)
		}
		paths := strings.Fields(h.git("diff", "--name-only", base, "HEAD"))
		memories[i] = map[string][]byte{}
		for _, name := range paths {
			if strings.HasPrefix(name, reconcile.PolicyDirectory) {
				t.Fatalf("sibling %d touches shared policy %s", i, name)
			}
			if strings.HasPrefix(name, "memory/") {
				body, err := os.ReadFile(filepath.Join(h.dir, name))
				if err != nil {
					t.Fatal(err)
				}
				memories[i][name] = body
			}
		}
		branches[i] = h.git("rev-parse", "HEAD")
	}
	// Every unordered pair, in both integration orders: 45 * 2 actual merges.
	// The pairwise proof uses pinned commits. Remove unrelated topic refs so
	// every check need not rediscover the other eight unrelated branches.
	h.git("checkout", "-q", "--detach", base)
	for i := range 10 {
		h.git("branch", "-D", fmt.Sprintf("sibling-%d", i))
	}
	merges := 0
	// Each pair has its own checkout and refs. Bound concurrent subprocesses
	// while retaining every real merge and gate invocation in both orders.
	var pairs sync.WaitGroup
	slots := make(chan struct{}, mergeSlots())
	for i := range 10 {
		for j := i + 1; j < 10; j++ {
			for _, pair := range [][2]int{{i, j}, {j, i}} {
				merges++
				slots <- struct{}{}
				pairs.Go(func() {
					defer func() { <-slots }()
					t.Run(fmt.Sprintf("merge-%d-then-%d", pair[0], pair[1]), func(t *testing.T) {
						dir := filepath.Join(t.TempDir(), "pair")
						gitIn(t, h.dir, "clone", "--shared", "--no-checkout", h.dir, dir)
						local := newHarness(t, dir)
						local.git("checkout", "-q", "--detach", branches[pair[0]])
						local.git("merge", "--no-ff", "--no-edit", "-q", branches[pair[1]])
						binary := memoryBinaries{t: t, dir: b.dir}
						binary.ok(dir, "memory-gate", "--base", branches[pair[0]])
						if !reflect.DeepEqual(baseline, policyFilesOnDisk(t, local)) {
							t.Fatalf("pair %v changed policy", pair)
						}
						for _, index := range pair {
							assertMemoryBytes(t, dir, memories[index])
						}
					})
				})
			}
		}
	}
	pairs.Wait()
	if merges != 90 {
		t.Fatalf("merge matrix incomplete: %d", merges)
	}
	// Integrate one sibling, rebase the older sibling without rewriting records,
	// then perform squash integration and verify a transport-only fresh clone.
	for _, pair := range [][2]int{{0, 9}, {9, 0}} {
		for _, squash := range []bool{false, true} {
			name := fmt.Sprintf("integrated-%d-%t", pair[0], squash)
			h.git("checkout", "-q", "-b", name, base)
			if squash {
				h.git("merge", "--squash", "-q", branches[pair[0]])
				h.commit("first squash")
			} else {
				h.git("merge", "--no-ff", "--no-edit", "-q", branches[pair[0]])
			}
			first := h.git("rev-parse", "HEAD")
			work := name + "-rebased"
			h.git("checkout", "-q", "-b", work, branches[pair[1]])
			h.git("rebase", name)
			assertMemoryBytes(t, h.dir, memories[pair[1]])
			b.ok(h.dir, "memory-gate", "--base", first)
			h.git("checkout", "-q", name)
			if squash {
				h.git("merge", "--squash", "-q", work)
				h.commit("second squash")
			} else {
				h.git("merge", "--no-ff", "--no-edit", "-q", work)
			}
			clone := filepath.Join(t.TempDir(), "clone")
			h.git("clone", "--no-local", "--single-branch", "--branch", name, "file://"+h.dir, clone)
			// The transported integration tip is this fixture's retained mainline.
			h.git("-C", clone, "branch", "-m", "main")
			if _, err := os.Stat(filepath.Join(clone, ".git/objects/info/alternates")); !os.IsNotExist(err) {
				t.Fatalf("transport clone has alternates: %v", err)
			}
			b.ok(clone, "memory-gate", "--base", first)
			b.ok(clone, "memory", "verify", "--offline")
			for _, index := range pair {
				assertMemoryBytes(t, clone, memories[index])
			}
		}
	}
	t.Logf("verified %d sibling merges, both rebase orders, merge and squash integration, and four fresh transport clones", merges)
}

func mergeSlots() int {
	n := runtime.GOMAXPROCS(0)
	if n < 8 {
		return 8
	}
	return n
}

func assertMemoryBytes(t *testing.T, dir string, want map[string][]byte) {
	t.Helper()
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("memory changed during integration at %s: %v", name, err)
		}
	}
}
