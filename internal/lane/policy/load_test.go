package policy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const policyRel = "docs/policy/risk-policy.yaml"

// fixtureRepo returns a temp git repo on main plus run/write helpers (the
// house temp-repo pattern; the pure parse/validate tests stay git-free).
func fixtureRepo(t *testing.T) (string, func(...string), func(string, string)) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	return root, run, write
}

func loadDefects(t *testing.T, root string) (*Policy, []string) {
	t.Helper()
	p, ds := Load(root)
	reasons := make([]string, len(ds))
	for i, d := range ds {
		reasons[i] = d.Reason
		if d.File != policyRel {
			t.Fatalf("defect File = %q, want the resolved policy path %q", d.File, policyRel)
		}
	}
	return p, reasons
}

// TestLoadRatified — a well-formed policy landed on the mainline under the
// operator ratification trailer loads clean.
func TestLoadRatified(t *testing.T) {
	root, run, write := fixtureRepo(t)
	write(policyRel, validPolicy)
	run("add", ".")
	run("commit", "-qm", "policy: ratify\n\n"+ratificationTrailer)
	p, reasons := loadDefects(t, root)
	if p == nil || len(reasons) != 0 {
		t.Fatalf("ratified policy failed to load: %v", reasons)
	}
	if len(p.Floor.HardGatedPaths) != len(SelfUnlockMinPaths) {
		t.Fatalf("reader sees floor = %v", p.Floor.HardGatedPaths)
	}
}

// TestLoadUnratifiedNoTrailer — the same content landed without the trailer
// is unratified and refuses to load (DINV-8).
func TestLoadUnratifiedNoTrailer(t *testing.T) {
	root, run, write := fixtureRepo(t)
	write(policyRel, validPolicy)
	run("add", ".")
	run("commit", "-qm", "policy: land without ratifying")
	p, reasons := loadDefects(t, root)
	if p != nil || !anyContains(reasons, "ratification trailer") {
		t.Fatalf("want an unratified-trailer refusal, got policy=%v defects=%v", p, reasons)
	}
}

// TestLoadUnratifiedDivergent — a working-tree edit not landed on the
// mainline lineage is unratified even though it parses (DINV-9).
func TestLoadUnratifiedDivergent(t *testing.T) {
	root, run, write := fixtureRepo(t)
	write(policyRel, validPolicy)
	run("add", ".")
	run("commit", "-qm", "policy: ratify\n\n"+ratificationTrailer)
	// A local, un-landed flip of the kill switch — well-formed but off-lineage.
	write(policyRel, strings.Replace(validPolicy, "kill_switch: false", "kill_switch: true", 1))
	p, reasons := loadDefects(t, root)
	if p != nil || !anyContains(reasons, "diverges from the ratified mainline content") {
		t.Fatalf("want a divergence refusal, got policy=%v defects=%v", p, reasons)
	}
}

// TestLoadAbsent — an absent policy is a named load defect (the bootstrap /
// decommission state); the lane stays all-HUMAN (DINV-8).
func TestLoadAbsent(t *testing.T) {
	root, _, _ := fixtureRepo(t)
	p, reasons := loadDefects(t, root)
	if p != nil || !anyContains(reasons, "the risk policy is absent") {
		t.Fatalf("want an absent-policy refusal, got policy=%v defects=%v", p, reasons)
	}
}

// TestLoadFileSubstitutableLineage — the lineage read is a substitutable
// seam: a reader can inject a local double, and its refusal surfaces as a
// named lineage defect (merge-lane-v1 §3.8).
func TestLoadFileSubstitutableLineage(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(policyRel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(validPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	// A stub lineage that refuses.
	pol, ds := LoadFile(root, policyRel, stubLineage{reason: "stub says off-lineage", remedy: "land it"})
	if pol != nil || len(ds) != 1 || ds[0].Op != "lineage" || ds[0].Reason != "stub says off-lineage" {
		t.Fatalf("substituted lineage refusal not surfaced: policy=%v defects=%v", pol, ds)
	}
	// A stub that accepts → the well-formed policy loads.
	pol, ds = LoadFile(root, policyRel, stubLineage{ok: true})
	if pol == nil || len(ds) != 0 {
		t.Fatalf("accepting lineage should load: %v", ds)
	}
}

type stubLineage struct {
	ok     bool
	reason string
	remedy string
}

func (s stubLineage) Ratified([]byte) (bool, string, string) {
	return s.ok, s.reason, s.remedy
}

// TestNormalizeRoundTrip guards the lineage byte comparison against CRLF /
// trailing-whitespace noise: the same logical content differing only in line
// endings still reads as ratified content.
func TestNormalizeRoundTrip(t *testing.T) {
	crlf := strings.ReplaceAll(validPolicy, "\n", "\r\n")
	if string(schema.Normalize([]byte(crlf))) != string(schema.Normalize([]byte(validPolicy))) {
		t.Fatalf("normalization should erase CRLF differences")
	}
}

// TestResolveMainlineTracksConfig proves the lineage mainline resolution honors
// the configured remote/branch (config-driven-portability-v1 §3.2, FR-2): it
// probes "<remote>/<branch>" first, then bare "<branch>", and a default config
// probes "origin/main" then "main" exactly as before (DINV-2).
func TestResolveMainlineTracksConfig(t *testing.T) {
	root, run, write := fixtureRepo(t) // an initial repo on branch "main"
	write("f", "x\n")
	run("add", ".")
	run("commit", "-qm", "seed")

	t.Run("default config probes origin/main then main", func(t *testing.T) {
		// No remote-tracking ref yet: falls through to bare "main".
		if got := resolveMainline(root, "origin", "main"); got != "main" {
			t.Fatalf("default resolveMainline = %q, want the bare-branch fallback main", got)
		}
		// With origin/main present, it wins (the first probe).
		run("update-ref", "refs/remotes/origin/main", "HEAD")
		if got := resolveMainline(root, "origin", "main"); got != "origin/main" {
			t.Fatalf("resolveMainline = %q, want origin/main once the remote ref exists", got)
		}
	})

	t.Run("blank remote/branch defaults to origin/main", func(t *testing.T) {
		// Byte-identical to the pre-change single-arg behavior.
		if got := resolveMainline(root, "", ""); got != "origin/main" {
			t.Fatalf("resolveMainline with blanks = %q, want the origin/main default", got)
		}
	})

	t.Run("non-default remote/branch is probed", func(t *testing.T) {
		// A configured upstream/trunk that exists is resolved; a default origin/main
		// probe would MISS it, proving the resolution tracks config.
		run("branch", "trunk")
		run("update-ref", "refs/remotes/upstream/trunk", "refs/heads/trunk")
		if got := resolveMainline(root, "upstream", "trunk"); got != "upstream/trunk" {
			t.Fatalf("resolveMainline(upstream, trunk) = %q, want upstream/trunk", got)
		}
		// The bare-branch fallback for a configured branch whose remote ref is absent.
		if got := resolveMainline(root, "nope", "trunk"); got != "trunk" {
			t.Fatalf("resolveMainline(nope, trunk) = %q, want the bare-branch fallback trunk", got)
		}
	})
}
