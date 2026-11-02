package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

// A squash carries the original baseline bytes without retaining the topic
// commit as an ancestor. Intervening source changes make its hashes stale.
func seedSquashedBaseline(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	h.commit("initialize memory")
	h.git("checkout", "-q", "-b", "original-baseline")
	baseline := h.ok("reconcile", "--baseline-generate")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("original baseline snapshot")
	original := h.git("rev-parse", "HEAD")
	h.git("checkout", "-q", "main")
	writeFixtureFile(t, h.dir, "README.md", "Source changed before the baseline landed.\n", 0o644)
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("squash baseline alongside intervening source changes")
	return h, original, h.git("rev-parse", "HEAD")
}

func refreshReplayCandidate(t *testing.T, h *harness) {
	t.Helper()
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("refresh candidate baseline")
}

func replayCheckArgs(trusted, original string) []string {
	return []string{"reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted, "--baseline-replay-ref", original}
}

func legacyV2Baseline(t *testing.T, h *harness) string {
	t.Helper()
	baseline, err := reconcile.ParseBaseline([]byte(h.ok("reconcile", "--baseline-generate")))
	if err != nil {
		t.Fatal(err)
	}
	baseline.Generator = "memory-reconcile/v2"
	body, err := reconcile.MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestBaselineReproductionOptionsAreGeneratorBounded(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		generator      string
		wantLogs       bool
		wantCodeV2     bool
		wantCoverageV3 bool
		wantSourcesV4  bool
		wantNativeV5   bool
	}{
		{generator: "memory-reconcile/v1", wantLogs: true, wantCodeV2: true, wantCoverageV3: true, wantSourcesV4: true, wantNativeV5: true},
		{generator: "memory-reconcile/v2", wantCodeV2: true, wantCoverageV3: true, wantSourcesV4: true, wantNativeV5: true},
		{generator: "memory-reconcile/v3", wantCoverageV3: true, wantSourcesV4: true, wantNativeV5: true},
		{generator: "memory-reconcile/v4", wantSourcesV4: true, wantNativeV5: true},
		{generator: "memory-reconcile/v5", wantNativeV5: true},
		{generator: "memory-reconcile/v6"},
		{generator: "memory-reconcile/v7"},
		{generator: "memory-reconcile/v0"},
		{generator: "memory-reconcile/future"},
	} {
		t.Run(tc.generator, func(t *testing.T) {
			got := baselineReproductionOptions(reconcile.Options{LegacySessionLogs: true, LegacyCodeInventory: true, LegacyContractCoverage: true, LegacySourceRetirement: true}, tc.generator)
			if got.LegacySourceRetirement != tc.wantSourcesV4 {
				t.Fatalf("historical source obligations = %t, want %t", got.LegacySourceRetirement, tc.wantSourcesV4)
			}
			if got.LegacyContractCoverage != tc.wantCoverageV3 {
				t.Fatalf("historical coverage = %t, want %t", got.LegacyContractCoverage, tc.wantCoverageV3)
			}
			if got.LegacySessionLogs != tc.wantLogs || got.LegacyCodeInventory != tc.wantCodeV2 {
				t.Fatalf("compatibility = logs:%t code-v2:%t, want logs:%t code-v2:%t", got.LegacySessionLogs, got.LegacyCodeInventory, tc.wantLogs, tc.wantCodeV2)
			}
			if got.LegacyNativeServes != tc.wantNativeV5 || got.LegacyNativeEvaluator != (tc.generator == "memory-reconcile/v5") {
				t.Fatalf("native serves replay = legacy:%t evaluator-v5:%t", got.LegacyNativeServes, got.LegacyNativeEvaluator)
			}
			if got.LegacyCodeCoverage != (tc.wantNativeV5 || tc.generator == "memory-reconcile/v6") {
				t.Fatalf("legacy code coverage = %t for %s", got.LegacyCodeCoverage, tc.generator)
			}
		})
	}
}

func TestE2EV2BaselineDirectRebuildKeepsCodeAsDerivedInventory(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("initialize memory")
	v2Baseline := legacyV2Baseline(t, h)
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, v2Baseline, 0o644)
	const zsh = ".pyxis/scripts/startup/codex-worktree.zsh"
	writeFixtureFile(t, h.dir, zsh, "codex() { command codex \"$@\"; }\n", 0o644)
	h.commit("carry v2 baseline with sourced zsh")
	trusted := h.git("rev-parse", "HEAD")

	code, _, errout := h.run("reconcile", "--write")
	if code != exitOK {
		t.Fatalf("source capture required a zsh summary: %d %s", code, errout)
	}
	h.commit("capture authored source")
	refreshReplayCandidate(t, h)
	out := h.ok(append(replayCheckArgs(trusted, "does-not-exist"), "--json")...)
	var got reconciliationRatchetOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.BaselineRef == nil || got.BaselineRef.ReplayCommit != "" || got.BaselineRef.Result == nil || got.BaselineRef.Result.Blocking() {
		t.Fatalf("trusted v2 baseline was not rebuilt directly: %s", out)
	}
}

func TestE2EV2BaselineReplayUsesExactPreZshClosureAndCurrentInventory(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("initialize memory")
	v2Baseline := legacyV2Baseline(t, h)

	h.git("checkout", "-q", "-b", "original-v2-baseline")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, v2Baseline, 0o644)
	const zsh = ".pyxis/scripts/startup/codex-worktree.zsh"
	writeFixtureFile(t, h.dir, zsh, "codex() { command codex \"$@\"; }\n", 0o644)
	h.commit("carry v2 baseline with sourced zsh")
	original := h.git("rev-parse", "HEAD")

	h.git("checkout", "-q", "-b", "bad-v2-replay", original)
	writeFixtureFile(t, h.dir, "README.md", "different legacy source closure\n", 0o644)
	h.commit("change a legacy inventory input")
	badReplay := h.git("rev-parse", "HEAD")

	h.git("checkout", "-q", "main")
	writeFixtureFile(t, h.dir, "README.md", "source changed before the v2 baseline landed\n", 0o644)
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, v2Baseline, 0o644)
	writeFixtureFile(t, h.dir, zsh, "codex() { command codex \"$@\"; }\n", 0o644)
	h.commit("squash v2 baseline alongside intervening source changes")
	trusted := h.git("rev-parse", "HEAD")

	code, out, errout := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("current reconciliation missed the changed authored source: %d %s %s", code, out, errout)
	}
	report := decodeReconciliationRun(t, out).Report
	if report.Counts.Code != 1 || hasFinding(report.Findings, "missing-code-memory") || hasFinding(report.Findings, "missing-code-anchor") {
		t.Fatalf("current report failed to derive zsh inventory: %#v", report)
	}

	code, _, errout = h.run("reconcile", "--write")
	if code != exitOK {
		t.Fatalf("source capture required a zsh summary: %d %s", code, errout)
	}
	h.commit("capture current source")
	refreshReplayCandidate(t, h)

	out = h.ok(append(replayCheckArgs(trusted, original), "--json")...)
	var got reconciliationRatchetOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.BaselineRef == nil || got.BaselineRef.ReplayCommit != original || got.BaselineRef.Result == nil || got.BaselineRef.Result.Blocking() {
		t.Fatalf("v2 replay did not prove the exact pre-zsh closure: %s", out)
	}

	code, out, errout = h.run(replayCheckArgs(trusted, badReplay)...)
	if code != exitRefused || out != "" || !strings.Contains(errout, "does not reproduce all four trusted input identities") {
		t.Fatalf("v2 replay accepted a changed legacy closure: %d %s %s", code, out, errout)
	}
}

func TestE2EBaselineReplayAfterSquashRetainsTrustedAuthority(t *testing.T) {
	t.Parallel()
	h, original, trusted := seedSquashedBaseline(t)
	if h.git("merge-base", original, trusted) == original {
		t.Fatal("fixture must replay a non-ancestor snapshot lost through squash")
	}
	h.ok("reconcile", "--write")
	h.commit("repair current source memory")
	refreshReplayCandidate(t, h)
	code, _, errout := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(errout, "cannot be reproduced") {
		t.Fatalf("stale baseline passed without proof: %d %s", code, errout)
	}
	args := replayCheckArgs(trusted, original)
	out := h.ok(append(args, "--json")...)
	var got reconciliationRatchetOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.BaselineRef.Commit != trusted || got.BaselineRef.ReplayCommit != original || !got.InputIdentitiesMatch || !got.Result.Exact() || got.BaselineRef.Result.Blocking() {
		t.Fatalf("replay replaced trusted authority or relaxed exactness: %s", out)
	}
	if text := h.ok(args...); !strings.Contains(text, "inputs replayed exactly at "+original) {
		t.Fatalf("human output omitted replay evidence: %s", text)
	}
	h.ok("check", "--base", trusted)
	// Once the repaired baseline becomes trusted, no historical replay is
	// needed. A stale optional replay argument must not create a dependency.
	fresh := h.git("rev-parse", "HEAD")
	h.git("commit", "-q", "--allow-empty", "-m", "later unchanged work")
	h.ok(replayCheckArgs(fresh, "no-longer-available")...)
}

func TestE2EBaselineReplayRejectsUnprovenSnapshots(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *harness)
		want   string
	}{
		{name: "changed inputs", mutate: func(t *testing.T, h *harness) { writeFixtureFile(t, h.dir, "README.md", "different\n", 0o644) }, want: "does not reproduce all four"},
		{name: "different baseline", mutate: func(t *testing.T, h *harness) {
			body, err := os.ReadFile(filepath.Join(h.dir, reconciliationBaselinePath))
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body)+"\n", 0o644)
		}, want: "unchanged trusted baseline"},
		{name: "missing baseline", mutate: func(t *testing.T, h *harness) {
			if err := os.Remove(filepath.Join(h.dir, reconciliationBaselinePath)); err != nil {
				t.Fatal(err)
			}
		}, want: "unchanged trusted baseline"},
		{name: "invalid baseline", mutate: func(t *testing.T, h *harness) { writeFixtureFile(t, h.dir, reconciliationBaselinePath, "{}\n", 0o644) }, want: "schema_version"},
		{name: "invalid config", mutate: func(t *testing.T, h *harness) {
			writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "unknown_replay_key: true\n", 0o644)
		}, want: "configuration at"},
		{name: "irregular source", mutate: func(t *testing.T, h *harness) {
			name := filepath.Join(h.dir, "README.md")
			if err := os.Remove(name); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("absent.md", name); err != nil {
				t.Fatal(err)
			}
		}, want: "not a regular file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, original, trusted := seedSquashedBaseline(t)
			h.git("checkout", "-q", "-b", "bad-replay", original)
			tc.mutate(t, h)
			h.commit("invalid replay evidence")
			bad := h.git("rev-parse", "HEAD")
			h.git("checkout", "-q", "main")
			refreshReplayCandidate(t, h)
			code, out, errout := h.run(replayCheckArgs(trusted, bad)...)
			if code != exitRefused || out != "" || !strings.Contains(errout, tc.want) {
				t.Fatalf("unproven replay passed: %d %s %s", code, out, errout)
			}
			code, out, errout = h.run(replayCheckArgs(trusted, "does-not-exist")...)
			if code != exitRefused || out != "" || !strings.Contains(errout, "resolve baseline replay ref") {
				t.Fatalf("missing replay ref passed: %d %s %s", code, out, errout)
			}
		})
	}
}

func TestE2EBaselineReplayStillRejectsWidening(t *testing.T) {
	t.Parallel()
	h, original, trusted := seedSquashedBaseline(t)
	writeFixtureFile(t, h.dir, "docs/new-uncovered.md", "New uncovered source.\n", 0o644)
	h.commit("introduce new debt")
	refreshReplayCandidate(t, h)
	code, out, errout := h.run(replayCheckArgs(trusted, original)...)
	if code != exitRefused || !strings.Contains(out, "missing-document docs/new-uncovered.md") || !strings.Contains(errout, "widens") {
		t.Fatalf("replay laundered new debt: %d %s %s", code, out, errout)
	}
}

func TestE2EBaselineReplayStillProtectsSourceRetirement(t *testing.T) {
	t.Parallel()
	for _, beforeTrusted := range []bool{true, false} {
		t.Run(map[bool]string{true: "between replay and trusted base", false: "after trusted base"}[beforeTrusted], func(t *testing.T) {
			h, original, trusted := seedSquashedBaseline(t)
			if err := os.Remove(filepath.Join(h.dir, "README.md")); err != nil {
				t.Fatal(err)
			}
			h.commit("remove previously authored source")
			if beforeTrusted {
				trusted = h.git("rev-parse", "HEAD")
			}
			refreshReplayCandidate(t, h)
			code, out, errout := h.run(replayCheckArgs(trusted, original)...)
			if code != exitRefused || !strings.Contains(out, "authored-source-retirement README.md") || !strings.Contains(errout, "widens") {
				t.Fatalf("replay erased source retirement: %d %s %s", code, out, errout)
			}
		})
	}
}

func TestE2EBaselineReplayDoesNotInventKnowledgeForInterveningCodeChange(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\nconst Answer = 1\n", 0o644)
	h.commit("seed production code")
	code, out, errout := h.run("reconcile", "--write")
	if code != exitOK {
		t.Fatalf("unexpected code coverage obligation: %d %s %s", code, out, errout)
	}
	h.commit("capture authored source")
	refreshReplayCandidate(t, h)
	original := h.git("rev-parse", "HEAD")
	before := memoryRecordCount(t, h.dir)
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\nconst Answer = 2\n", 0o644)
	h.commit("change code while carrying the original baseline")
	trusted := h.git("rev-parse", "HEAD")
	h.ok("reconcile", "--write")
	refreshReplayCandidate(t, h)
	code, out, errout = h.run(replayCheckArgs(trusted, original)...)
	if code != exitOK || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("replay required invented knowledge: %d %s %s", code, out, errout)
	}
}
