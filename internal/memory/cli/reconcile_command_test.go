package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestOfflineCommandUsageAndRouting(t *testing.T) {
	t.Parallel()
	h := newHarness(t, t.TempDir())
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "reconcile needs a mode", args: []string{"reconcile"}, want: "exactly one of --check, --write, --baseline-generate, or --baseline-sync"},
		{name: "reconcile modes are exclusive", args: []string{"reconcile", "--check", "--write"}, want: "exactly one of --check, --write, --baseline-generate, or --baseline-sync"},
		{name: "reconcile generator is exclusive", args: []string{"reconcile", "--check", "--baseline-generate"}, want: "exactly one of --check, --write, --baseline-generate, or --baseline-sync"},
		{name: "reconcile generator has no JSON flag", args: []string{"reconcile", "--baseline-generate", "--json"}, want: "baseline-generate has no other flags"},
		{name: "reconcile rejects positionals", args: []string{"reconcile", "--check", "extra"}, want: "exactly one of --check, --write, --baseline-generate, or --baseline-sync"},
		{name: "sync requires authority", args: []string{"reconcile", "--baseline-sync"}, want: "--baseline-sync requires --baseline-ref"},
		{name: "migration requires sync", args: []string{"reconcile", "--check", "--migrate-v2"}, want: "--baseline-sync requires"},
		{name: "sync rejects actor", args: []string{"reconcile", "--baseline-sync", "--baseline-ref", "main", "--actor", "agent:test"}, want: "--baseline-sync requires"},
		{name: "reconcile rejects unknown flags", args: []string{"reconcile", "--unknown"}, want: "flag provided but not defined"},
		{name: "reconcile baseline is check only", args: []string{"reconcile", "--write", "--baseline", ".github/baseline.json"}, want: "--check accepts"},
		{name: "reconcile baseline ref needs baseline", args: []string{"reconcile", "--check", "--baseline-ref", "main"}, want: "--check accepts"},
		{name: "reconcile replay needs trusted ref", args: []string{"reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-replay-ref", "main"}, want: "--check accepts"},
		{name: "reconcile baseline ref is check only", args: []string{"reconcile", "--write", "--baseline", ".github/baseline.json", "--baseline-ref", "main"}, want: "--check accepts"},
		{name: "reconcile check rejects writer actor", args: []string{"reconcile", "--check", "--actor", "agent:test"}, want: "--write accepts"},
		{name: "reconcile check rejects action selector", args: []string{"reconcile", "--check", "--action", "capture-document|README.md"}, want: "--action is accepted only with --write"},
		{name: "reconcile baseline generator rejects action selector", args: []string{"reconcile", "--baseline-generate", "--action", "capture-document|README.md"}, want: "--action is accepted only with --write"},
		{name: "reconcile rejects malformed action selector", args: []string{"reconcile", "--write", "--action", "capture-document"}, want: "must be an exact code|scope selector"},
		{name: "reconcile rejects empty action scope", args: []string{"reconcile", "--write", "--action", "capture-document|"}, want: "must be an exact code|scope selector"},
		{name: "reconcile rejects unknown action code", args: []string{"reconcile", "--write", "--action", "invent-document|README.md"}, want: "uses unknown action code"},
		{name: "reconcile rejects duplicate action selector", args: []string{"reconcile", "--write", "--action", "capture-document|README.md", "--action", "capture-document|README.md"}, want: "repeats selector"},
		{name: "closeout needs a feature", args: []string{"closeout", "--check"}, want: "closeout --feature name"},
		{name: "closeout needs a mode", args: []string{"closeout", "--feature", "widget"}, want: "exactly one of --check [--json] or --write"},
		{name: "closeout modes are exclusive", args: []string{"closeout", "--feature", "widget", "--check", "--write"}, want: "exactly one of --check [--json] or --write"},
		{name: "closeout check rejects writer actor", args: []string{"closeout", "--feature", "widget", "--check", "--actor", "agent:test"}, want: "--write [--actor who]"},
		{name: "closeout rejects positionals", args: []string{"closeout", "--feature", "widget", "--check", "extra"}, want: "closeout --feature name"},
		{name: "closeout rejects unknown flags", args: []string{"closeout", "--unknown"}, want: "flag provided but not defined"},
		{name: "verify requires offline", args: []string{"verify"}, want: "verify requires --offline"},
		{name: "verify rejects positionals", args: []string{"verify", "--offline", "extra"}, want: "verify requires --offline"},
		{name: "verify rejects unknown flags", args: []string{"verify", "--unknown"}, want: "flag provided but not defined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errout := h.run(tc.args...)
			if code != exitUsage || out != "" || !strings.Contains(errout, tc.want) {
				t.Fatalf("Run(%v) = exit %d, stdout %q, stderr %q; want usage containing %q", tc.args, code, out, errout, tc.want)
			}
		})
	}

	for _, args := range [][]string{
		{"reconcile", "--check"},
		{"reconcile", "--write", "--actor", "agent:test"},
		{"reconcile", "--baseline-generate"},
		{"closeout", "--feature", "widget", "--check"},
		{"closeout", "--feature", "widget", "--write", "--actor", "agent:test"},
		{"verify", "--offline"},
	} {
		code, out, errout := h.run(args...)
		if code != exitUsage || out != "" || !strings.Contains(errout, "not inside a git repository") {
			t.Fatalf("Run(%v) outside git = exit %d, stdout %q, stderr %q", args, code, out, errout)
		}
	}
}

func TestReconcileWriteActionSelectors(t *testing.T) {
	t.Parallel()
	newRepo := func(t *testing.T) *harness {
		t.Helper()
		h := initRepo(t)
		writeFixtureFile(t, h.dir, "docs/second.md", "# Second\n", 0o644)
		h.commit("add second source")
		if err := os.MkdirAll(filepath.Join(h.dir, store.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
		return h
	}

	t.Run("a zero match refuses the whole selection before writing", func(t *testing.T) {
		h := newRepo(t)
		before := memoryRecordCount(t, h.dir)
		code, out, errout := h.run("reconcile", "--write", "--actor", "agent:test", "--json",
			"--action", "capture-document|README.md",
			"--action", "capture-document|docs/absent.md")
		if code != exitRefused || out != "" || !strings.Contains(errout, `--action "capture-document|docs/absent.md" matches no current deterministic action`) {
			t.Fatalf("zero-match selection = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
		}
		if after := memoryRecordCount(t, h.dir); after != before {
			t.Fatalf("zero-match selection appended records: before %d, after %d", before, after)
		}
	})

	t.Run("an exact subset writes only that action", func(t *testing.T) {
		h := newRepo(t)
		code, out, errout := h.run("reconcile", "--write", "--actor", "agent:test", "--json",
			"--action", "capture-document|README.md")
		if code != exitRefused {
			t.Fatalf("selected write = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
		}
		selected := decodeReconciliationRun(t, out)
		if selected.Written != 1 || selected.Report.Counts.Actions != 1 || len(selected.Report.Findings) != 1 || selected.Report.Findings[0].Code != "missing-document" || selected.Report.Findings[0].Scope != "docs/second.md" {
			t.Fatalf("selected write report = written %d counts %+v findings %+v", selected.Written, selected.Report.Counts, selected.Report.Findings)
		}
		if got := memoryRecordCount(t, h.dir); got != 1 {
			t.Fatalf("selected write appended %d records, want 1", got)
		}

		code, out, errout = h.run("reconcile", "--write", "--actor", "agent:test", "--json",
			"--action", "capture-document|docs/second.md")
		if code != exitOK || errout != "" {
			t.Fatalf("remaining selected write = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
		}
		remaining := decodeReconciliationRun(t, out)
		if remaining.Written != 1 || remaining.Report.Blocking() || remaining.Report.Counts.Actions != 0 {
			t.Fatalf("remaining selected write report = %+v", remaining)
		}
	})

	t.Run("no selector preserves all-action behavior", func(t *testing.T) {
		h := newRepo(t)
		code, out, errout := h.run("reconcile", "--write", "--actor", "agent:test", "--json")
		if code != exitOK || errout != "" {
			t.Fatalf("unscoped write = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
		}
		result := decodeReconciliationRun(t, out)
		if result.Written != 2 || result.Report.Blocking() || result.Report.Counts.Actions != 0 {
			t.Fatalf("unscoped write report = %+v", result)
		}
	})
}

func TestSelectReconciliationActionsRejectsAmbiguousCurrentAction(t *testing.T) {
	t.Parallel()
	action := reconcile.Action{Code: "capture-document", Scope: "README.md"}
	_, err := selectReconciliationActions([]reconcile.Action{action, action}, []reconciliationActionSelector{{Code: action.Code, Scope: action.Scope}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous across 2") {
		t.Fatalf("ambiguous action selection error = %v", err)
	}
}

func TestReconcileCheckedInBaselineRatchetsWithoutWeakeningStrictCheck(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	code, out, errout := h.run("reconcile", "--check", "--json")
	if code != exitRefused || errout == "" {
		t.Fatalf("strict seed check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	bootstrapRef := h.git("rev-parse", "HEAD")
	seed := decodeReconciliationRun(t, out).Report
	baseline := reconcile.NewBaseline(seed)
	code, generated, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("baseline generation = exit %d\nstdout: %s\nstderr: %s", code, generated, errout)
	}
	parsedGenerated, err := reconcile.ParseBaseline([]byte(generated))
	if err != nil || !reflect.DeepEqual(parsedGenerated, baseline) {
		t.Fatalf("generated baseline = %#v, %v; want %#v", parsedGenerated, err, baseline)
	}
	body, err := reconcile.MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	baselinePath := ".github/memory-reconciliation-baseline.json"
	writeFixtureFile(t, h.dir, baselinePath, string(body), 0o644)
	h.commit("record reconciliation debt ceiling")
	baselineCommit := h.git("rev-parse", "HEAD")

	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", bootstrapRef)
	if code != exitOK || errout != "" {
		t.Fatalf("bootstrap baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	want(t, out, "offline reconciliation ratchet", "1 current finding(s), 1 baseline finding(s)", "bootstrap (path absent; candidate exactly matches current findings)", "baseline inputs: exact", "debt ratchet: exact", "strict reconciliation reports 1 blocker(s)")

	writeFixtureFile(t, h.dir, ".github/ratchet-test-marker", "advance\n", 0o644)
	h.commit("advance beyond trusted baseline")
	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", baselineCommit)
	if code != exitOK || errout != "" {
		t.Fatalf("unchanged trusted baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	want(t, out, "baseline monotonicity: ok; 0 tightening(s)")

	wider := baseline
	wider.Allowances = append(append([]reconcile.BaselineAllowance(nil), baseline.Allowances...), reconcile.BaselineAllowance{
		Code: "zz-future-debt", Scope: "future/scope", Count: 1, Blocking: true,
	})
	wider.FindingCount++
	widerBody, err := reconcile.MarshalBaseline(wider)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, baselinePath, string(widerBody), 0o644)
	h.commit("attempt to widen reconciliation ceiling")
	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", baselineCommit)
	if code != exitRefused || !strings.Contains(out, "[baseline-regression] zz-future-debt future/scope") || !strings.Contains(errout, "checked-in reconciliation baseline widens") {
		t.Fatalf("widened baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	writeFixtureFile(t, h.dir, baselinePath, string(body), 0o644)
	h.commit("restore reconciliation ceiling")
	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", "HEAD")
	if code != exitRefused || out != "" || !strings.Contains(errout, "must be a strict ancestor") {
		t.Fatalf("self-trusted baseline ref = exit %d, stdout %q, stderr %q", code, out, errout)
	}

	code, _, _ = h.run("reconcile", "--check")
	if code != exitRefused {
		t.Fatalf("ordinary strict reconciliation was weakened: exit %d", code)
	}
	code, _, errout = h.run("closeout", "--feature", "missing", "--check")
	if code != exitRefused || !strings.Contains(errout, "feature missing has 0 compiled PRD ledgers") {
		t.Fatalf("strict closeout was weakened: exit %d, stderr %q", code, errout)
	}

	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--json")
	if code != exitOK || errout != "" {
		t.Fatalf("JSON baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	var machine reconciliationRatchetOutput
	decodeJSON(t, out, &machine)
	if machine.Schema != "memory-reconciliation-ratchet-run/v2" || machine.BaselinePath != baselinePath || machine.BaselineSHA256 == "" || machine.Result.Blocking() || len(machine.Report.Findings) != 1 {
		t.Fatalf("JSON baseline output = %#v", machine)
	}
	if !machine.InputIdentitiesMatch || !machine.Result.Exact() {
		t.Fatalf("JSON baseline did not report an exact inventory: %#v", machine)
	}

	writeFixtureFile(t, h.dir, "docs/new-debt.md", "# New debt\n", 0o644)
	h.commit("introduce a new authored source gap")
	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", baselineCommit)
	if code != exitRefused || !strings.Contains(out, "[regression] missing-document docs/new-debt.md") || !strings.Contains(errout, "1 regression(s) beyond") {
		t.Fatalf("regressed baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	code, out, errout = h.run("reconcile", "--check", "--baseline", baselinePath, "--baseline-ref", "does-not-exist")
	if code != exitRefused || !strings.Contains(out, "evidence-history-unavailable") || !strings.Contains(errout, "durable evidence failed") {
		t.Fatalf("missing baseline ref = exit %d, stdout %q, stderr %q", code, out, errout)
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{name: "unsafe path", path: "../baseline.json", want: "one canonical lineage"},
		{name: "substitute path", path: ".github/absent.json", want: "one canonical lineage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errout := h.run("reconcile", "--check", "--baseline", tc.path)
			if code != exitUsage || out != "" || !strings.Contains(errout, tc.want) {
				t.Fatalf("baseline %q = exit %d, stdout %q, stderr %q", tc.path, code, out, errout)
			}
		})
	}
}

func TestReconcileBaselineBootstrapRejectsAnyTrustedTreeDeletion(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	trusted := h.git("rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(h.dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	h.commit("delete source before first baseline")
	code, generated, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("generate candidate baseline = exit %d\nstdout: %s\nstderr: %s", code, generated, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, generated, 0o644)
	h.commit("attempt first baseline after deletion")

	code, out, errout := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(out, "bootstrap-path-deletion README.md") || !strings.Contains(out, "baseline monotonicity: blocked") || !strings.Contains(errout, "checked-in reconciliation baseline widens") {
		t.Fatalf("bootstrap deletion check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestReconcileBaselineRequiresEveryTrustedCompilerIdentityToReplay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*reconcile.Baseline)
	}{
		{name: "code inventory", mutate: func(b *reconcile.Baseline) { b.CodeInventorySHA256 = strings.Repeat("e", 64) }},
		{name: "design index", mutate: func(b *reconcile.Baseline) { b.DesignIndexSHA256 = strings.Repeat("d", 64) }},
		{name: "memory closure", mutate: func(b *reconcile.Baseline) { b.MemoryClosureSHA256 = strings.Repeat("f", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := initRepo(t)
			h.ok("init")
			code, out, _ := h.run("reconcile", "--check", "--json")
			if code != exitRefused {
				t.Fatalf("seed check = exit %d", code)
			}
			exact := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
			trustedBaseline := exact
			tc.mutate(&trustedBaseline)
			body, err := reconcile.MarshalBaseline(trustedBaseline)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
			h.commit("record unreproducible trusted identity")
			trusted := h.git("rev-parse", "HEAD")

			body, err = reconcile.MarshalBaseline(exact)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
			h.commit("restore exact candidate identity")

			code, out, errout := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
			if code != exitRefused || out != "" || !strings.Contains(errout, "source, design, code, and memory identities cannot be reproduced") {
				t.Fatalf("trusted %s replay = exit %d\nstdout: %s\nstderr: %s", tc.name, code, out, errout)
			}
		})
	}
}

func TestReconcileBaselineRebuildsTrustedInputsFromTrustedMemory(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	writeFixtureFile(t, h.dir, "notes/context.txt", "historical optional context\n", 0o644)
	h.ok("init")
	code, out, _ := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("seed check = exit %d", code)
	}
	trustedBaseline := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
	body, err := reconcile.MarshalBaseline(trustedBaseline)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("record baseline before optional source is memory-bound")
	trusted := h.git("rev-parse", "HEAD")

	h.add("--kind", "Document", "--subject", "source/notes-context", "--family", "document",
		"--statement", "Captured optional historical context.", "--body-file", "notes/context.txt", "--literal-body",
		"--source", "notes/context.txt", "--commit", trusted)
	code, generated, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("candidate baseline generation = exit %d\nstdout: %s\nstderr: %s", code, generated, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, generated, 0o644)
	h.commit("bind optional source and its current identity")

	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitOK || errout != "" || !strings.Contains(out, "baseline monotonicity: ok") {
		t.Fatalf("candidate memory poisoned trusted source options: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestReconcileBaselineBindsInputInventoriesEvenWhenFindingsAreStable(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	code, out, _ := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("seed strict check = exit %d", code)
	}
	baseline := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
	body, err := reconcile.MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("record debt inventory")
	writeFixtureFile(t, h.dir, "README.md", "# Same gap, different source bytes\n", 0o644)
	h.commit("change uncovered source")

	code, out, errout := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath)
	if code != exitRefused || !strings.Contains(out, "baseline inputs: stale") || !strings.Contains(out, "0 improvement(s)") || !strings.Contains(errout, "does not match the checked-in baseline input identities") {
		t.Fatalf("stale identities = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestReconcileBaselineRequiresImmediateTighteningAfterImprovement(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	code, out, _ := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("seed strict check = exit %d", code)
	}
	seed := decodeReconciliationRun(t, out).Report
	baseline := reconcile.NewBaseline(seed)
	body, err := reconcile.MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("record debt inventory")
	trusted := h.git("rev-parse", "HEAD")

	code, _, errout := h.run("reconcile", "--write", "--actor", "agent:test")
	if code != exitOK || errout != "" {
		t.Fatalf("safe repair = exit %d, stderr %q", code, errout)
	}
	h.commit("resolve source gap")
	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(out, "[baseline-stale] missing-document README.md") || !strings.Contains(out, "baseline update required") || !strings.Contains(errout, "tighten the checked-in baseline") {
		t.Fatalf("stale improved baseline = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}

	code, strict, errout := h.run("reconcile", "--check", "--json")
	if code != exitOK || errout != "" {
		t.Fatalf("strict check after repair = exit %d, stderr %q", code, errout)
	}
	tight := reconcile.NewBaseline(decodeReconciliationRun(t, strict).Report)
	tightBody, err := reconcile.MarshalBaseline(tight)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(tightBody), 0o644)
	h.commit("tighten debt inventory")
	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitOK || errout != "" || !strings.Contains(out, "baseline monotonicity: ok; 1 tightening(s)") || !strings.Contains(out, "debt ratchet: exact") {
		t.Fatalf("tightened baseline = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}

	writeFixtureFile(t, h.dir, "docs/reintroduced.md", "# Debt returns\n", 0o644)
	h.commit("reintroduce debt elsewhere")
	code, _, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(errout, "regression(s) beyond") {
		t.Fatalf("reintroduced debt = exit %d, stderr %q", code, errout)
	}
}

func TestReconcileBaselineRejectsDeletingAnUnbackfilledSource(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	code, out, _ := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("seed strict check = exit %d", code)
	}
	prior := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
	body, err := reconcile.MarshalBaseline(prior)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("record unbackfilled source debt")
	trusted := h.git("rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(h.dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	h.commit("delete source instead of backfilling it")
	code, out, errout := h.run("reconcile", "--check", "--json")
	if code != exitOK || errout != "" {
		t.Fatalf("current strict check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	candidate := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
	body, err = reconcile.MarshalBaseline(candidate)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("attempt to tighten by deletion")

	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(out, "authored source disappeared without a closeout proven at the trusted ref") || !strings.Contains(errout, "checked-in reconciliation baseline widens") {
		t.Fatalf("deleted-source baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestReconcileBaselineRejectsDeletingASourceAddressCollision(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.add("--kind", "Need", "--subject", "source/readme", "--family", "document", "--statement", "This occupied address must not make README deletion look like debt reduction.")
	code, out, errout := h.run("reconcile", "--check", "--json")
	if code != exitRefused || errout == "" {
		t.Fatalf("collision check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	report := decodeReconciliationRun(t, out).Report
	if !hasFinding(report.Findings, "source-address-collision") {
		t.Fatalf("fixture has no source-address-collision: %v", findingCodes(report.Findings))
	}
	body, err := reconcile.MarshalBaseline(reconcile.NewBaseline(report))
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("record colliding source debt")
	trusted := h.git("rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(h.dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	h.commit("delete colliding source")
	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitOK || errout != "" {
		t.Fatalf("candidate strict check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	body, err = reconcile.MarshalBaseline(reconcile.NewBaseline(decodeReconciliationRun(t, out).Report))
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(body), 0o644)
	h.commit("attempt collision laundering")

	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(out, "authored source disappeared without a closeout proven at the trusted ref") || !strings.Contains(errout, "checked-in reconciliation baseline widens") {
		t.Fatalf("collision deletion baseline check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestOfflineCommandsPlainWorkflowAndWriterErrors(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	head := h.git("rev-parse", "HEAD")

	code, out, errout := h.run("reconcile", "--check")
	if code != exitRefused {
		t.Fatalf("reconcile --check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	want(t, out,
		"offline reconciliation "+shortCommit(head),
		"[blocking, writable] missing-document README.md",
		"integrity: blocked by 1 finding(s); 1 safe action(s) available",
	)
	want(t, errout, "offline reconciliation at "+shortCommit(head)+" found 1 blocker(s)")

	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:test")
	if code != exitOK || errout != "" {
		t.Fatalf("reconcile --write = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	want(t, out, "appended 1 deterministic repair record(s)", "integrity: ok")

	code, out, errout = h.run("verify", "--offline")
	if code != exitOK || errout != "" {
		t.Fatalf("verify --offline = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	want(t, out, "active closeouts: none", "offline reconciliation "+shortCommit(head), "integrity: ok")

	for _, args := range [][]string{{"reconcile", "--check", "--json"}, {"reconcile", "--write", "--json"}, {"verify", "--offline", "--json"}} {
		env, _, stderr := h.env("")
		env.Stdout = rejectingWriter{err: errors.New("closed output")}
		if code := Run(context.Background(), args, env); code != exitRefused || !strings.Contains(stderr.String(), "closed output") {
			t.Fatalf("Run(%v) with closed stdout = exit %d, stderr %q", args, code, stderr.String())
		}
	}

	code, _, errout = h.run("closeout", "--feature", "missing", "--check")
	if code != exitRefused || !strings.Contains(errout, "feature missing has 0 compiled PRD ledgers") {
		t.Fatalf("closeout of absent feature = exit %d, stderr %q", code, errout)
	}
}

func TestReconcileWriteRefusesUnsafeMemoryBeforeAppending(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	bad := filepath.Join(h.dir, store.Dir, "broken.json")
	if err := os.WriteFile(bad, []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := memoryRecordCount(t, h.dir)

	code, out, errout := h.run("reconcile", "--write", "--actor", "agent:test", "--json")
	if code != exitRefused || !strings.Contains(errout, "automatic writes refused while invalid-memory-file remains") {
		t.Fatalf("unsafe reconcile = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	result := decodeReconciliationRun(t, out)
	if result.Written != 0 || !hasFinding(result.Report.Findings, "invalid-memory-file") {
		t.Fatalf("unsafe reconcile output = written %d, findings %v", result.Written, findingCodes(result.Report.Findings))
	}
	if after := memoryRecordCount(t, h.dir); after != before {
		t.Fatalf("unsafe reconcile appended records: before %d, after %d", before, after)
	}

	env, _, stderr := h.env("")
	env.Stdout = rejectingWriter{err: errors.New("unsafe report closed")}
	if code := Run(context.Background(), []string{"reconcile", "--write", "--json"}, env); code != exitRefused || !strings.Contains(stderr.String(), "unsafe report closed") {
		t.Fatalf("unsafe reconcile with closed stdout = exit %d, stderr %q", code, stderr.String())
	}
}

func TestOfflineCommandsRejectSnapshotCompilerFailure(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "docs/tech-design/index.json", "{}\n", 0o644)
	h.commit("add stale design index")

	for _, args := range [][]string{
		{"reconcile", "--check"},
		{"reconcile", "--write", "--actor", "agent:test"},
		{"closeout", "--feature", "widget", "--check"},
		{"closeout", "--feature", "widget", "--write", "--actor", "agent:test"},
		{"verify", "--offline"},
	} {
		code, out, errout := h.run(args...)
		if code != exitRefused || out != "" || !strings.Contains(errout, "technical-design index") {
			t.Fatalf("Run(%v) with stale compiler output = exit %d, stdout %q, stderr %q", args, code, out, errout)
		}
	}
}

func TestReconcileWriteRequiresActorOnlyWhenItHasActions(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.git("config", "--unset", "user.email")

	code, out, errout := h.run("reconcile", "--write")
	if code != exitUsage || out != "" || !strings.Contains(errout, "no actor") {
		t.Fatalf("reconcile without actor = exit %d, stdout %q, stderr %q", code, out, errout)
	}
	if got := memoryRecordCount(t, h.dir); got != 0 {
		t.Fatalf("actor refusal appended %d record(s)", got)
	}
}

func TestReconciliationOutputFormats(t *testing.T) {
	t.Parallel()
	report := reconcile.Report{
		SourceCommit: strings.Repeat("a", 40),
		Counts: reconcile.Counts{
			Sources: 2, PRDUnits: 3, Designs: 4, Code: 5, Blocking: 1, Actions: 1,
		},
		Findings: []reconcile.Finding{
			{Code: "blocked", Scope: "scope/a", Detail: "must repair", Blocking: true, Writable: true},
			{Code: "notice", Scope: "scope/b", Detail: "look here"},
		},
	}

	var human bytes.Buffer
	if err := writeReconciliationOutput(Env{Stdout: &human}, false, 2, report); err != nil {
		t.Fatal(err)
	}
	want(t, human.String(),
		"offline reconciliation aaaaaaaaaaaa: 2 source(s), 3 PRD unit(s), 4 design(s), 5 code unit(s)",
		"appended 2 deterministic repair record(s)",
		"[blocking, writable] blocked scope/a",
		"[advisory] notice scope/b",
		"integrity: blocked by 1 finding(s); 1 safe action(s) available",
	)

	human.Reset()
	report.Counts.Blocking = 0
	report.Counts.Actions = 0
	report.Findings = nil
	if err := writeReconciliationOutput(Env{Stdout: &human}, false, 0, report); err != nil {
		t.Fatal(err)
	}
	want(t, human.String(), "integrity: ok")
	if strings.Contains(human.String(), "appended") {
		t.Fatalf("zero-write output claims an append:\n%s", human.String())
	}

	var machine bytes.Buffer
	if err := writeReconciliationOutput(Env{Stdout: &machine}, true, 7, report); err != nil {
		t.Fatal(err)
	}
	var got reconciliationOutput
	if err := json.Unmarshal(machine.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != "memory-reconciliation-run/v1" || got.Written != 7 || got.Report.SourceCommit != report.SourceCommit {
		t.Fatalf("JSON reconciliation output = %+v", got)
	}
	if err := writeReconciliationOutput(Env{Stdout: rejectingWriter{err: errors.New("no report")}}, true, 0, report); err == nil || !strings.Contains(err.Error(), "write reconciliation report: no report") {
		t.Fatalf("JSON write error = %v", err)
	}

	ratchet := reconcile.BaselineResult{
		Schema: reconcile.BaselineResultSchema, BaselineCount: 2, CurrentCount: 3,
		Regressions: []reconcile.BaselineDelta{{
			Code: "new-gap", Scope: "scope/new", Reason: "new finding address", CurrentCount: 1,
		}},
	}
	human.Reset()
	if err := writeReconciliationRatchetOutput(Env{Stdout: &human}, false, ".github/baseline.json", "digest", true, nil, ratchet, report, policyEvidence{}); err != nil {
		t.Fatal(err)
	}
	want(t, human.String(), "offline reconciliation ratchet aaaaaaaaaaaa", "baseline .github/baseline.json: digest", "[regression] new-gap scope/new", "debt ratchet: blocked by 1 regression(s)")

	ratchet.Regressions = nil
	ratchet.Improvements = []reconcile.BaselineDelta{{Code: "old-gap", Scope: "scope/old", Reason: "finding resolved"}}
	human.Reset()
	if err := writeReconciliationRatchetOutput(Env{Stdout: &human}, false, ".github/baseline.json", "digest", true, nil, ratchet, report, policyEvidence{}); err != nil {
		t.Fatal(err)
	}
	want(t, human.String(), "[baseline-stale] old-gap scope/old", "debt ratchet: baseline update required (1 improvement(s), inputs exact: true)")

	baselineChange := reconcile.BaselineResult{
		Schema: reconcile.BaselineResultSchema, BaselineCount: 3, CurrentCount: 3,
		Regressions: []reconcile.BaselineDelta{}, Improvements: []reconcile.BaselineDelta{},
	}
	baselineRef := &reconciliationBaselineRefOutput{
		Ref: "origin/main", Commit: strings.Repeat("b", 40), Status: "compared",
		BaselineSHA256: "trusted-digest", Result: &baselineChange,
	}
	machine.Reset()
	if err := writeReconciliationRatchetOutput(Env{Stdout: &machine}, true, ".github/baseline.json", "digest", true, baselineRef, ratchet, report, policyEvidence{}); err != nil {
		t.Fatal(err)
	}
	var ratchetOutput reconciliationRatchetOutput
	if err := json.Unmarshal(machine.Bytes(), &ratchetOutput); err != nil {
		t.Fatal(err)
	}
	if ratchetOutput.Schema != "memory-reconciliation-ratchet-run/v2" || ratchetOutput.BaselineSHA256 != "digest" || ratchetOutput.Result.Blocking() || ratchetOutput.BaselineRef == nil || ratchetOutput.BaselineRef.Ref != "origin/main" {
		t.Fatalf("JSON ratchet output = %+v", ratchetOutput)
	}
	if err := writeReconciliationRatchetOutput(Env{Stdout: rejectingWriter{err: errors.New("no ratchet")}}, true, "baseline", "digest", true, nil, ratchet, report, policyEvidence{}); err == nil || !strings.Contains(err.Error(), "write reconciliation ratchet report: no ratchet") {
		t.Fatalf("JSON ratchet write error = %v", err)
	}
}

func TestCloseoutOutputFormats(t *testing.T) {
	t.Parallel()
	receipt := reconcile.CloseoutReceipt{
		Feature:         "widget",
		SourceCommit:    strings.Repeat("b", 40),
		ClosureSHA256:   strings.Repeat("c", 64),
		SourceDocuments: make([]reconcile.CloseoutSourceDocument, 2),
		NativeRecords:   make([]reconcile.CloseoutNativeRecord, 3),
		CodeUnits:       make([]reconcile.CloseoutCodeUnit, 4),
	}
	var human bytes.Buffer
	if err := writeCloseoutOutput(Env{Stdout: &human}, false, "sealed", "Document-receipt", receipt); err != nil {
		t.Fatal(err)
	}
	want(t, human.String(),
		"closeout widget: sealed at bbbbbbbbbbbb (2 source document(s), 3 native record(s), 4 code unit(s))",
		"receipt: Document-receipt",
		"closure: "+receipt.ClosureSHA256,
	)

	human.Reset()
	if err := writeCloseoutOutput(Env{Stdout: &human}, false, "ready", "", receipt); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(human.String(), "receipt:") {
		t.Fatalf("readiness output has an unwritten record id:\n%s", human.String())
	}

	var machine bytes.Buffer
	if err := writeCloseoutOutput(Env{Stdout: &machine}, true, "ready", "", receipt); err != nil {
		t.Fatal(err)
	}
	var got closeoutOutput
	if err := json.Unmarshal(machine.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != "memory-closeout-run/v1" || got.Status != "ready" || got.RecordID != "" || got.Receipt.Feature != "widget" {
		t.Fatalf("JSON closeout output = %+v", got)
	}
	if err := writeCloseoutOutput(Env{Stdout: rejectingWriter{err: errors.New("no closeout")}}, true, "ready", "", receipt); err == nil || !strings.Contains(err.Error(), "write closeout report: no closeout") {
		t.Fatalf("JSON closeout write error = %v", err)
	}
}

func TestSafeToAppendRejectsEveryStructuralHazard(t *testing.T) {
	t.Parallel()
	fatal := []string{
		"invalid-memory-file", "dangling-memory-relation", "invalid-memory-projection", "memory-family-conflict",
		"duplicate-source-binding", "source-mode-conflict", "source-address-collision", "reconcile-action-collision",
		"code-anchor-collision", "duplicate-code-anchor", "unmappable-code-anchor",
	}
	for _, code := range fatal {
		t.Run(code, func(t *testing.T) {
			err := safeToAppend(reconcile.Report{Findings: []reconcile.Finding{{Code: code, Scope: "where", Detail: "why"}}})
			if err == nil || !strings.Contains(err.Error(), code) || !strings.Contains(err.Error(), "where: why") {
				t.Fatalf("safeToAppend(%s) = %v", code, err)
			}
		})
	}
	if err := safeToAppend(reconcile.Report{Findings: []reconcile.Finding{{Code: "missing-code-memory", Blocking: true}}}); err != nil {
		t.Fatalf("semantic blocker must not prevent independent lossless writes: %v", err)
	}
}

func TestRecordFromDraftMaterializesCanonicalRecords(t *testing.T) {
	t.Parallel()
	needA := record.NewID(record.Need, t0.Add(-3*time.Second), [10]byte{9: 1})
	needB := record.NewID(record.Need, t0.Add(-2*time.Second), [10]byte{9: 2})
	promise := record.NewID(record.Promise, t0.Add(-time.Second), [10]byte{9: 3})
	m := &memory{set: &store.Set{Records: map[string]*record.Record{
		needA: {ID: needA}, needB: {ID: needB}, promise: {ID: promise},
	}}}
	newEnv := func() Env {
		return Env{
			Now: func() time.Time { return t0 },
			Entropy: func() ([10]byte, error) {
				return [10]byte{9: 4}, nil
			},
		}
	}

	decision, err := m.recordFromDraft(reconcile.Draft{
		Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "choose widget",
		Tags: []string{"zeta", "alpha"},
		Relations: []record.Relation{
			{Type: record.Serves, Target: promise},
			{Type: record.About, Target: needB},
			{Type: record.About, Target: needA},
		},
	}, "agent:test", newEnv())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decision.Tags, []string{"alpha", "zeta"}) || !reflect.DeepEqual(decision.Relations, []record.Relation{
		{Type: record.About, Target: needA},
		{Type: record.About, Target: needB},
		{Type: record.Serves, Target: promise},
	}) {
		t.Fatalf("canonical decision = tags %v, relations %v", decision.Tags, decision.Relations)
	}
	if decision.Issuer != "agent:test" || decision.CreatedAt != t0.Format(time.RFC3339) {
		t.Fatalf("issuance fields = %q at %q", decision.Issuer, decision.CreatedAt)
	}

	narrative := []byte("# Widget\r\n\r\n{{prd/widget#FR-1}}\r\n")
	doc, err := m.recordFromDraft(reconcile.Draft{
		Kind: record.Document, Subject: "source/widget", Family: "document", Statement: "widget source", Body: narrative,
	}, "agent:test", newEnv())
	if err != nil {
		t.Fatal(err)
	}
	if doc.BodySHA256 != record.HashBody(narrative) || !reflect.DeepEqual(doc.Embeds, []string{"prd/widget#FR-1"}) {
		t.Fatalf("native document digest/embeds = %s %v", doc.BodySHA256, doc.Embeds)
	}

	literal, err := m.recordFromDraft(reconcile.Draft{
		Kind: record.Document, Subject: "source/literal", Family: "document", Statement: "literal source", LiteralBody: true, Body: narrative,
	}, "agent:test", newEnv())
	if err != nil {
		t.Fatal(err)
	}
	raw := sha256.Sum256(narrative)
	if literal.BodySHA256 != hex.EncodeToString(raw[:]) || len(literal.Embeds) != 0 {
		t.Fatalf("literal document digest/embeds = %s %v", literal.BodySHA256, literal.Embeds)
	}

	_, err = m.recordFromDraft(reconcile.Draft{
		Kind: record.Decision, Subject: "design/widget", Family: "D-2", Statement: "missing target",
		Relations: []record.Relation{{Type: record.About, Target: record.NewID(record.Need, t0, [10]byte{9: 8})}},
	}, "agent:test", newEnv())
	if err == nil || !strings.Contains(err.Error(), "targets absent record") {
		t.Fatalf("absent draft relation error = %v", err)
	}

	_, err = m.recordFromDraft(reconcile.Draft{Kind: record.Need, Subject: "NOT-A-SUBJECT", Family: "G-1", Statement: "invalid"}, "agent:test", newEnv())
	if err == nil || !strings.Contains(err.Error(), "invalid record") {
		t.Fatalf("invalid draft error = %v", err)
	}

	badEnv := newEnv()
	badEnv.Entropy = func() ([10]byte, error) { return [10]byte{}, errors.New("entropy unavailable") }
	_, err = m.recordFromDraft(reconcile.Draft{Kind: record.Need}, "agent:test", badEnv)
	if err == nil || err.Error() != "entropy unavailable" {
		t.Fatalf("entropy error = %v", err)
	}
}

func TestReconciliationOptionsSelectOnlyRepositoryDocumentSources(t *testing.T) {
	t.Parallel()
	records := map[string]*record.Record{}
	add := func(kind, source string, n byte) {
		id := record.NewID(kind, t0.Add(time.Duration(n)*time.Second), [10]byte{9: n})
		records[id] = &record.Record{ID: id, Kind: kind, Provenance: record.Provenance{Source: source}}
	}
	add(record.Document, "notes/z.md", 1)
	add(record.Document, "docs/a.md", 2)
	add(record.Decision, "docs/not-a-document.md", 3)
	add(record.Document, "../escape.md", 4)
	add(record.Document, "docs/../escape.md", 5)
	add(record.Document, "/absolute.md", 6)
	add(record.Document, `C:\\windows.md`, 7)

	m := &memory{
		set: &store.Set{Records: records},
		cfg: config.Config{
			ProductVision: "governing/vision.md", ArchitecturePrinciples: "governing/principles.md", Incidents: "governing/incidents.md",
			PRDs: "requirements", TechDesigns: "architecture", Plans: "roadmaps",
			Grandfathered: []string{"old-b", "old-a"}, PrecedenceGrandfathered: []string{"legacy"},
		},
	}
	got := reconciliationOptions(m)
	if !reflect.DeepEqual(got.ExtraSources, []string{"docs/a.md", "notes/z.md"}) {
		t.Fatalf("extra sources = %v", got.ExtraSources)
	}
	if got.PRDs != "requirements" || got.TechDesigns != "architecture" || got.Plans != "roadmaps" {
		t.Fatalf("configured roots = %+v", got)
	}
	if got.ProductVision != "governing/vision.md" || got.ArchitecturePrinciples != "governing/principles.md" || got.Incidents != "governing/incidents.md" {
		t.Fatalf("configured governing sources = %+v", got)
	}
	if !reflect.DeepEqual(got.Grandfathered, []string{"old-b", "old-a"}) || !reflect.DeepEqual(got.PrecedenceGrandfathered, []string{"legacy"}) {
		t.Fatalf("configured exceptions = %+v", got)
	}
	m.cfg.Grandfathered[0] = "mutated"
	if got.Grandfathered[0] != "old-b" {
		t.Fatal("options alias the mutable config slices")
	}
}

func TestAssertHeadDetectsMovementAndGitErrors(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	head := h.git("rev-parse", "HEAD")
	m := &memory{repo: repo}
	if err := assertHead(context.Background(), m, head); err != nil {
		t.Fatalf("stable HEAD: %v", err)
	}
	if err := assertHead(context.Background(), m, strings.Repeat("f", 40)); err == nil || !strings.Contains(err.Error(), "HEAD moved during reconciliation: pinned ffffffffffff, now "+shortCommit(head)) {
		t.Fatalf("moved HEAD error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := assertHead(canceled, m, head); err == nil || !strings.Contains(err.Error(), "re-read HEAD") {
		t.Fatalf("canceled HEAD error = %v", err)
	}
}

func TestReconciliationSnapshotPropagatesCanceledGit(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reconciliationSnapshot(canceled, m); err == nil || !strings.Contains(err.Error(), "pin HEAD for offline reconciliation") {
		t.Fatalf("canceled snapshot error = %v", err)
	}
	if _, _, err := reconciliationPass(canceled, m); err == nil || !strings.Contains(err.Error(), "pin HEAD for offline reconciliation") {
		t.Fatalf("canceled reconciliation pass error = %v", err)
	}
}

func TestReconciliationBaselineReadFailureModes(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo}
	head := h.git("rev-parse", "HEAD")

	if code, _, errout := h.run("reconcile", "--check", "--baseline", "../baseline.json"); code != exitUsage || !strings.Contains(errout, "canonical lineage") {
		t.Fatalf("unsafe baseline path error = %s", errout)
	}
	if _, present, err := readPolicyAt(context.Background(), m, head); err != nil || present {
		t.Fatalf("absent baseline: present=%t error=%v", present, err)
	}

	writeFixtureFile(t, h.dir, reconciliationBaselinePath, "{}\n", 0o644)
	h.commit("add malformed reconciliation baseline")
	head = h.git("rev-parse", "HEAD")
	if _, _, err := readPolicyAt(context.Background(), m, head); err == nil || !strings.Contains(err.Error(), "reconciliation baseline") {
		t.Fatalf("malformed baseline error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := readPolicyAt(canceled, m, head); err == nil || !strings.Contains(err.Error(), "read reconciliation baseline") {
		t.Fatalf("canceled baseline read error = %v", err)
	}
}

func TestReconciliationBaselineRefRejectsInvalidGitRelationships(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo}
	head := h.git("rev-parse", "HEAD")
	snapshot := reconcile.Snapshot{Commit: head}

	if _, err := checkReconciliationBaselineRef(context.Background(), m, snapshot, reconcile.Report{}, reconciliationBaselinePath, "missing-ref", "", reconcile.Baseline{}, reconcile.BaselineResult{}); err == nil || !strings.Contains(err.Error(), "resolve reconciliation baseline ref") {
		t.Fatalf("missing baseline ref error = %v", err)
	}
	invalidSnapshot := reconcile.Snapshot{Commit: strings.Repeat("f", 40)}
	if _, err := checkReconciliationBaselineRef(context.Background(), m, invalidSnapshot, reconcile.Report{}, reconciliationBaselinePath, head, "", reconcile.Baseline{}, reconcile.BaselineResult{}); err == nil || !strings.Contains(err.Error(), "compare reconciliation baseline ref") {
		t.Fatalf("invalid candidate commit error = %v", err)
	}

	h.git("checkout", "-q", "-b", "left")
	writeFixtureFile(t, h.dir, "left.txt", "left\n", 0o644)
	h.commit("left branch")
	left := h.git("rev-parse", "HEAD")
	h.git("checkout", "-q", "main")
	h.git("checkout", "-q", "-b", "right")
	writeFixtureFile(t, h.dir, "right.txt", "right\n", 0o644)
	h.commit("right branch")
	right := h.git("rev-parse", "HEAD")
	if _, err := checkReconciliationBaselineRef(context.Background(), m, reconcile.Snapshot{Commit: right}, reconcile.Report{}, reconciliationBaselinePath, left, "", reconcile.Baseline{}, reconcile.BaselineResult{}); err == nil || !strings.Contains(err.Error(), "is not an ancestor") {
		t.Fatalf("non-ancestor baseline ref error = %v", err)
	}
}

func TestReconciliationHistoricalStateFailureModes(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo}

	writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "# valid defaults\n", 0o644)
	h.commit("add explicit default config")
	head := h.git("rev-parse", "HEAD")
	opts, set, err := reconciliationStateAt(context.Background(), m, head)
	if err != nil {
		t.Fatalf("valid historical state: %v", err)
	}
	if opts.PRDs != config.Defaults().PRDs || set == nil {
		t.Fatalf("historical state = opts %+v, set %#v", opts, set)
	}

	writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "unknown_key: forbidden\n", 0o644)
	h.commit("break historical config")
	head = h.git("rev-parse", "HEAD")
	if _, _, err := reconciliationStateAt(context.Background(), m, head); err == nil || !strings.Contains(err.Error(), "configuration at") || !strings.Contains(err.Error(), "defect") {
		t.Fatalf("invalid historical config error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := reconciliationStateAt(canceled, m, head); err == nil || !strings.Contains(err.Error(), "read configuration") {
		t.Fatalf("canceled historical state error = %v", err)
	}
	if _, _, err := reconciliationStateAt(context.Background(), m, "not-a-commit"); err == nil {
		t.Fatal("missing historical commit unexpectedly loaded")
	}
}

func TestStableCloseoutAndReloadFailureModes(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.git("branch", "-m", "topic")
	repo, err := gitsrc.Open(context.Background(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo, remote: "origin", branch: "main"}
	if _, err := stableMainlineCloseoutSnapshot(context.Background(), m); err == nil || !strings.Contains(err.Error(), "requires a trusted mainline commit") {
		t.Fatalf("missing mainline error = %v", err)
	}

	rootFile := filepath.Join(t.TempDir(), "memory-file")
	if err := os.WriteFile(rootFile, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&memory{dir: rootFile}).reload(context.Background()); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("reload regular-file root error = %v", err)
	}
}

func TestShortCommitPreservesShortIdentifiers(t *testing.T) {
	t.Parallel()
	for _, commit := range []string{"", "abc123", "abcdefghijkl"} {
		if got := shortCommit(commit); got != commit {
			t.Fatalf("shortCommit(%q) = %q", commit, got)
		}
	}
}

type rejectingWriter struct{ err error }

func (w rejectingWriter) Write([]byte) (int, error) { return 0, w.err }
