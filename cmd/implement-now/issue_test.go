package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func TestDirectIssueCommandDeliversAndReplaysWithoutImplementation(t *testing.T) {
	f := directIssueFixture(t)
	legacyRef := "refs/claim/" + strconv.Itoa(e2eIssue+1)
	legacyHead := f.gitAt(f.repo, "rev-parse", "HEAD")
	f.gitBare("update-ref", legacyRef, legacyHead)
	var stdout, stderr bytes.Buffer
	inspectArgs := append(f.args(), "--inspect-issue", strconv.Itoa(e2eIssue))
	if code := run(inspectArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect exit %d: %s", code, stderr.String())
	}
	var inspected issueentry.Inspection
	if err := json.Unmarshal(stdout.Bytes(), &inspected); err != nil {
		t.Fatal(err)
	}
	req, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	input := issueentry.Input{Schema: issueentry.InputSchema, Source: inspected.Source, NativeInput: taskspec.NativeInput{Title: f.cfg.TaskTitle, Goal: "Produce e2e-output.txt", Scope: "Add the fixture output file", Acceptance: []string{"e2e-output.txt contains implemented by the stub harness"}, Requirements: req}}
	inputFile := filepath.Join(f.root, "issue-input.json")
	writeJSONFile(t, inputFile, input)
	args := append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", inputFile)
	clock := func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) }
	stdout.Reset()
	stderr.Reset()
	if code := runWithClock(args, &stdout, &stderr, clock); code != 0 {
		t.Fatalf("issue exit %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "verified_pr #77") {
		t.Fatalf("no verified terminal: %s", stdout.String())
	}
	if got := f.gitBare("rev-parse", legacyRef); got != legacyHead {
		t.Fatalf("unrelated legacy worker changed: %s", got)
	}
	pr, ok, err := readE2EPRState(f.cfg)
	if err != nil || !ok {
		t.Fatalf("proposal: %+v %v", pr, err)
	}
	if !strings.Contains(pr.Body, `"terminal": "verified_pr"`) || !strings.Contains(pr.Body, `"role": "implementation"`) || !strings.Contains(pr.Body, `"role": "review"`) || !strings.Contains(pr.Body, `"state": "unavailable"`) || !strings.Contains(pr.Body, "pyxis-task-proposal:") {
		t.Fatalf("missing attribution or association: %s", pr.Body)
	}
	if got := f.gitBare("show", pr.Head+":e2e-output.txt"); got != "implemented by the stub harness" {
		t.Fatalf("output %q", got)
	}
	assertIssueNativeUsage(t, f, pr.Body)
	assertIssueCommandJournal(t, f.journal, 0)
	calls := readE2ECalls(t, f.stateDir)
	patches := githubCallsContaining(calls, "PATCH", "repos/"+f.cfg.Repository+"/pulls/"+strconv.Itoa(f.cfg.PRNumber))
	if len(patches) != 2 {
		t.Fatalf("provisional and final metadata writes=%d, want 2", len(patches))
	}
	var provisional struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(patches[0].Stdin), &provisional); err != nil || !strings.Contains(provisional.Body, `"terminal": "running"`) {
		t.Fatalf("missing provisional public summary: %v %s", err, provisional.Body)
	}
	if len(codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")) != 1 || len(codexLaunchesWithArg(calls, "danger-full-access")) != 1 {
		t.Fatalf("wrong role processes: %+v", calls)
	}
	for _, call := range githubCallsContaining(calls, "GET", "issues?state=") {
		t.Fatalf("ordinary frontier read: %+v", call)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(inspectArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("restart inspect: %d %s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &inspected); err != nil || inspected.Admitted == nil {
		t.Fatalf("no shared admitted contract: %v %s", err, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("retry exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	after := readE2ECalls(t, f.stateDir)
	if len(codexLaunchesWithArg(after, "--dangerously-bypass-approvals-and-sandbox")) != 1 || len(codexLaunchesWithArg(after, "danger-full-access")) != 1 {
		t.Fatal("retry launched another worker")
	}
	assertIssueCommandJournal(t, f.journal, 0, 0)
	for _, path := range []string{"docs/plans", "docs/prds", "docs/tech-design", ".pyxis/drafts"} {
		if _, err := os.Stat(filepath.Join(f.repo, path)); !os.IsNotExist(err) {
			t.Fatalf("created planning artifact %s", path)
		}
	}
	// A later change carrying green fake CI cannot borrow the old local/review
	// proofs. The operator receives a pending result with no new model process.
	implementation := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")[0]
	if err := os.WriteFile(filepath.Join(implementation.Dir, "README.md"), []byte("changed after review\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f.gitAt(implementation.Dir, "add", "README.md")
	f.gitAt(implementation.Dir, "commit", "-m", "test: drift reviewed head")
	f.gitAt(implementation.Dir, "push", "origin", pr.Head)
	stdout.Reset()
	stderr.Reset()
	if code := run(args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "verification_pending") {
		t.Fatalf("head drift exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	if len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "danger-full-access")) != 1 {
		t.Fatal("head drift authorized a review")
	}
	assertIssueCommandJournal(t, f.journal, 0, 0, 1)
}

func TestDirectIssueReviewAndMetadataRetryPreserveImplementation(t *testing.T) {
	for _, failure := range []string{"review-unavailable", "summary-unavailable", "proposal-receipt-unavailable", "checks-unavailable"} {
		t.Run(failure, func(t *testing.T) {
			f := directIssueFixture(t)
			var out, diagnostic bytes.Buffer
			if code := run(append(f.args(), "--inspect-issue", strconv.Itoa(e2eIssue)), &out, &diagnostic); code != 0 {
				t.Fatalf("inspect %d: %s", code, &diagnostic)
			}
			input := issueInputFile(t, f, out.Bytes())
			marker := filepath.Join(f.stateDir, failure)
			if err := os.WriteFile(marker, nil, 0600); err != nil {
				t.Fatal(err)
			}
			args := append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", input)
			out.Reset()
			diagnostic.Reset()
			if code := run(args, &out, &diagnostic); code != 1 {
				t.Fatalf("failed dependency returned %d: %s %s", code, &out, &diagnostic)
			}
			want := "review_pending"
			switch failure {
			case "summary-unavailable":
				want = "summary_publication_failed"
			case "proposal-receipt-unavailable":
				want = "no retained proposal"
			case "checks-unavailable":
				want = "verification_pending"
			}
			if !strings.Contains(diagnostic.String(), want) || (failure != "proposal-receipt-unavailable" && !strings.Contains(diagnostic.String(), "PR #77")) {
				t.Fatalf("lost recoverable PR/failure: %s", &diagnostic)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			diagnostic.Reset()
			if code := run(args, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "verified_pr #77") {
				t.Fatalf("recovery %d: %s %s", code, &out, &diagnostic)
			}
			calls := readE2ECalls(t, f.stateDir)
			if len(codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")) != 1 {
				t.Fatal("recovery redispatched implementation")
			}
			wantReviews := 1
			if failure == "review-unavailable" {
				wantReviews = 2
			}
			if got := len(codexLaunchesWithArg(calls, "danger-full-access")); got != wantReviews {
				t.Fatalf("review processes=%d want=%d", got, wantReviews)
			}
			pr, _, err := readE2EPRState(f.cfg)
			if err != nil || !strings.Contains(pr.Body, "verified_pr") {
				t.Fatalf("recovery summary missing: %v %s", err, pr.Body)
			}
			if failure == "summary-unavailable" {
				assertIssueNativeUsage(t, f, pr.Body)
			}
		})
	}
}

func TestDirectIssueMissingHandoffRefusesMovedHeadWithoutSpendingReview(t *testing.T) {
	f := directIssueFixture(t)
	var out, diagnostic bytes.Buffer
	if code := run(append(f.args(), "--inspect-issue", strconv.Itoa(e2eIssue)), &out, &diagnostic); code != 0 {
		t.Fatalf("inspect %d: %s", code, &diagnostic)
	}
	input := issueInputFile(t, f, out.Bytes())
	marker := filepath.Join(f.stateDir, "checks-unavailable")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	args := append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", input)
	out.Reset()
	diagnostic.Reset()
	if code := run(args, &out, &diagnostic); code != 1 {
		t.Fatalf("interruption returned %d: %s %s", code, &out, &diagnostic)
	}
	pr, ok, err := readE2EPRState(f.cfg)
	if err != nil || !ok {
		t.Fatalf("proposal: %+v %v", pr, err)
	}
	implementation := codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "--dangerously-bypass-approvals-and-sandbox")
	if len(implementation) != 1 {
		t.Fatal("expected one interrupted implementation")
	}
	if err := os.WriteFile(filepath.Join(implementation[0].Dir, "README.md"), []byte("changed before review capture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f.gitAt(implementation[0].Dir, "add", "README.md")
	f.gitAt(implementation[0].Dir, "commit", "-m", "test: move proposal before review handoff")
	f.gitAt(implementation[0].Dir, "push", "origin", pr.Head)
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostic.Reset()
	if code := run(args, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "recover issue review") || !strings.Contains(diagnostic.String(), "verification_pending") {
		t.Fatalf("moved head borrowed old local proof: %d %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	if len(codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")) != 1 || len(codexLaunchesWithArg(calls, "danger-full-access")) != 0 {
		t.Fatal("missing handoff on an unverified head spent a model process")
	}
}

func TestIssueOperandsRefuseBeforeIO(t *testing.T) {
	for _, args := range [][]string{{"--issue", "0"}, {"--inspect-issue", "0"}, {"--issue", "-1"}, {"--issue", "17"}, {"--issue-input", "some.json"}, {"--issue", "17", "--issue-input", "some.json", "--plan-ref", "feature/001"}, {"--issue", "17", "--issue-input", "some.json", "--review-only"}, {"--inspect-issue", "17", "--issue", "17", "--issue-input", "some.json"}} {
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 2 || !strings.Contains(err.String(), "mutually exclusive") {
			t.Fatalf("args=%v exit=%d %s", args, code, err.String())
		}
	}
}

func TestDirectIssueCommandRefusesIncompleteIntakeBeforeModel(t *testing.T) {
	f := directIssueFixture(t)
	var inspection, diagnostic bytes.Buffer
	inspectArgs := append(f.args(), "--inspect-issue", strconv.Itoa(e2eIssue))
	if code := run(inspectArgs, &inspection, &diagnostic); code != 0 {
		t.Fatalf("inspect %d: %s", code, &diagnostic)
	}
	input := issueInputFile(t, f, inspection.Bytes())
	for _, tc := range []struct{ name, path string }{{"missing intake", input + "-missing"}, {"invalid intake", input}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.path == input {
				if err := os.WriteFile(input, []byte(`{"schema_version":"unknown"}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var out, diagnostic bytes.Buffer
			args := append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", tc.path)
			if code := run(args, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "admit issue") {
				t.Fatalf("invalid intake admitted: %d %s %s", code, &out, &diagnostic)
			}
		})
	}
	diagnostic.Reset()
	if code := run(inspectArgs, issueBrokenWriter{}, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "inspect issue") {
		t.Fatalf("lost inspection output reported success: %d %s", code, &diagnostic)
	}
	if launches := codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec"); len(launches) != 0 {
		t.Fatal("invalid intake spent a model call")
	}
	if err := os.WriteFile(filepath.Join(f.repo, ".pyxis", "config.yaml"), []byte("executor:\n  review:\n    backend: codex\n"), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostic.Reset()
	if code := run(append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", input), &inspection, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "issue preflight") {
		t.Fatalf("missing local gate admitted: %d %s", code, &diagnostic)
	}
}

func TestDirectIssuePreservesLegacyClaimMigrationAdmission(t *testing.T) {
	f := directIssueFixture(t)
	var out, diagnostic bytes.Buffer
	if code := run(append(f.args(), "--inspect-issue", strconv.Itoa(e2eIssue)), &out, &diagnostic); code != 0 {
		t.Fatalf("inspect %d: %s", code, &diagnostic)
	}
	input := issueInputFile(t, f, out.Bytes())
	head := f.gitAt(f.repo, "rev-parse", "HEAD")
	f.gitBare("update-ref", "refs/claim/"+strconv.Itoa(e2eIssue), head)
	out.Reset()
	diagnostic.Reset()
	args := append(f.args(), "--issue", strconv.Itoa(e2eIssue), "--issue-input", input)
	if code := run(args, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "legacy claim") {
		t.Fatalf("legacy namespace admitted: %d %s %s", code, &out, &diagnostic)
	}
	if len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")) != 0 {
		t.Fatal("legacy namespace conflict launched a model")
	}
}
func TestIssuePreflightRequiresReviewAndRequiredLocalGate(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	absent, err := config.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := issuePreflight(absent, dir); err == nil || !strings.Contains(err.Error(), "independent") {
		t.Fatalf("missing review: %v", err)
	}
	review, err := config.Parse([]byte("executor:\n  review:\n    backend: codex\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := issuePreflight(review, dir); err == nil || !strings.Contains(err.Error(), "required code-gate") {
		t.Fatalf("missing local gate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pyxis", "config.yaml"), []byte("code-gate:\n  required:\n    command: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := issuePreflight(review, dir); err != nil {
		t.Fatal(err)
	}
	active, err := config.Parse([]byte("executor:\n  review:\n    backend: codex\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: policy.json\n    activation_checkpoint: " + strings.Repeat("a", 64) + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := issuePreflight(active, dir); err == nil || !strings.Contains(err.Error(), "unsupported_issue_origin") {
		t.Fatalf("activation bypass: %v", err)
	}
}
