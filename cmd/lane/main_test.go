package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// policyBody is a well-formed risk policy carrying the full self-unlock
// minimum, with one ratified AUTO class so the AUTO path is exercisable.
const policyBody = `schema_version: risk-policy/v1
ratified_by: "@operator"
floor:
  hard_gated_paths:
    - ".github/workflows/**"
    - ".githooks/**"
    - "docs/product-vision.md"
    - "docs/architecture-principles.md"
    - "docs/observability/incidents.md"
    - "docs/policy/**"
    - "AGENTS.md"
    - "CLAUDE.md"
    - ".pyxis/**"
    - ".claude/**"
    - ".codex/**"
    - "cmd/lane/**"
    - "internal/lane/**"
    - "go.mod"
    - "go.sum"
  hard_gated_classes:
    - governing-doc-edit
    - ledger-frozen-entry-edit
auto_classes:
  - name: fleet-task
    paths: ["cmd/**", "internal/**"]
    requires_reviewer_verdict: false
thresholds:
  shadow_window_min_proposals: 20
  agreement_bar: 0.95
activation:
  enforce: false
  backstop_change_ref: ""
kill_switch: false
`

// repo builds a hermetic git repo whose MAIN branch carries the risk policy
// landed with the operator ratification trailer, plus a feature commit whose
// diff the tests classify. Returns the root and the (base, head) pair.
func repo(t *testing.T, featureFiles map[string]string) (root, base, head string) {
	t.Helper()
	root = t.TempDir()
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
	run("init", "-q", "-b", "main")
	write("docs/policy/risk-policy.yaml", policyBody)
	write("README.md", "seed\n")
	run("add", ".")
	run("commit", "-qm", "seed: risk policy\n\nGoverning-Change: operator")
	base = run("rev-parse", "HEAD")

	run("checkout", "-qb", "feature")
	for rel, content := range featureFiles {
		write(rel, content)
	}
	run("add", ".")
	run("commit", "-qm", "feature change")
	head = run("rev-parse", "HEAD")
	// The lineage read resolves origin/main first, then main; the hermetic repo
	// has no origin, so main IS the ratified mainline.
	run("checkout", "-q", "main")
	return root, base, head
}

// classify runs the classify verb and returns its exit code and parsed verdict.
func classify(t *testing.T, root, base, head string) (int, map[string]any) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := run([]string{"classify", "--repo", root, "--base", base, "--head", head, "--checks", "t"}, outW, errW)
	outW.Close()
	errW.Close()
	var stdout, stderr strings.Builder
	buf := make([]byte, 1<<16)
	for {
		n, err := outR.Read(buf)
		stdout.Write(buf[:n])
		if err != nil {
			break
		}
	}
	for {
		n, err := errR.Read(buf)
		stderr.Write(buf[:n])
		if err != nil {
			break
		}
	}
	t.Logf("stderr: %s", stderr.String())
	var v map[string]any
	if s := strings.TrimSpace(stdout.String()); s != "" {
		if err := json.Unmarshal([]byte(strings.Split(s, "\n")[0]), &v); err != nil {
			t.Fatalf("verdict parse: %v from %q", err, s)
		}
	}
	return code, v
}

func TestClassifyFloorHitRoutesHuman(t *testing.T) {
	root, base, head := repo(t, map[string]string{".pyxis/pipeline.md": "changed\n"})
	code, v := classify(t, root, base, head)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (ratified policy loaded)", code)
	}
	if v["lane"] != "HUMAN" || !strings.Contains(v["reason"].(string), "floor hit") {
		t.Fatalf("want floor-hit HUMAN, got %v", v)
	}
}

func TestClassifyWholeDiffInsideRatifiedClassIsAuto(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	code, v := classify(t, root, base, head)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if v["lane"] != "AUTO" || v["auto_class"] != "fleet-task" {
		t.Fatalf("want AUTO via fleet-task, got %v", v)
	}
}

func TestClassifyOutOfPerimeterHunkRoutesHuman(t *testing.T) {
	root, base, head := repo(t, map[string]string{
		"internal/foo/foo.go": "package foo\n",
		"docs/notes.md":       "outside the class perimeter\n",
	})
	code, v := classify(t, root, base, head)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if v["lane"] != "HUMAN" {
		t.Fatalf("want HUMAN for out-of-perimeter hunk, got %v", v)
	}
}

// TestClassifyLaneOwnCodeFloorHits pins the transitive-trust rule: the lane's
// own packages and build inputs are in the self-unlock minimum, so a diff
// touching them floor-hits HUMAN even though cmd/** and internal/** are inside
// the ratified fleet-task perimeter (merge-lane-v1 §3.1, FND-12's fix).
func TestClassifyLaneOwnCodeFloorHits(t *testing.T) {
	for _, rel := range []string{"internal/lane/x.go", "cmd/lane/x.go", "go.mod"} {
		root, base, head := repo(t, map[string]string{rel: "changed\n"})
		code, v := classify(t, root, base, head)
		if code != 0 {
			t.Fatalf("%s: exit = %d, want 0", rel, code)
		}
		if v["lane"] != "HUMAN" || !strings.Contains(v["reason"].(string), "floor hit") {
			t.Fatalf("%s: want floor-hit HUMAN, got %v", rel, v)
		}
	}
}

func TestClassifyUnratifiedPolicyRefusesWithHumanVerdict(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	// Rewrite the policy on main WITHOUT the trailer: the lineage read must
	// refuse it and the verb exits 2 with an all-HUMAN verdict (DINV-8/DINV-9).
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "docs/policy/risk-policy.yaml"), []byte(policyBody+"# unratified touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-qm", "policy change WITHOUT the ratification trailer")
	code, v := classify(t, root, base, head)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (refusal: unratified lineage)", code)
	}
	if v["lane"] != "HUMAN" {
		t.Fatalf("want all-HUMAN verdict alongside the refusal, got %v", v)
	}
}

func TestUsageAndUnknownVerb(t *testing.T) {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	if got := run(nil, devnull, devnull); got != 2 {
		t.Fatalf("no-verb exit = %d, want 2", got)
	}
	if got := run([]string{"bogus"}, devnull, devnull); got != 2 {
		t.Fatalf("unknown-verb exit = %d, want 2", got)
	}
}

func TestMainDelegatesItsExitCode(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"lane"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

func TestSweepReportsFetchAndMergeBaseFaults(t *testing.T) {
	root, _, head := repo(t, map[string]string{"x.txt": "x\n"})
	script := "#!/bin/sh\ncase \"$*\" in\n" +
		"*pulls?state=open*) printf '[{\"number\":7,\"head\":{\"sha\":\"" + head + "\"}}]' ;;\n" +
		"*comments?per_page*) printf '[]' ;;\n" +
		"*) printf '{}' ;;\nesac\n"
	for _, tc := range []struct {
		name string
		read func(context.Context, string, ...string) ([]byte, error)
	}{
		{name: "fetch", read: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("fetch unavailable")
		}},
		{name: "merge base", read: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "fetch" {
				return nil, nil
			}
			return nil, errors.New("merge base unavailable")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeGH(t, script)
			old := sweepGitRead
			sweepGitRead = tc.read
			t.Cleanup(func() { sweepGitRead = old })
			code, stdout, stderr := runLane(t, []string{"sweep", "--gh-repo", "owner/repo", "--repo", root})
			if code != 1 || !strings.Contains(stdout, `"outcome":"fault"`) || stderr != "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestAdditionalVerbAndPolicyBoundaries(t *testing.T) {
	if code, _, stderr := runLane(t, []string{"classify", "--unknown"}); code != 2 || stderr == "" {
		t.Fatalf("classify parse code=%d stderr=%q", code, stderr)
	}
	if code, _, stderr := runLane(t, []string{"classify", "--repo", t.TempDir(), "--base", "missing", "--head", "missing"}); code != 2 || !strings.Contains(stderr, "lane-classify") {
		t.Fatalf("classify diff code=%d stderr=%q", code, stderr)
	}
	if code, _, stderr := runLane(t, []string{"state", "--unknown"}); code != 2 || stderr == "" {
		t.Fatalf("state parse code=%d stderr=%q", code, stderr)
	}
	if got := policyVersion(nil); !strings.Contains(got, "unloaded") {
		t.Fatalf("policyVersion(nil) = %q", got)
	}
}

func TestHandleReportsRecorderInfrastructureFailure(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	fakeGH(t, "#!/bin/sh\nprintf 'host unavailable' >&2\nexit 1\n")
	code, _, stderr := runLane(t, []string{
		"handle", "--gh-repo", "owner/repo", "--pr", "7", "--repo", root,
		"--base", base, "--head", head,
	})
	if code != 1 || !strings.Contains(stderr, "lane handle") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestSweepRecordsPerProposalCommentReadFault(t *testing.T) {
	root, _, head := repo(t, map[string]string{"x.txt": "x\n"})
	script := "#!/bin/sh\ncase \"$*\" in\n" +
		"*pulls?state=open*) printf '[{\"number\":7,\"head\":{\"sha\":\"" + head + "\"}}]' ;;\n" +
		"*) printf 'comments unavailable' >&2; exit 1 ;;\nesac\n"
	fakeGH(t, script)
	code, stdout, stderr := runLane(t, []string{"sweep", "--gh-repo", "owner/repo", "--repo", root})
	if code != 1 || !strings.Contains(stdout, `"outcome":"fault"`) || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestStateVerbResolvesShadow(t *testing.T) {
	root, _, _ := repo(t, map[string]string{"x.txt": "x\n"})
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	code := run([]string{"state", "--repo", root}, outW, devnull)
	outW.Close()
	var sb strings.Builder
	buf := make([]byte, 1<<12)
	for {
		n, err := outR.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if code != 0 {
		t.Fatalf("state exit = %d, want 0", code)
	}
	var got struct {
		State  string `json:"state"`
		Policy string `json:"policy"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(sb.String())), &got); err != nil {
		t.Fatalf("state parse: %v from %q", err, sb.String())
	}
	if got.State != "shadow" || got.Policy != "risk-policy/v1" {
		t.Fatalf("want shadow under a loaded, unactivated policy, got %+v", got)
	}
}

func TestHandleValidatesArgumentsAndDiffBeforeHostWrites(t *testing.T) {
	root, _, _ := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	for name, args := range map[string][]string{
		"flag":       {"handle", "--unknown"},
		"missing pr": {"handle", "--gh-repo", "owner/repo"},
		"bad diff":   {"handle", "--gh-repo", "owner/repo", "--pr", "7", "--repo", root, "--base", "missing", "--head", "also-missing"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runLane(t, args)
			if code != 2 || stderr == "" {
				t.Fatalf("code=%d stderr=%q, want a named refusal", code, stderr)
			}
		})
	}
}

func TestHandleRecordsShadowVerdictThroughGitHubAdapter(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	fakeGH(t, "#!/bin/sh\nprintf '[]'\n")
	code, stdout, stderr := runLane(t, []string{
		"handle", "--gh-repo", "owner/repo", "--pr", "7", "--repo", root,
		"--base", base, "--head", head, "--checks", "snapshot-1", "--machine", "pyxis-bot",
	})
	if code != 0 {
		t.Fatalf("handle code=%d stderr=%q", code, stderr)
	}
	var result struct {
		State    string            `json:"State"`
		Refusals []json.RawMessage `json:"Refusals"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("result parse: %v from %q", err, stdout)
	}
	if result.State != "shadow" || len(result.Refusals) == 0 {
		t.Fatalf("shadow result = %#v", result)
	}
}

func TestSweepHandlesEmptyAndEvaluatedProposalSets(t *testing.T) {
	root, _, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("remote", "add", "origin", root)
	runGit("fetch", "-q", "origin", "main")

	t.Run("empty", func(t *testing.T) {
		fakeGH(t, "#!/bin/sh\nprintf '[]'\n")
		code, stdout, stderr := runLane(t, []string{"sweep", "--gh-repo", "owner/repo", "--repo", root})
		if code != 0 || strings.TrimSpace(stdout) != "null" {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("one proposal", func(t *testing.T) {
		script := "#!/bin/sh\ncase \"$*\" in\n" +
			"*pulls?state=open*) printf '[{\"number\":7,\"head\":{\"sha\":\"" + head + "\"}}]' ;;\n" +
			"*comments?per_page*) printf '[]' ;;\n" +
			"*) printf '{}' ;;\nesac\n"
		fakeGH(t, script)
		code, stdout, stderr := runLane(t, []string{"sweep", "--gh-repo", "owner/repo", "--repo", root})
		if code != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		var rows []struct {
			Proposal int    `json:"proposal"`
			Outcome  string `json:"outcome"`
			Lane     string `json:"lane"`
		}
		if err := json.Unmarshal([]byte(stdout), &rows); err != nil || len(rows) != 1 || rows[0].Proposal != 7 || rows[0].Outcome != "evaluated" || rows[0].Lane != "AUTO" {
			t.Fatalf("rows=%#v err=%v output=%q", rows, err, stdout)
		}
	})
}

func TestSweepRefusalsAndGitRead(t *testing.T) {
	root, _, _ := repo(t, map[string]string{"x.txt": "x\n"})
	for _, args := range [][]string{{"sweep", "--unknown"}, {"sweep"}} {
		code, _, stderr := runLane(t, args)
		if code != 2 || stderr == "" {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr)
		}
	}
	fakeGH(t, "#!/bin/sh\nprintf 'host unavailable' >&2\nexit 1\n")
	if code, _, stderr := runLane(t, []string{"sweep", "--gh-repo", "owner/repo", "--repo", root}); code != 1 || !strings.Contains(stderr, "listing open PRs") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if out, err := gitRead(context.Background(), root, "rev-parse", "HEAD"); err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("gitRead success = %q, %v", out, err)
	}
	if _, err := gitRead(context.Background(), root, "rev-parse", "missing-ref"); err == nil {
		t.Fatal("gitRead accepted a missing ref")
	}
}

func runLane(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	stdoutPath, stderrPath := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		stdout.Close()
		t.Fatal(err)
	}
	code := run(args, stdout, stderr)
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	errout, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(out), string(errout)
}

func fakeGH(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
