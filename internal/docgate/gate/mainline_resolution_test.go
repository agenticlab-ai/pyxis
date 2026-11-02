package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// TestMainlineRefComposesRemoteAndBranch pins MainlineRef's composition —
// "<remote>/<branch>" — and its default byte-identity (config-driven-
// portability-v1 §3.2, DINV-2). This is the single source the runtime-resolving
// pre-push hook and the lane/plan-tracker derive from (DINV-1/DINV-4).
func TestMainlineRefComposesRemoteAndBranch(t *testing.T) {
	if got := config.Defaults().MainlineRef(); got != "origin/main" {
		t.Fatalf("default MainlineRef = %q, want origin/main (byte-identical, DINV-2)", got)
	}
	for _, tc := range []struct {
		name           string
		remote, branch string
		want           string
	}{
		{"default", "origin", "main", "origin/main"},
		{"renamed-branch", "origin", "trunk", "origin/trunk"},
		{"renamed-remote", "upstream", "main", "upstream/main"},
		{"both-non-default", "fork", "develop", "fork/develop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config.Config{Remote: tc.remote, Branch: tc.branch}
			if got := c.MainlineRef(); got != tc.want {
				t.Fatalf("MainlineRef(remote=%q, branch=%q) = %q, want %q", tc.remote, tc.branch, got, tc.want)
			}
		})
	}
}

// TestPrePushMergeBaseResolutionTracksConfig is the A-7 LIVENESS proof: the
// runtime merge-base resolution the pre-push hook triggers (an empty
// --merge-base resolves cfg.MainlineRef() at invocation) tracks config — under
// two configs with DIFFERENT branch/remote it yields the two configured refs'
// fork points, so the hook reads live and cannot go stale (config-driven-
// portability-v1 §3.1, A-7, DINV-4).
//
// It is genuinely non-tautological: the repo carries two distinct
// remote-tracking refs at two DIFFERENT commits, and the test asserts the two
// configs resolve to those two different commits (not merely that the ref
// STRING differs). A resolution that ignored config would resolve both to the
// same base and fail here.
func TestPrePushMergeBaseResolutionTracksConfig(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
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
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A linear history c1 → c2 → c3(=HEAD). Two DIFFERENT commits become the two
	// remote-tracking mainline tips the two configs point at.
	run("init", "-q", "-b", "main")
	write("f", "one\n")
	run("add", ".")
	run("commit", "-qm", "c1")
	c1 := run("rev-parse", "HEAD")
	write("f", "two\n")
	run("add", ".")
	run("commit", "-qm", "c2")
	c2 := run("rev-parse", "HEAD")
	write("f", "three\n")
	run("add", ".")
	run("commit", "-qm", "c3")

	// Config A's mainline (origin/main) forks at c1; config B's (upstream/trunk)
	// forks at c2. Both are ancestors of HEAD, so `merge-base <ref> HEAD` is the
	// ref's own commit — a different base per config.
	run("update-ref", "refs/remotes/origin/main", c1)
	run("update-ref", "refs/remotes/upstream/trunk", c2)

	cfgA := config.Config{Remote: "origin", Branch: "main"}
	cfgB := config.Config{Remote: "upstream", Branch: "trunk"}

	// The exact resolution the docgate pre-push path performs when no
	// --merge-base is passed: resolve cfg.MainlineRef() to its fork point.
	baseA, defA := gate.ResolveMergeBase(root, cfgA.MainlineRef())
	if len(defA) > 0 {
		t.Fatalf("config A resolution failed: %v", defA)
	}
	baseB, defB := gate.ResolveMergeBase(root, cfgB.MainlineRef())
	if len(defB) > 0 {
		t.Fatalf("config B resolution failed: %v", defB)
	}

	if baseA != c1 {
		t.Errorf("config A (origin/main) resolved to %q, want c1 %q", baseA, c1)
	}
	if baseB != c2 {
		t.Errorf("config B (upstream/trunk) resolved to %q, want c2 %q", baseB, c2)
	}
	// The liveness property: two configs → two DIFFERENT resolved bases. A stale
	// or config-ignoring resolution would collapse these to one.
	if baseA == baseB {
		t.Fatalf("A-7 violated: both configs resolved to the same base %q — the resolution does not track config", baseA)
	}
}

// TestPrePushDefaultConfigResolvesOriginMain proves the default config's
// runtime resolution is byte-identical to the pre-change engine: with no
// non-default value, the pre-push path resolves against "origin/main" exactly as
// the old baked literal did (config-driven-portability-v1 §3.2, DINV-2, A-3).
func TestPrePushDefaultConfigResolvesOriginMain(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
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
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-qm", "seed")
	seed := run("rev-parse", "HEAD")
	run("update-ref", "refs/remotes/origin/main", seed)

	// The default config's MainlineRef must equal the old literal, and resolve.
	def := config.Defaults()
	if def.MainlineRef() != "origin/main" {
		t.Fatalf("default MainlineRef = %q, want origin/main", def.MainlineRef())
	}
	base, defs := gate.ResolveMergeBase(root, def.MainlineRef())
	if len(defs) > 0 {
		t.Fatalf("default-config resolution failed: %v", defs)
	}
	if base != seed {
		t.Fatalf("default-config resolved base = %q, want seed %q", base, seed)
	}
}
