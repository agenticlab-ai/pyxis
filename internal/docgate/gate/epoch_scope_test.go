package gate

// Install-epoch gate-scoping tests (existing-repo-onboarding/001 T9;
// prd:existing-repo-onboarding/FR-8, ABUSE-2, NFR-1; design §3.4, DINV-6, A-3,
// A-8). Internal (package gate) so they exercise scanScope and corpusScanFiles
// — the unexported scan-set machinery — directly, isolating the epoch behaviour
// from full-umbrella noise. The behavioural face (a defect surfaced through
// GateAt) is proven separately in epoch_gate_test.go.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// epochRepo makes a throwaway git repo and returns its root plus a git runner
// that returns trimmed combined output (failing the test on error).
func epochRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	return root, git
}

func writeAt(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const danglingLinkDoc = "# legacy\n\nsee [gone](missing/file.md)\n"

// AC-1: a non-conforming corpus present AT the epoch is grandfathered (green
// immediately after adoption); a post-epoch change to it is fully gated.
func TestEpochGrandfathersCorpusAndGatesPostEpochChange(t *testing.T) {
	root, git := epochRepo(t)
	cfg := config.Defaults()
	writeAt(t, root, "docs/legacy.md", danglingLinkDoc) // a dangling relative link
	git("add", "-A")
	git("commit", "-q", "-m", "pre-adoption corpus")
	cfg.Epoch = config.Epoch{Commit: git("rev-parse", "HEAD")}

	// Sanity — with no epoch the dangling link reds (the corpus IS non-conforming).
	if d := CheckLinks(root, cfg, ScanScope{}); len(d) == 0 {
		t.Fatal("expected the non-conforming corpus to red without an epoch")
	}

	// Immediately after adoption the corpus is unchanged since the epoch →
	// grandfathered → green.
	scope, sd := scanScope(root, cfg)
	if len(sd) != 0 {
		t.Fatalf("scanScope named a defect for a reachable epoch: %v", sd)
	}
	if d := CheckLinks(root, cfg, scope); len(d) != 0 {
		t.Fatalf("pre-existing corpus not grandfathered by the epoch: %v", d)
	}

	// A post-epoch change to the file brings it back into scope → fully gated.
	writeAt(t, root, "docs/legacy.md", danglingLinkDoc+"\nand [more](nope/x.md)\n")
	scope2, _ := scanScope(root, cfg)
	if d := CheckLinks(root, cfg, scope2); len(d) == 0 {
		t.Fatal("a post-epoch change to a grandfathered file was not gated (ABUSE-2)")
	}
}

// AC-3: an unreachable epoch fails closed — the scope is inactive (gate
// everything) and a defect names the cause.
func TestEpochUnreachableFailsClosedNamingTheCause(t *testing.T) {
	root, git := epochRepo(t)
	cfg := config.Defaults()
	writeAt(t, root, "docs/legacy.md", danglingLinkDoc)
	git("add", "-A")
	git("commit", "-q", "-m", "corpus")
	cfg.Epoch = config.Epoch{Commit: strings.Repeat("0", 40)} // a commit this repo lacks

	scope, sd := scanScope(root, cfg)
	if len(sd) == 0 {
		t.Fatal("an unreachable epoch produced no naming-the-cause defect")
	}
	if !strings.Contains(sd[0].Reason, "unreachable") {
		t.Errorf("the defect does not name the cause: %v", sd[0])
	}
	if scope.epochActive {
		t.Error("an unreachable epoch must yield an INACTIVE scope (gate everything, not grandfather)")
	}
	// Fail closed: the whole non-conforming corpus is judged.
	if d := CheckLinks(root, cfg, scope); len(d) == 0 {
		t.Fatal("an unreachable epoch must gate everything; the corpus was not scanned")
	}
}

// AC-2: for a fixed change-set the scan-set size is |changed ∩ corpus|,
// invariant to how large the grandfathered corpus grows.
func TestEpochScanSetInvariantToCorpusSize(t *testing.T) {
	sizeFor := func(grandfathered int) int {
		root, _ := epochRepo(t)
		cfg := config.Defaults()
		writeAt(t, root, "docs/changed.md", "# changed\n")
		for i := 0; i < grandfathered; i++ {
			writeAt(t, root, fmt.Sprintf("docs/old-%d.md", i), "# old\n")
		}
		// Only docs/changed.md changed since the epoch; the rest is grandfathered.
		scope := ScanScope{epochActive: true, changed: map[string]bool{"docs/changed.md": true}}
		return len(corpusScanFiles(root, cfg, scope, false))
	}
	if small, large := sizeFor(2), sizeFor(200); small != 1 || large != 1 {
		t.Fatalf("scan-set varied with corpus size: small=%d large=%d, want exactly the 1 changed file both times", small, large)
	}
}

// A changed-but-deleted path is not scanned (it has nothing to read), and a
// changed path outside the corpus is ignored — the intersection is with the
// scan-set, not merely the changed set.
func TestEpochScanSetSkipsDeletedAndNonCorpusPaths(t *testing.T) {
	root, _ := epochRepo(t)
	cfg := config.Defaults()
	writeAt(t, root, "docs/present.md", "# here\n")
	scope := ScanScope{epochActive: true, changed: map[string]bool{
		"docs/present.md": true, // in corpus, exists → kept
		"docs/gone.md":    true, // in corpus, but never written (a deletion) → skipped
		"src/main.go":     true, // changed but outside the corpus → skipped
	}}
	got := corpusScanFiles(root, cfg, scope, false)
	if len(got) != 1 || !strings.HasSuffix(got[0], "present.md") {
		t.Fatalf("scan-set = %v, want only docs/present.md", got)
	}
}

// Non-regression: with no epoch the whole corpus is scanned (byte-for-byte the
// pre-adoption behaviour), and no scan defect is produced.
func TestNoEpochScansWholeCorpus(t *testing.T) {
	root, _ := epochRepo(t)
	cfg := config.Defaults() // no epoch
	writeAt(t, root, "docs/bad.md", "# b\n\nsee [x](missing.md)\n")
	scope, sd := scanScope(root, cfg)
	if len(sd) != 0 || scope.epochActive {
		t.Fatalf("no epoch must yield an inactive, defect-free scope: defects=%v active=%v", sd, scope.epochActive)
	}
	if d := CheckLinks(root, cfg, scope); len(d) == 0 {
		t.Fatal("with no epoch the whole corpus must be scanned (the dangling link should red)")
	}
}
