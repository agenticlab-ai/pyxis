package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func TestApproveCommandRoutesExactReviewedSubjectAndReportsHostReceipt(t *testing.T) {
	oldRoot, oldPublish := draftRepoRoot, publishDriverApproval
	t.Cleanup(func() { draftRepoRoot, publishDriverApproval = oldRoot, oldPublish })
	root := t.TempDir()
	draftRepoRoot = func() (string, error) { return root, nil }
	head := strings.Repeat("a", 40)
	publishDriverApproval = func(_ context.Context, repo, tokenEnv string, d approval.DriverDecision) (approval.Approval, error) {
		if repo != "example/repo" || tokenEnv != "" || d.Head != head || d.PullRequest != 17 || d.Drive != "widget" || d.Evidence != "review:widget-001 approves" || d.Principal != "argus" {
			t.Fatalf("incorrect approval subject: %+v repo=%s token-env=%s", d, repo, tokenEnv)
		}
		return approval.Approval{Operator: "driver", Head: head, ReviewID: 27, Kind: "argus"}, nil
	}
	var stdout, stderr bytes.Buffer
	args := []string{"approve", "17", "--repo", "example/repo", "--head", head, "--drive", "widget", "--evidence", "review:widget-001 approves"}
	if exit := run(args, &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), `"Kind": "argus"`) || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	publishDriverApproval = func(context.Context, string, string, approval.DriverDecision) (approval.Approval, error) {
		return approval.Approval{}, errors.New("grant refused")
	}
	stdout.Reset()
	if exit := run(args, &stdout, &stderr); exit != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "grant refused") {
		t.Fatalf("refusal exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestApproveCommandRequiresExplicitHeadAndReviewEvidence(t *testing.T) {
	for _, args := range [][]string{nil, {"wrong"}, {"17"}, {"17", "--repo", "example/repo", "--head", strings.Repeat("a", 40), "--drive", "widget"}, {"17", "--unknown"}} {
		var stdout, stderr bytes.Buffer
		if exit := approveVerb(args, &stdout, &stderr); exit != 2 || stdout.Len() != 0 {
			t.Fatalf("invalid request %v exit=%d", args, exit)
		}
	}
}

type approvalOutputFailure struct{}

func (approvalOutputFailure) Write([]byte) (int, error) {
	return 0, errors.New("receipt output unavailable")
}

func TestApproveCommandRefusesInvalidLocalSetupBeforePublication(t *testing.T) {
	oldRoot, oldPublish := draftRepoRoot, publishDriverApproval
	t.Cleanup(func() { draftRepoRoot, publishDriverApproval = oldRoot, oldPublish })
	calls := 0
	publishDriverApproval = func(context.Context, string, string, approval.DriverDecision) (approval.Approval, error) {
		calls++
		return approval.Approval{}, nil
	}
	args := []string{"17", "--repo", "example/repo", "--head", strings.Repeat("a", 40), "--drive", "widget", "--evidence", "review:widget-001 approves"}
	draftRepoRoot = func() (string, error) { return "", errors.New("repository unavailable") }
	var stdout, stderr bytes.Buffer
	if exit := approveVerb(args, &stdout, &stderr); exit != 1 || calls != 0 || !strings.Contains(stderr.String(), "find repository") {
		t.Fatalf("invalid root exit=%d calls=%d error=%s", exit, calls, stderr.String())
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("unknown-policy: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	draftRepoRoot = func() (string, error) { return root, nil }
	stderr.Reset()
	if exit := approveVerb(args, &stdout, &stderr); exit != 1 || calls != 0 || !strings.Contains(stderr.String(), "configuration") {
		t.Fatalf("invalid config exit=%d calls=%d error=%s", exit, calls, stderr.String())
	}
}

func TestApproveCommandReportsLostOutputWithoutRetryingPublishedAct(t *testing.T) {
	oldRoot, oldPublish := draftRepoRoot, publishDriverApproval
	t.Cleanup(func() { draftRepoRoot, publishDriverApproval = oldRoot, oldPublish })
	root := t.TempDir()
	draftRepoRoot = func() (string, error) { return root, nil }
	calls := 0
	publishDriverApproval = func(_ context.Context, _, _ string, d approval.DriverDecision) (approval.Approval, error) {
		calls++
		return approval.Approval{Operator: "driver", Head: d.Head, ReviewID: 27, Kind: "argus"}, nil
	}
	args := []string{"17", "--repo", "example/repo", "--head", strings.Repeat("a", 40), "--drive", "widget", "--evidence", "review:widget-001 approves"}
	var stderr bytes.Buffer
	if exit := approveVerb(args, approvalOutputFailure{}, &stderr); exit != 1 || calls != 1 || !strings.Contains(stderr.String(), "report published approval") {
		t.Fatalf("lost output exit=%d calls=%d error=%s", exit, calls, stderr.String())
	}
	const credential = "PYXIS_APPROVAL_COMMAND_MISSING_CREDENTIAL"
	t.Setenv(credential, "")
	d := approval.DriverDecision{Repository: "example/repo", PullRequest: 17, Head: strings.Repeat("a", 40)}
	if _, err := oldPublish(t.Context(), d.Repository, credential, d); err == nil {
		t.Fatal("production adapter accepted a missing explicit credential")
	}
}
