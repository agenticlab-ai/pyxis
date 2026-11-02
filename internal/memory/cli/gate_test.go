package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/rewriteauth"
)

type gateBrokenWriter struct{}

func (gateBrokenWriter) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func TestMemoryGateOutputFailures(t *testing.T) {
	t.Parallel()
	h, _ := memoryGateFixture(t)
	for _, branch := range []string{"topic", "main"} {
		h.git("checkout", branch)
		env, _, _ := h.env("")
		env.Stdout = gateBrokenWriter{}
		if code := RunGate(t.Context(), nil, env); code != exitRefused {
			t.Fatalf("%s output failure = %d", branch, code)
		}
	}
	env, _, _ := h.env("")
	env.Stderr = gateBrokenWriter{}
	if code := RunGate(t.Context(), []string{"unexpected"}, env); code != exitRefused {
		t.Fatalf("diagnostic output failure = %d", code)
	}
}

func TestAuthorizeDirectRewrite(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	env := Env{Stdout: &out}
	if handled, code, err := authorizeDirectRewrite(rewriteauth.Result{}, env); handled || code != 0 || err != nil {
		t.Fatalf("inactive authorization = handled %t code %d err %v", handled, code, err)
	}
	if handled, code, err := authorizeDirectRewrite(rewriteauth.Result{Active: true, Err: errors.New("bad digest")}, env); !handled || code != exitRefused || err == nil {
		t.Fatalf("invalid authorization = handled %t code %d err %v", handled, code, err)
	}
	if handled, code, err := authorizeDirectRewrite(rewriteauth.Result{Active: true}, env); !handled || code != exitRefused || err == nil {
		t.Fatalf("incomplete authorization = handled %t code %d err %v", handled, code, err)
	}
	if handled, code, err := authorizeDirectRewrite(rewriteauth.Result{Active: true, Authorized: true}, env); !handled || code != exitOK || err != nil || !strings.Contains(out.String(), "operator-authorized") {
		t.Fatalf("valid authorization = handled %t code %d err %v output %q", handled, code, err, out.String())
	}
	if handled, code, err := authorizeDirectRewrite(rewriteauth.Result{Active: true, Authorized: true}, Env{Stdout: gateBrokenWriter{}}); !handled || code != exitRefused || err == nil {
		t.Fatalf("output failure = handled %t code %d err %v", handled, code, err)
	}
}

func (h *harness) gate(args ...string) (int, string, string) {
	env, out, stderr := h.env("")
	code := RunGate(context.Background(), args, env)
	return code, out.String(), stderr.String()
}

func (h *harness) gateOK(args ...string) string {
	h.t.Helper()
	code, out, stderr := h.gate(args...)
	if code != exitOK {
		h.t.Fatalf("memory-gate %v: exit %d\n%s\n%s", args, code, out, stderr)
	}
	return out
}

func memoryGateFixture(t *testing.T, configs ...string) (*harness, string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	for _, config := range configs {
		writeFixtureFile(t, h.dir, ".pyxis/config.yaml", config, 0o644)
	}
	h.commit("initialize memory")
	if len(configs) > 0 {
		h.git("update-ref", "refs/remotes/upstream/trunk", "HEAD")
	}
	baseline := h.ok("reconcile", "--baseline-generate")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("record exact debt inventory")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "topic")
	writeFixtureFile(t, h.dir, ".github/change-marker", "change\n", 0o644)
	h.commit("candidate")
	return h, base
}

func TestMemoryGateSharesLocalAndHostedPredicates(t *testing.T) {
	t.Parallel()
	h, base := memoryGateFixture(t)
	local := h.gateOK()
	want(t, local, "trusted base "+base, "check: ok", "baseline monotonicity: ok", "debt ratchet: exact")
	for _, event := range []string{"pull_request", "push"} {
		h.envs["GITHUB_ACTIONS"] = "true"
		h.envs["PYXIS_CI_EVENT"] = event
		h.envs["PYXIS_CI_BASE"] = base
		if remote := h.gateOK(); remote != local {
			t.Fatalf("%s differs from local:\n%s\n%s", event, local, remote)
		}
	}
	h.envs = map[string]string{"PYXIS_CI_EVENT": "schedule", "GITHUB_ACTIONS": "true"}
	want(t, h.gateOK(), "mainline inventory", "debt ratchet: exact")
	h.envs = map[string]string{}
	h.git("checkout", "main")
	want(t, h.gateOK(), "mainline inventory", "debt ratchet: exact")
}

func TestMemoryGateRefusesUntrustedOrMissingInputs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, event, base string
		args              []string
		want              string
	}{
		{"missing CI base", "pull_request", "", nil, "needs PYXIS_CI_BASE"},
		{"symbolic CI base", "push", "main", nil, "full commit SHA"},
		{"zero CI base", "push", strings.Repeat("0", 40), nil, "full commit SHA"},
		{"CI override", "pull_request", "", []string{"--base", "main"}, "cannot override"},
		{"scheduled base", "schedule", "main", nil, "must not supply a base"},
		{"unsupported event", "workflow_dispatch", "", nil, "unsupported"},
		{"explicit self comparison", "", "", []string{"--base", "HEAD"}, "cannot be checked against itself"},
		{"nonexistent base", "", "", []string{"--base", "absent"}, "does not name a commit"},
		{"positional argument", "", "", []string{"unexpected"}, "accepts only"},
		{"unknown option", "", "", []string{"--unknown"}, "flag provided but not defined"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := memoryGateFixture(t)
			h.envs["PYXIS_CI_EVENT"], h.envs["PYXIS_CI_BASE"] = tt.event, tt.base
			code, _, stderr := h.gate(tt.args...)
			if code == exitOK || !strings.Contains(stderr, tt.want) {
				t.Fatalf("gate = %d, %s; want %s", code, stderr, tt.want)
			}
		})
	}
	h, _ := memoryGateFixture(t)
	h.envs["GITHUB_ACTIONS"] = "true"
	if code, _, stderr := h.gate(); code == exitOK || !strings.Contains(stderr, "needs PYXIS_CI_EVENT") {
		t.Fatalf("missing event: %d, %s", code, stderr)
	}
	h.envs = map[string]string{}
	h.git("branch", "-D", "main")
	if code, _, stderr := h.gate(); code == exitOK || !strings.Contains(stderr, "no trusted mainline") {
		t.Fatalf("missing mainline: %d, %s", code, stderr)
	}
}

func TestMemoryGateRequiresCommittedInventoryWithoutInventedKnowledge(t *testing.T) {
	t.Parallel()
	h, base := memoryGateFixture(t)
	writeFixtureFile(t, h.dir, "cmd/new-check/main.go", "package main\nfunc main() {}\n", 0o644)
	if code, _, stderr := h.gate(); code == exitOK || !strings.Contains(stderr, "commit the working-tree changes") {
		t.Fatalf("dirty tree passed: %d, %s", code, stderr)
	}
	h.commit("new code without memory coverage")
	for _, event := range []string{"", "pull_request"} {
		h.envs["PYXIS_CI_EVENT"], h.envs["PYXIS_CI_BASE"] = event, base
		code, stdout, stderr := h.gate()
		if code == exitOK || !strings.Contains(stdout, "check: ok") || !strings.Contains(stderr, "baseline input identities") {
			t.Fatalf("stale legacy inventory passed %s: %d\n%s\n%s", event, code, stdout, stderr)
		}
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("record exact current inventory")
	if code, stdout, stderr := h.gate(); code != exitOK {
		t.Fatalf("exact code inventory required a semantic record: %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestMemoryGateHonorsConfiguredMainline(t *testing.T) {
	t.Parallel()
	h, base := memoryGateFixture(t, "schema_version: pyxis-config/v2\nbranch: trunk\nremote: upstream\n")
	h.git("update-ref", "refs/remotes/upstream/trunk", base)
	h.git("branch", "-D", "main")
	want(t, h.gateOK(), "trusted base "+base)
	// Explicit desk input and its environment form resolve once to the same SHA.
	h.envs["PYXIS_CI_BASE"] = base
	want(t, h.gateOK(), "trusted base "+base)
	want(t, h.gateOK("--base", base), "trusted base "+base)
}

func TestMemoryGateRejectsRewrittenRecords(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	id := h.add("--kind", "Learning", "--subject", "learning/ci", "--statement", "Original lesson", "--learned-from", "a CI mismatch")
	h.commit("initialize records")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("baseline")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "topic")
	path := filepath.Join(h.dir, "memory/learning/ci", id+".json")
	body := mustRead(t, path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(body), "Original lesson", "Rewritten lesson", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	h.commit("rewrite an immutable record")
	for _, event := range []string{"", "pull_request"} {
		h.envs["PYXIS_CI_EVENT"], h.envs["PYXIS_CI_BASE"] = event, base
		code, stdout, _ := h.gate()
		if code != exitRefused || !strings.Contains(stdout, "evidence-record-rewritten") {
			t.Fatalf("%s record rewrite passed: %d, %s", event, code, stdout)
		}
		if status := h.git("status", "--porcelain", "--untracked-files=all"); status != "" {
			t.Fatalf("%s failed gate wrote files: %s", event, status)
		}
		if _, err := os.Stat(filepath.Join(h.dir, ".pyxis/evidence")); !os.IsNotExist(err) {
			t.Fatalf("%s failed gate created a report home: %v", event, err)
		}
	}
}

func TestMemoryGateRejectsForcedTrackedReports(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"local", "hosted", "mainline"} {
		t.Run(mode, func(t *testing.T) {
			h, base := memoryGateFixture(t)
			writeFixtureFile(t, h.dir, ".gitignore", "/.pyxis/evidence/memory/\n", 0o644)
			h.commit("ignore generated reports")
			path := ".pyxis/evidence/memory/" + strings.Repeat("a", 64) + ".json"
			writeFixtureFile(t, h.dir, path, "{}\n", 0o644)
			h.gateOK() // Ignored local output does not become a gate operand.
			h.git("add", "--force", "--", path)
			h.git("commit", "-qm", "force a generated report into Git")
			switch mode {
			case "hosted":
				h.envs["PYXIS_CI_EVENT"] = "pull_request"
				h.envs["PYXIS_CI_BASE"] = base
				h.envs["GITHUB_ACTIONS"] = "true"
			case "mainline":
				h.git("update-ref", "refs/heads/main", "HEAD")
			}
			if code, _, stderr := h.gate(); code != exitRefused || !strings.Contains(stderr, "must not be tracked") || !strings.Contains(stderr, path) {
				t.Fatalf("tracked report passed %s: %d %s", mode, code, stderr)
			}
		})
	}
}

func TestMemoryGateOperationalFailures(t *testing.T) {
	t.Parallel()
	h := newHarness(t, t.TempDir())
	if code, _, _ := h.gate(); code == exitOK {
		t.Fatal("non-repository gate passed")
	}
	h, _ = memoryGateFixture(t)
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := memoryGateBase(ctx, m, "main", env); err == nil {
		t.Fatal("cancelled base resolution passed")
	}
	if err := os.RemoveAll(filepath.Join(h.dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := memoryGateBase(t.Context(), m, "", env); err == nil {
		t.Fatal("removed Git metadata passed")
	}
}
